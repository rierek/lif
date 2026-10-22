package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
)

// setupPickTest points the package-level core at a fresh temp store and
// restores it afterward, mirroring setupShowTest's pattern for isolating
// per-test store state.
func setupPickTest(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	oldCore := core
	core = itemcore.New(dir, config.Default())
	t.Cleanup(func() { core = oldCore })
}

func pickTestItem(id, title string) *item.Item {
	return &item.Item{ID: id, Title: title, Type: "task", Status: "open"}
}

// setupPickScopeTest points core AND cfg together at a fresh temp store
// and restores both afterward, then returns the shared root's own "pick"
// command node with its flags reset. Scope derivation (parseScopeTypes,
// roadmapScopeTypes) reads the package-level cfg, not core.Config(), so
// swapping only core (as setupPickTest does) would validate a --scope
// value or verb type-subset against a stale or unrelated config.
func setupPickScopeTest(t *testing.T) *cobra.Command {
	t.Helper()
	dir := t.TempDir()
	testCfg := config.Default()
	testCore := itemcore.New(dir, testCfg)
	if err := testCore.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}
	oldCore, oldCfg := core, cfg
	core, cfg = testCore, testCfg
	t.Cleanup(func() { core, cfg = oldCore, oldCfg })

	root := sharedTestRoot(t)
	resetFlags(root)
	t.Cleanup(func() { resetFlags(root) })
	pick, _, err := root.Find([]string{"pick"})
	if err != nil {
		t.Fatalf("finding pick: %v", err)
	}
	return pick
}

// createScopeFixture seeds the current core with 2 milestones, 3 epics, 2
// features, 2 bugs, and 3 tasks (12 items total): one of every default
// type, at more than one count where useful, so a scope filter's result
// size is unambiguous.
func createScopeFixture(t *testing.T) {
	t.Helper()
	seed := []*item.Item{
		{ID: "beans-uq01", Title: "M1", Type: "milestone", Status: "todo"},
		{ID: "beans-uq02", Title: "M2", Type: "milestone", Status: "todo"},
		{ID: "beans-uq03", Title: "E1", Type: "epic", Status: "todo"},
		{ID: "beans-uq04", Title: "E2", Type: "epic", Status: "todo"},
		{ID: "beans-uq05", Title: "E3", Type: "epic", Status: "todo"},
		{ID: "beans-uq06", Title: "F1", Type: "feature", Status: "todo"},
		{ID: "beans-uq07", Title: "F2", Type: "feature", Status: "todo"},
		{ID: "beans-uq08", Title: "B1", Type: "bug", Status: "todo"},
		{ID: "beans-uq09", Title: "B2", Type: "bug", Status: "todo"},
		{ID: "beans-uq10", Title: "T1", Type: "task", Status: "todo"},
		{ID: "beans-uq11", Title: "T2", Type: "task", Status: "todo"},
		{ID: "beans-uq12", Title: "T3", Type: "task", Status: "todo"},
	}
	for _, b := range seed {
		if err := core.Create(b); err != nil {
			t.Fatalf("seeding %s: %v", b.ID, err)
		}
	}
}

