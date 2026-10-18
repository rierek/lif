package itemgraph

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/itemgraph/model"
)

// call is one hook call that a recorder saw.
type call struct {
	stage string // "before" or "after"
	m     Mutation
}

// recorder logs the calls of the before/after seam of a resolver, in call
// order. Unless a test sets beforeFn or afterFn, it answers (false, nil) and
// false: no hook task ran, nothing blocks.
type recorder struct {
	log      []call
	beforeFn func(Mutation) (ran bool, err error)
	afterFn  func(Mutation) (ran bool)
}

func newRecorder(r *CoreResolver) *recorder {
	rec := &recorder{}
	r.BeforeMutation = func(m Mutation) (bool, error) {
		rec.log = append(rec.log, call{"before", m})
		if rec.beforeFn != nil {
			return rec.beforeFn(m)
		}
		return false, nil
	}
	r.AfterMutation = func(m Mutation) bool {
		rec.log = append(rec.log, call{"after", m})
		if rec.afterFn != nil {
			return rec.afterFn(m)
		}
		return false
	}
	return rec
}

func (rec *recorder) stages() []string {
	var stages []string
	for _, c := range rec.log {
		stages = append(stages, c.stage)
	}
	return stages
}

// one returns the mutation of the only call of stage and fails when the stage
// was called not exactly once.
func (rec *recorder) one(t *testing.T, stage string) Mutation {
	t.Helper()
	var found []Mutation
	for _, c := range rec.log {
		if c.stage == stage {
			found = append(found, c.m)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s called %d times, want once (calls: %v)", stage, len(found), rec.stages())
	}
	return found[0]
}

// wantStages fails unless the recorder saw exactly these calls, in this order.
func wantStages(t *testing.T, rec *recorder, want ...string) {
	t.Helper()
	if got := rec.stages(); !slices.Equal(got, want) {
		t.Fatalf("hook calls = %v, want %v", got, want)
	}
}

func statusInput(status string) model.UpdateItemInput {
	return model.UpdateItemInput{Status: &status}
}

// fileOf returns the bytes of the item's file in the store.
func fileOf(t *testing.T, r *CoreResolver, id string) []byte {
	t.Helper()
	b, err := r.Core.Get(id)
	if err != nil {
		t.Fatalf("Get(%s): %v", id, err)
	}
	data, err := os.ReadFile(filepath.Join(r.Core.Root(), b.Path))
	if err != nil {
		t.Fatalf("reading file of %s: %v", id, err)
	}
	return data
}

// wantFile fails when the file of the item differs from the bytes taken earlier.
func wantFile(t *testing.T, r *CoreResolver, id string, want []byte) {
	t.Helper()
	if got := fileOf(t, r, id); !bytes.Equal(got, want) {
		t.Fatalf("item file of %s changed:\n%s\nwant:\n%s", id, got, want)
	}
}

// onDisk parses the file of the item as it is stored, not as the store holds it in memory.
func onDisk(t *testing.T, r *CoreResolver, id string) *item.Item {
	t.Helper()
	b, err := item.Parse(bytes.NewReader(fileOf(t, r, id)))
	if err != nil {
		t.Fatalf("parsing file of %s: %v", id, err)
	}
	return b
}

const gateBody = "written by the gate"

// writeAsGate writes the item through a second Core.Update, as a gate task
// that runs `lif update` does.
func writeAsGate(t *testing.T, r *CoreResolver, id string) {
	t.Helper()
	g, err := r.Core.Get(id)
	if err != nil {
		t.Fatalf("Get(%s): %v", id, err)
	}
	g.Body = gateBody
	if err := r.Core.Update(g, nil); err != nil {
		t.Fatalf("gate write: %v", err)
	}
}

func TestE01_CreateFiresAfterOnly(t *testing.T) {
	r := newTestResolver(t)
	createTyped(t, r, "beans-ep01", "epic", "")
	rec := newRecorder(r)

	parent := "beans-ep01"
	got, err := r.CreateItem(context.Background(), model.CreateItemInput{Title: "Child", Parent: &parent})
	if err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	wantStages(t, rec, "after")
	m := rec.one(t, "after")
	if m.Event != "created" {
		t.Errorf("Event = %q, want created", m.Event)
	}
	if m.Prev != nil {
		t.Errorf("Prev = %+v, want nil", m.Prev)
	}
	wantChanged(t, m.Changed, []string{"status", "type", "parent"})
	if m.Item == nil || m.Item.ID != got.ID {
		t.Errorf("Item = %+v, want the created item %s", m.Item, got.ID)
	}
}

func TestE02_UpdateFiresBeforeThenAfter(t *testing.T) {
	r := newTestResolver(t)
	createTyped(t, r, "beans-ev02", "task", "")
	rec := newRecorder(r)

	in := statusInput("in-progress")
	in.AddTags = []string{"needs-wt"}
	if _, err := r.UpdateItem(context.Background(), "beans-ev02", in); err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}

	wantStages(t, rec, "before", "after")
	for _, c := range rec.log {
		if c.m.Event != "updated" {
			t.Errorf("%s: Event = %q, want updated", c.stage, c.m.Event)
		}
		wantChanged(t, c.m.Changed, []string{"status", "tags"})
		if c.m.Prev == nil || c.m.Prev.Status != "todo" {
			t.Errorf("%s: Prev = %+v, want the item with status todo", c.stage, c.m.Prev)
		}
		if c.m.Item == nil || c.m.Item.Status != "in-progress" {
			t.Errorf("%s: Item = %+v, want the item with status in-progress", c.stage, c.m.Item)
		}
	}
}

