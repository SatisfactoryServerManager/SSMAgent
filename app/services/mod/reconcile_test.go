package mod

import (
	"testing"

	"github.com/SatisfactoryServerManager/SSMAgent/app/types"
	v2 "github.com/SatisfactoryServerManager/ssmcloud-resources/models/v2"
)

func disk(ref, version string) types.InstalledMod {
	return types.InstalledMod{ModReference: ref, ModVersion: version}
}

func lock(ref, version, config string) v2.ModLock {
	return v2.ModLock{ModReference: ref, Version: version, Config: config}
}

// The whole safety argument for at-least-once delivery: running the same task
// twice must do nothing the second time.
func TestPlanSyncIsANoOpWhenTheDiskAlreadyMatches(t *testing.T) {
	onDisk := []types.InstalledMod{disk("RefinedPower", "3.3.0")}
	lf := v2.Lockfile{Mods: []v2.ModLock{lock("RefinedPower", "3.3.0", "{}")}}

	plan := PlanSync(onDisk, lf)

	if len(plan.Install) != 0 || len(plan.Remove) != 0 {
		t.Fatalf("expected nothing to do, got %+v", plan)
	}
}

func TestPlanSyncInstallsAMissingMod(t *testing.T) {
	lf := v2.Lockfile{Mods: []v2.ModLock{lock("RefinedPower", "3.3.0", "{}")}}

	plan := PlanSync(nil, lf)

	if len(plan.Install) != 1 || plan.Install[0].ModReference != "RefinedPower" {
		t.Fatalf("expected RefinedPower to be installed, got %+v", plan.Install)
	}
}

func TestPlanSyncReinstallsAVersionMismatch(t *testing.T) {
	onDisk := []types.InstalledMod{disk("RefinedPower", "3.2.1")}
	lf := v2.Lockfile{Mods: []v2.ModLock{lock("RefinedPower", "3.3.0", "{}")}}

	plan := PlanSync(onDisk, lf)

	if len(plan.Install) != 1 || plan.Install[0].Version != "3.3.0" {
		t.Fatalf("expected an upgrade to 3.3.0, got %+v", plan.Install)
	}
	// An upgrade replaces in place; it is not a remove plus an install, or the mod
	// would vanish if the download failed.
	if len(plan.Remove) != 0 {
		t.Fatalf("expected no removal for an upgrade, got %+v", plan.Remove)
	}
}

func TestPlanSyncRemovesAModAbsentFromTheLockfile(t *testing.T) {
	onDisk := []types.InstalledMod{
		disk("RefinedPower", "3.3.0"),
		disk("OldMod", "1.0.0"),
	}
	lf := v2.Lockfile{Mods: []v2.ModLock{lock("RefinedPower", "3.3.0", "{}")}}

	plan := PlanSync(onDisk, lf)

	if len(plan.Remove) != 1 || plan.Remove[0] != "OldMod" {
		t.Fatalf("expected OldMod to be removed, got %+v", plan.Remove)
	}
}

// Every mod's config is rewritten every sync: it is desired state, and the file
// on disk is not readable back into the lockfile to compare.
func TestPlanSyncWritesEveryConfig(t *testing.T) {
	onDisk := []types.InstalledMod{disk("RefinedPower", "3.3.0")}
	lf := v2.Lockfile{Mods: []v2.ModLock{lock("RefinedPower", "3.3.0", `{"a":1}`)}}

	plan := PlanSync(onDisk, lf)

	if len(plan.Configs) != 1 || plan.Configs["RefinedPower"] != `{"a":1}` {
		t.Fatalf("expected the config to be written, got %+v", plan.Configs)
	}
}
