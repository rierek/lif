package tui

// box_confirm_delete_test.go — TDD coverage for the Delete-Confirm modal
// (`d`, E3 Task 6, bean bt-ppzb): Kinder-Count captured SYNCHRONOUSLY from
// idx.Children (no async preview-load like devd box_confirm_delete.go), the
// orphaning wording (NOT a cascade-delete claim), enter/esc dispatch, and the
// cursor-clamp-after-delete end-to-end proof (reusing applyLoaded's EXISTING
// oldPos fallback, no new cursor logic).

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rierek/lif/internal/tui/data"
)

// --- openDeleteConfirm ---

// TestOpenDeleteConfirmCountsChildrenSynchronously guards `d` on a bean with
// children: delChildren is idx.Children[id]'s DIRECT count (ep-1 has two
// children, tk-1/tk-2, fixtureBeans), read straight from the in-memory
// index -- no loading state, no async Cmd (types.go/box_confirm_delete.go
// doc-stamps: deliberately NOT data.CollectDescendants's recursive walk,
// since only DIRECT children orphan when their parent is deleted).
func TestOpenDeleteConfirmCountsChildrenSynchronously(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "ep-1")

	tm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update('d') did not return a model, got %T", tm)
	}
	if cmd != nil {
		t.Fatal("opening the delete confirm must not itself fire a Cmd")
	}
	if nm.overlay != overlayDeleteConfirm {
		t.Fatalf("overlay = %v, want overlayDeleteConfirm", nm.overlay)
	}
	if nm.mutTarget != "ep-1" {
		t.Fatalf("mutTarget = %q, want ep-1", nm.mutTarget)
	}
	if nm.delTitle != "Epic One" {
		t.Fatalf("delTitle = %q, want %q", nm.delTitle, "Epic One")
	}
	if nm.delChildren != 2 {
		t.Fatalf("delChildren = %d, want 2 (tk-1, tk-2)", nm.delChildren)
	}
}

// TestOpenDeleteConfirmLeafHasZeroChildren guards the other half: a leaf
// (tk-1, no children) opens with delChildren == 0, so deleteBox's orphan
// warning line does not render at all (TestDeleteBoxLeafOmitsOrphanWarning
// below).
func TestOpenDeleteConfirmLeafHasZeroChildren(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-1")

	m = step(t, m, runeMsg('d'))
	if m.overlay != overlayDeleteConfirm {
		t.Fatalf("overlay = %v, want overlayDeleteConfirm", m.overlay)
	}
	if m.delChildren != 0 {
		t.Fatalf("delChildren = %d, want 0 (tk-1 is a leaf)", m.delChildren)
	}
}

// --- Q01 (E3-T6-Review PFLICHT finding, bean bt-qzwt): linked-bean warning ---
//
// Empirical finding (scratch-repo probe, two fresh beans A/B, A
// --blocked-by B, delete B, cat A's frontmatter -- both directions,
// mirrored in internal/data/client_mut_test.go's
// TestDeleteClearsOtherBeansBlockedByReference/...BlockingReference):
// `beans delete` silently clears blocking/blocked_by references OTHER
// beans hold on the deleted bean, exactly the same "actively rewrites the
// referencing file, no CLI warning" behavior box_confirm_delete.go's
// existing ERRATUM doc-stamp already pins for the parent field. deleteBox
// must warn about this too -- delLinks (computed at open time, synchronous,
// same idx-in-memory convention as delChildren) counts the DISTINCT other
// beans that reference the target via Blocking or BlockedBy.

// fixtureBeansLinked adds a Blocking/BlockedBy relationship to
// fixtureBeans() for Q01 coverage: tk-1 blocks tk-2 (tk-1.Blocking =
// [tk-2], tk-2.BlockedBy = [tk-1]) -- both halves of the SAME link, so
// deleting either end exercises countLinkedBeans from each side.
func fixtureBeansLinked() []data.Bean {
	beans := fixtureBeans()
	for i := range beans {
		switch beans[i].ID {
		case "tk-1":
			beans[i].Blocking = []string{"tk-2"}
		case "tk-2":
			beans[i].BlockedBy = []string{"tk-1"}
		}
	}
	return beans
}

// TestOpenDeleteConfirmCountsLinkedBeanSingular: deleting tk-1 (referenced
// by tk-2's BlockedBy) must count delLinks == 1.
func TestOpenDeleteConfirmCountsLinkedBeanSingular(t *testing.T) {
	m := fixtureModel(t, fixtureBeansLinked())
	m = focusBean(m, "tk-1")

	m = step(t, m, runeMsg('d'))
	if m.delLinks != 1 {
		t.Fatalf("delLinks = %d, want 1 (tk-2 references tk-1 via BlockedBy)", m.delLinks)
	}
}

