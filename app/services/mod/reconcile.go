package mod

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/SatisfactoryServerManager/SSMAgent/app/api"
	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/sf"
	"github.com/SatisfactoryServerManager/SSMAgent/app/types"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
)

// SyncPlan is what a sync will do. It is computed before anything is touched, so
// an empty plan short-circuits the whole task.
type SyncPlan struct {
	Install []v2.ModLock
	Remove  []string
	Configs map[string]string
}

func (p SyncPlan) IsEmpty() bool {
	return len(p.Install) == 0 && len(p.Remove) == 0
}

// PlanSync diffs the Mods directory against the lockfile.
//
// It is pure and separate from the execution so the no-op case — the one that
// makes at-least-once delivery safe — can be tested without a disk.
func PlanSync(onDisk []types.InstalledMod, lf v2.Lockfile) SyncPlan {
	plan := SyncPlan{
		Install: make([]v2.ModLock, 0),
		Remove:  make([]string, 0),
		Configs: make(map[string]string, len(lf.Mods)),
	}

	installed := make(map[string]string, len(onDisk))
	for _, m := range onDisk {
		installed[m.ModReference] = m.ModVersion
	}

	wanted := make(map[string]bool, len(lf.Mods))

	for _, lock := range lf.Mods {
		wanted[lock.ModReference] = true
		plan.Configs[lock.ModReference] = lock.Config

		if installed[lock.ModReference] != lock.Version {
			plan.Install = append(plan.Install, lock)
		}
	}

	for _, m := range onDisk {
		if !wanted[m.ModReference] {
			plan.Remove = append(plan.Remove, m.ModReference)
		}
	}

	return plan
}

// serverRunning is sf.IsRunning, indirected through a package variable purely so
// the guard below is testable. Nothing but the test may reassign it.
var serverRunning = sf.IsRunning

// downloadFile is api.DownloadNonSSMFile, indirected for the same reason.
var downloadFile = api.DownloadNonSSMFile

// ErrNilMods is returned for a syncmods payload with no mods field at all.
//
// nil and an explicit empty slice are NOT interchangeable, exactly as in the
// backend's agentmod.DeleteAbsent:
//   - lf.Mods == nil means the payload was absent, null, or {} — a malformed task.
//     Executing it would put every mod on disk into plan.Remove and uninstall the
//     lot on what is really a bug or a truncated message. Refuse it.
//   - lf.Mods == []v2.ModLock{} (non-nil, empty) means the backend resolved the set
//     and it is genuinely empty — the user removed their last mod. That is a real
//     outcome and MUST still wipe the Mods directory, or the last mod could never
//     be removed.
//
// Do not "simplify" this to a len() check: the two look identical there and mean
// opposite things.
var ErrNilMods = errors.New("syncmods payload has no mods list; refusing to uninstall every mod")

