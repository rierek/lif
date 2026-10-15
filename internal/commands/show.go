package commands

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph"
	"github.com/rierek/lif/pkg/itemgraph/model"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

var (
	showJSON     bool
	showRaw      bool
	showBodyOnly bool
	showETagOnly bool
	showMeta     bool
	showTable    bool
	showMaxWidth int
	showParent   string

	showStatus     []string
	showNoStatus   []string
	showType       []string
	showNoType     []string
	showPriority   []string
	showNoPriority []string
	showTag        []string
	showNoTag      []string
)

var showCmd = &cobra.Command{
	Use:   "show <id> [id...]",
	Short: "Show an item's contents",
	Long: `Displays the full contents of one or more items, including front matter and body.

The representation follows stdout. On a terminal the output is styled and the
body is rendered as markdown. When stdout is a pipe or a file, the output is the
raw markdown of the source file — the same text --raw produces, unpadded and
unwrapped, so it can be fed to a parser.

--meta drops the body from either representation and keeps the front matter:
the styled header on a terminal, the source YAML block off one.

--parent <id> adds that item's children to the ids given, so a container and
its children reach one page without naming each child. Given ids come first,
then the children in the order list --parent uses; an item named twice is shown
once. Without ids, --parent shows the children alone.

--status, --type, --priority and --tag, and their --no-* forms, narrow every
item shown — the ids given and the --parent children alike — with the same
meaning as in list. Each takes a comma-separated list or can be repeated. A
filter that leaves nothing to show is an error.`,
	// --parent supplies the ids, so a bare `show --parent <id>` is complete
	// while a bare `show` is still a usage error.
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && showParent == "" {
			return fmt.Errorf("requires at least 1 arg(s), only received 0")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		resolver := &itemgraph.CoreResolver{Core: core}

		// Collect all items
		var items []*item.Item
		ids := args
		if showParent != "" {
			children, err := showChildIDs(resolver, showParent)
			if err != nil {
				if showJSON {
					return output.Error(output.ErrNotFound, err.Error())
				}
				return err
			}
			ids = append(ids, children...)
		}
		seen := make(map[string]bool, len(ids))
		for _, id := range ids {
			if seen[id] {
				continue
			}
			seen[id] = true
			b, err := resolver.Item(context.Background(), id)
			if err != nil {
				if showJSON {
					return output.Error(output.ErrNotFound, err.Error())
				}
				return fmt.Errorf("failed to find item: %w", err)
			}
			if b == nil {
				if showJSON {
					return output.Error(output.ErrNotFound, fmt.Sprintf("item not found: %s", id))
				}
				return fmt.Errorf("item not found: %s", id)
			}
			items = append(items, b)
		}
		items = itemgraph.ApplyFilter(items, &model.ItemFilter{
			Status: showStatus, ExcludeStatus: showNoStatus,
			Type: showType, ExcludeType: showNoType,
			Priority: showPriority, ExcludePriority: showNoPriority,
			Tags: showTag, ExcludeTags: showNoTag,
		}, core)
		if len(items) == 0 {
			if showJSON {
				return output.Error(output.ErrNotFound, "no item matches the filters")
			}
			return fmt.Errorf("no item matches the filters")
		}

		// JSON output
		if showJSON {
			if len(items) == 1 {
				return output.SuccessSingle(items[0])
			}
			return output.SuccessMultiple(items)
		}

		// Raw markdown output (frontmatter + body)
		if showRaw {
			for i, b := range items {
				if i > 0 {
					fmt.Print("\n---\n\n")
				}
				content, err := b.Render()
				if err != nil {
					return fmt.Errorf("failed to render item: %w", err)
				}
				fmt.Print(string(content))
			}
			return nil
		}

		// Body only (no header, no styling)
		if showBodyOnly {
			for i, b := range items {
				if i > 0 {
					fmt.Print("\n---\n\n")
				}
				fmt.Print(b.Body)
			}
			return nil
		}

		// ETag only (for easy extraction in scripts)
		if showETagOnly {
			for i, b := range items {
				if i > 0 {
					fmt.Println()
				}
				fmt.Print(b.ETag())
			}
			return nil
		}

		isTTY := term.IsTerminal(int(os.Stdout.Fd()))
		width := resolveWidth(showMaxWidth, cmd.Flags().Changed("max-width"), cfg)

		// --table forces the grid in both directions, the way --raw forces
		// raw markdown on a terminal: an explicit arrangement flag outranks
		// the representation stdout would otherwise pick.
		if showTable {
			labelWidth := itemTableLabelWidth(items, cfg)
			for i, b := range items {
				if i > 0 {
					fmt.Println()
				}
				out, err := showOutputTable(b, showMeta, width, labelWidth)
				if err != nil {
					return err
				}
				fmt.Print(out)
			}
			return nil
		}

		// Default: styled for a terminal, raw markdown for a pipe or a file
		out, err := showOutputAll(items, isTTY, showMeta, width)
		if err != nil {
			return err
		}
		fmt.Print(out)

		return nil
	},
}