// TestOpenDeleteConfirmCountsLinkedBeanFromBlockingSide is the mirror:
// deleting tk-2 (referenced by tk-1's Blocking) must ALSO count
// delLinks == 1 -- countLinkedBeans checks both link families on the OTHER
// bean, not just one.
func TestOpenDeleteConfirmCountsLinkedBeanFromBlockingSide(t *testing.T) {
	m := fixtureModel(t, fixtureBeansLinked())
	m = focusBean(m, "tk-2")

	m = step(t, m, runeMsg('d'))
	if m.delLinks != 1 {
		t.Fatalf("delLinks = %d, want 1 (tk-1 references tk-2 via Blocking)", m.delLinks)
	}
}

// TestOpenDeleteConfirmCountsLinkedBeansDistinctNotDouble guards the
// distinct-bean-count semantics (mirrors delChildren's own distinct-count
// convention): a bean referencing the target via BOTH Blocking AND
// BlockedBy must still count ONCE, not twice.
func TestOpenDeleteConfirmCountsLinkedBeansDistinctNotDouble(t *testing.T) {
	beans := fixtureBeansLinked()
	for i := range beans {
		if beans[i].ID == "tk-2" {
			beans[i].Blocking = []string{"tk-1"} // tk-2 now ALSO blocks tk-1 back
		}
	}
	m := fixtureModel(t, beans)
	m = focusBean(m, "tk-1")

	m = step(t, m, runeMsg('d'))
	if m.delLinks != 1 {
		t.Fatalf("delLinks = %d, want 1 (tk-2 references tk-1 twice -- BlockedBy AND Blocking -- but counts as ONE bean)", m.delLinks)
	}
}

// TestOpenDeleteConfirmNoLinksLeavesDelLinksZero guards the common case:
// fixtureBeans() (unlinked) opens with delLinks == 0.
func TestOpenDeleteConfirmNoLinksLeavesDelLinksZero(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-1")

	m = step(t, m, runeMsg('d'))
	if m.delLinks != 0 {
		t.Fatalf("delLinks = %d, want 0 (fixtureBeans has no blocking/blocked_by links)", m.delLinks)
	}
}

// --- deleteBox ---

// TestDeleteBoxWarnsChildrenLoseParentBecomeRoots guards the semantic
// deviation from devd (bean bt-ppzb Ziel): `beans delete` does NOT cascade
// -- the modal must never claim children get deleted too ("deleted"). The
// exact wording ("lose their parent — become their own roots") is
// itself an ERRATUM correction (box_confirm_delete.go's own doc-stamp): the
// original "become '(orphaned)'-bucket orphans" assumption was empirically
// WRONG -- beans 0.4.2 clears the parent reference outright rather than
// leaving it dangling, so this test also guards against the STALE "orphan"
// wording reappearing.
func TestDeleteBoxWarnsChildrenLoseParentBecomeRoots(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "ep-1")
	m = step(t, m, runeMsg('d'))

	box := m.deleteBox()
	if !strings.Contains(box, "lose their parent") {
		t.Fatalf("deleteBox() = %q, want it to mention the children losing their parent (not \"orphan\" -- ERRATUM, beans clears the reference outright)", box)
	}
	if strings.Contains(box, "orphan") {
		t.Fatalf("deleteBox() = %q, must NOT use the stale \"orphan\" wording -- beans 0.4.2 clears the parent reference, it does not leave a dangling one", box)
	}
	if strings.Contains(box, "deleted") {
		t.Fatalf("deleteBox() = %q, must NOT claim children get deleted (\"deleted\") -- beans delete does not cascade", box)
	}
	if !strings.Contains(box, "2") {
		t.Fatalf("deleteBox() = %q, want the child count (2) rendered", box)
	}
	if !strings.Contains(box, "Epic One") {
		t.Fatalf("deleteBox() = %q, want the bean's title rendered", box)
	}
	if !strings.Contains(box, "Irreversible") {
		t.Fatalf("deleteBox() = %q, want the Irreversible warning", box)
	}
}

// TestDeleteBoxLeafOmitsChildrenWarning guards the delChildren==0 branch: a
// leaf's confirm modal must not render a children-count line at all.
func TestDeleteBoxLeafOmitsChildrenWarning(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-1")
	m = step(t, m, runeMsg('d'))

	box := m.deleteBox()
	if strings.Contains(box, "lose their parent") {
		t.Fatalf("deleteBox() for a leaf = %q, must not mention children losing a parent (delChildren == 0)", box)
	}
	if !strings.Contains(box, "Task One") {
		t.Fatalf("deleteBox() = %q, want the bean's title rendered", box)
	}
}

