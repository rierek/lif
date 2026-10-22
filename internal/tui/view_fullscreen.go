package tui

// view_fullscreen.go — F01 Kernmechanik: Vollbild-Modus `v` (design-spec.md
// §15 "F01 — Vollbild-Navigation", bean bt-13l7, E9 Task 7). keyFullscreen
// is this file's own dispatch (handleKey's checkpoint, update.go) --
// renderFullscreenBody is
// the shared single-pane renderer both viewBrowseRepo()/viewBacklog() call
// into (view_browse_repo.go/view_browse_backlog.go) instead of their normal
// JoinHorizontal(listBox, detailBox) split whenever m.fullscreen !=
// fullscreenNone.

import (
	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/pkg/item"
)

// keyFullscreen dispatches every F01 key: `v` (entry, no-op when already
// fullscreen or in the Lobby), `enter` while m.fullscreen == fullscreenList
// (Listen-Vollbild -> Detail-Vollbild), and `esc` while m.fullscreen ==
// fullscreenList (direct exit -- the fullscreenDetail esc-exit lives in
// keyDetailFocus's own Back-case instead, update.go). Signature mirrors keyNodeAction/
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
			// A stale box-form cursor from wherever the PO last visited the
			// detail must never leak into a fresh Vollbild-Detail entry.
			m.boxFormCursor, m.boxFormCursorBean = 0, ""
			return true, m, nil
		}
		if keybind.Matches(msg, keys.Back) {
			// Cursor war nie entkoppelt (fullscreenList never touches
			// m.cursorID/m.backlogList.cursor) -- keine Sync nötig, anders als
			// der fullscreenDetail-Exit (keyDetailFocus's Back-case).
			m.fullscreen = fullscreenNone
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
	// (orphan-root cursor) is a handled no-op.
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
// boxScroll is the Vollbild-Detail's box-form scroll offset,
// boxFormEffectiveScroll(m, detailBean) -- the SAME value the split's own
// renderBeanDetailBox passes. Ignored in the fullscreenList case.
func renderFullscreenBody(fs fullscreenMode, innerW, bodyH int, listRows []string, focused bool, idx *data.Index, detailBean *item.Item, boxScroll int) string {
	if fs == fullscreenList {
		return renderPane(pane{rows: listRows}, innerW, bodyH, focused)
	}
	// The field cursor stays -1 (no cursor) on purpose (bean bt-1o4g):
	// keyDetailFocus routes the Vollbild's up/down to plain viewport scrolling
	// rather than field navigation, so a cursor rendered here would be a Mauve
	// frame nothing can move -- worse than none.
	return renderDetailBox(idx, detailBean, innerW, bodyH, focused, boxScroll, -1)
}