// showOutput returns the text for a single item, choosing the representation
// from whether stdout is a terminal. metaOnly drops the body from either
// representation, leaving the front matter -- styled off the header for a
// terminal, and the source YAML block for a pipe, which still parses as a
// item file with an empty body.
func showOutput(b *item.Item, isTTY, metaOnly bool, width int) (string, error) {
	if !isTTY {
		source := b
		if metaOnly {
			source = b.Clone()
			source.Body = ""
		}
		content, err := source.Render()
		if err != nil {
			return "", fmt.Errorf("failed to render item: %w", err)
		}
		return string(content), nil
	}
	if metaOnly {
		return renderItemHeader(b, cfg, width), nil
	}
	return styledItemOutput(b, width)
}

// showOutputAll joins the output of several items with the separator that
// belongs to the chosen representation.
//
// Piped --meta output needs no separator: each block already ends with the
// front matter's own closing "---" plus a blank line, and adding the raw
// separator on top of that produced an empty third document between every
// pair of items.
// showChildIDs resolves --parent to its children's ids, in the order the
// resolver sorts them, so the page matches `list --parent` and `roadmap`
// rather than inventing a third order. It goes through
// CoreResolver.ItemChildren instead of re-deriving the parent link, which
// keeps one definition of "child" in the codebase.
//
// Both empty cases are errors: an unknown parent is a typo, and a childless
// one would render an empty page that reads like a broken command.
func showChildIDs(resolver *itemgraph.CoreResolver, parentID string) ([]string, error) {
	parent, err := resolver.Item(context.Background(), parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to find --parent item: %w", err)
	}
	if parent == nil {
		return nil, fmt.Errorf("item not found: %s", parentID)
	}
	children, err := resolver.ItemChildren(context.Background(), parent, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve children of %s: %w", parentID, err)
	}
	if len(children) == 0 {
		return nil, fmt.Errorf("item has no children: %s", parentID)
	}
	ids := make([]string, 0, len(children))
	for _, c := range children {
		ids = append(ids, c.ID)
	}
	return ids, nil
}

func showOutputAll(items []*item.Item, isTTY, metaOnly bool, width int) (string, error) {
	separator := "\n---\n\n"
	switch {
	case isTTY:
		separator = "\n" + ui.Muted.Render(strings.Repeat("═", 60)) + "\n\n"
	case metaOnly:
		separator = ""
	}

	var out strings.Builder
	for i, b := range items {
		if i > 0 {
			out.WriteString(separator)
		}
		text, err := showOutput(b, isTTY, metaOnly, width)
		if err != nil {
			return "", err
		}
		out.WriteString(text)
	}
	return out.String(), nil
}

// showOutputTable renders one item as the label/value grid. metaOnly keeps
// the grid alone; otherwise the body follows, separated by the same rule the
// styled detail view uses.
//
// The width is resolved by the caller through resolveWidth, so --max-width,
// display.max_width and the built-in default rank exactly as they do for
// lif list -- one width policy for the whole CLI rather than a second one
// here. Taking it as a parameter also keeps this function out of showCmd's
// initialisation cycle, which a flag lookup from here would create.
func showOutputTable(b *item.Item, metaOnly bool, width, labelWidth int) (string, error) {
	var sb strings.Builder
	sb.WriteString(renderItemTable(b, cfg, width, labelWidth))
	if metaOnly {
		return sb.String(), nil
	}

	if body := ui.RenderMarkdown(b.Body, min(width, 90)); body != "" {
		sb.WriteString("\n" + body + "\n")
	}
	return sb.String(), nil
}