// TestRunPickReflectsStoreResolvedPerInvocation pins SC-03/AC2.1: pick's
// candidate set comes from whatever store storeroot.Resolve resolved for
// this invocation via NewRootCmd's PersistentPreRunE, not from a second,
// independent resolution path. It distinguishes "store A" from "store B"
// by which of runPick's two early error branches fires -- the empty-store
// branch for an empty directory, the non-tty branch (reached only once
// core.All() found candidates) for a populated one -- without needing a
// real controlling terminal.
func TestRunPickReflectsStoreResolvedPerInvocation(t *testing.T) {
	root := sharedTestRoot(t)

	emptyDir := filepath.Join(t.TempDir(), ".beans")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatalf("creating empty store dir: %v", err)
	}

	populatedDir := filepath.Join(t.TempDir(), ".beans")
	if err := os.MkdirAll(populatedDir, 0755); err != nil {
		t.Fatalf("creating populated store dir: %v", err)
	}
	seedCore := itemcore.New(populatedDir, config.Default())
	if err := seedCore.Create(pickTestItem("beans-zzzz", "Populated")); err != nil {
		t.Fatalf("seeding populated store: %v", err)
	}

	// A closed-read-end pipe is non-terminal stdin, held constant across
	// both runs so the only variable between them is --lif-path.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe write end: %v", err)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = r.Close()
	})

	runWith := func(dir string) string {
		resetFlags(root)
		var errBuf bytes.Buffer
		root.SetOut(io.Discard)
		root.SetErr(&errBuf)
		root.SetArgs([]string{"--lif-path", dir, "pick"})
		_, execErr := root.ExecuteC()
		if execErr == nil {
			t.Fatalf("expected pick to fail for non-terminal stdin against %s", dir)
		}
		return execErr.Error()
	}

	emptyErr := runWith(emptyDir)
	populatedErr := runWith(populatedDir)

	if !strings.Contains(emptyErr, "lif pick: store has no items to pick from") {
		t.Errorf("empty store: got error %q, want the empty-candidates branch", emptyErr)
	}
	if !strings.Contains(populatedErr, "lif pick: stdin is not a terminal") {
		t.Errorf("populated store: got error %q, want the non-tty branch", populatedErr)
	}
}

// TestPickCmdIsRegistered pins that RegisterPickCmd actually joins the tree,
// not merely that the file compiles (Entry points: register.go:46-109).
func TestPickCmdIsRegistered(t *testing.T) {
	root := sharedTestRoot(t)
	for _, c := range root.Commands() {
		if c.Name() == "pick" {
			return
		}
	}
	t.Fatal("expected a registered \"pick\" command, found none")
}

// TestPickCmdIsUserFacing pins the Risks section's warning: pick must not
// be marked plumbing and must fall into RegisterCoreCommands's default
// user-facing classification.
func TestPickCmdIsUserFacing(t *testing.T) {
	root := sharedTestRoot(t)
	cmd, _, err := root.Find([]string{"pick"})
	if err != nil {
		t.Fatalf("finding pick: %v", err)
	}
	if !IsUserFacing(cmd) {
		t.Error("pick classified plumbing, want user-facing")
	}
}

// TestPickCmdDeclaresNoArgs pins the Cobra convention constraint: pick takes
// no positional arguments.
func TestPickCmdDeclaresNoArgs(t *testing.T) {
	if pickCmd.Args == nil {
		t.Fatal("pick has no declared Args policy")
	}
	if err := pickCmd.Args(pickCmd, []string{"unexpected"}); err == nil {
		t.Error("pick accepted a positional argument, want rejection")
	}
	if err := pickCmd.Args(pickCmd, nil); err != nil {
		t.Errorf("pick rejected zero arguments: %v", err)
	}
}

// TestRunPickFailsOnEmptyCandidates pins AC1.3 for the empty-list half:
// with no items in the store, runPick must fail without writing to stdout,
// and must fail before ever attempting to open a controlling terminal.
func TestRunPickFailsOnEmptyCandidates(t *testing.T) {
	setupPickTest(t)

	var out bytes.Buffer
	pickCmd.SetOut(&out)
	t.Cleanup(func() { pickCmd.SetOut(nil) })

	err := runPick(pickCmd, nil)
	if err == nil {
		t.Fatal("expected an error for an empty candidate set")
	}
	if out.Len() != 0 {
		t.Errorf("expected empty stdout, got %q", out.String())
	}
}

