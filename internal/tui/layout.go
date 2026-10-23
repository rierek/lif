package tui

// layout.go — the Browse/Ready layout cycle and the view keys. v steps
// forward, shift+v backward, both wrap. The same four layouts apply to Browse
// (tree and flat) and Ready; in Ready the Ready list takes the place of the
// roadmap. M, L and R switch between Roadmap, flat list and Ready; esc leaves
// flat list and Ready for the Roadmap (escBack).

import (
	"slices"

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
	if !l.hasFilter() {
		m.filterFocus = false
	}
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

// listFocus reports whether the list (tree, flat or ready) holds the focus:
// neither the detail pane nor the filter bar does.
func (m model) listFocus() bool { return !m.detailFocus && !m.filterFocus }

// focusRing sizes the focus ring of the active layout, in ring order: the
// list, the filter fields, the detail data fields (one target each). A part
// the layout does not show has size 0, and the detail fields need a focused
// bean. The roadmap layout is the list alone, a ring of one.
func (m model) focusRing() (list, filter, detail int) {
	if m.layout.hasList() {
		list = 1
	}
	if m.layout.hasFilter() {
		filter = len(filterFields)
	}
	if m.layout.hasDetail() && m.focusedBean() != nil {
		detail = len(boxFormFieldOrder)
	}
	return
}

// focusPos is the focus's index in the ring: derived from filterFocus,
// filterField, detailFocus and the box-form cursor, so a click that sets
// those moves tab's starting point too.
func (m model) focusPos() int {
	list, filter, _ := m.focusRing()
	switch {
	case m.filterFocus:
		return list + m.filterField
	case m.detailFocus:
		return list + filter + boxFormEffectiveCursor(m, m.focusedBean())
	}
	return 0
}

// stepFocus is tab (d=1) and shift+tab (d=-1): it moves the focus one target
// along the ring of the active layout, wrapping at both ends. It never
// changes the layout. A ring of fewer than two targets (the roadmap) has
// nowhere to go, so the key does nothing. Entering the detail from outside
// puts the field cursor on its first (tab) or last (shift+tab) field;
// inside the detail, boxFormNav steps the cursor and scrolls it into view.
func (m model) stepFocus(d int) model {
	if m.view != viewBrowseRepo && m.view != viewReady {
		return m
	}
	list, filter, detail := m.focusRing()
	n := list + filter + detail
	if n < 2 {
		return m
	}
	from := m.focusPos()
	to := ((from+d)%n + n) % n
	switch {
	case to < list:
		m.filterFocus, m.detailFocus = false, false
	case to < list+filter:
		return m.focusFilterField(filterFields[to-list])
	default:
		dir := "last" // shift+tab enters from outside at the last field
		switch inside := from >= list+filter; {
		case inside && d > 0:
			dir = "next"
		case inside:
			dir = "prev"
		case d > 0:
			dir = "first"
		}
		m.filterFocus, m.detailFocus = false, true
		return m.boxFormNav(m.focusedBean(), dir)
	}
	return m
}

// focusFilter is the f jump: it shows the filter bar when the layout has none
// (roadmap and details switch to roadmap+filter), then focuses the Type
// field. A no-op outside Browse and Ready, which have no layouts.
func (m model) focusFilter() model {
	if m.view != viewBrowseRepo && m.view != viewReady {
		return m
	}
	if !m.layout.hasFilter() {
		m = m.setLayout(layoutFilter)
	}
	return m.focusFilterField("type")
}

// focusFilterField moves the focus to the named filter field (an entry of
// filterFields).
func (m model) focusFilterField(facet string) model {
	m.filterFocus, m.detailFocus = true, false
	m.filterField = slices.Index(filterFields, facet)
	return m
}

// keyFilterField drives the focused filter field: left/right move to the
// neighbouring field (wrapping), enter, space and down open the value menu,
// X clears all facets, esc returns the focus to the list. Every other key is
// swallowed so the list underneath never reacts while the bar has focus.
func (m model) keyFilterField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(filterFields)
	switch navKey(msg.String()) {
	case "left":
		m.filterField = (m.filterField + n - 1) % n
		return m, nil
	case "right":
		m.filterField = (m.filterField + 1) % n
		return m, nil
	case "down":
		return m.openFilterMenu(), nil
	}
	switch {
	case keybind.Matches(msg, filterFieldOpenHint), keybind.Matches(msg, keys.Toggle):
		return m.openFilterMenu(), nil
	case keybind.Matches(msg, keys.FilterClear):
		return m.clearFacets().resetCursorToFirstVisible(), nil
	case keybind.Matches(msg, keys.Back):
		m.filterFocus = false
	}
	return m, nil
}
