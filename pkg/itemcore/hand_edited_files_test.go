package itemcore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeStoreFile puts one hand-made item file into the store.
func writeStoreFile(t *testing.T, itemsDir, name, content string) string {
	t.Helper()
	path := filepath.Join(itemsDir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return path
}

// beans-ih2r: an item file that starts with a UTF-8 BOM loads like any other,
// and `lif update` rewrites it with exactly one front matter block and no BOM.
func TestUpdateOfAFileWithBOMKeepsOneFrontMatterBlock(t *testing.T) {
	core, itemsDir := setupTestCore(t)
	raw := "\xEF\xBB\xBF---\n# bom-ab12\ntitle: Written in Notepad\nstatus: todo\ntype: task\n---\n\nBody text.\n"
	path := writeStoreFile(t, itemsDir, "bom-ab12--written-in-notepad.md", raw)
	if err := core.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	b, err := core.Get("bom-ab12")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if b.Title != "Written in Notepad" || b.Status != "todo" {
		t.Fatalf("loaded title = %q, status = %q; the front matter behind the BOM was not read", b.Title, b.Status)
	}

	// The etag a reader gets is the one the write path accepts.
	etag := b.ETag()
	b.Status = "in-progress"
	if err := core.Update(b, &etag); err != nil {
		t.Fatalf("Update() with the etag of the BOM file error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if strings.HasPrefix(got, "\xEF\xBB\xBF") {
		t.Errorf("updated file still starts with a BOM:\n%q", got)
	}
	if n := strings.Count(got, "---\n"); n != 2 {
		t.Errorf("updated file holds %d front matter delimiters, want 2 (one block):\n%s", n, got)
	}
	if !strings.Contains(got, "title: Written in Notepad") || !strings.Contains(got, "status: in-progress") {
		t.Errorf("updated file lost a field:\n%s", got)
	}

	reloaded := New(itemsDir, core.Config())
	reloaded.SetWarnWriter(nil)
	if err := reloaded.Load(); err != nil {
		t.Fatalf("reload error = %v", err)
	}
	again, err := reloaded.Get("bom-ab12")
	if err != nil {
		t.Fatalf("Get() after reload error = %v", err)
	}
	if again.Title != "Written in Notepad" || again.Status != "in-progress" || strings.Contains(again.Body, "title:") {
		t.Errorf("reloaded item = title %q, status %q, body %q; want the updated item with its own body", again.Title, again.Status, again.Body)
	}
}
