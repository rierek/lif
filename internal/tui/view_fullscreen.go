package tui

// view_fullscreen.go — F01 Kernmechanik: Vollbild-Modus `v` (design-spec.md
// §15 "F01 — Vollbild-Navigation", bean bt-13l7, E9 Task 7) PLUS its
// History-Stack (ctrl+left/ctrl+right, `[`/`]` -- E9 Task 8, bean bt-1vbp,
// s. types.go's own navBack/navForward doc-stamp). keyFullscreen is this
// file's own dispatch (handleKey's checkpoint, update.go) -- renderFullscreenBody is
// the shared single-pane renderer both viewBrowseRepo()/viewBacklog() call
// into (view_browse_repo.go/view_browse_backlog.go) instead of their normal
// JoinHorizontal(listBox, detailBox) split whenever m.fullscreen !=
// fullscreenNone.

import (
	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rierek/lif/internal/tui/data"
)

// keyFullscreen dispatches every F01 key: `v` (entry, no-op when already
// fullscreen or in the Lobby), `enter` while m.fullscreen == fullscreenList
// (Listen-Vollbild -> Detail-Vollbild), and `esc` while m.fullscreen ==
// fullscreenList (direct exit -- the fullscreenDetail esc-cascade lives in
// keyDetailFocus's own Back-case instead, update.go, since it needs
// m.detailLevel to pick the right D03 rung). Signature mirrors keyNodeAction/
// keyNodeActions handled-flag pattern (update.go) -- handled=false falls
// through to the caller's normal dispatch (handleKey), unmodified.
func (m model) keyFullscreen(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	// fullscreenList's OWN two keys (enter-in, esc-out) are checked FIRST,
	// ahead of the keys.Fullscreen match below -- neither is the Fullscreen
	// binding itself, and both must intercept BEFORE keyTree/keyBacklog ever
	// see them (this function runs earlier in handleKey's dispatch chain).
	if m.fullscreen == fullscreenList {
		if keybind.Matches(msg, keys.Enter) {
			b := m.focusedBean()
			if b == nil {
				return true, m, nil // handled no-op: blattloser Cursor / orphan-root
			}
			m.fullscreen = fullscreenDetail
			m.fullscreenBeanID = b.ID
			// Detail-Fokus-Maschine auf Meta/Sektions-Ebene reinitialisiert --
			// identisch zu FocusIns bestehendem Reset-Muster (handleKey,
			// keys.FocusIn oben): a stale cursor position from wherever the
			// PO last visited detail focus must never leak into a fresh
			// Vollbild-Detail entry.
			m.secCursor, m.accOpen, m.detailLevel, m.fieldCursor = 0, 1, 0, 0
			return true, m, nil
		}
		if keybind.Matches(msg, keys.Back) {
			// Cursor war nie entkoppelt (fullscreenList never touches
			// m.cursorID/m.backlogList.cursor) -- keine Sync nötig, anders als
			// der fullscreenDetail-Exit (keyDetailFocus's Back-case).
			m.fullscreen = fullscreenNone
			// F01 History-Stack (Supervisor-Entscheid, E9 Task 8, bean
			// bt-1vbp): EVERY Vollbild-Exit clears navBack/navForward --
			// defensive here (fullscreenList can only ever be REACHED with
			// empty stacks in practice: navBack/navForward are populated
			// exclusively inside fullscreenDetail, and fullscreenDetail
			// always exits via fullscreenNone first, never straight into
			// fullscreenList), but pinned so a future change to that
			// invariant cannot silently leak History across a Vollbild
			// session (types.go's navBack/navForward doc-stamp has the
			// full ERRATA rationale).
			m.navBack, m.navForward = nil, nil
			return true, m, nil
		}
	}

	// F01 History-Stack (E9 Task 8, bean bt-1vbp, design-spec.md §15):
	// ctrl+left/[ (HistoryBack) and ctrl+right/] (HistoryForward), checked
	// HERE mirroring the fullscreenList block above -- wirksam NUR bei
	// m.fullscreen == fullscreenDetail (Scope-Entscheidung: der Stack
	// trackt ausschliesslich Relations-Sprünge INNERHALB fullscreenDetail).
	// Elsewhere (fullscreenList, Split-Modus) these keys are unbelegte
	// No-Ops (verified free against the whole keymap,
	// TestHistoryBindingsUnbelegtElsewhere) -- this block simply does not
	// match there and falls through unhandled.
	//
	// Both directions LOOP past a bean that vanished externally (live
	// watch-reload, repo switch, parallel-agent delete -- the same
	// real-world trigger class as the F01 b==nil-Guard fix, update.go)
	// instead of landing on it: a Supervisor-Entscheid "F01-Analogie" --
	// stopping on the FIRST (possibly dead) entry would let one bad entry
	// permanently block Back/Forward even though perfectly good history
	// sits further along the stack, exactly the kind of Sackgasse the
	// b==nil-Guard fix was created to prevent. A stack that is empty OR
	// contains ONLY vanished entries left is a clean No-Op (dead entries
	// are discarded along the way -- "No-Op mit sauberem Zustand").
	if m.fullscreen == fullscreenDetail {
		if keybind.Matches(msg, keys.HistoryBack) {
			for len(m.navBack) > 0 {
				n := len(m.navBack)
				target := m.navBack[n-1]
				m.navBack = m.navBack[:n-1]
				if m.idx == nil {
					break
				}
				if _, ok := m.idx.ByID[target]; !ok {
					continue // vanished -- skip, keep looking further back
				}
				m.navForward = append(cloneStringSlice(m.navForward), m.fullscreenBeanID)
				m.fullscreenBeanID = target
				m.secCursor, m.accOpen, m.detailLevel, m.fieldCursor = 0, 1, 0, 0
				break
			}
			return true, m, nil
		}
		if keybind.Matches(msg, keys.HistoryForward) {
			for len(m.navForward) > 0 {
				n := len(m.navForward)
				target := m.navForward[n-1]
				m.navForward = m.navForward[:n-1]
				if m.idx == nil {
					break
				}
				if _, ok := m.idx.ByID[target]; !ok {
					continue // vanished -- skip, keep looking further forward
				}
				m.navBack = append(cloneStringSlice(m.navBack), m.fullscreenBeanID)
				m.fullscreenBeanID = target
				m.secCursor, m.accOpen, m.detailLevel, m.fieldCursor = 0, 1, 0, 0
				break
			}
			return true, m, nil
		}
	}

	if !keybind.Matches(msg, keys.Fullscreen) || m.view == viewLobby {
		return false, m, nil
	}
	if m.fullscreen != fullscreenNone {
		// `v` ist ein EINWEG-Einstieg, kein Toggle -- esc ist der einzige
		// Ausstieg (Planner-Entscheidung, design-spec.md §15: ein zweites `v`
		// spekulativ als Ausstieg zu belegen würde eine vom PO nicht
		// verlangte zweite Bedeutung einführen).
		return true, m, nil
	}
	if !m.detailFocus {
		// Browse/Backlog + links-fokussiert -> Beans-Liste Vollbild
		// (PO-Wortlaut).
		m.fullscreen = fullscreenList
		return true, m, nil
	}
	// Browse/Backlog + rechts-fokussiert -> Detail-View Vollbild. b == nil
	// (orphan-root cursor) is a handled no-op -- m.detailFocus/m.secCursor/
	// m.fieldCursor/m.detailLevel stay UNCHANGED (the SAME Detail-Fokus-
	// Maschine is reused in the Vollbild, no reset on this entry path --
	// design-spec.md §15, unlike the Listen-Vollbild `enter` case above).
	b := m.focusedBean()
	if b == nil {
		return true, m, nil
	}
	m.fullscreen = fullscreenDetail
	m.fullscreenBeanID = b.ID
	return true, m, nil
}