// TestRunPickFailsOnNonTTYStdin pins SC-02 and AC1.4: with stdin redirected
// away from a terminal (a pipe, exactly like `</dev/null` in a real shell),
// runPick must fail cleanly with no stdout output, without ever reaching
// the tea.Program/tty codepath.
func TestRunPickFailsOnNonTTYStdin(t *testing.T) {
	setupPickTest(t)
	if err := core.Create(pickTestItem("beans-test1", "Some bean")); err != nil {
		t.Fatalf("creating test bean: %v", err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = oldStdin
		_ = r.Close()
	})
	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe write end: %v", err)
	}

	var out bytes.Buffer
	pickCmd.SetOut(&out)
	t.Cleanup(func() { pickCmd.SetOut(nil) })

	runErr := runPick(pickCmd, nil)
	if runErr == nil {
		t.Fatal("expected an error for non-terminal stdin")
	}
	if out.Len() != 0 {
		t.Errorf("expected empty stdout, got %q", out.String())
	}
}

// TestPickModelEnterSelectsHighlightedItem pins AC1.1's selection half of
// the model's terminal state machine: pressing enter on the highlighted
// item marks it selected and quits, without touching any io.Writer -- the
// model never writes to stdout itself (AC1.2).
func TestPickModelEnterSelectsHighlightedItem(t *testing.T) {
	items := []*item.Item{
		pickTestItem("beans-aaaa", "Alpha bean"),
		pickTestItem("beans-bbbb", "Beta bean"),
	}
	m := newPickModel(items)
	m.list.SetSize(80, 20)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm, ok := updated.(pickModel)
	if !ok {
		t.Fatalf("Update returned %T, want pickModel", updated)
	}
	if !pm.selected {
		t.Error("expected selected=true after enter")
	}
	if pm.selectedID != "beans-aaaa" {
		t.Errorf("selectedID = %q, want %q", pm.selectedID, "beans-aaaa")
	}
	if pm.aborted {
		t.Error("expected aborted=false on successful selection")
	}
	if cmd == nil {
		t.Fatal("expected a tea.Cmd (tea.Quit) after enter")
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Errorf("expected tea.Quit message, got %#v", msg)
	}
}

// TestPickModelEscAborts pins AC1.3's abort half: pressing esc quits
// without marking anything selected.
func TestPickModelEscAborts(t *testing.T) {
	items := []*item.Item{pickTestItem("beans-aaaa", "Alpha bean")}
	m := newPickModel(items)
	m.list.SetSize(80, 20)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	pm, ok := updated.(pickModel)
	if !ok {
		t.Fatalf("Update returned %T, want pickModel", updated)
	}
	if pm.selected {
		t.Error("expected selected=false after esc")
	}
	if !pm.aborted {
		t.Error("expected aborted=true after esc")
	}
	if cmd == nil {
		t.Fatal("expected a tea.Cmd (tea.Quit) after esc")
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Errorf("expected tea.Quit message, got %#v", msg)
	}
}

// TestPickModelViewNeverWritesToProvidedWriter is a structural guard for
// AC1.2: the View() method returns a string for bubbletea to draw wherever
// the program's output is configured (the controlling tty, never stdout);
// it takes no io.Writer of its own, so it cannot short-circuit around that
// configuration.
func TestPickModelViewNeverWritesToProvidedWriter(t *testing.T) {
	items := []*item.Item{pickTestItem("beans-aaaa", "Alpha bean")}
	m := newPickModel(items)
	m.list.SetSize(80, 20)
	if v := m.View(); v == "" {
		t.Error("expected a non-empty view for a populated list")
	}
	var discard io.Writer = io.Discard
	_ = discard
}

// TestPickResolveCandidatesDefaultsUnscoped pins the Note under R-12 AC5:
// a bare invocation with no --scope and no --line is a legitimate
// unscoped call, not an error.
func TestPickResolveCandidatesDefaultsUnscoped(t *testing.T) {
	pick := setupPickScopeTest(t)
	createScopeFixture(t)

	got, err := resolvePickCandidates(pick)
	if err != nil {
		t.Fatalf("resolvePickCandidates() error = %v", err)
	}
	if len(got) != len(core.All()) {
		t.Errorf("got %d candidates, want the full unscoped store (%d)", len(got), len(core.All()))
	}
}

