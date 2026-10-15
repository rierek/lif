package commands

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/spf13/cobra"
)

// setupOrderTest installs a throwaway core and default config into the
// package globals orderCmd.RunE reads, creates a parent item plus n
// children (in order, with ascending single-letter Order keys "A".."?"),
// and returns the parent and its children.
func setupOrderTest(t *testing.T, n int) (parent *item.Item, children []*item.Item) {
	t.Helper()
	tmpDir := t.TempDir()
	itemsDir := filepath.Join(tmpDir, ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}

	testCfg := config.Default()
	testCore := itemcore.New(itemsDir, testCfg)
	if err := testCore.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}

	oldCore, oldCfg := core, cfg
	core, cfg = testCore, testCfg
	t.Cleanup(func() { core, cfg = oldCore, oldCfg })

	parent = &item.Item{
		ID:     "beans-parent",
		Slug:   item.Slugify("Parent"),
		Title:  "Parent",
		Status: "todo",
		Type:   "epic",
	}
	if err := core.Create(parent); err != nil {
		t.Fatalf("core.Create(parent) error = %v", err)
	}

	letters := "ABCDEFGHIJ"
	for i := 0; i < n; i++ {
		c := &item.Item{
			ID:     "beans-child" + string(letters[i]),
			Slug:   item.Slugify("Child " + string(letters[i])),
			Title:  "Child " + string(letters[i]),
			Status: "todo",
			Type:   "task",
			Parent: parent.ID,
			Order:  string(letters[i]),
		}
		if err := core.Create(c); err != nil {
			t.Fatalf("core.Create(child) error = %v", err)
		}
		children = append(children, c)
	}

	return parent, children
}

// resetOrderFlags clears every order* package global the tests below touch.
func resetOrderFlags(t *testing.T) {
	t.Helper()
	oldAfter, oldBefore, oldFirst, oldLast, oldJSON := orderAfter, orderBefore, orderFirst, orderLast, orderJSON
	orderAfter, orderBefore, orderFirst, orderLast, orderJSON = "", "", false, false, false
	t.Cleanup(func() {
		orderAfter, orderBefore, orderFirst, orderLast, orderJSON = oldAfter, oldBefore, oldFirst, oldLast, oldJSON
	})
}

// D05/D12: `order --json` returns the bare item document directly, not a
// {success,item,message} envelope -- same shape as `show`/`update`/`tag`.
func TestOrderCmdJSONReturnsBareItem(t *testing.T) {
	_, children := setupOrderTest(t, 3) // A, B, C
	resetOrderFlags(t)
	orderFirst = true
	orderJSON = true

	out := captureOrderStdout(t, func() {
		if err := orderCmd.RunE(orderCmd, []string{children[2].ID}); err != nil {
			t.Fatalf("orderCmd.RunE() error = %v", err)
		}
	})

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decoding JSON: %v; output = %s", err, out)
	}
	for _, envelopeKey := range []string{"success", "message", "item"} {
		if _, ok := got[envelopeKey]; ok {
			t.Errorf("captured JSON has envelope key %q; want the raw bean", envelopeKey)
		}
	}
	if got["id"] != children[2].ID {
		t.Errorf("captured JSON = %s, want top-level id %q", out, children[2].ID)
	}
}

// captureOrderStdout redirects os.Stdout for the duration of fn and returns
// everything written to it.
func captureOrderStdout(t *testing.T, fn func()) []byte {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = orig
	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe write end: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}
	return data
}

// allItemFileContents walks the items dir and returns a map of relative path
// to file content, so tests can diff "what changed on disk" without relying
// on mtime resolution.
func allItemFileContents(t *testing.T, root string) map[string]string {
	t.Helper()
	out := make(map[string]string)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walking beans dir: %v", err)
	}
	return out
}

