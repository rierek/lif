package tui

// box_confirm_delete.go — the Delete-Confirm modal (`d`, E3 Task 6, bean
// bt-ppzb): Port devd box_confirm_delete.go:57-126 (keyDelete enter/esc/n,
// deleteBox Rot=destruktiv, modalBox theme.Red) -- MINUS devd's
// loadDeletePreview (idx.Children[id] is already synchronous in memory here,
// no async count-load like devd's cascade-delete preview needs) and MINUS
// devd's removeIssueFromCaches (beans-tui does not cache per-view: the
// unconditional reload every mutationDoneMsg already triggers, design
// decision d, IS the cache invalidation).
//
// Semantic deviation from devd (bean bt-ppzb Ziel, mutations.go Delete's own
// doc-stamp): `beans delete` does NOT cascade -- there is no server-side
// cascade-delete, `--json` only skips the confirmation prompt and any
// reference/child WARNINGS, it does not delete anything beyond the one bean.
// deleteBox's copy therefore never claims children get deleted (devd's own
// "will also be deleted" cascade wording does not apply here).
//
// ERRATUM (empirically verified against beans 0.4.2 during this task's tmux
// smoke test + an isolated `beans create`/`beans delete` probe, both
// reproducible): the epic-E3-plan.md/bean bt-ppzb's ORIGINAL assumption --
// that a deleted bean's direct children survive with a now-DANGLING Parent
// field and surface under the synthetic "(orphaned)" root -- is WRONG. The
// real beans 0.4.2 `delete` command actively rewrites every direct child's
// frontmatter, REMOVING the `parent:` field entirely rather than leaving it
// dangling. A deleted bean's direct children therefore become genuine,
// ordinary ROOT beans (idx.Roots(), view_browse_repo.go) -- NOT
// "(orphaned)"-bucket orphans; they render at the top level exactly like any
// other parentless bean, not flagged or quarantined in any way. delChildren
// (below) is still an accurate PRE-delete count (computed from the CURRENT
// in-memory idx.Children before the mutation fires) -- only the POST-delete
// outcome the modal's copy describes was corrected. Grandchildren are
// unaffected either way: their own parent (the direct child, untouched by
// this rewrite) stays intact, so only DIRECT children are ever in play here
// (idx.Children[id], never data.CollectDescendants's full recursive walk).
// Regression: internal/data/client_mut_test.go's
// TestDeleteClearsFormerChildrensParentField.
//
// Q01 (E3-T6-Review PFLICHT finding, bean bt-qzwt): the SAME "beans
// actively rewrites the referencing file, no CLI warning of its own"
// behavior holds for `blocking`/`blocked_by` -- verified empirically the
// same way (isolated scratch-repo probe: two fresh beans, A `--blocked-by`
// B, delete B, `cat` A's frontmatter -- both directions, `blocked_by` and
// `blocking`), pinned as regressions in
// internal/data/client_mut_test.go's
// TestDeleteClearsOtherBeansBlockedByReference/...BlockingReference.
// delLinks/countLinkedBeans below extend the SAME open-time synchronous
// count (delChildren's own convention) to this second link family, and
// deleteBox's copy warns about it the same way.

import (
	"fmt"
	"slices"
	"strings"

	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/internal/tui/theme"
)

// openDeleteConfirm opens the Delete-Confirm overlay on the focused bean (the
// focusedBean()!=nil guard mirrors every other T1-T3 open* helper --
// keyNodeAction's own guard already covers the real dispatch path, this one
// makes openDeleteConfirm safe to call directly in tests too). mutTarget
// captures the bean ID (the shared field every other overlay already uses);
// delTitle/delChildren are captured HERE, at open time, purely for the
// modal's own render -- delChildren is idx.Children[id]'s DIRECT count only
// (types.go doc-stamp: NOT data.CollectDescendants's full recursive walk,
// since `beans delete` does not cascade -- only direct children orphan).
func (m model) openDeleteConfirm() model {
	b := m.focusedBean()
	if b == nil {
		return m
	}
	m.mutTarget = b.ID
	m.delTitle = b.Title
	m.delChildren = 0
	m.delLinks = 0
	if m.idx != nil {
		m.delChildren = len(m.idx.Children[b.ID])
		m.delLinks = countLinkedBeans(m.idx, b.ID)
	}
	m.overlay = overlayDeleteConfirm
	return m
}