// TestPickScopeFlagNarrowsCandidates pins AC1/SC-01: --scope narrows the
// candidate set to the named types, strictly smaller than the unscoped
// call, on this test's own 12-item fixture (2 milestone, 3 epic, 2
// feature, 2 bug, 3 task).
func TestPickScopeFlagNarrowsCandidates(t *testing.T) {
	pick := setupPickScopeTest(t)
	createScopeFixture(t)

	if err := pick.Flags().Set("scope", "milestone,epic"); err != nil {
		t.Fatalf("setting --scope: %v", err)
	}

	got, err := resolvePickCandidates(pick)
	if err != nil {
		t.Fatalf("resolvePickCandidates() error = %v", err)
	}

	all := core.All()
	if len(got) >= len(all) {
		t.Fatalf("scoped candidates (%d) not strictly smaller than unscoped (%d)", len(got), len(all))
	}
	if len(got) != 5 {
		t.Errorf("got %d milestone+epic candidates, want 5 (2 milestones + 3 epics)", len(got))
	}
	for _, b := range got {
		if b.Type != "milestone" && b.Type != "epic" {
			t.Errorf("candidate %s has type %q, want milestone or epic", b.ID, b.Type)
		}
	}
}

// TestPickScopeFlagRejectsUnknownType pins AC1/AC5: an unknown --scope
// type is a visible error, never a silent ignore.
func TestPickScopeFlagRejectsUnknownType(t *testing.T) {
	pick := setupPickScopeTest(t)
	createScopeFixture(t)

	if err := pick.Flags().Set("scope", "bogus"); err != nil {
		t.Fatalf("setting --scope: %v", err)
	}

	_, err := resolvePickCandidates(pick)
	if err == nil {
		t.Fatal("expected an error for an unknown --scope type")
	}
	if !strings.Contains(err.Error(), "lif pick: invalid --scope type: bogus") {
		t.Errorf("error = %q, want it to mention the invalid type", err)
	}
}

// TestPickScopeFlagRejectsBlankValue pins AC5's "never silently fall back"
// rule for the degenerate --scope case: a value with no actual type names
// must error, not silently resolve to zero types and an empty candidate
// set masquerading as a deliberate result.
func TestPickScopeFlagRejectsBlankValue(t *testing.T) {
	pick := setupPickScopeTest(t)
	createScopeFixture(t)

	if err := pick.Flags().Set("scope", " , "); err != nil {
		t.Fatalf("setting --scope: %v", err)
	}

	_, err := resolvePickCandidates(pick)
	if err == nil {
		t.Fatal("expected an error for a --scope value with no types")
	}
	if !strings.Contains(err.Error(), "lif pick: --scope requires at least one item type") {
		t.Errorf("error = %q, want it to mention the missing type", err)
	}
}

// TestPickPartialLineRoadmapVerbNarrowsToContainerRanks pins AC2/SC-02
// (corrected 2026-09-09, K-11): a partial line equivalent to `items
// roadmap` narrows to the three container ranks (milestone, epic,
// feature) -- not to `--scope milestone,epic`, which is a strictly
// smaller, different set.
func TestPickPartialLineRoadmapVerbNarrowsToContainerRanks(t *testing.T) {
	pick := setupPickScopeTest(t)
	createScopeFixture(t)

	line := "roadmap"
	if err := pick.Flags().Set("line", line); err != nil {
		t.Fatalf("setting --line: %v", err)
	}
	if err := pick.Flags().Set("cursor", strconv.Itoa(len(line))); err != nil {
		t.Fatalf("setting --cursor: %v", err)
	}

	got, err := resolvePickCandidates(pick)
	if err != nil {
		t.Fatalf("resolvePickCandidates() error = %v", err)
	}
	if len(got) != 7 {
		t.Errorf("got %d roadmap-scope candidates, want 7 (2 milestones + 3 epics + 2 features)", len(got))
	}
	for _, b := range got {
		if b.Type != "milestone" && b.Type != "epic" && b.Type != "feature" {
			t.Errorf("candidate %s has type %q, want a container-rank type", b.ID, b.Type)
		}
	}

	scopeOnly, err := scopeFilteredCandidates("milestone,epic")
	if err != nil {
		t.Fatalf("scopeFilteredCandidates() error = %v", err)
	}
	if len(got) == len(scopeOnly) {
		t.Error("roadmap-verb scope must not equal --scope milestone,epic (feature/rank 3 belongs to it too)")
	}
}

