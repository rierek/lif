package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/pkg/candidates"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph"
)

// errPickAborted is returned whenever nothing was selected -- the user
// cancelled, or reached the end of the picker without pressing enter on an
// item. R-11 AC3: stdout stays empty and the process still exits non-zero
// either way, so callers cannot tell "cancelled" from "picked nothing" and
// don't need to. It is an output.Silent error (beans-lk8t): backing out of
// an interactive picker is expected, ordinary use, not a failure worth a
// stderr line -- reportExecutionError still keeps quiet while Execute still
// exits 1.
var errPickAborted = output.Silent("lif pick: no item selected")

var pickCmd = &cobra.Command{
	Use:   "pick",
	Args:  cobra.NoArgs,
	Short: "Interactively pick an item and print its ID",
	Long: `Draws an interactive, fuzzy-filterable list of every item in the
resolved store on the controlling terminal. Selecting one prints exactly
that item's ID to stdout and exits 0; aborting the picker, an empty
candidate list, or non-interactive stdin produce no stdout output and a
non-zero exit, so scripts can safely capture the result with command
substitution, e.g. id=$(lif pick).`,
	RunE: runPick,
}

// pickScope, pickLine, and pickCursor back the --scope and --line/--cursor
// flags (R-12 AC1/AC2/AC3): explicit type-scope narrowing, and partial-
// command-line narrowing, respectively. --line/--cursor are defined as a
// plain string plus a byte offset (R-12 Risk mitigation) so scope
// derivation from a partial invocation is testable without any real shell
// integration.
var (
	pickScope  string
	pickLine   string
	pickCursor int
)

// RegisterPickCmd adds the pick command to root. It intentionally does not
// call markPlumbing: pick is a directly-typed interactive verb, so it
// composes with RegisterCoreCommands's closing loop (register.go:101-108)
// and lands user-facing by falling into that default, the same way every
// other non-plumbing verb does.
func RegisterPickCmd(root *cobra.Command) {
	pickCmd.Flags().StringVar(&pickScope, "scope", "", "Restrict candidates to a comma-separated list of item types")
	pickCmd.Flags().StringVar(&pickLine, "line", "", "Partial command line to derive scope from (used with --cursor)")
	pickCmd.Flags().IntVar(&pickCursor, "cursor", -1, "Byte offset of the cursor within --line")
	root.AddCommand(pickCmd)
}

// pickItem adapts an item to bubbles/list's list.Item interface. It carries
// only the id and title -- the two fields the picker's list and its final
// stdout write need -- rather than a full *item.Item, so the picker and
// internal/tui's own pickers (parentpicker.go, blockingpicker.go, etc.)
// share no type between them (R-18: neither surface imports the other).
type pickItem struct {
	id, title string
}

func (i pickItem) Title() string       { return i.title }
func (i pickItem) Description() string { return i.id }
func (i pickItem) FilterValue() string { return i.title + " " + i.id }

// pickModel is the standalone bubbletea model driving the picker's
// terminal state machine: highlighting, filtering (delegated to the
// embedded list.Model), and the selected/aborted outcome the picker's
// caller reads back once the program returns.
type pickModel struct {
	list       list.Model
	selected   bool
	selectedID string
	aborted    bool
}