// styledItemOutput builds the styled representation of a single item at the
// width the caller resolved. It used to resolve its own width from
// resolveWidth(0, false, cfg), which ignored --max-width unless --table was
// also given -- a flag that works in one combination and not in another.
func styledItemOutput(b *item.Item, width int) (string, error) {
	return renderItemDetail(b, cfg, width), nil
}

// renderItemDetail lays out one item for the terminal: the attribute header,
// then the rendered body.
//
// The body goes through ui.RenderMarkdown (Task 16) instead of glamour: that
// renderer emits no trailing padding and no painted backgrounds, which is
// exactly what glamour got wrong.
func renderItemDetail(b *item.Item, cfg *config.Config, width int) string {
	var sb strings.Builder

	sb.WriteString(renderItemHeader(b, cfg, width))
	sb.WriteString(ui.TreeLine.Render(strings.Repeat("─", width)) + "\n\n")

	if body := ui.RenderMarkdown(b.Body, min(width, 90)); body != "" {
		sb.WriteString(body + "\n")
	}
	return sb.String()
}

// renderItemHeader renders every front matter field the item carries, so the
// styled view withholds nothing the file holds: the attribute header reads
// the same order vertically that the items table reads horizontally -- type,
// id, then title, then status and priority -- so type, id and title carry the
// type's colour and weight and the header reads as one unit.
//
// Tags sit in the header rather than after the body, where a long item hid
// them below a screenful of markdown. Relationships, unknown ("extra") front
// matter keys, order and the timestamps follow, each labelled, which is what
// makes this the whole front matter and not a selection of it.
func renderItemHeader(b *item.Item, cfg *config.Config, width int) string {
	var sb strings.Builder

	tint := ""
	bold := false
	if tc := cfg.GetType(b.Type); tc != nil {
		tint, bold = tc.Color, tc.Emphasis
	}
	head := lipgloss.NewStyle().Bold(bold)
	if tint != "" {
		head = head.Foreground(ui.ResolveColor(tint))
	}

	first := head.Render(b.ID)
	if b.Type != "" {
		// An unconditional separator would leave a stray two-space lead
		// before the id when an item has no type.
		first = head.Render(b.Type) + "  " + first
	}
	sb.WriteString(first + "\n")
	sb.WriteString(head.Render(b.Title) + "\n")

	var attrs []string

	// Status and priority render whenever a value is present, falling back
	// to the legacy "gray" alias when the config has no matching entry --
	// the raw value is exactly what a reader needs to see in that case, not
	// less of it. "normal" priority is not filtered here: hiding it is a
	// table-only density decision (Task 10's prioCell), not a detail-view
	// one -- a detail view shows one item and has no density to buy.
	statusColor, statusBold := "gray", true
	if sc := cfg.GetStatus(b.Status); sc != nil {
		statusColor, statusBold = sc.Color, !sc.Archive
	}
	attrs = append(attrs, lipgloss.NewStyle().Foreground(ui.ResolveColor(statusColor)).
		Bold(statusBold).Render(b.Status))

	if b.Priority != "" {
		priorityColor := "gray"
		if pc := cfg.GetPriority(b.Priority); pc != nil {
			priorityColor = pc.Color
		}
		attrs = append(attrs, lipgloss.NewStyle().Foreground(ui.ResolveColor(priorityColor)).
			Render(b.Priority))
	}
	if implicitStatus, implicitStatusFrom := core.ImplicitStatus(b.ID); implicitStatus != "" {
		attrs = append(attrs, ui.Muted.Render("↑"+implicitStatus+" (from "+implicitStatusFrom+")"))
	}
	if len(attrs) > 0 {
		sb.WriteString(strings.Join(attrs, "  ") + "\n")
	}

	// Tags keep the table's "#tag" spelling so one reader recognises them
	// across both views.
	if len(b.Tags) > 0 {
		parts := make([]string, len(b.Tags))
		for i, t := range b.Tags {
			parts[i] = "#" + t
		}
		// ui.Secondary resolves to the same tone as ui.Muted (both
		// "overlay1"), so styling the values would only repeat the label's
		// colour. Plain foreground gives them the same weight every other
		// labelled value in this header has.
		sb.WriteString(ui.Muted.Render("tags:") + " " + strings.Join(parts, " ") + "\n")
	}

	if rel := formatRelationships(b); rel != "" {
		sb.WriteString(rel + "\n")
	}

	// Extra keys are front matter the schema does not name -- policy fields
	// like branch, commit or release. They are part of the item, so they are
	// part of its detail view; sorted, because a map has no order and a
	// reader needs a stable one.
	if len(b.Extra) > 0 {
		keys := make([]string, 0, len(b.Extra))
		for k := range b.Extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(ui.Muted.Render(k+":") + " " + formatExtraValue(b.Extra[k]) + "\n")
		}
	}

	// Attachments sit with the extra keys rather than with the stamps:
	// they are content the item carries, not bookkeeping the tool wrote.
	if att := attachmentValue(b.ID); att != "" {
		sb.WriteString(ui.Muted.Render("attachments:") + " " + att + "\n")
	}

	// created/updated/order: managed metadata, muted on one trailing line.
	var stamps []string
	if b.CreatedAt != nil {
		stamps = append(stamps, "created "+b.CreatedAt.Format("2006-01-02 15:04 UTC"))
	}
	if b.UpdatedAt != nil {
		stamps = append(stamps, "updated "+b.UpdatedAt.Format("2006-01-02 15:04 UTC"))
	}
	if b.Order != "" {
		stamps = append(stamps, "order "+b.Order)
	}
	if len(stamps) > 0 {
		sb.WriteString(ui.Muted.Render(strings.Join(stamps, "  ")) + "\n")
	}

	return wrapHeaderLines(sb.String(), width)
}