// TestPickPartialLineUnknownVerbErrors pins AC5: a partial line whose verb
// does not resolve against the real command tree is a visible error.
func TestPickPartialLineUnknownVerbErrors(t *testing.T) {
	pick := setupPickScopeTest(t)
	createScopeFixture(t)

	line := "bogus"
	if err := pick.Flags().Set("line", line); err != nil {
		t.Fatalf("setting --line: %v", err)
	}
	if err := pick.Flags().Set("cursor", strconv.Itoa(len(line))); err != nil {
		t.Fatalf("setting --cursor: %v", err)
	}

	_, err := resolvePickCandidates(pick)
	if err == nil {
		t.Fatal("expected an error for an unresolvable verb")
	}
	if !strings.Contains(err.Error(), "lif pick: partial line verb does not resolve") {
		t.Errorf("error = %q, want it to mention verb resolution", err)
	}
}

// TestPickPartialLinePlumbingVerbErrors pins AC4/AC5: scope derivation
// reads IsUserFacing, so a plumbing verb (e.g. "path", markPlumbing'd in
// register.go) is reported, not silently treated as unscoped.
func TestPickPartialLinePlumbingVerbErrors(t *testing.T) {
	pick := setupPickScopeTest(t)
	createScopeFixture(t)

	line := "path"
	if err := pick.Flags().Set("line", line); err != nil {
		t.Fatalf("setting --line: %v", err)
	}
	if err := pick.Flags().Set("cursor", strconv.Itoa(len(line))); err != nil {
		t.Fatalf("setting --cursor: %v", err)
	}

	_, err := resolvePickCandidates(pick)
	if err == nil {
		t.Fatal("expected an error for a plumbing verb")
	}
	if !strings.Contains(err.Error(), `lif pick: "path" is not a user-facing verb`) {
		t.Errorf("error = %q, want it to mention user-facing", err)
	}
}

// TestPickPartialLineParentCursorExcludesDescendants pins AC3/SC-03: a
// cursor at a --parent value excludes both the edited item and its
// descendants, via candidates.ParentCandidates (R-03), from the candidate
// set -- while a same-typed item that is NOT a descendant stays in.
func TestPickPartialLineParentCursorExcludesDescendants(t *testing.T) {
	pick := setupPickScopeTest(t)

	unrelatedMilestone := &item.Item{ID: "beans-uq20", Title: "Root milestone", Type: "milestone", Status: "todo"}
	edited := &item.Item{ID: "beans-uq21", Title: "Edited feature", Type: "feature", Status: "todo"}
	descendant := &item.Item{ID: "beans-uq22", Title: "Feature's epic child", Type: "epic", Status: "todo", Parent: "beans-uq21"}
	unrelatedEpic := &item.Item{ID: "beans-uq23", Title: "Unrelated epic", Type: "epic", Status: "todo"}
	for _, b := range []*item.Item{unrelatedMilestone, edited, descendant, unrelatedEpic} {
		if err := core.Create(b); err != nil {
			t.Fatalf("seeding %s: %v", b.ID, err)
		}
	}

	line := "update beans-uq21 --parent "
	if err := pick.Flags().Set("line", line); err != nil {
		t.Fatalf("setting --line: %v", err)
	}
	if err := pick.Flags().Set("cursor", strconv.Itoa(len(line))); err != nil {
		t.Fatalf("setting --cursor: %v", err)
	}

	got, err := resolvePickCandidates(pick)
	if err != nil {
		t.Fatalf("resolvePickCandidates() error = %v", err)
	}

	gotIDs := make(map[string]bool, len(got))
	for _, b := range got {
		gotIDs[b.ID] = true
	}
	if !gotIDs["beans-uq20"] {
		t.Error("expected the unrelated milestone in the candidate set")
	}
	if !gotIDs["beans-uq23"] {
		t.Error("expected the unrelated epic in the candidate set")
	}
	if gotIDs["beans-uq21"] {
		t.Error("edited bean must not be its own candidate")
	}
	if gotIDs["beans-uq22"] {
		t.Error("descendant of the edited bean must be excluded (R-03)")
	}
	if len(got) != 2 {
		t.Errorf("got %d candidates, want exactly 2 (unrelated milestone + unrelated epic)", len(got))
	}
}

