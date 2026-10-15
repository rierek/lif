package itemcore

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// prefixedCore returns a Core over a fresh store whose config carries prefix,
// the way a project made by lif init does, and the path of the store.
func prefixedCore(t *testing.T, prefix string) (*Core, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // keep the persisted search index out of the real home
	itemsDir := filepath.Join(t.TempDir(), config.DefaultStoreDir)
	if err := os.MkdirAll(itemsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Items.Prefix = prefix
	core := New(itemsDir, cfg)
	core.SetWarnWriter(nil)
	return core, itemsDir
}

// recordedItem is the text of an item file whose "# <id>" line records
// recorded, with extra front matter lines after the standard fields.
func recordedItem(recorded, title, extra string) string {
	return "---\n# " + recorded + "\ntitle: " + title + "\nstatus: todo\ntype: task\n" + extra + "---\n\nBody.\n"
}

// beans-533z (review N1): the IDs that hand-renamed files still record are
// the IDs the links to them name.
func TestRenamedAwayIDsNamesTheIDsOfFilesRenamedByHand(t *testing.T) {
	items := []*item.Item{
		{ID: "ex-new1", Path: "ex-new1--docs.md", RecordedID: "ex-zyo0"},
		{ID: "ex-ok01", Path: "ex-ok01--fine.md", RecordedID: "ex-ok01"},
		{ID: "ex-nore", Path: "ex-nore--none.md"},
		// A copy: the ID it records is answered by another item, so no link to
		// that ID has lost its target.
		{ID: "ex-copy", Path: "ex-copy--fine.md", RecordedID: "ex-ok01"},
		// Two files that record the same missing ID: the smaller path is named.
		{ID: "ex-two2", Path: "b/ex-two2--x.md", RecordedID: "ex-gone"},
		{ID: "ex-two3", Path: "a/ex-two3--x.md", RecordedID: "ex-gone"},
	}

	got := RenamedAwayIDs(items)

	want := map[string]string{"ex-zyo0": "ex-new1--docs.md", "ex-gone": "a/ex-two3--x.md"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RenamedAwayIDs() = %v, want %v", got, want)
	}
}

// beans-533z (review N1): FixBrokenLinks removes links to nothing, and the
// links to the ID of a file renamed by hand are links to something: they
// resolve again once the file has its ID back, or after lif rename.
func TestFixBrokenLinksKeepsTheLinksToAnIDAHandRenamedFileRecords(t *testing.T) {
	core, itemsDir := prefixedCore(t, "ex-")
	writeStoreFile(t, itemsDir, "ex-new1--write-docs.md", recordedItem("ex-zyo0", "Write docs", ""))
	ship := writeStoreFile(t, itemsDir, "ex-3ip9--ship.md", recordedItem("ex-3ip9", "Ship",
		"parent: ex-zyo0\nblocking:\n    - ex-ghost\n    - ex-3ip9\nblocked_by:\n    - ex-zyo0\n    - ex-gone\n"))
	if err := core.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	fixed, err := core.FixBrokenLinks()
	if err != nil {
		t.Fatalf("FixBrokenLinks() error = %v", err)
	}

	// Removed: blocking ex-ghost and ex-gone (no file records them) and the
	// self-reference. Kept: parent and blocked_by to ex-zyo0.
	if fixed != 3 {
		t.Errorf("FixBrokenLinks() = %d, want 3", fixed)
	}
	data, err := os.ReadFile(ship)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{"parent: ex-zyo0", "- ex-zyo0"} {
		if !strings.Contains(text, want) {
			t.Errorf("the item lost %q:\n%s", want, text)
		}
	}
	for _, gone := range []string{"ex-ghost", "ex-gone", "- ex-3ip9"} {
		if strings.Contains(text, gone) {
			t.Errorf("the item still holds %q:\n%s", gone, text)
		}
	}
}
