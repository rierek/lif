package tui

// layout.go — the Browse/Ready layout cycle. v steps forward, shift+v
// backward, both wrap. The same four layouts apply to Browse (tree and flat)
// and Ready; in Ready the Ready list takes the place of the roadmap.

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

// listFocus reports whether the list (tree, flat or ready) holds the focus:
// neither the detail pane nor the filter bar does.
func (m model) listFocus() bool { return !m.detailFocus && !m.filterFocus }

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