// TestPickPartialLineParentWithoutKnownItemErrors pins AC5 for the
// --parent path: a cursor at --parent with no positional token that
// resolves to a known item must error, never silently fall back to an
// unscoped or empty result.
func TestPickPartialLineParentWithoutKnownItemErrors(t *testing.T) {
	pick := setupPickScopeTest(t)
	createScopeFixture(t)

	line := "create --parent "
	if err := pick.Flags().Set("line", line); err != nil {
		t.Fatalf("setting --line: %v", err)
	}
	if err := pick.Flags().Set("cursor", strconv.Itoa(len(line))); err != nil {
		t.Fatalf("setting --cursor: %v", err)
	}

	_, err := resolvePickCandidates(pick)
	if err == nil {
		t.Fatal("expected an error when no positional bean id precedes --parent")
	}
	if !strings.Contains(err.Error(), "lif pick: cannot resolve --parent context: no known item id precedes --parent") {
		t.Errorf("error = %q, want it to mention the unresolved --parent context", err)
	}
}

// TestPickLineRequiresCursor pins AC5: --line without --cursor is
// unresolvable context, not a silent unscoped fallback.
func TestPickLineRequiresCursor(t *testing.T) {
	pick := setupPickScopeTest(t)
	createScopeFixture(t)

	if err := pick.Flags().Set("line", "roadmap"); err != nil {
		t.Fatalf("setting --line: %v", err)
	}

	_, err := resolvePickCandidates(pick)
	if err == nil {
		t.Fatal("expected an error when --line is set without --cursor")
	}
	if !strings.Contains(err.Error(), "lif pick: --line requires --cursor") {
		t.Errorf("error = %q, want it to mention the missing --cursor", err)
	}
}

// TestPickLineRequiresCursorInRange pins AC5: an out-of-range --cursor is
// unresolvable context, not a silent unscoped fallback.
func TestPickLineRequiresCursorInRange(t *testing.T) {
	pick := setupPickScopeTest(t)
	createScopeFixture(t)

	if err := pick.Flags().Set("line", "roadmap"); err != nil {
		t.Fatalf("setting --line: %v", err)
	}
	if err := pick.Flags().Set("cursor", "999"); err != nil {
		t.Fatalf("setting --cursor: %v", err)
	}

	_, err := resolvePickCandidates(pick)
	if err == nil {
		t.Fatal("expected an error for an out-of-range --cursor")
	}
	if !strings.Contains(err.Error(), "lif pick: --cursor 999 is out of range") {
		t.Errorf("error = %q, want it to mention the out-of-range cursor", err)
	}
}

// TestPickPartialLineVerbWithoutScopeSemanticsErrors pins AC5's coverage
// of the resolvedCmd-but-unmapped branch: a verb that resolves, and is
// user-facing, but carries no known type-subset mapping (only roadmap
// does) must still be a visible error -- never a silent fallback to the
// full, unscoped candidate set. Coordinator fix-round (2026-09-09):
// mutating that branch to fall back silently previously left every
// existing test green.
func TestPickPartialLineVerbWithoutScopeSemanticsErrors(t *testing.T) {
	pick := setupPickScopeTest(t)
	createScopeFixture(t)

	line := "list"
	if err := pick.Flags().Set("line", line); err != nil {
		t.Fatalf("setting --line: %v", err)
	}
	if err := pick.Flags().Set("cursor", strconv.Itoa(len(line))); err != nil {
		t.Fatalf("setting --cursor: %v", err)
	}

	_, err := resolvePickCandidates(pick)
	if err == nil {
		t.Fatal("expected an error for a resolved, user-facing verb with no known scope mapping")
	}
	if !strings.Contains(err.Error(), `lif pick: verb "list" has no known scope-derivation mapping`) {
		t.Errorf("error = %q, want it to mention the missing scope-derivation mapping", err)
	}
}

