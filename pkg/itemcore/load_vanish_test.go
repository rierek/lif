package itemcore

import (
	"os"
	"path/filepath"
	"testing"
)

// A file renamed between directory read and open must not fail the load:
// the walk restarts and finds the file under its new name.
func TestLoadFromDiskRetriesWhenFileVanishes(t *testing.T) {
	core, dir := setupTestCore(t)
	old := filepath.Join(dir, "vanish--old.md")
	renamed := filepath.Join(dir, "vanish--new.md")
	body := "---\ntitle: Vanish\nstatus: todo\ntype: task\n---\n"
	if err := os.WriteFile(old, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	renames := 0
	beforeLoadItemHook = func(path string) {
		if path == old && renames == 0 {
			renames++
			if err := os.Rename(old, renamed); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Cleanup(func() { beforeLoadItemHook = nil })

	core.mu.Lock()
	err := core.loadFromDisk()
	core.mu.Unlock()
	if err != nil {
		t.Fatalf("loadFromDisk: %v", err)
	}
	if renames != 1 {
		t.Fatalf("hook renamed %d times, want 1", renames)
	}
	if got := len(core.All()); got != 1 {
		t.Fatalf("loaded %d items, want 1", got)
	}
}
