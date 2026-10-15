package commands

// Tests for parseLink and isKnownLinkType have been moved to content_test.go
// since those functions now live in content.go

import (
	"bytes"
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

// setupUpdateTest installs a throwaway core and default config into the
// package globals updateCmd.RunE reads, and returns an item already persisted
// in that core.
func setupUpdateTest(t *testing.T) *item.Item {
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

	b := &item.Item{
		ID:     "beans-test1",
		Slug:   item.Slugify("A test bean"),
		Title:  "A test bean",
		Status: "todo",
		Type:   "task",
	}
	if err := core.Create(b); err != nil {
		t.Fatalf("core.Create() error = %v", err)
	}
	return b
}

// resetUpdateFlags clears every update* package global the tests below touch
// and restores the pre-test values afterwards, so tests stay isolated despite
// sharing the updateCmd package-level flag vars.
func resetUpdateFlags(t *testing.T) {
	t.Helper()
	oldSet, oldUnset := updateSet, updateUnset
	updateSet, updateUnset = nil, nil
	t.Cleanup(func() {
		updateSet, updateUnset = oldSet, oldUnset
	})
}

// AC1/AC2/SC-01: --set writes extra front matter keys, and repeats accumulate.
func TestUpdateCmdSetWritesExtraKeys(t *testing.T) {
	b := setupUpdateTest(t)
	resetUpdateFlags(t)

	updateSet = []string{"release=0-4-1", "klasse=bugfix"}

	if err := updateCmd.RunE(updateCmd, []string{b.ID}); err != nil {
		t.Fatalf("updateCmd.RunE() error = %v", err)
	}

	got, err := core.Get(b.ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	if got.Extra["release"] != "0-4-1" {
		t.Errorf("Extra[release] = %v, want %q", got.Extra["release"], "0-4-1")
	}
	if got.Extra["klasse"] != "bugfix" {
		t.Errorf("Extra[klasse] = %v, want %q", got.Extra["klasse"], "bugfix")
	}
}

// AC3: --unset removes a key that is present.
func TestUpdateCmdUnsetRemovesExtraKey(t *testing.T) {
	b := setupUpdateTest(t)
	resetUpdateFlags(t)

	updateSet = []string{"release=0-4-1"}
	if err := updateCmd.RunE(updateCmd, []string{b.ID}); err != nil {
		t.Fatalf("updateCmd.RunE() (set) error = %v", err)
	}
	resetUpdateFlags(t)

	updateSet = nil
	updateUnset = []string{"release"}
	if err := updateCmd.RunE(updateCmd, []string{b.ID}); err != nil {
		t.Fatalf("updateCmd.RunE() (unset) error = %v", err)
	}

	got, err := core.Get(b.ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	if _, ok := got.Extra["release"]; ok {
		t.Errorf("expected release to be removed, Extra = %#v", got.Extra)
	}
}

// AC5: --unset of a key the item does not carry leaves the item unchanged
// and exits zero.
func TestUpdateCmdUnsetAbsentKeyIsNoop(t *testing.T) {
	b := setupUpdateTest(t)
	resetUpdateFlags(t)

	updateUnset = []string{"nope"}

	if err := updateCmd.RunE(updateCmd, []string{b.ID}); err != nil {
		t.Fatalf("updateCmd.RunE() error = %v", err)
	}

	got, err := core.Get(b.ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	if len(got.Extra) != 0 {
		t.Errorf("expected no Extra keys, got %#v", got.Extra)
	}
}

// AC4: --set on a reserved key fails naming the native flag, and the item is
// left unchanged.
func TestUpdateCmdSetReservedKeyFails(t *testing.T) {
	b := setupUpdateTest(t)
	resetUpdateFlags(t)

	updateSet = []string{"status=done"}

	err := updateCmd.RunE(updateCmd, []string{b.ID})
	if err == nil {
		t.Fatal("expected error for reserved key")
	}
	if !contains(err.Error(), "--status") {
		t.Errorf("expected error to name --status, got %q", err.Error())
	}

	got, err := core.Get(b.ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	if got.Status != "todo" {
		t.Errorf("expected status to be unchanged, got %q", got.Status)
	}
}

// AC6: --set without "=" is a usage error.
func TestUpdateCmdSetWithoutEqualsFails(t *testing.T) {
	b := setupUpdateTest(t)
	resetUpdateFlags(t)

	updateSet = []string{"release"}

	if err := updateCmd.RunE(updateCmd, []string{b.ID}); err == nil {
		t.Fatal("expected error for --set without '='")
	}
}

// Conflict resolution: a key named by both --set and --unset in the same
// invocation ends up unset (see applyExtraOps).
func TestUpdateCmdSetAndUnsetSameKey(t *testing.T) {
	b := setupUpdateTest(t)
	resetUpdateFlags(t)

	updateSet = []string{"release=0-4-1"}
	updateUnset = []string{"release"}

	if err := updateCmd.RunE(updateCmd, []string{b.ID}); err != nil {
		t.Fatalf("updateCmd.RunE() error = %v", err)
	}

	got, err := core.Get(b.ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	if _, ok := got.Extra["release"]; ok {
		t.Errorf("expected release to be unset, Extra = %#v", got.Extra)
	}
}

// --set/--unset alone (no other update flags) is a valid, sufficient change.
func TestUpdateCmdSetAloneCountsAsChange(t *testing.T) {
	b := setupUpdateTest(t)
	resetUpdateFlags(t)

	updateSet = []string{"release=0-4-1"}

	if err := updateCmd.RunE(updateCmd, []string{b.ID}); err != nil {
		t.Fatalf("updateCmd.RunE() error = %v", err)
	}
}

// B01 regression: --set alone (no other update flags, so no field-update
// write runs first) used to pass ifMatch=nil to the extra-key write,
// silently ignoring a caller-supplied --if-match entirely -- a stale or
// outright wrong etag was accepted just the same as a correct one. This is
// the same optimistic-concurrency contract --priority/--status already
// honor; --set must not be a silent side door around it.
//
// Asserts against the file on disk, not core.Get(): Core.Update takes b by
// pointer and applyExtraOps mutates that same pointer before the etag check
// runs, so c.items[id] (and therefore core.Get) reflects the attempted
// mutation regardless of whether the write to disk was accepted -- this is
// documented in Core.Update's own comment and is exactly why it validates
// ifMatch against the on-disk content instead of the in-memory item. The
// property this test protects is "the file was not overwritten," which only
// a fresh read off disk can show.
func TestUpdateCmdSetAloneRejectsStaleIfMatch(t *testing.T) {
	b := setupUpdateTest(t)
	resetUpdateFlags(t)

	oldIfMatch := updateIfMatch
	updateIfMatch = "deadbeefdeadbeef" // not b's real etag
	t.Cleanup(func() { updateIfMatch = oldIfMatch })

	updateSet = []string{"release=0-4-1"}

	err := updateCmd.RunE(updateCmd, []string{b.ID})
	if err == nil {
		t.Fatal("expected an etag-mismatch error for a stale --if-match, got nil")
	}

	onDisk, readErr := readItemFromDisk(t, b)
	if readErr != nil {
		t.Fatalf("reading bean from disk: %v", readErr)
	}
	if _, ok := onDisk.Extra["release"]; ok {
		t.Errorf("file on disk should be unchanged after a rejected stale --if-match, got Extra = %#v", onDisk.Extra)
	}
}

// readItemFromDisk re-parses b's file directly off disk, bypassing Core's
// in-memory cache entirely -- see TestUpdateCmdSetAloneRejectsStaleIfMatch
// for why that distinction matters.
func readItemFromDisk(t *testing.T, b *item.Item) (*item.Item, error) {
	t.Helper()
	data, err := os.ReadFile(core.FullPath(b))
	if err != nil {
		return nil, err
	}
	return item.Parse(bytes.NewReader(data))
}

// Sanity counterpart to the regression above: the correct current --if-match
// still lets a --set-only update through.
func TestUpdateCmdSetAloneAcceptsCorrectIfMatch(t *testing.T) {
	b := setupUpdateTest(t)
	resetUpdateFlags(t)

	oldIfMatch := updateIfMatch
	updateIfMatch = b.ETag()
	t.Cleanup(func() { updateIfMatch = oldIfMatch })

	updateSet = []string{"release=0-4-1"}

	if err := updateCmd.RunE(updateCmd, []string{b.ID}); err != nil {
		t.Fatalf("updateCmd.RunE() error = %v (correct --if-match should be accepted)", err)
	}

	got, err := core.Get(b.ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	if got.Extra["release"] != "0-4-1" {
		t.Errorf("Extra[release] = %v, want %q", got.Extra["release"], "0-4-1")
	}
}

// B01 regression, require_if_match:true variant: the extra-key write used to
// pass ifMatch=nil, which under require_if_match:true fails outright even
// though the caller never asked for optimistic locking to be skipped.
func TestUpdateCmdSetSucceedsUnderRequireIfMatch(t *testing.T) {
	tmpDir := t.TempDir()
	itemsDir := tmpDir + "/.beans"
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

	b := &item.Item{ID: "beans-test1", Slug: item.Slugify("A test bean"), Title: "A test bean", Status: "todo", Type: "task"}
	if err := core.Create(b); err != nil {
		t.Fatalf("core.Create() error = %v", err)
	}

	resetUpdateFlags(t)
	oldIfMatch := updateIfMatch
	updateIfMatch = b.ETag()
	t.Cleanup(func() { updateIfMatch = oldIfMatch })

	updateSet = []string{"release=0-4-1"}

	if err := updateCmd.RunE(updateCmd, []string{b.ID}); err != nil {
		t.Fatalf("updateCmd.RunE() error = %v (a correct --if-match should satisfy require_if_match:true)", err)
	}

	got, err := core.Get(b.ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	if got.Extra["release"] != "0-4-1" {
		t.Errorf("Extra[release] = %v, want %q", got.Extra["release"], "0-4-1")
	}
}

// beans-13ae AC1/AC3/SC-01/SC-03: `update --json` returns the resulting
// item, read back after the write and shaped exactly like `show --json` --
// not a {success,item,message} envelope -- so a caller applying several
// changes in one call (parent plus an extra key here) can verify all of
// them from one process launch, without a second `show` call.
func TestUpdateCmdJSONReturnsResultingItem(t *testing.T) {
	b := setupUpdateTest(t)
	other := &item.Item{ID: "beans-parent1", Slug: item.Slugify("Parent"), Title: "Parent", Status: "todo", Type: "epic"}
	if err := core.Create(other); err != nil {
		t.Fatalf("core.Create(other) error = %v", err)
	}
	resetUpdateFlags(t)

	oldJSON := updateJSON
	updateJSON = true
	t.Cleanup(func() { updateJSON = oldJSON })
	scratch := &cobra.Command{Use: "update"}
	scratch.Flags().StringVar(&updateParent, "parent", "", "")
	if err := scratch.Flags().Set("parent", other.ID); err != nil {
		t.Fatalf("setting --parent flag: %v", err)
	}
	updateSet = []string{"release=0-6-0"}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	runErr := updateCmd.RunE(scratch, []string{b.ID})

	os.Stdout = oldStdout
	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe write end: %v", err)
	}
	captured, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}
	if runErr != nil {
		t.Fatalf("updateCmd.RunE() error = %v", runErr)
	}

	// SC-01/SC-03: the payload IS the item, at the top level -- no ".item"
	// wrapper, no ".success"/".message" envelope keys.
	var got map[string]any
	if err := json.Unmarshal(captured, &got); err != nil {
		t.Fatalf("unmarshaling captured stdout: %v (raw: %s)", err, captured)
	}
	for _, envelopeKey := range []string{"success", "message", "item"} {
		if _, ok := got[envelopeKey]; ok {
			t.Errorf("captured JSON has envelope key %q; want the raw bean, same shape as `show --json`", envelopeKey)
		}
	}

	// SC-01: parent is visible directly, no second command.
	if got["parent"] != other.ID {
		t.Errorf("parent = %v, want %q", got["parent"], other.ID)
	}
	// AC3: the extra-key write (a second, separate persist inside RunE)
	// is also reflected -- proves this is read after BOTH writes, not the
	// request echoed back.
	extra, _ := got["extra"].(map[string]any)
	if extra["release"] != "0-6-0" {
		t.Errorf("extra[release] = %v, want %q", extra["release"], "0-6-0")
	}

	// Cross-check against `show --json`'s own schema: unmarshal into the
	// same item.Item struct show uses and confirm nothing was lost.
	var asItem item.Item
	if err := json.Unmarshal(captured, &asItem); err != nil {
		t.Fatalf("unmarshaling captured stdout into item.Item: %v", err)
	}
	if asItem.ID != b.ID || asItem.Parent != other.ID {
		t.Errorf("decoded bean = %+v, want ID=%q Parent=%q", asItem, b.ID, other.ID)
	}
}

// SC-04: update without --json is unaffected by the --json schema change.
func TestUpdateCmdNonJSONOutputUnchanged(t *testing.T) {
	b := setupUpdateTest(t)
	resetUpdateFlags(t)

	oldJSON := updateJSON
	updateJSON = false
	t.Cleanup(func() { updateJSON = oldJSON })
	scratch := &cobra.Command{Use: "update"}
	scratch.Flags().StringVarP(&updateStatus, "status", "s", "", "")
	if err := scratch.Flags().Set("status", "completed"); err != nil {
		t.Fatalf("setting --status flag: %v", err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	runErr := updateCmd.RunE(scratch, []string{b.ID})

	os.Stdout = oldStdout
	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe write end: %v", err)
	}
	captured, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}
	if runErr != nil {
		t.Fatalf("updateCmd.RunE() error = %v", runErr)
	}

	if bytes.Contains(captured, []byte("{")) {
		t.Errorf("non-JSON update output looks like JSON: %q", captured)
	}
	want := "Updated " + b.ID
	if !bytes.Contains(captured, []byte(want)) {
		t.Errorf("captured = %q, want it to contain %q", captured, want)
	}
}

// batchUpdateFixtures adds an epic, a second task and a milestone next to
// setupUpdateTest's beans-test1.
func batchUpdateFixtures(t *testing.T) {
	t.Helper()
	for _, b := range []*item.Item{
		{ID: "beans-upe01", Type: "epic", Title: "Epic"},
		{ID: "beans-upt02", Type: "task", Title: "Task two"},
		{ID: "beans-upm01", Type: "milestone", Title: "Milestone"},
	} {
		b.Slug, b.Status = item.Slugify(b.Title), "todo"
		if err := core.Create(b); err != nil {
			t.Fatalf("core.Create(%s) error = %v", b.ID, err)
		}
	}
	t.Cleanup(func() { resetFlags(sharedTestRoot(t)) })
}

func diskItem(t *testing.T, id string) *item.Item {
	t.Helper()
	cached, err := core.Get(id)
	if err != nil {
		t.Fatalf("core.Get(%s) error = %v", id, err)
	}
	got, err := readItemFromDisk(t, cached)
	if err != nil {
		t.Fatalf("readBeanFromDisk(%s) error = %v", id, err)
	}
	return got
}

func TestUpdateCmdReparentsSeveralItems(t *testing.T) {
	setupUpdateTest(t)
	batchUpdateFixtures(t)

	stdout, _, err := runRootInDir(t, core.Root(), "update", "--parent", "beans-upe01", "beans-test1", "beans-upt02")
	if err != nil {
		t.Fatalf("update error = %v", err)
	}
	for _, id := range []string{"beans-test1", "beans-upt02"} {
		if !strings.Contains(stdout, "Updated "+id) {
			t.Errorf("stdout = %q, want it to contain %q", stdout, "Updated "+id)
		}
		if got := diskItem(t, id).Parent; got != "beans-upe01" {
			t.Errorf("%s parent = %q, want beans-upe01", id, got)
		}
	}
}

func TestUpdateCmdBatchWritesNothingOnUnknownID(t *testing.T) {
	setupUpdateTest(t)
	batchUpdateFixtures(t)

	_, _, err := runRootInDir(t, core.Root(), "update", "--parent", "beans-upe01", "beans-test1", "beans-nope9")
	if err == nil || !strings.Contains(err.Error(), "item not found: beans-nope9") {
		t.Fatalf("err = %v, want item not found: beans-nope9", err)
	}
	if got := diskItem(t, "beans-test1").Parent; got != "" {
		t.Errorf("beans-test1 parent = %q, want empty", got)
	}
}

func TestUpdateCmdBatchPreflightsParent(t *testing.T) {
	setupUpdateTest(t)
	batchUpdateFixtures(t)

	_, _, err := runRootInDir(t, core.Root(), "update", "--parent", "beans-upe01", "beans-test1", "beans-upm01")
	if err == nil || !strings.Contains(err.Error(), "beans-upm01") || !strings.Contains(err.Error(), "cannot have a parent") {
		t.Fatalf("err = %v, want beans-upm01 ... cannot have a parent", err)
	}
	if got := diskItem(t, "beans-test1").Parent; got != "" {
		t.Errorf("beans-test1 parent = %q, want empty", got)
	}
}

// childOfEpicFixtures puts beans-upt02 under beans-upe01, so that the epic can
// neither wait for its child nor block it (beans-o8bs).
func childOfEpicFixtures(t *testing.T) {
	t.Helper()
	setupUpdateTest(t)
	batchUpdateFixtures(t)
	child, err := core.Get("beans-upt02")
	if err != nil {
		t.Fatalf("core.Get(beans-upt02) error = %v", err)
	}
	child.Parent = "beans-upe01"
	if err := core.Update(child, nil); err != nil {
		t.Fatalf("core.Update(beans-upt02) error = %v", err)
	}
}

// A batch whose second item would close a cycle writes nothing, not even the
// first item: the blocking links are checked for every target before the first
// write, like the parent and the type.
func TestUpdateCmdBatchPreflightsBlockedBy(t *testing.T) {
	childOfEpicFixtures(t)

	_, _, err := runRootInDir(t, core.Root(), "update", "--blocked-by", "beans-upt02", "beans-test1", "beans-upe01")
	if err == nil || !strings.Contains(err.Error(), "beans-upe01") || !strings.Contains(err.Error(), "would create cycle") {
		t.Fatalf("err = %v, want beans-upe01 ... would create cycle", err)
	}
	if strings.Contains(err.Error(), "already written") {
		t.Errorf("err = %q, want a refusal before the first write", err)
	}
	if got := diskItem(t, "beans-test1").BlockedBy; len(got) != 0 {
		t.Errorf("beans-test1 blocked_by = %v, want none: the batch was refused as a whole", got)
	}
}

func TestUpdateCmdBatchPreflightsBlocking(t *testing.T) {
	childOfEpicFixtures(t)

	_, _, err := runRootInDir(t, core.Root(), "update", "--blocking", "beans-upe01", "beans-test1", "beans-upt02")
	if err == nil || !strings.Contains(err.Error(), "beans-upt02") || !strings.Contains(err.Error(), "would create cycle") {
		t.Fatalf("err = %v, want beans-upt02 ... would create cycle", err)
	}
	if strings.Contains(err.Error(), "already written") {
		t.Errorf("err = %q, want a refusal before the first write", err)
	}
	if got := diskItem(t, "beans-test1").Blocking; len(got) != 0 {
		t.Errorf("beans-test1 blocking = %v, want none: the batch was refused as a whole", got)
	}
}

func TestUpdateCmdIfMatchTakesOneID(t *testing.T) {
	setupUpdateTest(t)
	batchUpdateFixtures(t)

	_, _, err := runRootInDir(t, core.Root(), "update", "--if-match", "x", "--status", "draft", "beans-test1", "beans-upt02")
	if err == nil || !strings.Contains(err.Error(), "--if-match") {
		t.Fatalf("err = %v, want it to mention --if-match", err)
	}
	if got := diskItem(t, "beans-test1").Status; got != "todo" {
		t.Errorf("beans-test1 status = %q, want todo", got)
	}
}
