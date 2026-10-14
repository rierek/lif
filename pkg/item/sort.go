package item

import (
	"sort"
	"strings"
)

// SortByStatusPriorityAndType sorts items by status order, then manual order, then priority, then type, then title.
// This is the default sorting used by both CLI and TUI.
// Unrecognized statuses, priorities, and types are sorted last within their category.
// Items without priority are treated as "normal" priority for sorting purposes.
func SortByStatusPriorityAndType(items []*Item, statusNames, priorityNames, typeNames []string) {
	statusOrder := make(map[string]int)
	for i, s := range statusNames {
		statusOrder[s] = i
	}
	priorityOrder := make(map[string]int)
	for i, p := range priorityNames {
		priorityOrder[p] = i
	}
	typeOrder := make(map[string]int)
	for i, t := range typeNames {
		typeOrder[t] = i
	}

	// Find the index of "normal" priority for items without priority set
	normalPriorityOrder := len(priorityNames) // default to last if "normal" not found
	for i, p := range priorityNames {
		if p == "normal" {
			normalPriorityOrder = i
			break
		}
	}

	// Helper to get order with unrecognized values sorted last
	getStatusOrder := func(status string) int {
		if order, ok := statusOrder[status]; ok {
			return order
		}
		return len(statusNames) // Unrecognized statuses come last
	}
	getPriorityOrder := func(priority string) int {
		if priority == "" {
			return normalPriorityOrder // No priority = normal
		}
		if order, ok := priorityOrder[priority]; ok {
			return order
		}
		return len(priorityNames) // Unrecognized priorities come last
	}
	getTypeOrder := func(typ string) int {
		if order, ok := typeOrder[typ]; ok {
			return order
		}
		return len(typeNames) // Unrecognized types come last
	}

	sort.Slice(items, func(i, j int) bool {
		// Primary: status order
		oi, oj := getStatusOrder(items[i].Status), getStatusOrder(items[j].Status)
		if oi != oj {
			return oi < oj
		}
		// Secondary: manual order (fractional index) — items with order come first
		oiHas, ojHas := items[i].Order != "", items[j].Order != ""
		if oiHas && ojHas {
			if items[i].Order != items[j].Order {
				return items[i].Order < items[j].Order
			}
		} else if oiHas != ojHas {
			// Items with explicit order come before those without
			return oiHas
		}
		// Tertiary: priority order (for items without manual order, or as tiebreaker)
		pi, pj := getPriorityOrder(items[i].Priority), getPriorityOrder(items[j].Priority)
		if pi != pj {
			return pi < pj
		}
		// Quaternary: type order
		ti, tj := getTypeOrder(items[i].Type), getTypeOrder(items[j].Type)
		if ti != tj {
			return ti < tj
		}
		// Final: title (case-insensitive) for stable, user-friendly ordering
		return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
	})
}