func TestE03_ParentOnlyChange(t *testing.T) {
	ctx := context.Background()
	r := newTestResolver(t)
	createTyped(t, r, "beans-ep03", "epic", "")
	createTyped(t, r, "beans-tk03", "task", "")
	createTyped(t, r, "beans-tk04", "task", "")
	rec := newRecorder(r)

	parent := "beans-ep03"
	if _, err := r.UpdateItem(ctx, "beans-tk03", model.UpdateItemInput{Parent: &parent}); err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}
	if _, err := r.SetParent(ctx, "beans-tk04", &parent, nil); err != nil {
		t.Fatalf("SetParent: %v", err)
	}

	wantStages(t, rec, "before", "after", "before", "after")
	for i, c := range rec.log {
		wantChanged(t, c.m.Changed, []string{"parent"})
		if c.m.Event != "updated" {
			t.Errorf("call %d (%s): Event = %q, want updated", i, c.stage, c.m.Event)
		}
	}
}

func TestE04_UnwatchedChangesFireNothing(t *testing.T) {
	ctx := context.Background()
	r := newTestResolver(t)
	b := &item.Item{ID: "beans-ev04", Slug: item.Slugify("Tagged"), Title: "Tagged", Status: "todo", Type: "task", Tags: []string{"a", "b"}}
	if err := r.Core.Create(b); err != nil {
		t.Fatalf("Create: %v", err)
	}
	rec := newRecorder(r)

	title := "Retitled"
	if _, err := r.UpdateItem(ctx, b.ID, model.UpdateItemInput{Title: &title}); err != nil {
		t.Fatalf("title only: %v", err)
	}
	if _, err := r.UpdateItem(ctx, b.ID, model.UpdateItemInput{Tags: []string{"b", "a"}}); err != nil {
		t.Fatalf("tag reorder: %v", err)
	}

	wantStages(t, rec)
}

func TestE05_BeforeSeesExtraOpsOfTheSameCall(t *testing.T) {
	r := newTestResolver(t)
	createTyped(t, r, "beans-ev05", "task", "")
	rec := newRecorder(r)

	extras := itemcore.WithExtraOps(map[string]any{"target_entry": "x"}, nil)
	if _, err := r.UpdateItem(context.Background(), "beans-ev05", model.UpdateItemInput{}, extras); err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}

	m := rec.one(t, "before")
	if m.Item.Extra["target_entry"] != "x" {
		t.Errorf("Item.Extra[target_entry] = %v, want x", m.Item.Extra["target_entry"])
	}
	wantChanged(t, m.Changed, []string{"target_entry"})
	if _, has := m.Prev.Extra["target_entry"]; has {
		t.Errorf("Prev.Extra = %v, want no target_entry", m.Prev.Extra)
	}
}

func TestE06_BeforeErrorAbortsWriteUnchanged(t *testing.T) {
	r := newTestResolver(t)
	createTyped(t, r, "beans-ev06", "task", "")
	disk := fileOf(t, r, "beans-ev06")
	rec := newRecorder(r)
	blocked := errors.New("gate says no")
	rec.beforeFn = func(Mutation) (bool, error) { return true, blocked }

	_, err := r.UpdateItem(context.Background(), "beans-ev06", statusInput("in-progress"))

	if err != blocked {
		t.Fatalf("error = %v, want the gate's error unchanged", err)
	}
	wantStages(t, rec, "before")
	wantFile(t, r, "beans-ev06", disk)
}

