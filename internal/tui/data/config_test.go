package data

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/rierek/lif/pkg/config"
)

const customCfg = `items:
  prefix: t-
statuses:
  - name: review
    color: blue
types:
  - name: story
    rank: 3
priorities:
  - name: urgent
`

func applyCustom(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".lif.yml"), []byte(customCfg), 0o644); err != nil {
		t.Fatal(err)
	}
	applyRepoConfig(dir)
	t.Cleanup(func() { SetConfig(nil) })
	return dir
}

func TestRepoConfigUsesConfiguredLists(t *testing.T) {
	applyCustom(t)
	for name, got := range map[string][]string{"status": StatusValues(), "type": TypeValues(), "priority": PriorityValues()} {
		want := map[string]string{"status": "review", "type": "story", "priority": "urgent"}[name]
		if !contains(got, want) {
			t.Errorf("%s list %v lacks configured value %q", name, got, want)
		}
	}
	if StatusRank("review") >= len(StatusValues()) {
		t.Errorf("configured status has no rank: %d", StatusRank("review"))
	}
}

func TestRepoConfigFallsBackToDefaults(t *testing.T) {
	applyRepoConfig(t.TempDir())
	t.Cleanup(func() { SetConfig(nil) })
	if !contains(StatusValues(), "scrapped") || contains(StatusValues(), "review") {
		t.Errorf("no config must give built-in statuses, got %v", StatusValues())
	}
}

func TestValidParentTypesUseConfiguredRanks(t *testing.T) {
	applyCustom(t)
	// story has rank 3 (like feature): it may hang under rank 1 and 2 types only.
	want := []string{"milestone", "epic"}
	if got := validParentTypes("story"); !reflect.DeepEqual(got, want) {
		t.Fatalf("validParentTypes(story) = %v, want %v", got, want)
	}
}

func contains(l []string, v string) bool {
	for _, x := range l {
		if x == v {
			return true
		}
	}
	return false
}

func TestDefaultsFallBackToFirstEntryWhenBuiltinDefaultMissing(t *testing.T) {
	dir := t.TempDir()
	cfg := "items:\n  prefix: t-\ntypes_exclusive: true\ntypes:\n  - name: story\n    rank: 1\n"
	if err := os.WriteFile(filepath.Join(dir, ".lif.yml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	applyRepoConfig(dir)
	t.Cleanup(func() { SetConfig(nil) })
	if got := DefaultType(); got != "story" {
		t.Errorf("DefaultType() = %q, want story (task is not configured)", got)
	}
	if got := DefaultStatus(); got != "todo" {
		t.Errorf("DefaultStatus() = %q, want todo", got)
	}
}

// applyRepoConfig makes the config found at dir the active one, the way the
// TUI does for an opened Store.
func applyRepoConfig(dir string) {
	cfg, err := config.LoadFromDirectory(dir)
	if err != nil {
		cfg = nil
	}
	SetConfig(cfg)
}