func newPickModel(pickable []*item.Item) pickModel {
	rows := make([]list.Item, len(pickable))
	for i, b := range pickable {
		rows[i] = pickItem{id: b.ID, title: b.Title}
	}
	l := list.New(rows, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Pick an item"
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	return pickModel{list: l}
}

func (m pickModel) Init() tea.Cmd { return nil }

func (m pickModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyMsg:
		if m.list.FilterState() != list.Filtering {
			switch msg.String() {
			case "esc", "ctrl+c":
				m.aborted = true
				return m, tea.Quit
			case "enter":
				if item, ok := m.list.SelectedItem().(pickItem); ok {
					m.selected = true
					m.selectedID = item.id
				} else {
					m.aborted = true
				}
				return m, tea.Quit
			}
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View draws the picker. It returns a plain string for bubbletea to render
// wherever the program's output is configured -- runPick points that at
// the controlling tty, never at stdout -- so the model itself has no way
// to leak interactive chrome onto stdout even by mistake (R-11 AC2).
func (m pickModel) View() string {
	return m.list.View()
}

// runPick is pickCmd's RunE. Candidates come from the already-loaded store
// (R-16: core is populated exclusively by resolveStorePath inside
// NewRootCmd's PersistentPreRunE; pick adds itself to no skip-list and
// opens no second, independent store-resolution path). Every interactive
// frame is drawn on the process's controlling terminal (opened explicitly
// as /dev/tty, never inherited stdin/stdout), so the only byte pick ever
// writes to stdout is the single selected ID on success.
func runPick(cmd *cobra.Command, _ []string) error {
	items, err := resolvePickCandidates(cmd)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return errors.New("lif pick: store has no items to pick from")
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("lif pick: stdin is not a terminal")
	}

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("lif pick: opening controlling terminal: %w", err)
	}
	defer tty.Close()

	return runPickWith(cmd, items, tty, tty)
}

// runPickWith is runPick's testable core: it drives the picker's
// bubbletea program against the supplied input/output pair instead of
// reaching for /dev/tty itself, so a test can inject in-memory pipes
// (beans-gofn). runPick is the sole production caller and always points
// both in and out at the same already-opened controlling terminal.
func runPickWith(cmd *cobra.Command, items []*item.Item, in io.Reader, out io.Writer) error {
	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
	})

	program := tea.NewProgram(newPickModel(items), tea.WithInput(in), tea.WithOutput(out), tea.WithAltScreen())
	final, err := program.Run()
	if err != nil {
		return fmt.Errorf("lif pick: %w", err)
	}

	result, ok := final.(pickModel)
	if !ok || !result.selected {
		return errPickAborted
	}

	fmt.Fprintln(cmd.OutOrStdout(), result.selectedID)
	return nil
}

// resolvePickCandidates derives the picker's candidate set from the
// invocation context (R-12): an explicit --scope, a partial --line/
// --cursor, or -- absent either -- the full, unscoped store, which is a
// legitimate bare `lif pick` call and not itself an AC5 failure. Once
// given a context it cannot resolve, it always returns a visible error
// (AC5): it never falls back to the unscoped set.
func resolvePickCandidates(cmd *cobra.Command) ([]*item.Item, error) {
	switch {
	case cmd.Flags().Changed("scope"):
		return scopeFilteredCandidates(pickScope)
	case cmd.Flags().Changed("line"):
		if !cmd.Flags().Changed("cursor") {
			return nil, errors.New("lif pick: --line requires --cursor")
		}
		return partialLineCandidates(cmd, pickLine, pickCursor)
	default:
		return core.All(), nil
	}
}

// filterByTypes returns the subset of the store whose type is in types. It
// is the single internal narrowing primitive both --scope (AC1) and
// verb-derived scope (AC2) route through (Risk mitigation: one shared
// type-set resolver, so the two entry paths cannot silently diverge).
func filterByTypes(types map[string]bool) []*item.Item {
	all := core.All()
	out := make([]*item.Item, 0, len(all))
	for _, b := range all {
		if types[b.Type] {
			out = append(out, b)
		}
	}
	return out
}

// scopeFilteredCandidates implements AC1: --scope narrows the candidate
// set to the named, comma-separated item type(s).
func scopeFilteredCandidates(scope string) ([]*item.Item, error) {
	types, err := parseScopeTypes(scope)
	if err != nil {
		return nil, err
	}
	return filterByTypes(types), nil
}

