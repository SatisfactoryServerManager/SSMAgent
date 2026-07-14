package mod

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/SatisfactoryServerManager/SSMAgent/app/config"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
)

func TestMain(m *testing.M) {
	// The loggers are nil until SetupLoggers runs against a real log dir; the code
	// under test logs on its happy paths.
	utils.DebugLogger = log.New(io.Discard, "", 0)
	utils.InfoLogger = log.New(io.Discard, "", 0)
	utils.WarnLogger = log.New(io.Discard, "", 0)
	utils.ErrorLogger = log.New(io.Discard, "", 0)
	utils.SteamLogger = log.New(io.Discard, "", 0)

	os.Exit(m.Run())
}

// seedConfig points the mod service at a throwaway tree. DataDir and SFDir are
// separate roots on purpose, mirroring the real deployment.
func seedConfig(t *testing.T) *config.Config {
	t.Helper()

	root := t.TempDir()
	c := &config.Config{
		DataDir:       filepath.Join(root, "data"),
		SFDir:         filepath.Join(root, "sf"),
		ModsDir:       filepath.Join(root, "sf", "FactoryGame", "Mods"),
		ModConfigsDir: filepath.Join(root, "sf", "FactoryGame", "Configs"),
	}
	config.SetConfig(c)
	t.Cleanup(func() { config.SetConfig(nil) })

	return c
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func sha256Of(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// --- verify ------------------------------------------------------------------

// A catalogue entry with no hash cannot be verified. Refusing would make the mod
// permanently uninstallable, so an empty expected hash must pass.
func TestVerifyPassesWithAnEmptyHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mod.zip")
	writeFile(t, path, "anything at all")

	if err := verify(path, ""); err != nil {
		t.Fatalf("expected an empty hash to pass, got %v", err)
	}
}

func TestVerifyPassesTheCorrectHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mod.zip")
	writeFile(t, path, "the real archive")

	if err := verify(path, sha256Of("the real archive")); err != nil {
		t.Fatalf("expected the correct hash to pass, got %v", err)
	}
}

func TestVerifyRejectsAWrongHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mod.zip")
	writeFile(t, path, "a truncated archive")

	if err := verify(path, sha256Of("the real archive")); err == nil {
		t.Fatal("expected a hash mismatch to fail verification")
	}
}

// --- download ----------------------------------------------------------------

// A cached archive that no longer hashes correctly must be DELETED and
// re-downloaded. Re-verifying the same corrupt bytes forever would wedge the sync,
// and unzipping them would put a truncated mod into the game.
func TestDownloadReplacesACorruptCacheEntry(t *testing.T) {
	seedConfig(t)

	path := filepath.Join(t.TempDir(), "RefinedPower.3.3.0.zip")
	writeFile(t, path, "truncated garbage")

	good := "the real archive"

	calls := 0
	restore := downloadFile
	downloadFile = func(url, dest string) error {
		calls++
		writeFile(t, dest, good)
		return nil
	}
	t.Cleanup(func() { downloadFile = restore })

	err := download(path, v2.ModLock{
		ModReference: "RefinedPower",
		Version:      "3.3.0",
		Hash:         sha256Of(good),
		DownloadURL:  "https://example.invalid/rp.zip",
	})
	if err != nil {
		t.Fatalf("expected the corrupt cache entry to be re-downloaded, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly one re-download, got %d", calls)
	}

	b, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(b) != good {
		t.Fatalf("expected the cache to hold the re-downloaded archive, got %q", string(b))
	}
}

// The counterpart: a cache entry that still hashes correctly is reused, not
// re-fetched.
func TestDownloadReusesAValidCacheEntry(t *testing.T) {
	seedConfig(t)

	path := filepath.Join(t.TempDir(), "RefinedPower.3.3.0.zip")
	writeFile(t, path, "the real archive")

	restore := downloadFile
	downloadFile = func(url, dest string) error {
		t.Fatal("a valid cache entry must not be re-downloaded")
		return nil
	}
	t.Cleanup(func() { downloadFile = restore })

	if err := download(path, v2.ModLock{
		ModReference: "RefinedPower",
		Version:      "3.3.0",
		Hash:         sha256Of("the real archive"),
	}); err != nil {
		t.Fatalf("expected the valid cache entry to be reused, got %v", err)
	}
}