// wrapHeaderLines folds every header line to width visible cells, hanging
// the continuation under the value rather than under the label.
//
// It runs over the assembled header instead of inside each of the eight
// write sites above: the width concern is uniform, and threading it through
// every branch would put the same three lines in eight places. A value that
// carries no "label:" prefix -- the type/id and title lines -- wraps flush,
// because there is no label to hang under.
func wrapHeaderLines(header string, width int) string {
	var sb strings.Builder
	for _, line := range strings.Split(header, "\n") {
		if line == "" {
			continue
		}
		if visibleWidth(line) <= width {
			sb.WriteString(line + "\n")
			continue
		}

		indent := ""
		if plain := stripANSI(line); strings.Contains(plain, ": ") {
			label := plain[:strings.Index(plain, ": ")+2]
			if !strings.Contains(strings.TrimSuffix(label, ": "), " ") {
				indent = strings.Repeat(" ", ui.DisplayWidth(label))
			}
		}

		for i, folded := range wrapVisible(line, width-ui.DisplayWidth(indent)) {
			if i == 0 {
				sb.WriteString(folded + "\n")
				continue
			}
			sb.WriteString(indent + folded + "\n")
		}
	}
	return sb.String()
}

// formatExtraValue renders one unknown front matter value as a single line.
// Scalars print as themselves; a sequence or mapping goes through YAML in
// flow style -- "[a, b]", "{k: v}" -- which is the value's own notation on
// one line, rather than Go's map[...] debug spelling.
//
// The flow style has to be set on the encoded node: yaml.Marshal defaults to
// block style, and folding that back onto one line by collapsing whitespace
// yields the block markers without their meaning ("- a - b").
func formatExtraValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []any, map[string]any, map[any]any:
		var node yaml.Node
		if err := node.Encode(v); err != nil {
			return fmt.Sprint(v)
		}
		setFlowStyle(&node)
		out, err := yaml.Marshal(&node)
		if err != nil {
			return fmt.Sprint(v)
		}
		return strings.TrimRight(string(out), "\n")
	default:
		return fmt.Sprint(v)
	}
}

// setFlowStyle marks a node and everything under it as flow style, so a
// nested sequence inside a mapping stays on the same line as its parent.
func setFlowStyle(n *yaml.Node) {
	if n.Kind == yaml.SequenceNode || n.Kind == yaml.MappingNode {
		n.Style = yaml.FlowStyle
	}
	for _, child := range n.Content {
		setFlowStyle(child)
	}
}

// formatRelationships formats parent, blocking and blocked_by for display.
// blocked_by is the direction a reader acts on -- what has to finish before
// this item can move -- and was the one relationship the detail view left
// out.
func formatRelationships(b *item.Item) string {
	var parts []string

	if b.Parent != "" {
		parts = append(parts, fmt.Sprintf("%s %s",
			ui.Muted.Render("parent:"),
			ui.ID.Render(b.Parent)))
	}

	for _, target := range b.Blocking {
		parts = append(parts, fmt.Sprintf("%s %s",
			ui.Muted.Render("blocking:"),
			ui.ID.Render(target)))
	}

	for _, blocker := range b.BlockedBy {
		parts = append(parts, fmt.Sprintf("%s %s",
			ui.Muted.Render("blocked by:"),
			ui.ID.Render(blocker)))
	}
	return strings.Join(parts, "\n")
}