// parseScopeTypes validates and parses a comma-separated --scope value
// against the configured item types, rejecting any unknown type (AC5)
// rather than silently ignoring it.
func parseScopeTypes(scope string) (map[string]bool, error) {
	types := make(map[string]bool)
	for _, raw := range strings.Split(scope, ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if !cfg.IsValidType(name) {
			return nil, fmt.Errorf("lif pick: invalid --scope type: %s (must be %s)", name, strings.Join(cfg.TypeNames(), ", "))
		}
		types[name] = true
	}
	if len(types) == 0 {
		return nil, errors.New("lif pick: --scope requires at least one item type")
	}
	return types, nil
}

// roadmapScopeTypes returns the item types occupying the container ranks
// that roadmap.go's own validateRoadmapRootType/isContainerRank already
// define as the roadmap command's root-type constraint. It reads
// roadmap.go's containerTypeNames -- the single, shared loop over
// config.MaxContainerRank that isContainerRank and validateRoadmapRootType
// also route through (beans-v2ox) -- rather than defining a type table, or
// a second 1..MaxContainerRank loop, of its own (AC4).
func roadmapScopeTypes() map[string]bool {
	types := make(map[string]bool)
	for _, name := range containerTypeNames() {
		types[name] = true
	}
	return types
}

// verbScopeTypes maps a resolved verb command to its own type-subset
// constraint (AC2/AC4), read off that verb's own already-implemented type
// semantics. roadmapCmd (roadmap.go, same package) is recognized by direct
// pointer identity against the package's own singleton -- never by
// comparing verb name strings, and without any second verb-to-type table.
// A verb with no known type semantics of its own returns ok=false: AC5
// requires that be reported, never silently treated as unscoped.
func verbScopeTypes(resolvedCmd *cobra.Command) (map[string]bool, bool) {
	if resolvedCmd == roadmapCmd {
		return roadmapScopeTypes(), true
	}
	return nil, false
}

// lineToken is one whitespace-delimited token of a partial command line,
// carrying its byte offsets within that line so a cursor offset can be
// mapped back onto it.
type lineToken struct {
	text       string
	start, end int
}