// --- Sync guards -------------------------------------------------------------

// The single most important guard in the file. Writing the Mods directory under a
// live server corrupts it, and the old code silently no-op'd here, which is how
// mods quietly never installed. It must be a hard error.
func TestSyncRefusesToRunWhileTheServerIsRunning(t *testing.T) {
	c := seedConfig(t)

	restore := serverRunning
	serverRunning = func() bool { return true }
	t.Cleanup(func() { serverRunning = restore })

	lf := v2.Lockfile{Mods: []v2.ModLock{lock("RefinedPower", "3.3.0", "{}")}}

	installed, err := Sync(context.Background(), lf, func(int32, string) {})
	if err == nil {
		t.Fatal("expected Sync to hard-fail while the server is running, got nil (a silent no-op)")
	}
	if installed != nil {
		t.Fatalf("expected no report from a refused sync, got %+v", installed)
	}

	// And it must not have touched the disk on its way out.
	if utils.CheckFileExists(c.ModsDir) {
		t.Fatal("Sync created the Mods directory despite refusing to run")
	}
	if utils.CheckFileExists(c.ModConfigsDir) {
		t.Fatal("Sync wrote mod configs despite refusing to run")
	}
}

// A payload of null, {}, or one with no "mods" key unmarshals to a nil Mods, and
// would otherwise put every mod on disk into plan.Remove.
func TestSyncRefusesANilModsPayload(t *testing.T) {
	c := seedConfig(t)

	restore := serverRunning
	serverRunning = func() bool { return false }
	t.Cleanup(func() { serverRunning = restore })

	// A mod is on disk and must survive the malformed task.
	writeFile(t, filepath.Join(c.ModsDir, "RefinedPower", "RefinedPower.uplugin"), `{"SemVersion":"3.3.0"}`)

	installed, err := Sync(context.Background(), v2.Lockfile{}, func(int32, string) {})
	if !errors.Is(err, ErrNilMods) {
		t.Fatalf("expected ErrNilMods for a payload with no mods list, got %v", err)
	}
	if installed != nil {
		t.Fatalf("expected no report from a refused sync, got %+v", installed)
	}

	if !utils.CheckFileExists(filepath.Join(c.ModsDir, "RefinedPower")) {
		t.Fatal("a nil-mods payload uninstalled an installed mod")
	}
}

// The other half of the same distinction: an explicitly empty list is the
// legitimate "the user removed their last mod" wipe and MUST still execute, or the
// last mod could never be removed.
func TestSyncHonoursAnExplicitlyEmptyModsPayloadAsAWipe(t *testing.T) {
	c := seedConfig(t)

	restore := serverRunning
	serverRunning = func() bool { return false }
	t.Cleanup(func() { serverRunning = restore })

	writeFile(t, filepath.Join(c.ModsDir, "RefinedPower", "RefinedPower.uplugin"), `{"SemVersion":"3.3.0"}`)

	installed, err := Sync(context.Background(), v2.Lockfile{Mods: []v2.ModLock{}}, func(int32, string) {})
	if err != nil {
		t.Fatalf("expected an empty lockfile to wipe the mods, got %v", err)
	}
	if len(installed) != 0 {
		t.Fatalf("expected an empty report, got %+v", installed)
	}
	if installed == nil {
		t.Fatal("expected a non-nil (reportable) empty installed set")
	}

	if utils.CheckFileExists(filepath.Join(c.ModsDir, "RefinedPower")) {
		t.Fatal("an explicitly empty lockfile failed to remove the last mod")
	}
}