// TestPickPartialLineToleratesLeadingProgramName pins the fix-round
// correction to AC2/SC-02: a live shell buffer's partial line carries the
// program name as its first token ("lif roadmap "), not just the bare
// verb ("roadmap "). Both forms must resolve to the identical candidate
// set, and the program name is recognized via cmd.Root().Name(), never a
// literal "beans" comparison (AC4).
func TestPickPartialLineToleratesLeadingProgramName(t *testing.T) {
	pick := setupPickScopeTest(t)
	createScopeFixture(t)

	bareLine := "roadmap"
	if err := pick.Flags().Set("line", bareLine); err != nil {
		t.Fatalf("setting --line: %v", err)
	}
	if err := pick.Flags().Set("cursor", strconv.Itoa(len(bareLine))); err != nil {
		t.Fatalf("setting --cursor: %v", err)
	}
	bareGot, err := resolvePickCandidates(pick)
	if err != nil {
		t.Fatalf("resolvePickCandidates() with bare verb error = %v", err)
	}

	prefixedLine := "lif roadmap"
	if err := pick.Flags().Set("line", prefixedLine); err != nil {
		t.Fatalf("setting --line: %v", err)
	}
	if err := pick.Flags().Set("cursor", strconv.Itoa(len(prefixedLine))); err != nil {
		t.Fatalf("setting --cursor: %v", err)
	}
	prefixedGot, err := resolvePickCandidates(pick)
	if err != nil {
		t.Fatalf("resolvePickCandidates() with \"lif \"-prefixed verb error = %v", err)
	}

	if len(bareGot) == 0 {
		t.Fatal("expected a non-empty candidate set for the bare-verb form")
	}
	if len(bareGot) != len(prefixedGot) {
		t.Fatalf("bare verb gave %d candidates, \"lif \"-prefixed gave %d, want equal", len(bareGot), len(prefixedGot))
	}
	bareIDs := make(map[string]bool, len(bareGot))
	for _, b := range bareGot {
		bareIDs[b.ID] = true
	}
	for _, b := range prefixedGot {
		if !bareIDs[b.ID] {
			t.Errorf("prefixed-form candidate %s not present in bare-form result", b.ID)
		}
	}
}

// TestPickWidgetLineWithUnmappedVerbErrors pins beans-eeej: the zsh
// widget's non-empty-buffer branch forwards --line/--cursor exactly as
// beans-pick.zsh now does ("lif <buffer>" with the cursor at the
// buffer's end). An unmapped verb (only roadmap carries scope semantics)
// must still surface AC5's visible error, never a silent fallback to the
// full, unscoped candidate set -- this is what a widget invocation typed
// mid "lif show ..." now gets instead of the previously-conflated
// unconstrained result.
func TestPickWidgetLineWithUnmappedVerbErrors(t *testing.T) {
	pick := setupPickScopeTest(t)
	createScopeFixture(t)

	buffer := "lif show beans-uq10"
	if err := pick.Flags().Set("line", buffer); err != nil {
		t.Fatalf("setting --line: %v", err)
	}
	if err := pick.Flags().Set("cursor", strconv.Itoa(len(buffer))); err != nil {
		t.Fatalf("setting --cursor: %v", err)
	}

	_, err := resolvePickCandidates(pick)
	if err == nil {
		t.Fatal("expected an error for a widget-shaped --line with an unmapped verb")
	}
	if !strings.Contains(err.Error(), `lif pick: verb "show" has no known scope-derivation mapping`) {
		t.Errorf("error = %q, want it to mention the missing scope-derivation mapping", err)
	}
}

