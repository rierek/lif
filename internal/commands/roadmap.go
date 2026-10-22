package commands

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/roadmap"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

//go:embed roadmap.tmpl
var roadmapTemplateContent string

var (
	roadmapJSON        bool
	roadmapIncludeDone bool
	roadmapStatus      []string
	roadmapNoStatus    []string
	roadmapNoLinks     bool
	roadmapLinkPrefix  string
	roadmapDepth       int
	roadmapTags        bool
	roadmapView        string
	roadmapFormat      string
	roadmapWidthFlag   int
)

var roadmapCmd = &cobra.Command{
	Use:   "roadmap [id]",
	Short: "Generate a Markdown roadmap from milestones and epics",
	Long: `Displays a roadmap of milestones, epics, and their child items.

With no argument, renders the entire roadmap. With an ID argument (a milestone,
epic, or feature), scopes the output to that item's subtree only.

The --status and --no-status flags cannot be combined with an ID argument.

--depth limits how many levels below the roadmap root are rendered, following
the tree -L n convention: the root itself never counts. Without an ID argument the root is
the roadmap as a whole, so --depth 1 lists milestones only. With an ID
argument the root is that item, so --depth 1 lists its direct children.

--tags renders each item's tags on a line of their own beneath its title.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Query all items via GraphQL resolver
		resolver := newResolver()
		allItems, err := resolver.Items(context.Background(), nil)
		if err != nil {
			return cmdError(roadmapJSON, output.ErrValidation, "querying items: %v", err)
		}

		if err := validateRoadmapDepth(roadmapDepth, cmd.Flags().Changed("depth")); err != nil {
			return cmdError(roadmapJSON, output.ErrValidation, "%s", err)
		}

		form, ok := ui.ParseForm(roadmapView)
		if !ok {
			return cmdError(roadmapJSON, output.ErrValidation, "invalid --view %q: must be one of \"tree\", \"table\"", roadmapView)
		}

		var formatOverride roadmapFormatOverride
		switch roadmapFormat {
		case "":
			formatOverride = roadmapFormatAuto
		case "tty":
			formatOverride = roadmapFormatTTY
		case "markdown":
			formatOverride = roadmapFormatMarkdown
		default:
			return cmdError(roadmapJSON, output.ErrValidation, "invalid --format %q: must be one of \"tty\", \"markdown\"", roadmapFormat)
		}

		// Build the roadmap
		var data *roadmap.Data
		scoped := len(args) == 1
		if scoped {
			if len(roadmapStatus) > 0 || len(roadmapNoStatus) > 0 {
				return cmdError(roadmapJSON, output.ErrValidation, "--status/--no-status cannot be combined with a roadmap root ID")
			}
			root, err := core.Get(args[0])
			if err != nil {
				return cmdError(roadmapJSON, output.ErrNotFound, "unknown item: %s", args[0])
			}
			if err := roadmap.ValidateRootType(cfg, root); err != nil {
				return cmdError(roadmapJSON, output.ErrValidation, "%s", err)
			}
			data = roadmap.BuildScoped(cfg, allItems, roadmapIncludeDone, root)
		} else {
			data = roadmap.Build(cfg, allItems, roadmapIncludeDone, roadmapStatus, roadmapNoStatus)
		}
		roadmap.PruneDepth(data, roadmapDepth, scoped)

		// JSON output
		if roadmapJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(data)
		}

		// TTY-aware rendering: gerendert am Terminal, Markdown bei Pipe/Redirect
		// (D02). The width is only queried when stdout is a terminal (EARS-6) --
		// a non-TTY caller never pays for term.GetSize, and its result would be
		// discarded by roadmapOutput's markdown branch anyway.
		links := !roadmapNoLinks
		linkPrefix := roadmapLinkPrefix
		if links && linkPrefix == "" {
			// Default: relative path from cwd to .beans directory
			linkPrefix = defaultLinkPrefix()
		}

		isTTY := term.IsTerminal(int(os.Stdout.Fd()))
		cols := resolveWidth(roadmapWidthFlag, cmd.Flags().Changed("max-width"), cfg)

		fmt.Print(roadmapOutput(data, isTTY, formatOverride, cols, links, linkPrefix, roadmapTags, form, cfg))
		return nil
	},
}

// roadmapFormatOverride lets --format force which branch roadmapOutput takes,
// independent of isTTY detection. roadmapFormatAuto preserves today's
// detection-only behaviour exactly -- this step does not change what either
// branch renders, only how the branch is chosen.
type roadmapFormatOverride int

const (
	roadmapFormatAuto roadmapFormatOverride = iota
	roadmapFormatTTY
	roadmapFormatMarkdown
)

// roadmapOutput is the testable TTY switch (EARS-1/EARS-2/EARS-5): TTY gets
// the shared layout engine (ui.Render) in the requested form, everything else
// (pipe, redirect, non-terminal) gets renderRoadmapMarkdown unchanged --
// byte-identical to calling renderRoadmapMarkdown directly (Q07/D02). cols is
// passed through to ui.Render as-is -- resolveWidth (roadmap.go's RunE) is
// the one place the width is decided (floor 80, cap from --max-width/config/
// terminal, or uncapped for --max-width 0); a second clamp here would make
// roadmap silently disagree with every other command about what --max-width
// means (beans-dbph). format overrides the isTTY-derived choice when
// it is not roadmapFormatAuto, so a caller can force either branch (e.g. for
// tests, or a user explicitly asking for --format markdown/tty).
func roadmapOutput(data *roadmap.Data, isTTY bool, format roadmapFormatOverride, cols int, links bool, linkPrefix string, showTags bool, form ui.Form, cfg *config.Config) string {
	renderTTY := isTTY
	switch format {
	case roadmapFormatTTY:
		renderTTY = true
	case roadmapFormatMarkdown:
		renderTTY = false
	}
	if renderTTY {
		return ui.Render(roadmapRows(data), form, "Roadmap", cols, showTags, cfg)
	}
	return renderRoadmapMarkdown(data, links, linkPrefix, showTags)
}

// roadmapRows bridges the grouped roadmap.Data produced by roadmap.Build /
// roadmap.BuildScoped into the flat row list ui.Render expects. It mirrors the
// walk roadmap.tmpl renders (milestone -> epics -> features -> leafs,
// then epic/feature/other for the unscheduled bucket), but builds
// ui.FlatItems instead of writing text directly, and does no sorting of its
// own -- order comes entirely from the builder's slices.
//
// Depth counts tree levels, not the old renderer's indent-in-spaces: each
// milestone (or, for the unscheduled bucket, each of its top-level epics/
// features/other) is its own depth-0 root. ui.RowsFromFlatItems resets its
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
func roadmapRows(data *roadmap.Data) []ui.Row {
	var items []ui.FlatItem

	if data.Root != nil {
		if data.Root.Epic != nil {
			appendRoadmapEpicGroup(&items, *data.Root.Epic, 0, true)
		}
		if data.Root.Feature != nil {
			appendRoadmapFeatureGroup(&items, *data.Root.Feature, 0, true)
		}
		return ui.RowsFromFlatItems(items)
	}

	for _, mg := range data.Milestones {
		appendRoadmapMilestoneGroup(&items, mg)
	}

	unscheduledAt := -1
	if data.Unscheduled != nil {
		unscheduledAt = len(items)
		appendRoadmapUnscheduledGroup(&items, *data.Unscheduled)
	}

	rows := ui.RowsFromFlatItems(items)
	if unscheduledAt >= 0 && unscheduledAt < len(rows) {
		rows[unscheduledAt].Section = "Unscheduled"
	}
	return rows
}

// appendRoadmapMilestoneGroup appends a milestone, then its container
// children and other leaf items in mg.Containers / mg.Other order, the
// order roadmap.tmpl renders.
func appendRoadmapMilestoneGroup(items *[]ui.FlatItem, mg roadmap.MilestoneGroup) {
	// Unreachable in production -- every construction site sets Milestone --
	// but a hand-built roadmap.Data literal in a test would otherwise panic
	// inside ui.Render instead of producing an empty render (beans-dbph
	// review F5).
	if mg.Milestone == nil {
		return
	}
	*items = append(*items, ui.FlatItem{Item: mg.Milestone, Depth: 0, IsLast: true})

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
		*items = append(*items, ui.FlatItem{Item: leaf, Depth: 1, IsLast: i == total})
	}
}

// appendRoadmapUnscheduledGroup appends the unscheduled bucket's containers
// and other leaf items at depth 0 -- each is its own root, exactly like a
// milestone-less scoped root, since there is no milestone item to hang them
// under.
func appendRoadmapUnscheduledGroup(items *[]ui.FlatItem, ug roadmap.UnscheduledGroup) {
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
		*items = append(*items, ui.FlatItem{Item: leaf, Depth: 0, IsLast: i == total})
	}
}

// appendRoadmapEpicGroup appends an epic at depth, followed by its direct
// leaf items and nested feature groups at depth+1 -- items before features,
// per roadmap.tmpl.
func appendRoadmapEpicGroup(items *[]ui.FlatItem, eg roadmap.EpicGroup, depth int, isLast bool) {
	// Unreachable in production -- every construction site sets Epic -- but
	// a hand-built roadmap.EpicGroup literal in a test would otherwise panic inside
	// ui.Render instead of producing an empty render (beans-dbph review F5).
	if eg.Epic == nil {
		return
	}
	*items = append(*items, ui.FlatItem{Item: eg.Epic, Depth: depth, IsLast: isLast})

	childDepth := depth + 1
	total := len(eg.Items) + len(eg.Features)
	i := 0
	for _, leaf := range eg.Items {
		i++
		*items = append(*items, ui.FlatItem{Item: leaf, Depth: childDepth, IsLast: i == total})
	}
	for _, fg := range eg.Features {
		i++
		appendRoadmapFeatureGroup(items, fg, childDepth, i == total)
	}
}

// appendRoadmapFeatureGroup appends a feature at depth, followed by its
// flattened leaf items at depth+1.
func appendRoadmapFeatureGroup(items *[]ui.FlatItem, fg roadmap.FeatureGroup, depth int, isLast bool) {
	// Unreachable in production -- every construction site sets Feature --
	// but a hand-built featureGroup literal in a test would otherwise panic
	// inside ui.Render instead of producing an empty render (beans-dbph
	// review F5).
	if fg.Feature == nil {
		return
	}
	*items = append(*items, ui.FlatItem{Item: fg.Feature, Depth: depth, IsLast: isLast})

	childDepth := depth + 1
	n := len(fg.Items)
	for i, leaf := range fg.Items {
		*items = append(*items, ui.FlatItem{Item: leaf, Depth: childDepth, IsLast: i == n-1})
	}
}

// renderRoadmapMarkdown renders the roadmap as Markdown using the template.
func renderRoadmapMarkdown(data *roadmap.Data, links bool, linkPrefix string, showTags bool) string {
	// Create template with closures that capture link and tag settings
	tmpl := template.Must(
		template.New("roadmap").Funcs(template.FuncMap{
			"firstParagraph": firstParagraph,
			"typeBadge":      typeBadge,
			"typeLabel":      typeHeadingLabel,
			"beanRef": func(b *item.Item) string {
				return renderItemRef(b, links, linkPrefix)
			},
			"tagLine": func(b *item.Item, indent string) string {
				return renderTagLine(b, indent, showTags)
			},
		}).Parse(roadmapTemplateContent),
	)

	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		panic(err)
	}
	// The template's trailing text emits a final newline after the last
	// block; normalise so the document always ends in exactly one.
	return strings.TrimRight(sb.String(), "\n") + "\n"
}

// renderItemRef renders an item ID, optionally as a markdown link.
func renderItemRef(b *item.Item, asLink bool, linkPrefix string) string {
	if !asLink {
		return "(" + b.ID + ")"
	}
	if linkPrefix == "" {
		return fmt.Sprintf("([%s](%s))", b.ID, b.Path)
	}
	// Ensure prefix ends with / for clean concatenation
	if !strings.HasSuffix(linkPrefix, "/") {
		linkPrefix += "/"
	}
	return fmt.Sprintf("([%s](%s%s))", b.ID, linkPrefix, b.Path)
}

// typeHeadingLabel renders an item's own type name for a roadmap heading: the
// name (identity, D03/D05) with its first letter upper-cased, purely a
// display transform on the string itself -- not a config lookup, so it
// renders the same whether or not cfg still knows the type (lif check's
// unknownTypeItems). This mirrors the same first-letter-uppercase idiom
// Config.ShortOf already uses for the one-character type code, rather than
// progressStatusLabel's per-word title-casing, which exists to turn a
// hyphenated status like "in-progress" into "In Progress" and does not fit a
// single-word type name. An empty type -- RankOf("") resolves to LeafRank,
// so roadmap.Build itself never hands an empty-typed item to a heading, but a
// hand-built roadmap.Data could -- falls back to "Untyped" rather than
// rendering an empty label.
func typeHeadingLabel(typeName string) string {
	if typeName == "" {
		return "Untyped"
	}
	return strings.ToUpper(typeName[:1]) + typeName[1:]
}

// typeBadge returns a shields.io badge markdown for the item type.
func typeBadge(b *item.Item) string {
	if b.Type == "" {
		return ""
	}
	// One colour source: the badge shows the same tone the terminal renders,
	// resolved through the active theme. shields.io wants the hex without the
	// leading hash.
	color := "gray"
	if t := cfg.GetType(b.Type); t != nil && t.Color != "" {
		color = strings.TrimPrefix(string(ui.ResolveColor(t.Color)), "#")
	}
	return fmt.Sprintf("![%s](https://img.shields.io/badge/%s-%s?style=flat-square)", b.Type, b.Type, color)
}

// renderTagLine renders an item's tags as their own Markdown line, mirroring
// the TTY tag row: each tag as inline code, prefixed with "#". The returned
// string starts with a newline so a template can append it directly to the
// item's line; indent keeps a leaf's tags inside its list item. Returns ""
// when tags are off or the item has none.
func renderTagLine(b *item.Item, indent string, showTags bool) string {
	if !showTags || len(b.Tags) == 0 {
		return ""
	}
	tags := make([]string, len(b.Tags))
	for i, t := range b.Tags {
		tags[i] = "`#" + t + "`"
	}
	return "\n" + indent + strings.Join(tags, " ")
}