// TestDeleteBoxSingularChildWording guards I02 (E3-T6-Review PFLICHT
// finding, bean bt-qzwt): a delChildren == 1 case must read "1 child
// loses ... becomes its own root" (singular verb/noun), not the
// plural "children lose ... become their own roots" wording the
// count>1 case uses (TestDeleteBoxWarnsChildrenLoseParentBecomeRoots
// above). ms-1 (fixtureBeans' milestone) has exactly ONE child (ep-1).
func TestDeleteBoxSingularChildWording(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "ms-1")
	m = step(t, m, runeMsg('d'))

	if m.delChildren != 1 {
		t.Fatalf("setup: delChildren = %d, want 1 (ms-1 has exactly one child, ep-1)", m.delChildren)
	}
	// Short substrings deliberately (not the full sentence): the modal
	// word-wraps at clampModalWidth(48, ...), so "becomes its own root"
	// itself splits across two rendered lines -- the same reason the
	// EXISTING plural test above only checks "lose their parent", not
	// the full sentence, either.
	box := m.deleteBox()
	if !strings.Contains(box, "1 child loses its parent") {
		t.Fatalf("deleteBox() = %q, want the singular \"1 child loses its parent\" wording", box)
	}
	if strings.Contains(box, "children lose") {
		t.Fatalf("deleteBox() = %q, must not use the plural \"children lose\" wording for delChildren == 1", box)
	}
	if strings.Contains(box, "roots") {
		t.Fatalf("deleteBox() = %q, must not use the plural noun \"roots\" for delChildren == 1", box)
	}
}

// TestDeleteBoxWarnsLinkedBeanSingular guards Q01's linked-bean warning
// line, singular branch (delLinks == 1) -- built with correct singular
// grammar from the start (I02's lesson applied immediately, not retrofitted
// after a bug report).
func TestDeleteBoxWarnsLinkedBeanSingular(t *testing.T) {
	m := fixtureModel(t, fixtureBeansLinked())
	m = focusBean(m, "tk-1")
	m = step(t, m, runeMsg('d'))

	// Short substring, same word-wrap reasoning as
	// TestDeleteBoxSingularChildWording above.
	box := m.deleteBox()
	if !strings.Contains(box, "1 item loses its blocking") {
		t.Fatalf("deleteBox() = %q, want the singular linked-item warning", box)
	}
	if strings.Contains(box, "items lose their blocking") {
		t.Fatalf("deleteBox() = %q, must not use plural wording for delLinks == 1", box)
	}
}

// TestDeleteBoxWarnsLinkedBeansPlural guards the plural branch (delLinks >
// 1): ep-1 additionally references tk-1 via Blocking, on top of tk-2's
// existing BlockedBy reference from fixtureBeansLinked -- two distinct
// referencing beans.
func TestDeleteBoxWarnsLinkedBeansPlural(t *testing.T) {
	beans := fixtureBeansLinked()
	for i := range beans {
		if beans[i].ID == "ep-1" {
			beans[i].Blocking = []string{"tk-1"}
		}
	}
	m := fixtureModel(t, beans)
	m = focusBean(m, "tk-1")
	m = step(t, m, runeMsg('d'))

	box := m.deleteBox()
	if !strings.Contains(box, "2 items lose their blocking") {
		t.Fatalf("deleteBox() = %q, want the plural linked-bean warning (\"2 beans lose\")", box)
	}
}

// TestDeleteBoxOmitsLinkedWarningWhenZero guards the delLinks == 0 case
// (the common, unlinked path): no linked-bean line renders at all, mirrors
// TestDeleteBoxLeafOmitsChildrenWarning's own omission convention for
// delChildren == 0.
func TestDeleteBoxOmitsLinkedWarningWhenZero(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-1")
	m = step(t, m, runeMsg('d'))

	box := m.deleteBox()
	if strings.Contains(box, "blocking/blocked-by link") {
		t.Fatalf("deleteBox() = %q, must not render the linked-bean warning when delLinks == 0", box)
	}
}

// --- keyDeleteConfirm ---