func TestE07_PolicyViolationFiresNoAfter(t *testing.T) {
	r := newTestResolver(t)
	r.Core.Config().Items.RequireFieldsOn = map[string][]string{"completed": {"commit"}}
	createTyped(t, r, "beans-ev07", "task", "")
	disk := fileOf(t, r, "beans-ev07")
	afterCalls := 0
	r.AfterMutation = func(Mutation) bool { afterCalls++; return false }

	_, err := r.UpdateItem(context.Background(), "beans-ev07", statusInput("completed"))

	var violation *itemcore.PolicyViolationError
	if !errors.As(err, &violation) {
		t.Fatalf("error = %v, want PolicyViolationError", err)
	}
	if afterCalls != 0 {
		t.Errorf("AfterMutation called %d times, want 0", afterCalls)
	}
	wantFile(t, r, "beans-ev07", disk)
}

func TestE08_GateThatWroteMakesTheWriteConflict(t *testing.T) {
	r := newTestResolver(t)
	createTyped(t, r, "beans-ev08", "task", "")
	rec := newRecorder(r)
	rec.beforeFn = func(m Mutation) (bool, error) {
		writeAsGate(t, r, m.Item.ID)
		return true, nil
	}

	_, err := r.UpdateItem(context.Background(), "beans-ev08", statusInput("in-progress"))

	var mismatch *itemcore.ETagMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("error = %v, want ETagMismatchError", err)
	}
	wantStages(t, rec, "before")
	disk := onDisk(t, r, "beans-ev08")
	if !strings.Contains(disk.Body, gateBody) {
		t.Errorf("the gate's own write is gone: body = %q", disk.Body)
	}
	if disk.Status != "todo" {
		t.Errorf("status on disk = %q, want todo: the outer change must be absent", disk.Status)
	}
}

func TestE09_RequireIfMatchStopsBeforeTheGate(t *testing.T) {
	r := newTestResolver(t)
	r.Core.Config().Items.RequireIfMatch = true
	createTyped(t, r, "beans-ev09", "task", "")
	disk := fileOf(t, r, "beans-ev09")
	rec := newRecorder(r)

	_, err := r.UpdateItem(context.Background(), "beans-ev09", statusInput("in-progress"))

	var required *itemcore.ETagRequiredError
	if !errors.As(err, &required) {
		t.Fatalf("error = %v, want ETagRequiredError", err)
	}
	wantStages(t, rec)
	wantFile(t, r, "beans-ev09", disk)
}

func TestE10_GateThatDidNotRunLeavesTheWriteUnchecked(t *testing.T) {
	r := newTestResolver(t)
	createTyped(t, r, "beans-ev10", "task", "")
	rec := newRecorder(r)
	rec.beforeFn = func(m Mutation) (bool, error) {
		writeAsGate(t, r, m.Item.ID)
		return false, nil
	}

	if _, err := r.UpdateItem(context.Background(), "beans-ev10", statusInput("in-progress")); err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}

	wantStages(t, rec, "before", "after")
	if disk := onDisk(t, r, "beans-ev10"); disk.Status != "in-progress" {
		t.Errorf("status on disk = %q, want in-progress", disk.Status)
	}
}

func TestE11_PolicyViolationStopsBeforeTheGate(t *testing.T) {
	r := newTestResolver(t)
	r.Core.Config().Items.RequireFieldsOn = map[string][]string{"completed": {"commit"}}
	createTyped(t, r, "beans-ev11", "task", "")
	disk := fileOf(t, r, "beans-ev11")
	rec := newRecorder(r)

	_, err := r.UpdateItem(context.Background(), "beans-ev11", statusInput("completed"))

	var violation *itemcore.PolicyViolationError
	if !errors.As(err, &violation) {
		t.Fatalf("error = %v, want PolicyViolationError", err)
	}
	if violation.ItemID != "beans-ev11" || violation.Status != "completed" || !slices.Equal(violation.Missing, []string{"commit"}) {
		t.Errorf("violation = %+v, want item beans-ev11, status completed, missing [commit]", violation)
	}
	wantStages(t, rec)
	wantFile(t, r, "beans-ev11", disk)
}