// defaultLinkPrefix returns the relative path from cwd to the .beans directory.
func defaultLinkPrefix() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(cwd, core.Root())
	if err != nil {
		return ""
	}
	// Convert to forward slashes for URL compatibility
	return filepath.ToSlash(rel)
}

// firstParagraph extracts the first paragraph from a body text.
func firstParagraph(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}

	// Find the first blank line (paragraph separator)
	lines := strings.Split(body, "\n")
	var para []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			break
		}
		// Skip markdown headers
		if strings.HasPrefix(line, "#") {
			continue
		}
		para = append(para, strings.TrimSpace(line))
	}

	result := strings.Join(para, " ")
	// Truncate if too long
	if len(result) > 200 {
		result = result[:197] + "..."
	}
	return result
}

func RegisterRoadmapCmd(root *cobra.Command) {
	roadmapCmd.ValidArgsFunction = completionUpTo(1)
	// Registration is idempotent: the command is a package-level singleton,
	// so a second Register call in the same process (tests build several
	// roots) would panic in pflag with "flag redefined". list.go carries the
	// same guard; without it here, adding a test that registers this command
	// alongside another one panics rather than failing.
	if roadmapCmd.Flags().Lookup("json") != nil {
		root.AddCommand(roadmapCmd)
		return
	}
	roadmapCmd.Flags().BoolVar(&roadmapJSON, "json", false, "Output as JSON")
	roadmapCmd.Flags().BoolVar(&roadmapIncludeDone, "include-done", false, "Include completed items")
	roadmapCmd.Flags().StringSliceVar(&roadmapStatus, "status", nil, "Filter milestones by status (comma-separated or repeated)")
	roadmapCmd.Flags().StringSliceVar(&roadmapNoStatus, "no-status", nil, "Exclude milestones by status (comma-separated or repeated)")
	_ = roadmapCmd.RegisterFlagCompletionFunc("status", statusFlagCompletion)
	_ = roadmapCmd.RegisterFlagCompletionFunc("no-status", statusFlagCompletion)
	roadmapCmd.Flags().BoolVar(&roadmapNoLinks, "no-links", false, "Don't render item IDs as markdown links")
	roadmapCmd.Flags().StringVar(&roadmapLinkPrefix, "link-prefix", "", "URL prefix for links")
	roadmapCmd.Flags().IntVar(&roadmapDepth, "depth", 0, "Limit output to n levels below the roadmap root (default: no limit)")
	roadmapCmd.Flags().BoolVar(&roadmapTags, "tags", false, "Render each item's tags")
	roadmapCmd.Flags().StringVar(&roadmapView, "view", "tree",
		"Arrangement: table (flat, sortable) or tree (nested)")
	roadmapCmd.Flags().StringVar(&roadmapFormat, "format", "", `Output format: "tty" or "markdown" (default: detect)`)
	roadmapCmd.Flags().IntVar(&roadmapWidthFlag, "max-width", 0,
		"Cap the rendered width; 0 disables the cap (default: display.max_width, else 110)")
	root.AddCommand(roadmapCmd)
}