// renderFullscreenBody is the shared single-pane renderer for BOTH
// fullscreen flavors (F01, design-spec.md §15) -- Chrome (breadcrumb/footer/
// status line) stays IDENTICAL to the split view (browseRepoChrome/
// backlogChrome, unchanged callers, view_browse_repo.go/
// view_browse_backlog.go), only the body swaps from
// JoinHorizontal(listBox, detailBox) to a single full-width pane. listRows is
// the caller's own (unverändert berechneten) Tree-/Backlog-rows, ALREADY
// including the search/sort head line as row 0 (same convention as the Split
// pane, only nil/unused in the fullscreenDetail case). focused is passed
// through rather than hardcoded so a future caller isn't forced into always-
// focused -- both of this task's OWN call sites (view_browse_repo.go/
// view_browse_backlog.go) always pass true (a Vollbild pane is by
// definition the ONLY visible pane, there is no split-focus ambiguity to
// resolve).
// boxScroll (bt-s90e, epic bt-vy1q) is the Vollbild-Detail's box-form scroll
// offset, which F1 (bean bt-ze10) had left as a literal 0 here: back then
// boxFormScrollBounds (mouse.go) reconstructed the SPLIT pane's accW only, so
// m.boxFormScroll was clamped against a budget this single full-width pane
// does not have. That helper branches on m.fullscreen now, so both callers
// (view_browse_repo.go/view_browse_backlog.go) simply hand in
// boxFormEffectiveScroll(m, detailBean) -- the SAME value the split's own
// renderBeanAccordionPane passes -- and the offset means the same thing on
// both geometries. Ignored entirely in the fullscreenList case and while
// boxFormEnabled() is off (renderAccordionPane's own accordion branch never
// reads it).
func renderFullscreenBody(fs fullscreenMode, innerW, bodyH int, listRows []string, focused bool, idx *data.Index, detailBean *data.Bean, secCursor, accOpen, fieldCursor, detailLevel, boxScroll int) string {
	if fs == fullscreenList {
		return renderPane(pane{rows: listRows}, innerW, bodyH, focused)
	}
	// bt-s90e closed F1's Vollbild gap: boxFormScrollBounds is fullscreen-aware
	// now (mouse.go's accW branch), so this call site passes the REAL offset
	// instead of the literal 0 it used while the geometry was split-only.
	// The field cursor stays -1 (no cursor) on purpose (bean bt-1o4g):
	// keyDetailFocus routes the Vollbild's up/down to plain viewport scrolling
	// rather than field navigation, so a cursor rendered here would be a Mauve
	// frame nothing can move -- worse than none.
	return renderAccordionPane(idx, detailBean, innerW, bodyH, accOpen, secCursor, fieldCursor, detailLevel, focused, boxScroll, -1)
}
