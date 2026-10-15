package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
)

// runInitCmd runs the real initCmd.RunE against a fresh temp directory as the
// current working directory, restoring both the working directory and the
// package-level flag variables it touches afterwards. It does not go through
// cobra's flag parsing (initCmd is a package-level singleton shared with the
// live CLI, so re-registering its flags would panic on a second call); the
// three variables it sets are exactly what RunE reads.
func runInitCmd(t *testing.T, profile string, json bool) (dir string, err error) {
	t.Helper()

	dir = t.TempDir()
	t.Chdir(dir)

	origProfile, origJSON, origItemsPath := initProfile, initJSON, storePath
	initProfile, initJSON, storePath = profile, json, ""
	t.Cleanup(func() { initProfile, initJSON, storePath = origProfile, origJSON, origItemsPath })

	err = initCmd.RunE(initCmd, nil)
	return dir, err
}

func TestInitWithAProfileWritesTheExpandedTypeList(t *testing.T) {
	dir := t.TempDir()

	cfg := config.DefaultWithPrefix("demo-")
	types, ok := config.ProfileTypes("simple")
	if !ok {
		t.Fatal("simple profile missing")
	}
	cfg.Types = types
	cfg.SetConfigDir(dir)
	if err := cfg.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := config.Load(filepath.Join(dir, config.ConfigFileName))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := loaded.RankOf("bucket"); got != 1 {
		t.Errorf("RankOf(\"bucket\") after round-trip = %d, want 1", got)
	}
	if loaded.IsRoadmapType("bucket") {
		t.Error("bucket must stay hidden after a round-trip through the config file")
	}
}

func TestInitCmdWithKnownProfileWritesTheExpandedTypesToDisk(t *testing.T) {
	dir, err := runInitCmd(t, "simple", false)
	if err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	loaded, err := config.Load(filepath.Join(dir, config.ConfigFileName))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := len(loaded.Types); got != 6 {
		t.Errorf("len(loaded.Types) = %d, want 6 (the full simple profile, not a partial write)", got)
	}
	if got := loaded.RankOf("bucket"); got != 1 {
		t.Errorf("RankOf(\"bucket\") = %d, want 1", got)
	}
	if loaded.IsRoadmapType("bucket") {
		t.Error("bucket must carry roadmap: false in the written file")
	}
	if got := loaded.RankOf("feature"); got != 2 {
		t.Errorf("RankOf(\"feature\") = %d, want 2 (simple profile)", got)
	}
	if got := loaded.ShortOf("bucket"); got != "K" {
		t.Errorf("ShortOf(\"bucket\") = %q, want \"K\"", got)
	}
	if loaded.Items.DefaultType != "task" {
		t.Errorf("Beans.DefaultType = %q, want \"task\"", loaded.Items.DefaultType)
	}
	if !loaded.TypesExclusive {
		t.Error("TypesExclusive = false, want true - a profile must give a project exactly its own types")
	}
}

// The path every existing user hits: lif init with no --profile at all
// must not be affected by any of this task's changes - no types: block, no
// types_exclusive key, same file as before this task existed.
func TestInitCmdWithoutProfileWritesNoTypesBlock(t *testing.T) {
	dir, err := runInitCmd(t, "", false)
	if err != nil {
		t.Fatalf("initCmd.RunE: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, config.ConfigFileName))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(raw), "types_exclusive") {
		t.Errorf("plain init must not write types_exclusive:\n%s", raw)
	}
	if strings.Contains(string(raw), "types:") {
		t.Errorf("plain init must not write a types: block:\n%s", raw)
	}

	loaded, err := config.Load(filepath.Join(dir, config.ConfigFileName))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.TypesExclusive {
		t.Error("plain init: TypesExclusive = true, want false")
	}
}

func TestInitCmdWithUnknownProfileFailsBeforeAnySideEffect(t *testing.T) {
	dir, err := runInitCmd(t, "enterprise", false)
	if err == nil {
		t.Fatal("expected an error for an unknown profile")
	}
	if !strings.Contains(err.Error(), "enterprise") {
		t.Errorf("error %q does not name the unknown profile", err.Error())
	}
	for _, name := range config.ProfileNames() {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q is missing profile name %q", err.Error(), name)
		}
	}

	// Neither the store directory nor the config file may appear, under the
	// new or the legacy name.
	requireAbsent(t, dir, config.DefaultStoreDir, config.LegacyDefaultStoreDir, config.ConfigFileName, config.LegacyConfigFileName)
}
