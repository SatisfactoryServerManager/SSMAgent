package task

import (
	"context"
	"encoding/json"

	"github.com/SatisfactoryServerManager/SSMAgent/app/services/mod"
	"github.com/SatisfactoryServerManager/SSMAgent/app/services/sf"
	"github.com/SatisfactoryServerManager/SSMAgent/app/utils"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
)

// RegisterDefaults wires the action names the backend enqueues. Each handler is
// a no-op when its post-condition already holds, which is what makes at-least-once
// delivery safe.
func RegisterDefaults() {
	Register("installsfserver", func(ctx context.Context, data json.RawMessage, progress func(int32, string)) error {
		progress(0, "installing")
		return sf.EnsureInstalled()
	})

	Register("reinstallsfserver", func(ctx context.Context, data json.RawMessage, progress func(int32, string)) error {
		progress(0, "reinstalling")
		return sf.Reinstall()
	})

	Register("updatesfserver", func(ctx context.Context, data json.RawMessage, progress func(int32, string)) error {
		progress(0, "updating")
		return sf.UpdateSFServer()
	})

	Register("startsfserver", func(ctx context.Context, data json.RawMessage, progress func(int32, string)) error {
		if sf.IsRunning() {
			return nil
		}
		return sf.StartSFServer()
	})

	Register("stopsfserver", func(ctx context.Context, data json.RawMessage, progress func(int32, string)) error {
		if !sf.IsRunning() {
			return nil
		}
		return sf.ShutdownSFServer()
	})

	Register("killsfserver", func(ctx context.Context, data json.RawMessage, progress func(int32, string)) error {
		if !sf.IsRunning() {
			return nil
		}
		return sf.KillSFServer()
	})

	// sf.ClaimServer requires the server to be running, which is why the workflow
	// orders install, start, claim.
	Register("claimserver", func(ctx context.Context, data json.RawMessage, progress func(int32, string)) error {
		var d v2.ClaimServer_PostData
		if err := json.Unmarshal(data, &d); err != nil {
			return err
		}
		return sf.ClaimServer(d.AdminPass, d.ClientPass)
	})

	// The agent resolves nothing: the payload is a fully pinned lockfile, and the
	// handler reconciles the Mods directory to it and reports what actually landed.
	Register("syncmods", func(ctx context.Context, data json.RawMessage, progress func(int32, string)) error {
		var lf v2.Lockfile
		if err := json.Unmarshal(data, &lf); err != nil {
			return err
		}

		// Sync returns what actually landed even when it fails part-way: mods before
		// the failing one are on disk at their new version. Report that BEFORE failing
		// the task, or the backend keeps believing the pre-sync installed-set until
		// the next reconnect.
		installed, syncErr := mod.Sync(ctx, lf, progress)

		// nil means Sync refused before touching anything (a malformed payload, or the
		// server was up); an empty non-nil slice is a real "nothing is installed" and
		// must be reported.
		if installed != nil {
			// The report is best effort. A failure to report must never mask the sync
			// error that caused the task to fail.
			if err := ReportInstalledMods(ctx, installed); err != nil {
				utils.ErrorLogger.Printf("error reporting installed mods: %s\r\n", err.Error())
				if syncErr == nil {
					return err
				}
			}
		}

		return syncErr
	})
}