// Sync brings the Mods directory to the lockfile's desired state and returns what
// it ended up with. The caller holds serverlock.
//
// On failure it returns the report ALONGSIDE the error whenever it has already
// begun touching the disk: mods installed before the failing one are really on
// disk at their new version, and if the caller reported nothing the backend would
// keep believing the pre-sync installed-set until the next reconnect.
func Sync(ctx context.Context, lf v2.Lockfile, progress func(int32, string)) ([]v2.InstalledMod, error) {
	if lf.Mods == nil {
		return nil, ErrNilMods
	}

	// The queue guarantees this: a syncmods task is either behind a stopsfserver in
	// its chain, or gated on requiresServerStopped. If the server is up anyway,
	// something is wrong, and writing the Mods directory under a live server would
	// corrupt it. Fail loudly rather than no-op silently, which is what the old
	// InstallAllMods did.
	if serverRunning() {
		return nil, fmt.Errorf("cannot sync mods while the satisfactory server is running")
	}

	utils.CreateFolder(config.GetConfig().ModsDir)
	utils.CreateFolder(config.GetConfig().ModConfigsDir)

	plan := PlanSync(FindModsOnDisk(), lf)

	if !plan.IsEmpty() {
		cacheDir := filepath.Join(config.GetConfig().DataDir, "modcache")
		utils.CreateFolder(cacheDir)

		for idx, lock := range plan.Install {
			if err := ctx.Err(); err != nil {
				return installedReport(lf), err
			}

			pct := int32(float64(idx) / float64(len(plan.Install)) * 100)
			progress(pct, fmt.Sprintf("%s %s (%d/%d)", lock.ModReference, lock.Version, idx+1, len(plan.Install)))

			archive := filepath.Join(cacheDir, lock.ModReference+"."+lock.Version+".zip")

			if err := download(archive, lock); err != nil {
				return installedReport(lf), err
			}

			// An upgrade is a replace-in-place: InstallModArchive clears the old
			// layout only once the new archive is on disk and verified.
			if err := InstallModArchive(archive, lock.ModReference); err != nil {
				// The archive is unusable — an unverifiable hash-less mod whose cached
				// bytes are an error page, say. Nothing else deletes it (the cache
				// delete otherwise only lives on the verification path), so every later
				// sync would re-extract the same garbage. Drop it and let the retry
				// re-download.
				os.Remove(archive)
				return installedReport(lf), fmt.Errorf("error installing mod %s: %w", lock.ModReference, err)
			}
		}

		for _, ref := range plan.Remove {
			if err := UninstallMod(ref); err != nil {
				return installedReport(lf), fmt.Errorf("error uninstalling mod %s: %w", ref, err)
			}
		}
	}

	for ref, cfg := range plan.Configs {
		if err := WriteModConfigFile(ref, cfg); err != nil {
			return installedReport(lf), err
		}
	}

	return installedReport(lf), nil
}

// download fetches a mod archive and verifies it.
//
// A cached archive is reused, but only if it still hashes correctly: a truncated
// download left in the cache would otherwise be unzipped into the Mods directory
// forever. On any mismatch the cache entry is deleted, so the retry re-downloads
// rather than re-verifying the same corrupt bytes.
func download(path string, lock v2.ModLock) error {
	if utils.CheckFileExists(path) {
		if err := verify(path, lock.Hash); err == nil {
			return nil
		}
		os.Remove(path)
	}

	if err := downloadFile(lock.DownloadURL, path); err != nil {
		return fmt.Errorf("error downloading mod %s: %w", lock.ModReference, err)
	}

	if err := verify(path, lock.Hash); err != nil {
		os.Remove(path)
		return fmt.Errorf("mod %s failed verification: %w", lock.ModReference, err)
	}

	return nil
}

func verify(path, want string) error {
	// A catalogue entry with no hash cannot be verified. Do not invent a pass or a
	// fail: install it, because refusing would make the mod uninstallable.
	if want == "" {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	got := hex.EncodeToString(h.Sum(nil))
	if got != want {
		return fmt.Errorf("hash mismatch: expected %s, got %s", want, got)
	}

	return nil
}

// installedReport is what the agent tells the backend it now has. It is derived
// from a fresh disk scan rather than from the lockfile, so a mod that silently
// failed to land is reported as absent rather than as installed.
func installedReport(lf v2.Lockfile) []v2.InstalledMod {
	onDisk := make(map[string]string)
	for _, m := range FindModsOnDisk() {
		onDisk[m.ModReference] = m.ModVersion
	}

	out := make([]v2.InstalledMod, 0, len(lf.Mods))
	for _, lock := range lf.Mods {
		version, present := onDisk[lock.ModReference]
		out = append(out, v2.InstalledMod{
			ModReference:     lock.ModReference,
			InstalledVersion: version,
			Installed:        present,
		})
	}

	return out
}

// ReportOnDisk is the unconditional truth about the Mods directory, reported once
// on subscribe so a mod deleted by hand is noticed rather than believed installed
// forever.
func ReportOnDisk() []v2.InstalledMod {
	found := FindModsOnDisk()

	out := make([]v2.InstalledMod, 0, len(found))
	for _, m := range found {
		out = append(out, v2.InstalledMod{
			ModReference:     m.ModReference,
			InstalledVersion: m.ModVersion,
			Installed:        true,
		})
	}

	return out
}