// TestDeleteConfirmEnterFiresDeleteAndCloses guards enter: closes the
// overlay and returns a Cmd whose resolved mutationDoneMsg's error names
// "lif delete" (proves data.Client.Delete dispatched, NO --if-match --
// mutations.go's Delete signature takes no etag at all), verified the same
// no-binary-required way box_picker_parent_test.go/box_confirm_create_test.go
// already use.
func TestDeleteConfirmEnterFiresDeleteAndCloses(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.client = &data.Client{RepoDir: "/nonexistent-bt-e3-t6-scratch-dir"}
	m = focusBean(m, "tk-1")
	m = step(t, m, runeMsg('d'))

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update(enter) did not return a model, got %T", tm)
	}
	if nm.overlay != overlayNone {
		t.Fatalf("overlay after enter = %v, want overlayNone", nm.overlay)
	}
	if cmd == nil {
		t.Fatal("enter must return the delete mutateCmd")
	}
	msg := cmd()
	mdm, ok := msg.(mutationDoneMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want mutationDoneMsg", msg)
	}
	if mdm.err == nil || !strings.Contains(mdm.err.Error(), "lif delete") {
		t.Fatalf("mutationDoneMsg.err = %v, want an error containing %q (proves Delete dispatched)", mdm.err, "lif delete")
	}
}

// TestDeleteConfirmEscCancels guards esc: closes the overlay without firing
// any Cmd.
func TestDeleteConfirmEscCancels(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-1")
	m = step(t, m, runeMsg('d'))

	tm, cmd := m.Update(keyMsg(tea.KeyEsc))
	nm := tm.(model)
	if nm.overlay != overlayNone {
		t.Fatal("esc did not close the delete confirm")
	}
	if cmd != nil {
		t.Fatal("esc must not fire a mutation Cmd")
	}
}

// TestDeleteConfirmNCancels guards the devd-parity "n" shorthand (Port
// box_confirm_delete.go's own esc/n cancel, DD2-174) alongside keys.Back.
func TestDeleteConfirmNCancels(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = focusBean(m, "tk-1")
	m = step(t, m, runeMsg('d'))

	tm, cmd := m.Update(runeMsg('n'))
	nm := tm.(model)
	if nm.overlay != overlayNone {
		t.Fatal("n did not close the delete confirm")
	}
	if cmd != nil {
		t.Fatal("n must not fire a mutation Cmd")
	}
}

// --- vanished-target guard on enter (design decision d parity) ---

// TestDeleteConfirmEnterTargetVanishedClosesGracefully mirrors every other
// overlay's vanished-mutTarget regression (TestValueMenuTargetVanishedClose
// sGracefully, TestTagPickerEnterTargetVanishedClosesGracefully): the focused
// bean disappears (external delete + reload) between open and enter -- enter
// must close the overlay and set a status-line note, WITHOUT firing a doomed
// second Delete.
func TestDeleteConfirmEnterTargetVanishedClosesGracefully(t *testing.T) {
	beans := fixtureBeans()
	m := fixtureModel(t, beans)
	m = focusBean(m, "tk-2")
	m = step(t, m, runeMsg('d'))
	if m.overlay != overlayDeleteConfirm {
		t.Fatal("setup: d did not open the delete confirm")
	}

	remaining := []data.Bean{beans[0], beans[1], beans[2]} // ms-1, ep-1, tk-1 -- no tk-2
	m = step(t, m, beansLoadedMsg{beans: remaining})

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	nm := tm.(model)
	if nm.overlay != overlayNone {
		t.Fatal("enter on a vanished target must close the overlay")
	}
	if nm.err == "" {
		t.Fatal("enter on a vanished target must set a status-line note (m.err)")
	}
	// bt-81f0: cmd is no longer nil -- it is now the Toast's own
	// auto-dismiss tick (showToast's toastTimeout Cmd, non-sticky), NOT a
	// doomed mutation (structurally guaranteed: this branch returns before
	// any mutateCmd(...) is ever built). Not invoked here -- toastError's
	// own duration (8s, overlay_show_toast.go toastDuration) would block
	// this test for real.
	if cmd == nil {
		t.Fatal("enter on a vanished target must still fire a Cmd (the Toast's own auto-dismiss tick, bt-81f0)")
	}
	// bt-81f0 (Notifications vereinheitlichen): the status-line note above
	// no longer renders anywhere -- Toast is the ONE visible channel, so a
	// vanished-target guard that only set m.err would go completely silent.
	if nm.toast == nil {
		t.Fatal("enter on a vanished target must also show a Toast (m.err lost its rendering, bt-81f0)")
	} else if nm.toast.kind != toastError {
		t.Errorf("toast.kind = %v, want toastError", nm.toast.kind)
	}
}

// --- cursor clamp after delete (reuse, no new logic) ---

