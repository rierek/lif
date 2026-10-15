package tui

// context.go — Yank `y`'s Markdown-Kontext-Generator (E5 Task 3, bean
// bt-e4a6, epic-E5-plan.md »Task 3«, design decision b): ONE view-agnostic
// beanContext(idx, b) instead of devd's three separate milestoneClip/
// sprintClip/backlogClip functions (internal/tui/context.go in devd's own
// cli-go) -- beans has no devd Milestone/Sprint/Issue three-way split, so a
// single generator covers both a leaf Issue (no "## Children" section) and
// an Epic/Milestone (idx.Children[b.ID] non-empty -> "## Children" table)
// via the same code path. `y` always acts on m.focusedBean() (Tree/Backlog
// identical, update.go's keyNodeAction) -- the Review-Cockpit used to be a
// view-local exception here (its own reviewStandMarkdown override instead of
// beanContext), removed by E7 T1 (PF-14, bean bt-wmtb, 2026-07-15).
//
// Relation resolution (Parent/Blocking/Blocked By) mirrors view_detail_bean.
// go's resolveSorted: resolved via idx.ByID to "ID Title", canonically
// sorted (data.SortBeans) among the resolved half, with a dangling
// reference (beans-legal) rendered as "ID (unresolved)" rather than
// silently dropped.

import (
	"fmt"
	"strings"

	"github.com/rierek/lif/internal/tui/data"
)

// beanContext renders b as Markdown for the clipboard: a header (ID/Title/
// Status/Type/Priority/Tags/Parent/Blocking/Blocked By) + Body, PLUS a
// "## Children" table when idx.Children[b.ID] is non-empty. nil b returns
// an empty string (defensive -- callers already guard the orphan-root
// cursor before reaching here, update.go keyNodeAction Plan Step 6).
func beanContext(idx *data.Index, b *data.Bean) string {
	if b == nil {
		return ""
	}

	var out strings.Builder
	fmt.Fprintf(&out, "# %s — %s\n\n", b.ID, b.Title)
	fmt.Fprintf(&out, "- Status: %s\n", b.Status)
	fmt.Fprintf(&out, "- Type: %s\n", b.Type)
	fmt.Fprintf(&out, "- Priority: %s\n", contextPriority(b.Priority))
	if len(b.Tags) > 0 {
		fmt.Fprintf(&out, "- Tags: %s\n", strings.Join(b.Tags, ", "))
	}
	if b.Parent != "" {
		fmt.Fprintf(&out, "- Parent: %s\n", contextRelLabel(idx, b.Parent))
	}
	if len(b.Blocking) > 0 {
		fmt.Fprintf(&out, "- Blocking: %s\n", contextRelLabels(idx, b.Blocking))
	}
	if len(b.BlockedBy) > 0 {
		fmt.Fprintf(&out, "- Blocked By: %s\n", contextRelLabels(idx, b.BlockedBy))
	}

	if strings.TrimSpace(b.Body) != "" {
		fmt.Fprintf(&out, "\n%s\n", b.Body)
	}

	if idx != nil {
		if children := idx.Children[b.ID]; len(children) > 0 {
			out.WriteString("\n## Children\n\n")
			out.WriteString(contextBeanTable(children))
		}
	}

	return out.String()
}

// contextPriority applies the same "" -> "normal" default every other view
// in this package already uses (metaSectionBody, view_detail_bean.go).
func contextPriority(p string) string {
	if p == "" {
		return "normal"
	}
	return p
}

// contextRelLabel resolves a single bean ID against idx.ByID to "ID Title"
// -- falls back to "ID (unresolved)" for a dangling reference (beans-legal,
// mirrors view_detail_bean.go's resolveSorted fallback).
func contextRelLabel(idx *data.Index, id string) string {
	if idx != nil {
		if rel, ok := idx.ByID[id]; ok {
			return id + " " + rel.Title
		}
	}
	return id + " (unresolved)"
}

// contextRelLabels resolves a slice of IDs (Blocking/Blocked By) to
// comma-joined "ID Title" labels: the resolved half canonically sorted
// (data.SortBeans, mirrors resolveSorted), unresolved IDs appended last in
// their input order.
func contextRelLabels(idx *data.Index, ids []string) string {
	var resolved []*data.Bean
	var unresolved []string
	for _, id := range ids {
		if idx != nil {
			if rel, ok := idx.ByID[id]; ok {
				resolved = append(resolved, rel)
				continue
			}
		}
		unresolved = append(unresolved, id)
	}
	data.SortBeans(resolved)

	labels := make([]string, 0, len(resolved)+len(unresolved))
	for _, rel := range resolved {
		labels = append(labels, rel.ID+" "+rel.Title)
	}
	for _, id := range unresolved {
		labels = append(labels, id+" (unresolved)")
	}
	return strings.Join(labels, ", ")
}

// contextBeanTable renders an already-resolved, already-sorted bean slice
// (idx.Children -- NewIndex sorts via sortBeans at build time, no re-sort
// needed here) as a Markdown table: ID | Type | Status | Prio | Title.
// Used by beanContext's Children table (the former Review-Cockpit's
// reviewStandMarkdown per-group/Rework tables shared this too, removed by
// E7 T1, PF-14, bean bt-wmtb, 2026-07-15).
func contextBeanTable(beans []*data.Bean) string {
	var out strings.Builder
	out.WriteString("| ID | Type | Status | Prio | Title |\n|---|---|---|---|---|\n")
	for _, b := range beans {
		fmt.Fprintf(&out, "| %s | %s | %s | %s | %s |\n", b.ID, b.Type, b.Status, contextPriority(b.Priority), contextOneline(b.Title))
	}
	return out.String()
}

// contextOneline collapses newlines/pipes for a Markdown table cell (devd
// port: context.go's own `oneline`).
func contextOneline(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.TrimSpace(s)
}