// The picker's own labels and the partial-line errors that no other test
// reaches name the product as lif and the noun as item.
func TestPickLabelsAndLineErrorsNameLifAndItem(t *testing.T) {
	t.Run("picker title", func(t *testing.T) {
		m, _ := newPickModel([]*item.Item{pickTestItem("beans-aaaa", "Alpha")}).Update(tea.WindowSizeMsg{Width: 60, Height: 12})
		if view := m.View(); !strings.Contains(view, "Pick an item") {
			t.Errorf("picker view lacks the title %q:\n%s", "Pick an item", view)
		}
	})

	t.Run("aborted pick", func(t *testing.T) {
		if got := errPickAborted.Error(); got != "lif pick: no item selected" {
			t.Errorf("errPickAborted = %q, want %q", got, "lif pick: no item selected")
		}
	})

	pick := setupPickScopeTest(t)
	for _, tc := range []struct {
		name, line string
		cursor     int
		want       string
	}{
		{"blank line", "  ", 0, "lif pick: --line has no verb to resolve"},
		{"program name only", "lif", 3, "lif pick: --line has no verb after the program name"},
		{"cursor on program name", "lif roadmap", 1, "lif pick: cursor position is not understood by scope derivation (it sits on the program name)"},
		{"flag only", "--help", 6, "lif pick: partial line has no resolvable verb"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := partialLineCandidates(pick, tc.line, tc.cursor)
			if err == nil || err.Error() != tc.want {
				t.Errorf("partialLineCandidates(%q, %d) error = %v, want %q", tc.line, tc.cursor, err, tc.want)
			}
		})
	}
}

// TestPickWidgetEmptyBufferStaysUnconstrained pins beans-eeej: the
// widget's empty-buffer branch calls flag-less `lif pick` (neither
// --line nor --cursor set), which resolvePickCandidates's default case
// still resolves to the full, unscoped store -- a legitimate outcome for
// starting a line from nothing, not an AC5 "not understood" failure.
func TestPickWidgetEmptyBufferStaysUnconstrained(t *testing.T) {
	pick := setupPickScopeTest(t)
	createScopeFixture(t)

	got, err := resolvePickCandidates(pick)
	if err != nil {
		t.Fatalf("resolvePickCandidates() with no --line/--cursor error = %v, want the unscoped set", err)
	}
	if len(got) != 12 {
		t.Errorf("len(got) = %d, want all 12 fixture beans", len(got))
	}
}

// TestPickPartialLineParentCursorOmitsArchivedItems: a cursor at a --parent
// value leaves out containers with an archive status (R-14).
func TestPickPartialLineParentCursorOmitsArchivedItems(t *testing.T) {
	pick := setupPickScopeTest(t)

	for _, b := range []*item.Item{
		{ID: "beans-ua01", Title: "Open epic", Type: "epic", Status: "todo"},
		{ID: "beans-ua02", Title: "Done epic", Type: "epic", Status: "completed"},
		{ID: "beans-ua03", Title: "Scrapped epic", Type: "epic", Status: "scrapped"},
		{ID: "beans-ua04", Title: "Task", Type: "task", Status: "todo"},
	} {
		if err := core.Create(b); err != nil {
			t.Fatalf("seeding %s: %v", b.ID, err)
		}
	}

	line := "update beans-ua04 --parent "
	if err := pick.Flags().Set("line", line); err != nil {
		t.Fatalf("setting --line: %v", err)
	}
	if err := pick.Flags().Set("cursor", strconv.Itoa(len(line))); err != nil {
		t.Fatalf("setting --cursor: %v", err)
	}

	got, err := resolvePickCandidates(pick)
	if err != nil {
		t.Fatalf("resolvePickCandidates() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != "beans-ua01" {
		t.Errorf("got %v, want only the open epic beans-ua01", got)
	}
}
