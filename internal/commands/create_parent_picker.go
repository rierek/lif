package commands

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/xRiErOS/lif/internal/ui"
	"github.com/xRiErOS/lif/pkg/candidates"
	"github.com/xRiErOS/lif/pkg/config"
	"github.com/xRiErOS/lif/pkg/item"
	"github.com/xRiErOS/lif/pkg/itemgraph"
	"github.com/xRiErOS/lif/pkg/roadmap"
)

const (
	// parentPickerBesideMinWidth is the form width from which the context
	// panel stands beside the list; below it the panel stands under it.
	parentPickerBesideMinWidth = 100
	// List height (title or filter line included) and panel height (border
	// included) per layout. Below: 12 + help 2 + 9 = 23 rows; beside: 22 + 2.
	parentPickerBesideHeight = 22
	parentPickerBelowHeight  = 12
	parentPickerPanelBelow   = 9
	// parentPickerNoParent is the label of the row that clears the parent.
	parentPickerNoParent = "(no parent)"
)

// parentPicker is the parent step of the `lif create -i` form: one huh
// select over the open parent candidates as a one-row-per-item roadmap tree,
// and a huh.Layout that joins the group view with a context panel for the
// highlighted row. It runs no bubbletea program of its own.
type parentPicker struct {
	ctx       context.Context
	resolver  *itemgraph.CoreResolver
	cfg       *config.Config
	itemType  *string
	parentID  *string
	field     *huh.Select[string]
	group     *huh.Group
	listWidth int // width of the group; written by GroupWidth only

	mu       sync.RWMutex // options are built in a tea.Cmd goroutine, View reads below
	total    int          // full form width
	byID     map[string]*item.Item
	children map[string][]*item.Item
	loadErr  error
}

// newParentPicker builds the picker. itemType points at the form's type
// value; the options follow it. parentID is the bound parent: pre-filled by
// the caller, written on selection, "" for "(no parent)".
func newParentPicker(ctx context.Context, resolver *itemgraph.CoreResolver, cfg *config.Config, itemType *string, parentID *string) *parentPicker {
	p := &parentPicker{ctx: ctx, resolver: resolver, cfg: cfg, itemType: itemType, parentID: parentID}

	p.field = huh.NewSelect[string]().
		Title("Parent").
		Value(parentID).
		OptionsFunc(p.options, []any{itemType, &p.listWidth}).
		Validate(func(string) error {
			p.mu.RLock()
			defer p.mu.RUnlock()
			return p.loadErr
		})
	p.group = huh.NewGroup(p.field)
	return p
}