// TestDeleteCursorClampsViaApplyLoadedOldPos guards the plan's explicit
// "keine neue Cursor-Logik" requirement: cursor parked on the LAST visible
// node (tk-2), delete it, reload without it -- the cursor must clamp near
// its old spot (applyLoaded's EXISTING oldPos fallback, update.go/T8) rather
// than resetting to the top. Mirrors the real flow WITHOUT executing the
// actual mutateCmd (that dispatch is covered by
// TestDeleteConfirmEnterFiresDeleteAndCloses above) -- a successful Delete's
// mutationDoneMsg always carries err == nil, which is all applyMutationResult
// needs to fire its unconditional reload.
func TestDeleteCursorClampsViaApplyLoadedOldPos(t *testing.T) {
	beans := fixtureBeans() // ms-1 -> ep-1 -> tk-1, tk-2
	m := fixtureModel(t, beans)
	m.expanded = map[string]bool{"ms-1": true, "ep-1": true}
	m.cursorID = "tk-2"

	m = step(t, m, runeMsg('d'))
	if m.overlay != overlayDeleteConfirm {
		t.Fatal("setup: d did not open the delete confirm")
	}

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	nm := tm.(model)
	if nm.overlay != overlayNone {
		t.Fatal("enter did not close the delete confirm")
	}
	if cmd == nil {
		t.Fatal("enter must return the delete mutateCmd")
	}

	nm = step(t, nm, mutationDoneMsg{err: nil})
	remaining := []data.Bean{beans[0], beans[1], beans[2]} // ms-1, ep-1, tk-1 -- tk-2 deleted
	nm = step(t, nm, beansLoadedMsg{beans: remaining})

	if nm.cursorID == "tk-2" {
		t.Fatal("cursor must clamp off the deleted bean")
	}
	if nm.cursorID != "tk-1" {
		t.Fatalf("cursorID after delete-clamp = %q, want tk-1 (neighbor clamp)", nm.cursorID)
	}
}

// --- keyNodeAction dispatch ---

// TestKeyNodeActionDNoFocusedBeanIsSilentNoOp guards keyNodeAction's shared
// focusedBean()==nil guard (update.go) also covers Delete: no node to act on
// on an empty/pre-load repo, handled but silent.
func TestKeyNodeActionDNoFocusedBeanIsSilentNoOp(t *testing.T) {
	m := newModel(nil, "/tmp/bt-fixture-repo") // pre-load: idx == nil, no focused bean

	tm, cmd := m.Update(runeMsg('d'))
	nm, ok := tm.(model)
	if !ok {
		t.Fatalf("Update('d') did not return a model, got %T", tm)
	}
	if nm.overlay != overlayNone {
		t.Fatalf("overlay = %v, want overlayNone (no focused bean to delete)", nm.overlay)
	}
	if cmd != nil {
		t.Fatal("d with no focused bean must not fire a Cmd")
	}
}

// --- I03 (E3-T6-Review optional finding, bean bt-qzwt): delete the last bean ---

// TestDeleteLastBeanInRepoClearsCursorGracefully guards the edge the
// cursor-clamp tests above never hit: a repo with exactly ONE bean (no
// siblings to clamp onto). applyLoaded's existing len(nodes)==0 branch
// (update.go) already handles this -- cursorID resets to "" -- but nothing
// drove it end-to-end through the ACTUAL delete flow before, nor proved
// View() survives rendering a fully empty repo without panicking.
func TestDeleteLastBeanInRepoClearsCursorGracefully(t *testing.T) {
	beans := []data.Bean{
		{ID: "tk-1", Title: "Only Task", Status: "todo", Type: "task", Priority: "normal"},
	}
	m := fixtureModel(t, beans)
	m.cursorID = "tk-1"

	m = step(t, m, runeMsg('d'))
	if m.overlay != overlayDeleteConfirm {
		t.Fatal("setup: d did not open the delete confirm")
	}

	tm, cmd := m.Update(keyMsg(tea.KeyEnter))
	nm := tm.(model)
	if nm.overlay != overlayNone {
		t.Fatal("enter did not close the delete confirm")
	}
	if cmd == nil {
		t.Fatal("enter must return the delete mutateCmd")
	}

	nm = step(t, nm, mutationDoneMsg{err: nil})
	nm = step(t, nm, beansLoadedMsg{beans: nil}) // repo now completely empty

	if nm.cursorID != "" {
		t.Fatalf("cursorID after deleting the last bean = %q, want \"\" (empty repo)", nm.cursorID)
	}
	if nm.delChildren != 0 || nm.delLinks != 0 {
		t.Fatalf("delChildren/delLinks after reload = %d/%d, want 0/0 (no dangling state from the closed overlay)", nm.delChildren, nm.delLinks)
	}
	// Rendering the view on a fully empty index must not panic.
	_ = nm.View()
}