// AC1: --after places the item strictly between that sibling and its
// successor.
func TestOrderCmdAfterSetsBetweenSiblings(t *testing.T) {
	_, children := setupOrderTest(t, 3) // A, B, C
	resetOrderFlags(t)

	orderAfter = children[0].ID // A

	if err := orderCmd.RunE(orderCmd, []string{children[2].ID}); err != nil { // move C
		t.Fatalf("orderCmd.RunE() error = %v", err)
	}

	got, err := core.Get(children[2].ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	if !(got.Order > "A" && got.Order < "B") {
		t.Errorf("Order = %q, want strictly between %q and %q", got.Order, "A", "B")
	}
}

// AC2: --before places the item strictly between that sibling and its
// predecessor.
func TestOrderCmdBeforeSetsBetweenSiblings(t *testing.T) {
	_, children := setupOrderTest(t, 3) // A, B, C
	resetOrderFlags(t)

	orderBefore = children[2].ID // C

	if err := orderCmd.RunE(orderCmd, []string{children[0].ID}); err != nil { // move A
		t.Fatalf("orderCmd.RunE() error = %v", err)
	}

	got, err := core.Get(children[0].ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	if !(got.Order > "B" && got.Order < "C") {
		t.Errorf("Order = %q, want strictly between %q and %q", got.Order, "B", "C")
	}
}

// AC3: --first sorts before every sibling.
func TestOrderCmdFirstSortsBeforeAllSiblings(t *testing.T) {
	_, children := setupOrderTest(t, 3) // A, B, C
	resetOrderFlags(t)

	orderFirst = true

	if err := orderCmd.RunE(orderCmd, []string{children[2].ID}); err != nil { // move C
		t.Fatalf("orderCmd.RunE() error = %v", err)
	}

	got, err := core.Get(children[2].ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	if got.Order >= "A" {
		t.Errorf("Order = %q, want sorting before %q", got.Order, "A")
	}
}

// AC4: --last sorts after every sibling.
func TestOrderCmdLastSortsAfterAllSiblings(t *testing.T) {
	_, children := setupOrderTest(t, 3) // A, B, C
	resetOrderFlags(t)

	orderLast = true

	if err := orderCmd.RunE(orderCmd, []string{children[0].ID}); err != nil { // move A
		t.Fatalf("orderCmd.RunE() error = %v", err)
	}

	got, err := core.Get(children[0].ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	if got.Order <= "C" {
		t.Errorf("Order = %q, want sorting after %q", got.Order, "C")
	}
}

// AC5/SC-01: moving the fifth of six siblings to second position leaves the
// six in the expected sequence and modifies exactly one file on disk —
// asserted by a write count (file-content diff), not merely the resulting
// order, so a full renumbering implementation would fail this test even if
// the sequence came out right.
func TestOrderCmdWritesExactlyOneFile(t *testing.T) {
	_, children := setupOrderTest(t, 6) // A..F
	resetOrderFlags(t)

	before := allItemFileContents(t, core.Root())

	orderAfter = children[0].ID // A

	if err := orderCmd.RunE(orderCmd, []string{children[4].ID}); err != nil { // move E after A
		t.Fatalf("orderCmd.RunE() error = %v", err)
	}

	after := allItemFileContents(t, core.Root())

	changed := 0
	var changedPath string
	for path, content := range before {
		if after[path] != content {
			changed++
			changedPath = path
		}
	}
	if changed != 1 {
		t.Fatalf("expected exactly 1 file to change, got %d", changed)
	}
	if changedPath != children[4].Path {
		t.Errorf("changed file = %q, want the moved bean's file %q", changedPath, children[4].Path)
	}

	// Resulting sequence: A, E, B, C, D, F.
	all, err := core.Get(children[0].Parent)
	if err != nil {
		t.Fatalf("core.Get(parent) error = %v", err)
	}
	_ = all
	siblings := make([]*item.Item, 0, 6)
	for _, c := range children {
		got, err := core.Get(c.ID)
		if err != nil {
			t.Fatalf("core.Get() error = %v", err)
		}
		siblings = append(siblings, got)
	}
	item.SortByOrder(siblings)

	wantSeq := []string{children[0].ID, children[4].ID, children[1].ID, children[2].ID, children[3].ID, children[5].ID}
	for i, id := range wantSeq {
		if siblings[i].ID != id {
			t.Errorf("siblings[%d].ID = %q, want %q", i, siblings[i].ID, id)
		}
	}
}

// AC6: --after/--before naming an item under a different parent is scoped
// per parent and fails.
func TestOrderCmdDifferentParentFails(t *testing.T) {
	parent, children := setupOrderTest(t, 2)
	resetOrderFlags(t)

	// A sibling epic with its own child, unrelated to `parent`.
	otherParent := &item.Item{
		ID: "beans-other", Slug: item.Slugify("Other"), Title: "Other", Status: "todo", Type: "epic",
	}
	if err := core.Create(otherParent); err != nil {
		t.Fatalf("core.Create(otherParent) error = %v", err)
	}
	outsider := &item.Item{
		ID: "beans-outsider", Slug: item.Slugify("Outsider"), Title: "Outsider",
		Status: "todo", Type: "task", Parent: otherParent.ID, Order: "A",
	}
	if err := core.Create(outsider); err != nil {
		t.Fatalf("core.Create(outsider) error = %v", err)
	}
	_ = parent

	orderAfter = outsider.ID

	err := orderCmd.RunE(orderCmd, []string{children[0].ID})
	if err == nil {
		t.Fatal("expected error for cross-parent --after")
	}
	if !contains(err.Error(), "scoped per parent") {
		t.Errorf("expected error to mention scope, got %q", err.Error())
	}
}

// AC7: more than one of --after/--before/--first/--last is a usage error.
// Cobra enforces this via MarkFlagsMutuallyExclusive, which only fires
// during flag parsing/validation on a real Execute() — not when calling
// RunE directly (as the other tests here do) — so this test drives the
// command the way the CLI actually does.
func TestOrderCmdMutuallyExclusiveFlags(t *testing.T) {
	_, children := setupOrderTest(t, 2)
	resetOrderFlags(t)

	root := &cobra.Command{Use: "beans"}
	RegisterOrderCmd(root)
	root.SetArgs([]string{"order", children[0].ID, "--first", "--last"})
	root.SetOut(new(nopWriter))
	root.SetErr(new(nopWriter))

	if err := root.Execute(); err == nil {
		t.Fatal("expected error for mutually exclusive flags")
	}
}

// No placement flag at all is also a usage error (nothing to do).
func TestOrderCmdRequiresAPlacementFlag(t *testing.T) {
	_, children := setupOrderTest(t, 2)
	resetOrderFlags(t)

	if err := orderCmd.RunE(orderCmd, []string{children[0].ID}); err == nil {
		t.Fatal("expected error when no placement flag is given")
	}
}

// setupOrderTestUnordered is setupOrderTest, but the children carry no Order
// at all — the state every real store starts in, since nothing before this
// epic ever wrote to Item.Order. setupOrderTest's fixture (every child
// pre-seeded with a letter) can't see B01: SortByOrder always puts
// Order-having siblings first, so picking siblings[0]/siblings[len-1]
// happens to be correct whenever every sibling already has an Order.
func setupOrderTestUnordered(t *testing.T, n int) (parent *item.Item, children []*item.Item) {
	t.Helper()
	tmpDir := t.TempDir()
	itemsDir := filepath.Join(tmpDir, ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}

	testCfg := config.Default()
	testCore := itemcore.New(itemsDir, testCfg)
	if err := testCore.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}

	oldCore, oldCfg := core, cfg
	core, cfg = testCore, testCfg
	t.Cleanup(func() { core, cfg = oldCore, oldCfg })

	parent = &item.Item{
		ID:     "beans-parent",
		Slug:   item.Slugify("Parent"),
		Title:  "Parent",
		Status: "todo",
		Type:   "epic",
	}
	if err := core.Create(parent); err != nil {
		t.Fatalf("core.Create(parent) error = %v", err)
	}

	letters := "abcdefghij"
	for i := 0; i < n; i++ {
		c := &item.Item{
			ID:     "beans-child" + string(letters[i]),
			Slug:   item.Slugify("Child " + string(letters[i])),
			Title:  "Child " + string(letters[i]),
			Status: "todo",
			Type:   "task",
			Parent: parent.ID,
		}
		if err := core.Create(c); err != nil {
			t.Fatalf("core.Create(child) error = %v", err)
		}
		children = append(children, c)
	}

	return parent, children
}

// B01 regression: on a fresh store where no sibling has an Order yet (the
// normal starting state), --last used to read siblings[len(siblings)-1].Order
// — always "" here, since SortByOrder sorts Order-less items to the end by
// title, not to the front — so every --last call computed
// OrderBetween("", "") and got the same constant "V" back, regardless of how
// many items had already been placed last.
func TestOrderCmdLastOnUnorderedSiblingsProducesDistinctKeys(t *testing.T) {
	_, children := setupOrderTestUnordered(t, 3)
	resetOrderFlags(t)

	// --last needs one ordered sibling to anchor on (beans-7zfw), so seed
	// the first with --first.
	orderFirst = true
	if err := orderCmd.RunE(orderCmd, []string{children[0].ID}); err != nil {
		t.Fatalf("orderCmd.RunE() (seed --first) error = %v", err)
	}
	first, err := core.Get(children[0].ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	resetOrderFlags(t)
	orderLast = true
	if err := orderCmd.RunE(orderCmd, []string{children[1].ID}); err != nil {
		t.Fatalf("orderCmd.RunE() (1st --last) error = %v", err)
	}
	second, err := core.Get(children[1].ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	resetOrderFlags(t)
	orderLast = true
	if err := orderCmd.RunE(orderCmd, []string{children[2].ID}); err != nil {
		t.Fatalf("orderCmd.RunE() (2nd --last) error = %v", err)
	}
	third, err := core.Get(children[2].ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}

	if !(first.Order < second.Order && second.Order < third.Order) {
		t.Errorf("orders not strictly increasing: %q, %q, %q", first.Order, second.Order, third.Order)
	}
}

// beans-7zfw: with siblings but none ordered, --last has no anchor and used
// to fall back to the first position silently. It must fail and leave the
// item untouched.
func TestOrderCmdLastWithNoOrderedSiblingFails(t *testing.T) {
	_, children := setupOrderTestUnordered(t, 3)
	resetOrderFlags(t)
	orderLast = true

	err := orderCmd.RunE(orderCmd, []string{children[0].ID})
	if err == nil || !strings.Contains(err.Error(), "--last") {
		t.Fatalf("error = %v, want --last rejection naming the workaround", err)
	}
	got, gerr := core.Get(children[0].ID)
	if gerr != nil {
		t.Fatalf("core.Get() error = %v", gerr)
	}
	if got.Order != "" {
		t.Errorf("bean mutated: Order = %q, want empty", got.Order)
	}
}

// An item without siblings has no position to get wrong; --last stays valid.
func TestOrderCmdLastWithoutSiblingsSucceeds(t *testing.T) {
	_, children := setupOrderTestUnordered(t, 1)
	resetOrderFlags(t)
	orderLast = true
	if err := orderCmd.RunE(orderCmd, []string{children[0].ID}); err != nil {
		t.Fatalf("orderCmd.RunE() error = %v", err)
	}
}

// B01 regression: --after/--before naming a reference sibling that has no
// Order itself used to compute OrderBetween(ref.Order="", next="") = "V"
// unconditionally, ignoring the reference entirely — "aaa --after ddd" could
// place aaa before ddd instead of after it. Placing relative to an unordered
// reference should fail clearly instead of silently producing a wrong key.
func TestOrderCmdAfterUnorderedRefFails(t *testing.T) {
	_, children := setupOrderTestUnordered(t, 2)
	resetOrderFlags(t)

	orderAfter = children[0].ID

	err := orderCmd.RunE(orderCmd, []string{children[1].ID})
	if err == nil {
		t.Fatal("expected error placing relative to a sibling with no explicit order")
	}
	if !contains(err.Error(), "no explicit order") {
		t.Errorf("expected error to mention the missing order, got %q", err.Error())
	}

	got, err := core.Get(children[1].ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	if got.Order != "" {
		t.Errorf("expected moved bean's Order to be unchanged (empty), got %q", got.Order)
	}
}

// B02 regression: order.go's core.Update(b, nil) bypassed ETag concurrency
// control the same way create.go's did before its own B01 fix — under
// require_if_match:true this makes `lif order` fail outright on every
// call, since nil never satisfies a required if-match.
func TestOrderCmdSucceedsUnderRequireIfMatch(t *testing.T) {
	tmpDir := t.TempDir()
	itemsDir := filepath.Join(tmpDir, ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}
	testCfg := config.Default()
	testCfg.Items.RequireIfMatch = true
	testCore := itemcore.New(itemsDir, testCfg)
	if err := testCore.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}
	oldCore, oldCfg := core, cfg
	core, cfg = testCore, testCfg
	t.Cleanup(func() { core, cfg = oldCore, oldCfg })

	parent := &item.Item{ID: "beans-parent", Slug: item.Slugify("Parent"), Title: "Parent", Status: "todo", Type: "epic"}
	if err := core.Create(parent); err != nil {
		t.Fatalf("core.Create(parent) error = %v", err)
	}
	a := &item.Item{ID: "beans-childA", Slug: item.Slugify("Child A"), Title: "Child A", Status: "todo", Type: "task", Parent: parent.ID, Order: "A"}
	if err := core.Create(a); err != nil {
		t.Fatalf("core.Create(a) error = %v", err)
	}
	b := &item.Item{ID: "beans-childB", Slug: item.Slugify("Child B"), Title: "Child B", Status: "todo", Type: "task", Parent: parent.ID}
	if err := core.Create(b); err != nil {
		t.Fatalf("core.Create(b) error = %v", err)
	}

	resetOrderFlags(t)
	orderLast = true

	if err := orderCmd.RunE(orderCmd, []string{b.ID}); err != nil {
		t.Fatalf("orderCmd.RunE() error = %v (order should succeed under require_if_match:true, not fail on a nil ifMatch)", err)
	}
}

// B05 regression: every real `lif order` invocation is its own OS process,
// so the item it operates on was loaded fresh via Core.Load() -- which
// defaults empty Priority to "normal" and empty Type to "task" in memory
// (core.go: "Apply defaults for GraphQL non-nullable fields"). An item
// created without -p/-t (the common case) has neither field on disk, so
// Render()-ing the freshly loaded, now-defaulted item produces bytes that
// differ from the file Core.Update reads and hashes for its own etag check.
// Using item.ETag() as the auto-captured ifMatch (mirroring create.go's
// pattern) therefore makes `lif order` fail with a false etag mismatch on
// essentially every ordinary item. setupOrderTest/setupOrderTestUnordered
// can't see this: they create items and operate on them in the same process
// with the same Core, so the in-memory item IS the exact object Render()
// already wrote to disk -- no reload, no defaulting, no discrepancy.
func TestOrderCmdSucceedsOnItemLoadedFreshFromDisk(t *testing.T) {
	tmpDir := t.TempDir()
	itemsDir := filepath.Join(tmpDir, ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}
	testCfg := config.Default()

	// First "process": write parent + two children with no priority/type
	// set, exactly as `lif create` without -p/-t leaves them.
	writerCore := itemcore.New(itemsDir, testCfg)
	if err := writerCore.Load(); err != nil {
		t.Fatalf("failed to load writer core: %v", err)
	}
	parent := &item.Item{ID: "beans-parent", Slug: item.Slugify("Parent"), Title: "Parent", Status: "todo", Type: "epic"}
	if err := writerCore.Create(parent); err != nil {
		t.Fatalf("core.Create(parent) error = %v", err)
	}
	a := &item.Item{ID: "beans-childA", Slug: item.Slugify("Child A"), Title: "Child A", Status: "todo", Parent: parent.ID}
	if err := writerCore.Create(a); err != nil {
		t.Fatalf("core.Create(a) error = %v", err)
	}
	c2 := &item.Item{ID: "beans-childB", Slug: item.Slugify("Child B"), Title: "Child B", Status: "todo", Parent: parent.ID}
	if err := writerCore.Create(c2); err != nil {
		t.Fatalf("core.Create(c2) error = %v", err)
	}

	// Second "process": a fresh Core reloads the same directory from disk --
	// what every real `lif order` invocation does.
	freshCore := itemcore.New(itemsDir, testCfg)
	if err := freshCore.Load(); err != nil {
		t.Fatalf("failed to load fresh core: %v", err)
	}

	oldCore, oldCfg := core, cfg
	core, cfg = freshCore, testCfg
	t.Cleanup(func() { core, cfg = oldCore, oldCfg })

	resetOrderFlags(t)
	orderFirst = true
	if err := orderCmd.RunE(orderCmd, []string{a.ID}); err != nil {
		t.Fatalf("orderCmd.RunE() error = %v (order should succeed on a freshly loaded bean, not report a false etag mismatch)", err)
	}
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }

// AC3: order's lookup and placement errors name the noun as item and quote
// the lif command.
func TestOrderErrorsNameItems(t *testing.T) {
	_, children := setupOrderTestUnordered(t, 3)
	a, b := children[0].ID, children[1].ID

	for _, tc := range []struct {
		name string
		set  func()
		args []string
		want string
	}{
		{"unknown item", func() { orderFirst = true }, []string{"beans-nope"}, "item not found: beans-nope"},
		{"unknown reference", func() { orderAfter = "beans-nope" }, []string{a}, "item not found: beans-nope"},
		{"reference is the item", func() { orderAfter = a }, []string{a}, "cannot place an item relative to itself"},
		{"unordered reference", func() { orderAfter = a }, []string{b},
			"item " + a + " has no explicit order yet; place it first (e.g. `lif order " + a + " --first`) before placing another item relative to it"},
		{"last without an ordered sibling", func() { orderLast = true }, []string{a}, "(e.g. `lif order <id> --first`, then `--after`)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetOrderFlags(t)
			tc.set()
			err := orderCmd.RunE(orderCmd, tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
