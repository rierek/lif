package tui

// layout.go — the Browse/Ready layout cycle. v steps forward, shift+v
// backward, both wrap. The same four layouts apply to Browse (tree and flat)
// and Ready; in Ready the Ready list takes the place of the roadmap.

import (
	keybind "github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type layoutMode int

// The order is the v cycle order.
const (
	layoutRoadmap      layoutMode = iota // list at full width; the start layout
	layoutFilter                         // filter bar above the list
	layoutFilterDetail                   // filter bar above list + detail split
	layoutDetail                         // detail of the focused item at full width
	layoutCount
)

func (l layoutMode) hasList() bool   { return l != layoutDetail }
func (l layoutMode) hasDetail() bool { return l == layoutFilterDetail || l == layoutDetail }
func (l layoutMode) hasFilter() bool { return l == layoutFilter || l == layoutFilterDetail }

// filterRows is the height the filter bar takes in the current layout.
func (m model) filterRows() int {
	if m.layout.hasFilter() {
		return filterBarHeight
	}
	return 0
}

// setLayout switches the layout and moves focus into a region the new layout
// shows: the list, or the detail in layoutDetail.
func (m model) setLayout(l layoutMode) model {
	m.layout = l
	switch {
	case !l.hasList():
		if !m.detailFocus {
			m.boxFormCursor, m.boxFormCursorBean = 0, ""
		}
		m.detailFocus = true
	case !l.hasDetail():
		m.detailFocus = false
	}
	return m
}

// keyLayout handles v (next layout) and shift+v (previous layout).
func (m model) keyLayout(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	if m.view != viewBrowseRepo && m.view != viewReady {
		return false, m, nil
	}
	switch {
	case keybind.Matches(msg, keys.LayoutNext):
		return true, m.setLayout((m.layout + 1) % layoutCount), nil
	case keybind.Matches(msg, keys.LayoutPrev):
		return true, m.setLayout((m.layout + layoutCount - 1) % layoutCount), nil
	}
	return false, m, nil
}
