package tui

// context.go — Yank's clipboard text generator. `y` is a prefix (update.go
// handleKey, yankPrefixKey); the second key picks one of five variants of
// the focused item:
//
//	yy  full: the item file as on disk (front matter + body), plus a
//	    "## Children" table when the item has children
//	ym  meta: the complete front matter, no body (same text as a piped
//	    `lif show --meta`)
//	yt  meta + toc: the front matter plus the body's heading outline
//	yi  item: one "ID - Title" line
//	yl  list: the yi line plus one "  +-- ID - Title" line per direct child
//	    the active search/facets/archive visibility keep (yankChildren)
//
// The front matter comes from item.Render, never a hand-picked field list,
// so every field the file carries (Extra, created/updated, order, ...) lands
// in the clipboard.

import (
	"fmt"
	"strings"

	"github.com/rierek/lif/internal/tui/data"
	"github.com/rierek/lif/pkg/item"
)

// yankMode selects the variant yankText renders.
type yankMode int

const (
	yankFull yankMode = iota
	yankMeta
	yankTOC
	yankItem
	yankList
)

// yankChildren returns the direct children of parentID that pass the same
// membership test as the flat roadmap list (flatMatchGroup), in idx.Children
// order. Folds, scroll position and Ready membership do not constrain it.
func (m model) yankChildren(parentID string) []*item.Item {
	if m.idx == nil {
		return nil
	}
	grouped := strings.TrimSpace(m.searchPrefixRest) != ""
	target := m.searchTarget()
	var out []*item.Item
	for _, c := range m.idx.Children[parentID] {
		if m.flatMatchGroup(target, c, grouped) != groupNone {
			out = append(out, c)
		}
	}
	return out
}

// yankText renders b in the chosen mode. nil b returns "" (defensive --
// callers already guard the orphan-root cursor). children only feeds yankList.
func yankText(idx *data.Index, b *item.Item, mode yankMode, children []*item.Item) (string, error) {
	if b == nil {
		return "", nil
	}
	if mode == yankItem || mode == yankList {
		var out strings.Builder
		out.WriteString(b.ID + " - " + b.Title + "\n")
		if mode == yankList {
			for _, c := range children {
				out.WriteString("  +-- " + c.ID + " - " + c.Title + "\n")
			}
		}
		return out.String(), nil
	}
	source := b
	if mode != yankFull {
		source = b.Clone()
		source.Body = ""
	}
	content, err := source.Render()
	if err != nil {
		return "", fmt.Errorf("render item %s: %w", b.ID, err)
	}

	text := string(content)
	if mode != yankFull {
		text = stripFrontMatterFence(text)
	}
	section := func(title, body string) string {
		return strings.TrimRight(text, "\n") + "\n\n## " + title + "\n\n" + body
	}
	switch mode {
	case yankFull:
		if idx != nil {
			if children := idx.Children[b.ID]; len(children) > 0 {
				return section("Children", contextBeanTable(children)), nil
			}
		}
	case yankTOC:
		if toc := bodyTOC(b.Body); toc != "" {
			return section("TOC", toc), nil
		}
	}
	return text, nil
}

// stripFrontMatterFence drops the opening and closing "---" lines of a
// rendered front matter block, leaving only the key: value lines.
func stripFrontMatterFence(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "---\n")
	s = strings.TrimSuffix(s, "\n---")
	return s + "\n"
}

// bodyTOC returns the ATX headings of a Markdown body as a nested list, one
// two-space indent per level below the shallowest heading. Lines inside
// fenced code blocks (``` or ~~~) are not headings.
func bodyTOC(body string) string {
	type heading struct {
		level int
		text  string
	}
	var hs []heading
	minLevel := 7
	fence := ""
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fence = trimmed[:3]
			continue
		}
		level := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
		if level == 0 || level > 6 || (len(trimmed) > level && trimmed[level] != ' ') {
			continue
		}
		text := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(trimmed[level:]), "#"))
		if text == "" {
			continue
		}
		hs = append(hs, heading{level, text})
		minLevel = min(minLevel, level)
	}
	var out strings.Builder
	for _, h := range hs {
		fmt.Fprintf(&out, "%s- %s\n", strings.Repeat("  ", h.level-minLevel), h.text)
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

// contextBeanTable renders an already-resolved, already-sorted bean slice
// (idx.Children -- NewIndex sorts via sortBeans at build time, no re-sort
// needed here) as a Markdown table: ID | Type | Status | Prio | Title.
func contextBeanTable(beans []*item.Item) string {
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