// tokenizeLine splits a partial command line into lineTokens on plain
// ASCII whitespace, tracking each token's byte offsets.
func tokenizeLine(line string) []lineToken {
	var toks []lineToken
	start := -1
	for i, r := range line {
		if r == ' ' || r == '\t' {
			if start >= 0 {
				toks = append(toks, lineToken{text: line[start:i], start: start, end: i})
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		toks = append(toks, lineToken{text: line[start:], start: start, end: len(line)})
	}
	return toks
}

// cursorTokenIndex returns the index into toks the cursor offset lands at
// or would insert a new token at: the first token whose end the cursor
// does not exceed, or len(toks) if the cursor sits past every token
// (trailing whitespace).
func cursorTokenIndex(toks []lineToken, cursor int) int {
	for i, t := range toks {
		if cursor <= t.end {
			return i
		}
	}
	return len(toks)
}

// parentFlagName is the literal flag name AC3 itself names ("cursor
// position at a --parent ... argument"); it is a flag name, not a verb
// name, so it does not fall under AC4's hardcoded-verb-name-literal ban.
const parentFlagName = "--parent"

// atParentValue reports whether the cursor -- resolved to token index idx
// by cursorTokenIndex -- sits in the value slot of a --parent flag: either
// a fresh or partially-typed value token right after a standalone
// "--parent" token, or inside a "--parent=value" token's value part. When
// found, flagIdx is the index of the "--parent"/"--parent=..." token
// itself.
func atParentValue(toks []lineToken, idx int) (flagIdx int, ok bool) {
	if idx > 0 && idx <= len(toks) && toks[idx-1].text == parentFlagName {
		return idx - 1, true
	}
	if idx < len(toks) && strings.HasPrefix(toks[idx].text, parentFlagName+"=") {
		return idx, true
	}
	return 0, false
}

// deriveParentContext walks the tokens before the --parent flag at
// flagIdx and resolves the itemIDs/itemTypes ParentCandidates needs from
// whichever of those tokens already resolve to an existing item in the
// store -- the item(s) being edited. Token 0 is always the verb and is
// never itself resolved as a positional id (a verb name never collides
// with an item ID prefix in practice, but it is skipped by position, not by
// comparing its text, so no verb-name literal is introduced here either).
// A flag token that is not itself --parent consumes its own value token
// (if any) so that value is never mistaken for a positional item id.
func deriveParentContext(toks []lineToken, flagIdx int) (itemIDs, itemTypes []string, err error) {
	seenType := map[string]bool{}
	for i := 1; i < flagIdx; i++ {
		tok := toks[i].text
		if strings.HasPrefix(tok, "-") {
			if !strings.Contains(tok, "=") && i+1 < flagIdx {
				i++
			}
			continue
		}
		b, getErr := core.Get(tok)
		if getErr != nil {
			continue
		}
		itemIDs = append(itemIDs, b.ID)
		if !seenType[b.Type] {
			seenType[b.Type] = true
			itemTypes = append(itemTypes, b.Type)
		}
	}
	if len(itemIDs) == 0 {
		return nil, nil, errors.New("lif pick: cannot resolve --parent context: no known item id precedes --parent")
	}
	return itemIDs, itemTypes, nil
}

// partialLineCandidates implements AC2/AC3/AC5 for a supplied --line and
// --cursor: a cursor at a --parent value position applies the R-03
// exclusion via candidates.ParentCandidates (AC3); otherwise the line's
// verb is resolved against the real command tree (cmd.Root().Find, cobra's
// own structural resolution -- no verb-name literal) and, if that verb
// carries a known type-subset constraint (AC2/AC4), the candidate set is
// narrowed to it. Any position this does not understand is a visible
// error (AC5), never a silent unscoped fallback.
//
// A live shell buffer's partial line carries the program name as its
// first token (e.g. "lif roadmap "); a bare verb-first line ("roadmap
// ") is also accepted directly. A leading token is tolerated as the
// program name by comparing it against the resolved root command's own
// Name() -- never against a literal like "beans" (AC4) -- so both forms
// resolve to the same scope.
func partialLineCandidates(cmd *cobra.Command, line string, cursor int) ([]*item.Item, error) {
	if cursor < 0 || cursor > len(line) {
		return nil, fmt.Errorf("lif pick: --cursor %d is out of range for a %d-byte --line", cursor, len(line))
	}
	toks := tokenizeLine(line)
	if len(toks) == 0 {
		return nil, errors.New("lif pick: --line has no verb to resolve")
	}
	idx := cursorTokenIndex(toks, cursor)

	if toks[0].text == cmd.Root().Name() {
		toks = toks[1:]
		idx--
		if len(toks) == 0 {
			return nil, errors.New("lif pick: --line has no verb after the program name")
		}
		if idx < 0 {
			return nil, errors.New("lif pick: cursor position is not understood by scope derivation (it sits on the program name)")
		}
	}

	if flagIdx, ok := atParentValue(toks, idx); ok {
		itemIDs, itemTypes, err := deriveParentContext(toks, flagIdx)
		if err != nil {
			return nil, err
		}
		resolver := &itemgraph.CoreResolver{Core: core}
		return candidates.ParentCandidates(context.Background(), resolver, cfg, itemIDs, itemTypes)
	}

	tokenTexts := make([]string, len(toks))
	for i, t := range toks {
		tokenTexts[i] = t.text
	}
	resolvedCmd, _, err := cmd.Root().Find(tokenTexts)
	if err != nil {
		return nil, fmt.Errorf("lif pick: partial line verb does not resolve: %w", err)
	}
	if resolvedCmd == cmd.Root() {
		return nil, errors.New("lif pick: partial line has no resolvable verb")
	}
	if !IsUserFacing(resolvedCmd) {
		return nil, fmt.Errorf("lif pick: %q is not a user-facing verb", resolvedCmd.Name())
	}
	types, ok := verbScopeTypes(resolvedCmd)
	if !ok {
		return nil, fmt.Errorf("lif pick: verb %q has no known scope-derivation mapping", resolvedCmd.Name())
	}
	return filterByTypes(types), nil
}
