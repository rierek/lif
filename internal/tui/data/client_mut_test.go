package data

import (
	"errors"
	"strings"
	"testing"
)

// findBean is a small test helper -- every mutation test below needs to
// pull one specific bean (with its current ETag) out of a List() result.
func findBean(t *testing.T, beans []Bean, id string) Bean {
	t.Helper()
	for _, b := range beans {
		if b.ID == id {
			return b
		}
	}
	t.Fatalf("bean %s not found in List() result", id)
	return Bean{}
}

func TestSetStatusRoundtrip(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	task := findBean(t, beans, "tt-task")

	if err := client.SetStatus(task.ID, "completed", task.ETag); err != nil {
		t.Fatalf("SetStatus() error = %v", err)
	}

	after, err := client.List()
	if err != nil {
		t.Fatalf("List() after SetStatus error = %v", err)
	}
	updated := findBean(t, after, "tt-task")
	if updated.Status != "completed" {
		t.Errorf("Status = %q, want %q", updated.Status, "completed")
	}
}

func TestCreateReturnsNewBean(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	created, err := client.Create(CreateOpts{
		Title:  "New Feature Bean",
		Type:   "feature",
		Status: "todo",
		Tags:   []string{"alpha"},
		Body:   "Created by TestCreateReturnsNewBean.",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" {
		t.Error("Create() bean.ID is empty")
	}
	if created.ETag == "" {
		t.Error("Create() bean.ETag is empty")
	}
	if created.Title != "New Feature Bean" {
		t.Errorf("Create() bean.Title = %q, want %q", created.Title, "New Feature Bean")
	}
	if created.Type != "feature" {
		t.Errorf("Create() bean.Type = %q, want %q", created.Type, "feature")
	}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	found := findBean(t, beans, created.ID)
	if found.Title != "New Feature Bean" {
		t.Errorf("List() bean.Title = %q, want %q", found.Title, "New Feature Bean")
	}
}

func TestCreateTitleWithLeadingDash(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	const title = "- fix bug"
	created, err := client.Create(CreateOpts{Title: title, Type: "task"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Title != title {
		t.Errorf("Create() bean.Title = %q, want %q", created.Title, title)
	}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	found := findBean(t, beans, created.ID)
	if found.Title != title {
		t.Errorf("List() bean.Title = %q, want %q", found.Title, title)
	}
}

func TestValidationErrorContainingEtagMismatchIsNotConflict(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	task := findBean(t, beans, "tt-task")

	// The literal string "etag mismatch" here is user-supplied (an invalid
	// --type value), not a real conflict -- the CLI reports this as a
	// VALIDATION_ERROR, not CONFLICT. A naive strings.Contains(err.Error(),
	// "etag mismatch") check would misclassify this as ErrConflict (B02).
	err = client.SetType(task.ID, "etag mismatch", task.ETag)
	if err == nil {
		t.Fatal("SetType() with invalid type: error = nil, want error")
	}
	if errors.Is(err, ErrConflict) {
		t.Errorf("SetType() error = %v, want NOT errors.Is(err, ErrConflict)", err)
	}
}

func TestConflictOnStaleETag(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	task := findBean(t, beans, "tt-task")
	staleETag := task.ETag

	// Change the bean externally (a second, successful update), which
	// rotates the ETag out from under the stale copy above.
	if err := client.SetStatus(task.ID, "in-progress", staleETag); err != nil {
		t.Fatalf("first SetStatus() error = %v", err)
	}

	err = client.SetPriority(task.ID, "high", staleETag)
	if err == nil {
		t.Fatal("SetPriority() with stale ETag: error = nil, want ErrConflict")
	}
	if !errors.Is(err, ErrConflict) {
		t.Errorf("SetPriority() error = %v, want errors.Is(err, ErrConflict)", err)
	}
}

func TestAppendBodyAddsSection(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	task := findBean(t, beans, "tt-task")

	const appended = "## Notes\nAppended via TestAppendBodyAddsSection."
	if err := client.AppendBody(task.ID, "\n"+appended, task.ETag); err != nil {
		t.Fatalf("AppendBody() error = %v", err)
	}

	after, err := client.List()
	if err != nil {
		t.Fatalf("List() after AppendBody error = %v", err)
	}
	updated := findBean(t, after, "tt-task")
	if !strings.Contains(updated.Body, "Appended via TestAppendBodyAddsSection.") {
		t.Errorf("Body = %q, want it to contain the appended section", updated.Body)
	}
}

// TestSetBodyReplacesWholeBody guards SetBody's FULL-replace contract (E3
// Task 5, bean bt-sl45, `--body` -- verified against beans 0.4.2 --help: "New
// body"): the old fixture body ("Task fixture body.") must be GONE
// afterwards, not merely have the new text appended alongside it --
// distinguishing this from AppendBody above (--body-append, additive).
func TestSetBodyReplacesWholeBody(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	task := findBean(t, beans, "tt-task")

	const newBody = "Replaced via TestSetBodyReplacesWholeBody."
	if err := client.SetBody(task.ID, newBody, task.ETag); err != nil {
		t.Fatalf("SetBody() error = %v", err)
	}

	after, err := client.List()
	if err != nil {
		t.Fatalf("List() after SetBody error = %v", err)
	}
	updated := findBean(t, after, "tt-task")
	if !strings.Contains(updated.Body, newBody) {
		t.Errorf("Body = %q, want it to contain %q", updated.Body, newBody)
	}
	if strings.Contains(updated.Body, "Task fixture body.") {
		t.Errorf("Body = %q, still contains the OLD fixture body -- SetBody must be a FULL replace, not append", updated.Body)
	}
}

// TestSetTagsAddsAndRemovesInOneCall guards the E3 Task 2 design decision
// (bean bt-8v69): a SINGLE SetTags call combining --tag/--remove-tag adds
// AND removes tags against ONE etag.
//
// Uses tt-task, not tt-epic: testrepo_test.go's newTestRepo doc comment
// records a real beans 0.4.2 CLI bug where a bean with HAND-AUTHORED
// "tags:" frontmatter reports an ETag from list/show that diverges from
// update --if-match's internal conflict check, until the file has been
// rewritten once by beans itself. tt-task ships with no hand-authored tags
// (deliberately, per that same doc comment), so a first AddTag "seed" call
// establishes a tag to remove without ever touching the divergent path --
// the combined SetTags call below is then the thing under test.
func TestSetTagsAddsAndRemovesInOneCall(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	task := findBean(t, beans, "tt-task")

	if err := client.AddTag(task.ID, "seed", task.ETag); err != nil {
		t.Fatalf("seed AddTag() error = %v", err)
	}

	beans, err = client.List()
	if err != nil {
		t.Fatalf("List() after seed error = %v", err)
	}
	task = findBean(t, beans, "tt-task")

	if err := client.SetTags(task.ID, []string{"alpha", "beta"}, []string{"seed"}, task.ETag); err != nil {
		t.Fatalf("SetTags() error = %v", err)
	}

	after, err := client.List()
	if err != nil {
		t.Fatalf("List() after SetTags error = %v", err)
	}
	updated := findBean(t, after, "tt-task")

	want := map[string]bool{"alpha": true, "beta": true}
	for _, tg := range updated.Tags {
		if tg == "seed" {
			t.Fatal("SetTags() left the removed tag \"seed\" in place")
		}
		if !want[tg] {
			t.Errorf("SetTags() left unexpected tag %q, want only %v", tg, want)
			continue
		}
		delete(want, tg)
	}
	if len(want) != 0 {
		t.Errorf("SetTags() missing tags %v, got %v", want, updated.Tags)
	}
}

// TestAddRemoveBlockingRoundtrip guards the E3 Task 3 addition (bean
// bt-p1uz): AddBlocking/RemoveBlocking directly mutate the CALLING bean's
// OWN Blocking field (design decision g), not blocked_by on the target.
func TestAddRemoveBlockingRoundtrip(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	// Not tt-epic: tt-task is its child, and a child that blocks its parent can
	// never be released (beans-o8bs), so lif refuses that link.
	target, err := client.Create(CreateOpts{Title: "Blocking Target", Type: "task"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	task := findBean(t, beans, "tt-task")

	if err := client.AddBlocking(task.ID, target.ID, task.ETag); err != nil {
		t.Fatalf("AddBlocking() error = %v", err)
	}

	after, err := client.List()
	if err != nil {
		t.Fatalf("List() after AddBlocking error = %v", err)
	}
	updated := findBean(t, after, "tt-task")
	if len(updated.Blocking) != 1 || updated.Blocking[0] != target.ID {
		t.Fatalf("Blocking after AddBlocking() = %v, want [%s]", updated.Blocking, target.ID)
	}

	if err := client.RemoveBlocking(task.ID, target.ID, updated.ETag); err != nil {
		t.Fatalf("RemoveBlocking() error = %v", err)
	}

	after2, err := client.List()
	if err != nil {
		t.Fatalf("List() after RemoveBlocking error = %v", err)
	}
	updated2 := findBean(t, after2, "tt-task")
	if len(updated2.Blocking) != 0 {
		t.Fatalf("Blocking after RemoveBlocking() = %v, want empty", updated2.Blocking)
	}
}

// TestSetBlockingAddsAndRemovesInOneCall guards the E3 Task 3 design
// decision (bean bt-p1uz, mirrors TestSetTagsAddsAndRemovesInOneCall): a
// SINGLE SetBlocking call combining --blocking/--remove-blocking adds AND
// removes blocking targets against ONE etag.
//
// Uses a freshly-created bean as the "add" target, not the fixture's
// tt-mlst: tt-mlst's own fixture frontmatter already carries
// "blocking: [tt-task]" (testrepo_test.go), so tt-task additionally blocking
// tt-mlst would close a real two-bean cycle (tt-mlst -> tt-task -> tt-mlst)
// -- the beans 0.4.2 CLI rejects that server-side with a VALIDATION_ERROR
// (empirically verified), independent of this task's own design decision
// that the Blocking-Picker itself does no client-side cycle exclusion
// (design decision g: that decision is about the TUI not PRE-filtering, not
// about the CLI never validating).
func TestSetBlockingAddsAndRemovesInOneCall(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	target, err := client.Create(CreateOpts{Title: "Blocking Target", Type: "task"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// The seed is a fresh bean too: tt-epic is the parent of tt-task, and a child
	// that blocks its parent is refused (beans-o8bs).
	seed, err := client.Create(CreateOpts{Title: "Seed Target", Type: "task"})
	if err != nil {
		t.Fatalf("Create() seed error = %v", err)
	}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	task := findBean(t, beans, "tt-task")

	if err := client.AddBlocking(task.ID, seed.ID, task.ETag); err != nil {
		t.Fatalf("seed AddBlocking() error = %v", err)
	}

	beans, err = client.List()
	if err != nil {
		t.Fatalf("List() after seed error = %v", err)
	}
	task = findBean(t, beans, "tt-task")

	if err := client.SetBlocking(task.ID, []string{target.ID}, []string{seed.ID}, task.ETag); err != nil {
		t.Fatalf("SetBlocking() error = %v", err)
	}

	after, err := client.List()
	if err != nil {
		t.Fatalf("List() after SetBlocking error = %v", err)
	}
	updated := findBean(t, after, "tt-task")

	want := map[string]bool{target.ID: true}
	for _, id := range updated.Blocking {
		if id == seed.ID {
			t.Fatalf("SetBlocking() left the removed blocking target %q in place", seed.ID)
		}
		if !want[id] {
			t.Errorf("SetBlocking() left unexpected blocking target %q, want only %v", id, want)
			continue
		}
		delete(want, id)
	}
	if len(want) != 0 {
		t.Errorf("SetBlocking() missing blocking targets %v, got %v", want, updated.Blocking)
	}
}

// TestUpdateWholeSendsOnlyChangedFields guards data.Client.UpdateWhole (D01,
// design-spec.md §15 PF-17, bean bt-z4b1): the whole-bean $EDITOR round-trip
// fires exactly ONE `beans update` call carrying only the flags for fields
// that actually changed -- mirrors SetTags'/SetBlocking's own
// single-etag-no-cascade rationale, generalized to every editable field at
// once. The "empty diff" subtest is the Akzeptanz-Checkliste's explicit
// "No-op-Save -> kein CLI-Call" claim: RepoDir points at a directory with NO
// beans repo at all, so ANY shelled-out `beans` invocation would fail --
// UpdateWhole must still return nil.
func TestUpdateWholeSendsOnlyChangedFields(t *testing.T) {
	t.Run("title only", func(t *testing.T) {
		repo := newTestRepo(t)
		client := &Client{RepoDir: repo}
		beans, err := client.List()
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		task := findBean(t, beans, "tt-task")

		title := "Renamed Title"
		if err := client.UpdateWhole(task.ID, WholeEditDiff{Title: &title}, task.ETag); err != nil {
			t.Fatalf("UpdateWhole() error = %v", err)
		}

		after, err := client.List()
		if err != nil {
			t.Fatalf("List() after UpdateWhole error = %v", err)
		}
		updated := findBean(t, after, "tt-task")
		if updated.Title != title {
			t.Fatalf("Title = %q, want %q", updated.Title, title)
		}
		if updated.Status != task.Status || updated.Type != task.Type || updated.Priority != task.Priority {
			t.Fatalf("UpdateWhole(title-only) touched other fields: %+v", updated)
		}
	})

	t.Run("tags add and remove combined", func(t *testing.T) {
		repo := newTestRepo(t)
		client := &Client{RepoDir: repo}

		beans, err := client.List()
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		task := findBean(t, beans, "tt-task")
		if err := client.AddTag(task.ID, "seed", task.ETag); err != nil {
			t.Fatalf("seed AddTag() error = %v", err)
		}

		beans, err = client.List()
		if err != nil {
			t.Fatalf("List() after seed error = %v", err)
		}
		task = findBean(t, beans, "tt-task")

		diff := WholeEditDiff{TagsAdd: []string{"alpha", "beta"}, TagsRemove: []string{"seed"}}
		if err := client.UpdateWhole(task.ID, diff, task.ETag); err != nil {
			t.Fatalf("UpdateWhole() error = %v", err)
		}

		after, err := client.List()
		if err != nil {
			t.Fatalf("List() after UpdateWhole error = %v", err)
		}
		updated := findBean(t, after, "tt-task")
		want := map[string]bool{"alpha": true, "beta": true}
		for _, tg := range updated.Tags {
			if tg == "seed" {
				t.Fatal("UpdateWhole() left the removed tag \"seed\" in place")
			}
			if !want[tg] {
				t.Errorf("UpdateWhole() left unexpected tag %q, want only %v", tg, want)
				continue
			}
			delete(want, tg)
		}
		if len(want) != 0 {
			t.Errorf("UpdateWhole() missing tags %v, got %v", want, updated.Tags)
		}
	})

	t.Run("parent clear", func(t *testing.T) {
		repo := newTestRepo(t)
		client := &Client{RepoDir: repo}

		beans, err := client.List()
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		task := findBean(t, beans, "tt-task") // fixture: parent: tt-epic

		diff := WholeEditDiff{ParentChanged: true, Parent: ""}
		if err := client.UpdateWhole(task.ID, diff, task.ETag); err != nil {
			t.Fatalf("UpdateWhole() error = %v", err)
		}

		after, err := client.List()
		if err != nil {
			t.Fatalf("List() after UpdateWhole error = %v", err)
		}
		updated := findBean(t, after, "tt-task")
		if updated.Parent != "" {
			t.Fatalf("Parent = %q, want empty (--remove-parent)", updated.Parent)
		}
	})

	t.Run("body only", func(t *testing.T) {
		repo := newTestRepo(t)
		client := &Client{RepoDir: repo}

		beans, err := client.List()
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		task := findBean(t, beans, "tt-task")

		body := "Replaced body text."
		if err := client.UpdateWhole(task.ID, WholeEditDiff{Body: &body}, task.ETag); err != nil {
			t.Fatalf("UpdateWhole() error = %v", err)
		}

		after, err := client.List()
		if err != nil {
			t.Fatalf("List() after UpdateWhole error = %v", err)
		}
		updated := findBean(t, after, "tt-task")
		// The CLI's own JSON "body" field always carries a leading "\n"
		// (the blank line between the frontmatter's closing "---" and the
		// body text on disk) -- verified against every OTHER body-bearing
		// fixture in this package (e.g. testrepo_test.go's own bodies).
		if updated.Body != "\n"+body {
			t.Fatalf("Body = %q, want %q", updated.Body, "\n"+body)
		}
	})

	t.Run("empty diff fires no CLI call", func(t *testing.T) {
		client := &Client{RepoDir: t.TempDir()}
		if err := client.UpdateWhole("nonexistent-id", WholeEditDiff{}, "some-etag"); err != nil {
			t.Fatalf("UpdateWhole(empty diff) error = %v, want nil (no CLI call fired at all)", err)
		}
	})
}

// TestAddBlockingCyclePinsValidationErrorShape closes the E3-T3-Review
// PFLICHT finding carried into bean bt-ppzb/E3 Task 6 ("Blocking-Zyklus via
// CLI -> VALIDATION_ERROR-Shape-Test gepinnt, bisher nur Kommentar"):
// TestSetBlockingAddsAndRemovesInOneCall's own doc comment above already
// CLAIMS the beans 0.4.2 CLI rejects a two-bean blocking cycle server-side
// with a VALIDATION_ERROR "(empirically verified)" -- but until now nothing
// actually PINNED that shape with an assertion, only a comment. newTestRepo's
// fixture already has tt-mlst blocking tt-task (testrepo_test.go);
// AddBlocking(tt-task, tt-mlst, ...) closes the cycle the OTHER direction and
// must fail -- specifically with code VALIDATION_ERROR (classifyError's
// non-CONFLICT envelope branch, mutations.go), NOT errors.Is(_, ErrConflict):
// a cycle rejection is a data-integrity rule, not an optimistic-lock race.
// Verified empirically against beans 0.4.2's actual JSON envelope:
// {"success":false,"error":"adding blocking relationship would create
// cycle: [...]","code":"VALIDATION_ERROR"}.
func TestAddBlockingCyclePinsValidationErrorShape(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	task := findBean(t, beans, "tt-task")

	err = client.AddBlocking(task.ID, "tt-mlst", task.ETag)
	if err == nil {
		t.Fatal("AddBlocking() closing a 2-bean cycle: error = nil, want a VALIDATION_ERROR rejection")
	}
	if errors.Is(err, ErrConflict) {
		t.Fatalf("AddBlocking() cycle rejection classified as ErrConflict: %v, want a NON-conflict error (a cycle is a data-integrity rule, not a stale-etag race)", err)
	}
	if !strings.Contains(err.Error(), "VALIDATION_ERROR") {
		t.Fatalf("AddBlocking() cycle rejection error = %v, want it to carry the VALIDATION_ERROR code (classifyError's non-CONFLICT envelope branch)", err)
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("AddBlocking() cycle rejection error = %v, want it to mention the cycle (beans CLI's own diagnostic text)", err)
	}
}

// TestDeleteClearsFormerChildrensParentField pins an ERRATUM discovered
// during E3 Task 6's tmux smoke test (bean bt-ppzb, corrects
// epic-E3-plan.md's original assumption -- see Delete's own doc-stamp above
// and box_confirm_delete.go's matching doc-stamp for the full story):
// deleting tt-mlst (newTestRepo's fixture milestone, parent of tt-epic) must
// NOT leave tt-epic with a dangling `parent: tt-mlst` reference -- the real
// beans 0.4.2 CLI clears the field outright, turning tt-epic into an
// ordinary parentless bean, not a "(orphaned)"-bucket orphan.
func TestDeleteClearsFormerChildrensParentField(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	if err := client.Delete("tt-mlst"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	child := findBean(t, beans, "tt-epic")
	if child.Parent != "" {
		t.Fatalf("tt-epic.Parent = %q after deleting tt-mlst, want \"\" (beans clears the dangling reference, does not leave it orphaned)", child.Parent)
	}
}

// TestDeleteClearsOtherBeansBlockedByReference pins the Q01 empirical
// finding from E3-T7's review-findings sweep (E3-T6-Review PFLICHT, bean
// bt-qzwt): deleting a bean that OTHER beans reference via `blocked_by`
// silently clears that reference too -- the exact same "beans actively
// rewrites the referencing file" behavior TestDeleteClearsFormerChildrensPar
// entField already pins for `parent`, just for a different link family.
// Verified empirically in an isolated scratch repo (two fresh beans, A
// `--blocked-by` B, delete B, `cat` A's frontmatter -- both directions,
// `blocked_by` and `blocking`, mirror this test's sibling
// TestDeleteClearsOtherBeansBlockingReference below) before this test was
// written; `beans check` reports zero link issues afterwards, and the CLI
// prints no warning of its own -- box_confirm_delete.go's deleteBox is the
// ONLY place the PO ever sees this coming.
//
// newTestRepo's fixture already carries this exact relationship: tt-mlst
// has `blocking: [tt-task]`, tt-task has `blocked_by: [tt-mlst]` -- no
// fixture change needed, deleting tt-mlst is the same act
// TestDeleteClearsFormerChildrensParentField already exercises for the
// parent side (tt-mlst is also tt-epic's parent).
func TestDeleteClearsOtherBeansBlockedByReference(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	if err := client.Delete("tt-mlst"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	task := findBean(t, beans, "tt-task")
	if len(task.BlockedBy) != 0 {
		t.Fatalf("tt-task.BlockedBy = %v after deleting tt-mlst, want empty (Q01 ERRATUM: beans delete silently clears blocked_by references held by OTHER beans, same as parent)", task.BlockedBy)
	}
}

// TestDeleteClearsOtherBeansBlockingReference is
// TestDeleteClearsOtherBeansBlockedByReference's mirror for the other link
// direction: newTestRepo's tt-mlst carries `blocking: [tt-task]` -- deleting
// tt-task (the REFERENCED bean this time, not the referencing one) must
// silently clear tt-mlst's `blocking` entry too.
func TestDeleteClearsOtherBeansBlockingReference(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	if err := client.Delete("tt-task"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	milestone := findBean(t, beans, "tt-mlst")
	if len(milestone.Blocking) != 0 {
		t.Fatalf("tt-mlst.Blocking = %v after deleting tt-task, want empty (Q01 ERRATUM: beans delete silently clears blocking references held by OTHER beans, same as parent)", milestone.Blocking)
	}
}

// TestPassReviewSetsCompletedAndRemovesTag guards the E4 Task 4 design
// decision (bean bt-yy6w, design decision d, design-spec.md §5's Pass row):
// a SINGLE PassReview call combining --status/--remove-tag sets Status to
// "completed" AND removes the "to-review" tag against ONE etag.
//
// Seeds "to-review" via AddTag first, then re-Lists for a fresh ETag before
// calling PassReview -- same divergent-tags-ETag workaround
// TestSetTagsAddsAndRemovesInOneCall documents (newTestRepo's own doc
// comment): a bean whose on-disk frontmatter carries a hand-authored "tags:"
// block reports an ETag from list/show that diverges from update
// --if-match's internal conflict check until beans itself has rewritten the
// file once.
func TestPassReviewSetsCompletedAndRemovesTag(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	task := findBean(t, beans, "tt-task")

	if err := client.AddTag(task.ID, "to-review", task.ETag); err != nil {
		t.Fatalf("seed AddTag() error = %v", err)
	}

	beans, err = client.List()
	if err != nil {
		t.Fatalf("List() after seed error = %v", err)
	}
	task = findBean(t, beans, "tt-task")

	if err := client.PassReview(task.ID, task.ETag); err != nil {
		t.Fatalf("PassReview() error = %v", err)
	}

	after, err := client.List()
	if err != nil {
		t.Fatalf("List() after PassReview error = %v", err)
	}
	updated := findBean(t, after, "tt-task")
	if updated.Status != "completed" {
		t.Errorf("Status = %q, want %q", updated.Status, "completed")
	}
	for _, tg := range updated.Tags {
		if tg == "to-review" {
			t.Fatalf("PassReview() left the \"to-review\" tag in place: %v", updated.Tags)
		}
	}
}

// TestRejectReviewSwapsTagAndAppendsSection guards the E4 Task 4 design
// decision (bean bt-yy6w, design decision d, design-spec.md §5's Reject
// row): a SINGLE RejectReview call combining --remove-tag/--tag/
// --body-append swaps "to-review" -> "rework" AND appends a dated "##
// Review <date>" section to the body, against ONE etag.
func TestRejectReviewSwapsTagAndAppendsSection(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	task := findBean(t, beans, "tt-task")

	if err := client.AddTag(task.ID, "to-review", task.ETag); err != nil {
		t.Fatalf("seed AddTag() error = %v", err)
	}

	beans, err = client.List()
	if err != nil {
		t.Fatalf("List() after seed error = %v", err)
	}
	task = findBean(t, beans, "tt-task")

	const comment = "bitte X korrigieren"
	if err := client.RejectReview(task.ID, comment, "2026-07-15", task.ETag); err != nil {
		t.Fatalf("RejectReview() error = %v", err)
	}

	after, err := client.List()
	if err != nil {
		t.Fatalf("List() after RejectReview error = %v", err)
	}
	updated := findBean(t, after, "tt-task")

	foundRework, foundToReview := false, false
	for _, tg := range updated.Tags {
		if tg == "rework" {
			foundRework = true
		}
		if tg == "to-review" {
			foundToReview = true
		}
	}
	if !foundRework {
		t.Errorf("Tags = %v, want it to contain %q", updated.Tags, "rework")
	}
	if foundToReview {
		t.Errorf("Tags = %v, still contains \"to-review\" -- RejectReview must swap, not just add", updated.Tags)
	}

	// List() already carries --full (client.go), so updated.Body is
	// populated -- no separate Show() round-trip needed. ERRATUM vs. the
	// plan's literal assumption (epic-E4-plan.md »Task 4« Step 1): the real
	// beans 0.4.2 CLI trims exactly ONE trailing newline off the on-disk
	// body when serializing it back via `list --json --full` (empirically
	// observed -- the appended section's own trailing "\n" collapses into
	// the file's EOF normalization), so the section text itself is asserted
	// via Contains, not a "...\n"-suffixed string -- same convention
	// TestAppendBodyAddsSection above already uses for the identical
	// reason.
	wantSection := "## Review 2026-07-15\n\n" + comment
	if !strings.Contains(updated.Body, wantSection) {
		t.Errorf("Body = %q, want it to contain %q", updated.Body, wantSection)
	}
}

// TestRejectReviewCommentSpecialCharactersSurviveArgv (I01, E4-T4-Review
// PFLICHT, closed in T5, bean bt-v7ti) pins the "argv-Garantie" client.go's
// run already provides structurally (exec.Command("beans", args...) --
// Go never invokes a shell for this, so there is no shell-metacharacter
// interpretation to guard against by construction) as an EMPIRICAL,
// end-to-end regression instead of leaving it an unverified assumption:
// double quotes, a backtick, an apostrophe, an embedded (non-trailing)
// newline, and umlauts all survive comment -> CLI invocation -> on-disk
// bean file -> List() round-trip byte-for-byte. Same fixture/assertion shape
// as TestRejectReviewSwapsTagAndAppendsSection just above, just with a
// deliberately hostile comment string instead of a plain one.
func TestRejectReviewCommentSpecialCharactersSurviveArgv(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	task := findBean(t, beans, "tt-task")

	if err := client.AddTag(task.ID, "to-review", task.ETag); err != nil {
		t.Fatalf("seed AddTag() error = %v", err)
	}

	beans, err = client.List()
	if err != nil {
		t.Fatalf("List() after seed error = %v", err)
	}
	task = findBean(t, beans, "tt-task")

	const comment = "Bitte \"Grenzfälle\" prüfen: `foo` != 'bar'\nZeile zwei: Änderung nötig, äöüß"
	if err := client.RejectReview(task.ID, comment, "2026-07-15", task.ETag); err != nil {
		t.Fatalf("RejectReview() error = %v", err)
	}

	after, err := client.List()
	if err != nil {
		t.Fatalf("List() after RejectReview error = %v", err)
	}
	updated := findBean(t, after, "tt-task")

	// Same trailing-newline-trim caveat as TestRejectReviewSwapsTagAndAppendsSection
	// above -- asserted via Contains against a "\n"-suffix-free string.
	wantSection := "## Review 2026-07-15\n\n" + comment
	if !strings.Contains(updated.Body, wantSection) {
		t.Errorf("Body = %q, want it to contain %q (argv-Garantie: quotes/backtick/apostrophe/umlaut/embedded newline byte-for-byte)", updated.Body, wantSection)
	}
}

// TestPassReviewConflictOnStaleEtag mirrors TestConflictOnStaleETag: a
// PassReview call against an etag already rotated out from under it by an
// earlier successful update must fail errors.Is(err, ErrConflict).
func TestPassReviewConflictOnStaleEtag(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	task := findBean(t, beans, "tt-task")
	staleETag := task.ETag

	if err := client.SetStatus(task.ID, "in-progress", staleETag); err != nil {
		t.Fatalf("first SetStatus() error = %v", err)
	}

	err = client.PassReview(task.ID, staleETag)
	if err == nil {
		t.Fatal("PassReview() with stale ETag: error = nil, want ErrConflict")
	}
	if !errors.Is(err, ErrConflict) {
		t.Errorf("PassReview() error = %v, want errors.Is(err, ErrConflict)", err)
	}
}

// TestRejectReviewConflictOnStaleEtag is TestPassReviewConflictOnStaleEtag's
// RejectReview counterpart.
func TestRejectReviewConflictOnStaleEtag(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	task := findBean(t, beans, "tt-task")
	staleETag := task.ETag

	if err := client.SetStatus(task.ID, "in-progress", staleETag); err != nil {
		t.Fatalf("first SetStatus() error = %v", err)
	}

	err = client.RejectReview(task.ID, "bitte X korrigieren", "2026-07-15", staleETag)
	if err == nil {
		t.Fatal("RejectReview() with stale ETag: error = nil, want ErrConflict")
	}
	if !errors.Is(err, ErrConflict) {
		t.Errorf("RejectReview() error = %v, want errors.Is(err, ErrConflict)", err)
	}
}

func TestDeleteRemovesBean(t *testing.T) {
	repo := newTestRepo(t)
	client := &Client{RepoDir: repo}

	if err := client.Delete("tt-task"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	beans, err := client.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	for _, b := range beans {
		if b.ID == "tt-task" {
			t.Error("tt-task still present in List() after Delete()")
		}
	}
}