// options recomputes the select options for the bound type and the row
// width. It is the huh OptionsFunc.
func (p *parentPicker) options() []huh.Option[string] {
	cands, err := candidates.ParentCandidates(p.ctx, p.resolver, p.cfg, nil, []string{*p.itemType})
	var all []*item.Item
	if err == nil {
		all, err = p.resolver.Items(p.ctx, nil)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.loadErr = err
	width := p.listWidth
	if err != nil {
		p.byID, p.children = nil, nil
		return []huh.Option[string]{huh.NewOption(ui.Truncate("Could not load parents: "+err.Error(), p.rowWidth(width)), "")}
	}

	p.byID = make(map[string]*item.Item, len(all))
	p.children = make(map[string][]*item.Item)
	for _, it := range all {
		p.byID[it.ID] = it
		if it.Parent != "" {
			p.children[it.Parent] = append(p.children[it.Parent], it)
		}
	}

	sortFn := func(s []*item.Item) { roadmap.SortSiblings(p.cfg, s) }
	opts := []huh.Option[string]{huh.NewOption(parentPickerNoParent, "")}
	isCandidate := false
	for _, fi := range ui.FlattenTree(ui.BuildTree(cands, cands, sortFn, nil)) {
		opts = append(opts, huh.NewOption(p.rowLabel(fi, width), fi.Item.ID))
		if fi.Item.ID == *p.parentID {
			isCandidate = true
		}
	}
	if !isCandidate {
		// A parent that is no candidate for this type must not stay bound:
		// the cursor would otherwise keep a stale index and write a stranger.
		*p.parentID = ""
	}
	return opts
}

// rowWidth is the cell budget of one option label inside a list of the given
// width: the field frame and the selector come off the width. 0 = unbounded.
func (p *parentPicker) rowWidth(listWidth int) int {
	if listWidth <= 0 {
		return 1 << 20
	}
	st := ui.FormTheme(ui.FlavourFormPalette(ui.ActiveTheme())).Focused
	return listWidth - st.Base.GetHorizontalFrameSize() - lipgloss.Width(st.SelectSelector.String())
}

// shortLine is "short-id type title" of one item.
func (p *parentPicker) shortLine(it *item.Item) (head, title string) {
	return item.IDSuffix(it.ID, p.cfg.Items.Prefix) + " " + p.cfg.ShortOf(it.Type) + " ", it.Title
}

// rowLabel builds the one-row label: tree prefix, short ID, type, title. Only
// the title is cut, so prefix and ID stay intact.
func (p *parentPicker) rowLabel(fi ui.FlatItem, listWidth int) string {
	head, title := p.shortLine(fi.Item)
	head = fi.TreePrefix + head
	room := p.rowWidth(listWidth) - ui.DisplayWidth(head)
	if room < 1 {
		return ui.Truncate(head+title, p.rowWidth(listWidth))
	}
	return head + ui.Truncate(title, room)
}

// pathTo returns the ancestors of id from the root down to id itself.
func (p *parentPicker) pathTo(id string) []*item.Item {
	var path []*item.Item
	seen := map[string]bool{}
	for it := p.byID[id]; it != nil && !seen[it.ID]; it = p.byID[it.Parent] {
		seen[it.ID] = true
		path = append(path, it)
	}
	slices.Reverse(path)
	return path
}

// openChildren returns the children of id outside an archive status, in
// roadmap order.
func (p *parentPicker) openChildren(id string) []*item.Item {
	var out []*item.Item
	for _, c := range p.children[id] {
		if !p.cfg.IsArchiveStatus(c.Status) {
			out = append(out, c)
		}
	}
	roadmap.SortSiblings(p.cfg, out)
	return out
}

// panel renders the context panel for the highlighted option: width and
// height are outer cells, border included. Content beyond the height is cut
// with a "… N more" marker.
func (p *parentPicker) panel(width, height int) string {
	pal := ui.FlavourFormPalette(ui.ActiveTheme())
	text := func(c lipgloss.Color) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(c).Background(pal.Background)
	}
	inner := width - 4
	row := func(st lipgloss.Style, s string) string { return st.Render(ui.Truncate(s, inner)) }

	var lines []string
	hovered, ok := p.field.Hovered()
	p.mu.RLock()
	switch {
	case !ok:
		lines = append(lines, row(text(pal.Muted), "No matching parent"))
	case hovered == "":
		lines = append(lines, row(text(pal.Muted), "Created at the top level"))
	default:
		lines = append(lines, row(text(pal.Accent).Bold(true), "Path"))
		for depth, it := range p.pathTo(hovered) {
			head, title := p.shortLine(it)
			lines = append(lines, row(text(pal.Text), strings.Repeat("  ", depth)+head+title))
		}
		kids := p.openChildren(hovered)
		lines = append(lines, row(text(pal.Accent).Bold(true), fmt.Sprintf("Open children (%d)", len(kids))))
		for _, c := range kids {
			head, title := p.shortLine(c)
			lines = append(lines, row(text(pal.Text), head+title))
		}
	}
	p.mu.RUnlock()

	if budget := height - 2; len(lines) > budget {
		cut := len(lines) - (budget - 1)
		lines = append(lines[:budget-1], row(text(pal.Muted), fmt.Sprintf("… %d more", cut)))
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(pal.Muted).BorderBackground(pal.Background).
		Background(pal.Background).
		Padding(0, 1).
		Width(width - 2).Height(height - 2).
		Render(strings.Join(lines, "\n"))
}

// GroupWidth makes the picker a huh.Layout. Any group other than the
// picker's renders exactly as huh.LayoutDefault. The picker group gets the
// list share of the form width and every other group the full width. It also
// fixes the list height, because the form sizes its groups right after this
// call.
func (p *parentPicker) GroupWidth(_ *huh.Form, g *huh.Group, w int) int {
	if g != p.group {
		return w
	}
	p.total = w
	if w < parentPickerBesideMinWidth {
		p.listWidth = w
		p.field.Height(parentPickerBelowHeight)
		return w
	}
	p.listWidth = w * 55 / 100
	p.field.Height(parentPickerBesideHeight)
	return p.listWidth
}

func (p *parentPicker) View(f *huh.Form) string {
	view := huh.LayoutDefault.View(f)
	if f.GetFocusedField() != huh.Field(p.field) {
		return view
	}
	total := p.total
	if total == 0 {
		total = 80
	}
	if total >= parentPickerBesideMinWidth {
		return lipgloss.JoinHorizontal(lipgloss.Top, view, " ", p.panel(total-p.listWidth-1, parentPickerBesideHeight))
	}
	return lipgloss.JoinVertical(lipgloss.Left, view, p.panel(total, parentPickerPanelBelow))
}