// countLinkedBeans (Q01, bean bt-qzwt) returns how many OTHER beans
// reference id via Blocking or BlockedBy -- a bean referencing id through
// BOTH fields still counts ONCE (distinct-bean semantics, mirroring
// delChildren's own convention: idx.Children[id] already counts distinct
// children, never link INSTANCES).
func countLinkedBeans(idx *data.Index, id string) int {
	if idx == nil {
		return 0
	}
	n := 0
	for otherID, b := range idx.ByID {
		if otherID == id {
			continue
		}
		if slices.Contains(b.Blocking, id) || slices.Contains(b.BlockedBy, id) {
			n++
		}
	}
	return n
}

// keyDeleteConfirm drives the open Delete-Confirm: enter fires
// data.Store.Delete (NO --if-match -- the beans CLI's `delete` subcommand
// takes no etag at all, mutations.go Delete's own doc-stamp; m.beanETag(id)
// is still consulted here, but ONLY for its ok bool, the same vanished-
// target guard shape as every other overlay's applyXSelection/
// applyEditorFinished), esc/n cancels without mutating anything (Port devd
// keyDelete's own esc/n cancel, DD2-174). Cursor-clamping after a successful
// delete needs NO new logic here: applyMutationResult's unconditional reload
// (design decision d) runs applyLoaded's EXISTING oldPos-fallback path
// (update.go, T8/E1) exactly like any other reload that drops the cursored
// bean.
func (m model) keyDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keybind.Matches(msg, keys.Back), msg.String() == "n":
		m.overlay = overlayNone
		return m, nil
	case keybind.Matches(msg, keys.Enter):
		m.overlay = overlayNone
		id := m.mutTarget
		if _, ok := m.beanETag(id); !ok {
			m.err = "Item no longer exists — deletion discarded"
			// bt-81f0 (Notifications vereinheitlichen): m.err's rendering
			// anbindung is gone (view.go statusBar/ErrNote) -- Toast is now
			// the ONE visible channel, so this vanished-target guard needs
			// its own showToast call to stay visible at all.
			var toastCmd tea.Cmd
			m, toastCmd = m.showToast(toastError, m.err, "", nil, false)
			return m, toastCmd
		}
		store := m.store
		return m, mutateCmd(store, func() error { return store.Delete(id) })
	}
	return m, nil
}

// deleteBox renders the floating Delete-Confirm modal -- Red border/header
// (destructive, Port devd's theme.Red convention, mirrors quitBox's own
// Mauve-for-non-destructive counterpart, box_confirm_quit.go). typ resolves
// from the LIVE index at render time (mirrors valueMenuBox's own
// m.idx.ByID[m.mutTarget] lookup) rather than a cached field, since it is
// only needed for display, not for dispatch.
func (m model) deleteBox() string {
	typ := "item"
	if m.idx != nil {
		if cur, ok := m.idx.ByID[m.mutTarget]; ok {
			typ = cur.Type
		}
	}

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(theme.Red).Bold(true).Render("Delete "+typ) + "\n")
	b.WriteString(theme.Header.Render(m.delTitle) + "\n\n")
	// I02 (E3-T6-Review PFLICHT finding, bean bt-qzwt): singular/plural
	// grammar branch -- the original single Sprintf read "1 child(ren)
	// lose", grammatically wrong for the count==1 case ("child(ren)"
	// AND the plural verb "lose" both need their own singular form).
	switch m.delChildren {
	case 0:
		// no line -- TestDeleteBoxLeafOmitsChildrenWarning
	case 1:
		b.WriteString("1 child loses its parent — becomes its own root\n")
	default:
		b.WriteString(fmt.Sprintf("%d children lose their parent — become their own roots\n", m.delChildren))
	}
	// Q01 (E3-T6-Review PFLICHT finding, bean bt-qzwt): same singular/plural
	// discipline applied from the start for the linked-bean warning (never
	// introduced the I02 bug here to begin with).
	switch m.delLinks {
	case 0:
		// no line -- TestDeleteBoxOmitsLinkedWarningWhenZero
	case 1:
		b.WriteString("1 item loses its blocking/blocked-by link to this item\n")
	default:
		b.WriteString(fmt.Sprintf("%d items lose their blocking/blocked-by link to this item\n", m.delLinks))
	}
	b.WriteString("\n" + lipgloss.NewStyle().Foreground(theme.Red).Render("Irreversible.") + "\n")
	b.WriteString("\n" + theme.Dim.Render("enter: delete permanently   esc/n: cancel"))
	return modalBox(b.String(), clampModalWidth(48, m.width), theme.Red)
}
