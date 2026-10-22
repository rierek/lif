package itemcore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rierek/lif/pkg/item"
)

// A one-character ID has 36 values: fill 35 of them, 20 of those only as files
// that another process wrote after this core loaded. Create has to find the
// one free ID instead of reusing a taken one.
func TestCreateGeneratesUnusedID(t *testing.T) {
	core, dir := setupTestCore(t)
	core.config.Items.IDLength = 1
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyz"
	free := "z"
	for i, ch := range alphabet {
		id := string(ch)
		if id == free {
			continue
		}
		if i%2 == 0 {
			b := &item.Item{ID: id, Slug: "mem", Title: "m", Status: "todo", Type: "task"}
			if err := core.Create(b); err != nil {
				t.Fatal(err)
			}
			continue
		}
		name := item.BuildFilename(id, "disk")
		if err := os.WriteFile(filepath.Join(dir, name), []byte("---\ntitle: d\nstatus: todo\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for range 20 {
		b := &item.Item{Slug: "new", Title: "n", Status: "todo", Type: "task"}
		if err := core.Create(b); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if b.ID != free {
			t.Fatalf("ID = %q, want the only free ID %q", b.ID, free)
		}
		if err := core.Delete(b.ID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNewIDExhausted(t *testing.T) {
	core, _ := setupTestCore(t)
	core.config.Items.IDLength = 1
	for _, ch := range "0123456789abcdefghijklmnopqrstuvwxyz" {
		if err := core.Create(&item.Item{ID: string(ch), Slug: "x", Title: "x", Status: "todo", Type: "task"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := core.NewID(""); err == nil {
		t.Fatal("NewID succeeded with every ID taken")
	}
}

// Two files with one ID are reported by DuplicateIDs; one wins in the store.
func TestLoadReportsDuplicateIDs(t *testing.T) {
	core, dir := setupTestCore(t)
	for _, slug := range []string{"one", "two"} {
		name := item.BuildFilename("dup1", slug)
		if err := os.WriteFile(filepath.Join(dir, name), []byte("---\ntitle: "+slug+"\nstatus: todo\n---\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := core.Load(); err != nil {
		t.Fatal(err)
	}
	dups := core.DuplicateIDs()
	if len(dups) != 1 || len(dups["dup1"]) != 2 {
		t.Fatalf("DuplicateIDs() = %v, want dup1 with 2 paths", dups)
	}
}