// attachmentValue is the displayed value of the attachments row: the file
// names in .beans/attachments/<id>/, space-separated, empty when the item
// has none.
//
// It reads the directory on every render instead of a front matter key.
// The directory is already keyed by the item id, so a key would be a second
// list of the same files that goes stale at the first edit outside the CLI
// -- derived, not asserted. A read failure is shown rather than swallowed,
// the same choice attachmentOrphanIssues makes: an unreadable directory is
// exactly the case where silence hides the drift.
func attachmentValue(id string) string {
	names, err := core.AttachmentNames(id)
	if err != nil {
		return fmt.Sprintf("<unreadable: %v>", err)
	}
	return strings.Join(names, " ")
}

func RegisterShowCmd(root *cobra.Command) {
	showCmd.Flags().BoolVar(&showJSON, "json", false, "Output as JSON")
	showCmd.Flags().BoolVar(&showRaw, "raw", false, "Force raw markdown output even on a terminal (already the default off a terminal)")
	showCmd.Flags().BoolVar(&showBodyOnly, "body-only", false, "Output only the body content")
	showCmd.Flags().BoolVar(&showETagOnly, "etag-only", false, "Output only the etag")
	showCmd.Flags().BoolVar(&showMeta, "meta", false, "Output only the front matter, without the body")
	showCmd.Flags().BoolVar(&showTable, "table", false,
		"Arrange the front matter as a label/value grid (forces the grid into a pipe too)")
	showCmd.Flags().StringVar(&showParent, "parent", "",
		"Also show the children of this item, in the order list --parent uses")
	showCmd.Flags().IntVar(&showMaxWidth, "max-width", 0,
		"Cap the rendered width; 0 disables the cap (default: display.max_width, else 110)")
	showCmd.Flags().StringSliceVarP(&showStatus, "status", "s", nil, "Filter by status (comma-separated or repeated)")
	showCmd.Flags().StringSliceVar(&showNoStatus, "no-status", nil, "Exclude by status (comma-separated or repeated)")
	showCmd.Flags().StringSliceVarP(&showType, "type", "t", nil, "Filter by type (comma-separated or repeated)")
	showCmd.Flags().StringSliceVar(&showNoType, "no-type", nil, "Exclude by type (comma-separated or repeated)")
	showCmd.Flags().StringSliceVarP(&showPriority, "priority", "p", nil, "Filter by priority (comma-separated or repeated)")
	showCmd.Flags().StringSliceVar(&showNoPriority, "no-priority", nil, "Exclude by priority (comma-separated or repeated)")
	showCmd.Flags().StringSliceVar(&showTag, "tag", nil, "Filter by tag (comma-separated or repeated, OR logic)")
	showCmd.Flags().StringSliceVar(&showNoTag, "no-tag", nil, "Exclude items with tag (comma-separated or repeated)")
	_ = showCmd.RegisterFlagCompletionFunc("status", statusFlagCompletion)
	_ = showCmd.RegisterFlagCompletionFunc("type", typeFlagCompletion)
	_ = showCmd.RegisterFlagCompletionFunc("priority", priorityFlagCompletion)
	_ = showCmd.RegisterFlagCompletionFunc("tag", tagFlagCompletion)
	// Two groups rather than one: --meta and --table each exclude the four
	// wholesale representations, but not each other -- "--meta --table" is
	// the combination the grid exists for.
	showCmd.MarkFlagsMutuallyExclusive("json", "raw", "body-only", "etag-only", "meta")
	showCmd.MarkFlagsMutuallyExclusive("json", "raw", "body-only", "etag-only", "table")
	showCmd.ValidArgsFunction = completionUnbounded
	// --parent matches any existing item ID, like list --parent and unlike
	// create/update --parent, which offer type-eligible new parents.
	_ = showCmd.RegisterFlagCompletionFunc("parent", listParentFlagCompletion)
	root.AddCommand(showCmd)
}
