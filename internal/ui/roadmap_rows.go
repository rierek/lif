package ui

import "github.com/rierek/lif/pkg/roadmap"

// RoadmapRows bridges the grouped roadmap.Data produced by roadmap.Build /
// roadmap.BuildScoped into the flat row list Render expects. It mirrors the
// walk roadmap.tmpl renders (milestone -> epics -> features -> leafs,
// then epic/feature/other for the unscheduled bucket), but builds
// FlatItems instead of writing text directly, and does no sorting of its
// own -- order comes entirely from the builder's slices.
//
// Depth counts tree levels, not the old renderer's indent-in-spaces: each
// milestone (or, for the unscheduled bucket, each of its top-level epics/
// features/other) is its own depth-0 root. RowsFromFlatItems resets its
// ancestry stack whenever depth drops back to 0, so consecutive roots never
// bleed tree lines into each other -- which is also why a milestone's own
// IsLast is irrelevant to what gets drawn (Connector/Stem never consult
// AncestorsLast[0]) and is set to true throughout rather than tracked.
//
// Only the unscheduled bucket gets a Section heading ("Unscheduled"): each
// rank-1 container is itself a Item row, which is heading enough on its own,
// matching the one documented use of Row.Section (ui/columns.go). The wording
// is "Unscheduled" rather than "No Milestone" because a project's rank-1 type
// need not be called "milestone", and a profile may define none at all.
func RoadmapRows(data *roadmap.Data) []Row {
	var items []FlatItem

	if data.Root != nil {
		if data.Root.Epic != nil {
			appendRoadmapEpicGroup(&items, *data.Root.Epic, 0, true)
		}
		if data.Root.Feature != nil {
			appendRoadmapFeatureGroup(&items, *data.Root.Feature, 0, true)
		}
		return RowsFromFlatItems(items)
	}

	for _, mg := range data.Milestones {
		appendRoadmapMilestoneGroup(&items, mg)
	}

	unscheduledAt := -1
	if data.Unscheduled != nil {
		unscheduledAt = len(items)
		appendRoadmapUnscheduledGroup(&items, *data.Unscheduled)
	}

	rows := RowsFromFlatItems(items)
	if unscheduledAt >= 0 && unscheduledAt < len(rows) {
		rows[unscheduledAt].Section = "Unscheduled"
	}
	return rows
}

// appendRoadmapMilestoneGroup appends a milestone, then its container
// children and other leaf items in mg.Containers / mg.Other order, the
// order roadmap.tmpl renders.
func appendRoadmapMilestoneGroup(items *[]FlatItem, mg roadmap.MilestoneGroup) {
	*items = append(*items, FlatItem{Item: mg.Milestone, Depth: 0, IsLast: true})

	total := len(mg.Containers) + len(mg.Other)
	i := 0
	for _, c := range mg.Containers {
		i++
		if c.Epic != nil {
			appendRoadmapEpicGroup(items, *c.Epic, 1, i == total)
		} else {
			appendRoadmapFeatureGroup(items, *c.Feature, 1, i == total)
		}
	}
	for _, leaf := range mg.Other {
		i++
		*items = append(*items, FlatItem{Item: leaf, Depth: 1, IsLast: i == total})
	}
}

// appendRoadmapUnscheduledGroup appends the unscheduled bucket's containers
// and other leaf items at depth 0 -- each is its own root, exactly like a
// milestone-less scoped root, since there is no milestone item to hang them
// under.
func appendRoadmapUnscheduledGroup(items *[]FlatItem, ug roadmap.UnscheduledGroup) {
	total := len(ug.Containers) + len(ug.Other)
	i := 0
	for _, c := range ug.Containers {
		i++
		if c.Epic != nil {
			appendRoadmapEpicGroup(items, *c.Epic, 0, i == total)
		} else {
			appendRoadmapFeatureGroup(items, *c.Feature, 0, i == total)
		}
	}
	for _, leaf := range ug.Other {
		i++
		*items = append(*items, FlatItem{Item: leaf, Depth: 0, IsLast: i == total})
	}
}

// appendRoadmapEpicGroup appends an epic at depth, followed by its direct
// leaf items and nested feature groups at depth+1 -- items before features,
// per roadmap.tmpl.
func appendRoadmapEpicGroup(items *[]FlatItem, eg roadmap.EpicGroup, depth int, isLast bool) {
	*items = append(*items, FlatItem{Item: eg.Epic, Depth: depth, IsLast: isLast})

	childDepth := depth + 1
	total := len(eg.Items) + len(eg.Features)
	i := 0
	for _, leaf := range eg.Items {
		i++
		*items = append(*items, FlatItem{Item: leaf, Depth: childDepth, IsLast: i == total})
	}
	for _, fg := range eg.Features {
		i++
		appendRoadmapFeatureGroup(items, fg, childDepth, i == total)
	}
}

// appendRoadmapFeatureGroup appends a feature at depth, followed by its
// flattened leaf items at depth+1.
func appendRoadmapFeatureGroup(items *[]FlatItem, fg roadmap.FeatureGroup, depth int, isLast bool) {
	*items = append(*items, FlatItem{Item: fg.Feature, Depth: depth, IsLast: isLast})

	childDepth := depth + 1
	n := len(fg.Items)
	for i, leaf := range fg.Items {
		*items = append(*items, FlatItem{Item: leaf, Depth: childDepth, IsLast: i == n-1})
	}
}
