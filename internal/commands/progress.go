package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/roadmap"
	"github.com/spf13/cobra"
)

var progressJSON bool

// progressBarWidth is the fixed width, in characters, of the plain-text
// progress bar rendered below the per-status counts.
const progressBarWidth = 20

// progressResult is the --json shape for `lif progress`: per-status
// counts plus the derived completed/total/percent figures. It intentionally
// carries no bar string — the bar is a presentation-only concern of the
// plain-text renderer below.
type progressResult struct {
	Root      string         `json:"root,omitempty"`
	Counts    map[string]int `json:"counts"`
	Completed int            `json:"completed"`
	Total     int            `json:"total"`
	Percent   int            `json:"percent"`
}

var progressCmd = &cobra.Command{
	Use:   "progress [id]",
	Short: "Show a summary of work status across all items",
	Long: `Shows counts by status across every configured status, plus a percent-complete figure (completed / (total - scrapped)).

With an ID argument (for example a milestone or epic), the counts cover that item's descendants — through any number of parent levels — instead of the whole workspace. The root item's own status is not counted: it is the container, not an item of work.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		resolver := newResolver()

		allItems, err := resolver.Items(ctx, nil)
		if err != nil {
			return cmdError(progressJSON, output.ErrValidation, "querying items: %v", err)
		}

		scope := allItems
		var root *item.Item
		if len(args) == 1 {
			root, err = resolver.Item(ctx, args[0])
			if err != nil {
				return cmdError(progressJSON, output.ErrNotFound, "failed to find item: %v", err)
			}
			if root == nil {
				return cmdError(progressJSON, output.ErrNotFound, "item not found: %s", args[0])
			}
			idx := roadmap.ChildrenIndex(allItems)
			scope = descendants(root.ID, idx)
		}

		statusNames := cfg.StatusNames()
		counts := make(map[string]int, len(statusNames))
		for _, s := range statusNames {
			counts[s] = 0
		}
		for _, b := range scope {
			counts[b.Status]++
		}

		completed := counts["completed"]
		total := len(scope) - counts["scrapped"]
		// Integer division truncates toward zero (23*100/40 = 57.5 -> 57),
		// matching the epic's own worked example (beans-m364) which states
		// "57%" for that exact input rather than a rounded-up 58%.
		percent := 0
		if total > 0 {
			percent = completed * 100 / total
		}

		if progressJSON {
			result := progressResult{Completed: completed, Total: total, Percent: percent, Counts: counts}
			if root != nil {
				result.Root = root.ID
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		}

		if root != nil {
			printProgressRootHeader(root, cfg)
		}

		for _, sc := range cfg.StatusList() {
			// Measure and pad the plain label first, then colour it — never
			// the other way around, or the padding would count escape bytes
			// as visible width.
			label := progressStatusLabel(sc.Name)
			style := lipgloss.NewStyle().Foreground(ui.ResolveColor(sc.Color)).Bold(!sc.Archive)
			fmt.Printf("%s: %d\n", style.Render(label), counts[sc.Name])
		}

		filled, empty := progressBarSegments(percent)
		bar := lipgloss.NewStyle().Foreground(ui.ResolveColor("green")).Render(filled) +
			lipgloss.NewStyle().Foreground(ui.ResolveColor("surface2")).Render(empty)
		fmt.Printf("%s %d%% complete\n", bar, percent)

		return nil
	},
}

// printProgressRootHeader names the scope so a reader of a scoped run
// cannot mistake it for a workspace-wide figure. The style mirrors the
// table's type tint: colour only when the type configures one, bold on
// emphasis, so an unstyled type keeps the terminal's own text colour.
func printProgressRootHeader(b *item.Item, cfg *config.Config) {
	st := lipgloss.NewStyle()
	if tc := cfg.GetType(b.Type); tc != nil {
		st = st.Bold(tc.Emphasis)
		if tc.Color != "" {
			st = st.Foreground(ui.ResolveColor(tc.Color))
		}
	}
	fmt.Printf("%s %s\n\n", st.Render(b.ID), st.Render(b.Title))
}

// progressStatusLabel renders a status name like "in-progress" as "In
// Progress" for plain-text display, matching the epic's example rendering.
func progressStatusLabel(status string) string {
	words := strings.Split(status, "-")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// progressBarSegments returns the filled and empty plain-text runs of the
// bar separately, so a caller can colour each run independently without
// measuring against text that already carries colour escapes.
func progressBarSegments(percent int) (filled, empty string) {
	n := percent * progressBarWidth / 100
	if n > progressBarWidth {
		n = progressBarWidth
	}
	if n < 0 {
		n = 0
	}
	return strings.Repeat("━", n), strings.Repeat("░", progressBarWidth-n)
}

func RegisterProgressCmd(root *cobra.Command) {
	progressCmd.Flags().BoolVar(&progressJSON, "json", false, "Output as JSON")
	progressCmd.ValidArgsFunction = completionUpTo(1)
	root.AddCommand(progressCmd)
}
