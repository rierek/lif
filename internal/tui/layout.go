package tui

// layout.go — the Browse/Ready layout cycle and the view keys. v steps
// forward, shift+v backward, both wrap. The same four layouts apply to Browse
// (tree and flat) and Ready; in Ready the Ready list takes the place of the
// roadmap. M, L and R switch between Roadmap, flat list and Ready; esc leaves
// flat list and Ready for the Roadmap (escBack).

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

// showView switches to a list view: viewBrowseRepo (flat picks the flat list,
// else the Roadmap) or viewReady. The layout stays; focus moves to the region
// the layout shows, so a detail focus from the old view does not leak into the
// new one.
func (m model) showView(v viewID, flat bool) model {
	m.view, m.flatView = v, flat
	m.detailFocus = m.layout == layoutDetail
	if v == viewReady {
		m.readyList.setLen(len(m.readyVisible()))
	}
	return m
}

// keyView handles M (Roadmap), L (flat list) and R (Ready).
func (m model) keyView(msg tea.KeyMsg) (bool, model) {
	switch {
	case keybind.Matches(msg, keys.Roadmap):
		return true, m.showView(viewBrowseRepo, false)
	case keybind.Matches(msg, keys.FlatList):
		return true, m.showView(viewBrowseRepo, true)
	case keybind.Matches(msg, keys.Ready):
		return true, m.showView(viewReady, false)
	}
	return false, m
}

// escBack is esc on a list view whose own overlays are closed: it first
// clears an active search or filter, then leaves the flat list or Ready for
// the Roadmap. In the Roadmap with nothing to clear it does nothing.
func (m model) escBack() model {
	switch {
	case m.treeActive():
		m.searchQuery = ""
		m.searchPrefixFacets = nil // bt-2kfl D02: typed prefixes are part of the query being cleared here
		m.searchPrefixRest = ""
		m.searchBleveIDs = nil
		m.searchBleveFor = ""
		m = m.clearFacets()
		if m.view == viewReady {
			m.readyList.setLen(len(m.readyVisible()))
			return m
		}
		return m.resetCursorToFirstVisible()
	case m.view == viewReady || m.flatView:
		return m.showView(viewBrowseRepo, false)
	}
	return m
}