func TestE12_FailedRenameStillFiresAfterOnce(t *testing.T) {
	r := newTestResolver(t)
	b := &item.Item{ID: "beans-ev12", Slug: item.Slugify("Old title"), Title: "Old title", Status: "todo", Type: "task"}
	if err := r.Core.Create(b); err != nil {
		t.Fatalf("Create: %v", err)
	}
	oldPath := item.BuildFilename(b.ID, item.Slugify("Old title"))
	// A directory under the target file name makes the rename fail.
	target := filepath.Join(r.Core.Root(), item.BuildFilename(b.ID, item.Slugify("New title")))
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	rec := newRecorder(r)

	// The title alone is no watched change: the status change gives the hook its event.
	title, status := "New title", "in-progress"
	_, err := r.UpdateItem(context.Background(), b.ID, model.UpdateItemInput{Title: &title, Status: &status})

	var renameErr *os.LinkError
	if !errors.As(err, &renameErr) {
		t.Fatalf("error = %v, want the rename error", err)
	}
	m := rec.one(t, "after")
	if m.Item.Path != oldPath {
		t.Errorf("Item.Path = %q, want the old path %q", m.Item.Path, oldPath)
	}
	wantChanged(t, m.Changed, []string{"status"})
	if disk := onDisk(t, r, b.ID); disk.Status != "in-progress" {
		t.Errorf("status on disk = %q, want in-progress: the write is on disk", disk.Status)
	}
}

// After a successful rename the hook sees the item under its new path.
func TestUpdateItemAfterHookSeesRenamedPath(t *testing.T) {
	r := newTestResolver(t)
	b := &item.Item{ID: "beans-rn01", Slug: item.Slugify("Old title"), Title: "Old title", Status: "todo", Type: "task"}
	if err := r.Core.Create(b); err != nil {
		t.Fatalf("Create: %v", err)
	}
	rec := newRecorder(r)

	title, status := "New title", "in-progress"
	if _, err := r.UpdateItem(context.Background(), b.ID, model.UpdateItemInput{Title: &title, Status: &status}); err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}

	m := rec.one(t, "after")
	if want := item.BuildFilename(b.ID, item.Slugify("New title")); m.Item.Path != want {
		t.Errorf("Item.Path = %q, want %q", m.Item.Path, want)
	}
}

// A created item carries the extra fields of the same call, and they count as
// watched fields of its created event.
func TestCreateItemAfterHookSeesExtraOps(t *testing.T) {
	r := newTestResolver(t)
	rec := newRecorder(r)

	extras := itemcore.WithExtraOps(map[string]any{"target_entry": "x"}, nil)
	if _, err := r.CreateItem(context.Background(), model.CreateItemInput{Title: "With extras"}, extras); err != nil {
		t.Fatalf("CreateItem: %v", err)
	}

	m := rec.one(t, "after")
	wantChanged(t, m.Changed, []string{"status", "type", "target_entry"})
	if m.Item.Extra["target_entry"] != "x" {
		t.Errorf("Item.Extra[target_entry] = %v, want x", m.Item.Extra["target_entry"])
	}
}

// The status policy counts the extra fields of the same call: a gate must run
// for `complete --commit`, which supplies the field the status demands.
func TestUpdateItemPolicyCountsExtraOpsOfTheSameCall(t *testing.T) {
	r := newTestResolver(t)
	r.Core.Config().Items.RequireFieldsOn = map[string][]string{"completed": {"commit"}}
	createTyped(t, r, "beans-pc01", "task", "")
	rec := newRecorder(r)

	extras := itemcore.WithExtraOps(map[string]any{"commit": "abc123"}, nil)
	if _, err := r.UpdateItem(context.Background(), "beans-pc01", statusInput("completed"), extras); err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}

	wantStages(t, rec, "before", "after")
	wantChanged(t, rec.one(t, "before").Changed, []string{"status", "commit"})
}

// An after hook that ran may have written the next target behind this
// process's back: the next write without If-Match starts from the file on
// disk, so it keeps the hook's edit and still lands.
func TestAfterHookThatRanRereadsLaterTargets(t *testing.T) {
	ctx := context.Background()
	r := newTestResolver(t)
	createTyped(t, r, "beans-ah01", "task", "")
	createTyped(t, r, "beans-ah02", "task", "")
	rec := newRecorder(r)
	rec.afterFn = func(Mutation) bool {
		// Write the file directly: the store of this process keeps the old state.
		b, err := r.Core.Get("beans-ah02")
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(r.Core.Root(), b.Path)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, "\nedited by the hook\n"...), 0o644); err != nil {
			t.Fatal(err)
		}
		return true
	}

	if _, err := r.UpdateItem(ctx, "beans-ah01", statusInput("in-progress")); err != nil {
		t.Fatalf("first target: %v", err)
	}
	if _, err := r.UpdateItem(ctx, "beans-ah02", statusInput("in-progress")); err != nil {
		t.Fatalf("second target: %v", err)
	}
	disk := onDisk(t, r, "beans-ah02")
	if disk.Status != "in-progress" {
		t.Errorf("status on disk = %q, want in-progress", disk.Status)
	}
	if !strings.Contains(disk.Body, "edited by the hook") {
		t.Errorf("body on disk = %q, lost the edit of the hook", disk.Body)
	}
}
