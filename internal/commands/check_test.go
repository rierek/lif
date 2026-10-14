package commands

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/xRiErOS/lif/internal/ui"
	"github.com/xRiErOS/lif/pkg/config"
	"github.com/xRiErOS/lif/pkg/item"
	"github.com/xRiErOS/lif/pkg/itemcore"
)

// TestCheckHelperProcess is not a real test: it is the in-process side of
// the subprocess trick runCheckInTestStore uses to exercise checkCmd safely.
//
// checkCmd.RunE ends with `if totalIssues > 0 { os.Exit(1) }`, and a config
// or item-content issue in the store under test can legitimately trigger
// that exit. Calling checkCmd.RunE in-process would os.Exit the whole
// `go test` binary -- taking every other test in this package down with
// it. Running it as a subprocess keeps the os.Exit call harmless: only the
// subprocess exits, and the parent test just reads what it printed first.
func TestCheckHelperProcess(t *testing.T) {
	if os.Getenv("BEANS_CHECK_HELPER") != "1" {
		t.Skip("only runs as a subprocess of runCheckInTestStore")
	}

	lipgloss.SetColorProfile(termenv.TrueColor)
	ui.SetTheme(os.Getenv("BEANS_CHECK_THEME"))
	if os.Getenv("BEANS_CHECK_JSON") == "1" {
		checkJSON = true
	}

	itemsDir := os.Getenv("BEANS_CHECK_DIR")
	testCfg := config.Default()
	// Lets a test put a USER override into the subprocess's config, which is
	// the only way to exercise the merged-list colour validation across the
	// process boundary.
	if c := os.Getenv("BEANS_CHECK_BAD_STATUS_COLOR"); c != "" {
		testCfg.Statuses = []config.StatusOverride{{Name: "todo", Color: c}}
	}
	if c := os.Getenv("BEANS_CHECK_BAD_TYPE_COLOR"); c != "" {
		testCfg.Types = []config.TypeOverride{{Name: "bug", Color: c}}
	}
	if th := os.Getenv("BEANS_CHECK_BAD_THEME"); th != "" {
		testCfg.Display.Theme = th
	}
	testCore := itemcore.New(itemsDir, testCfg)
	if err := testCore.Load(); err != nil {
		os.Stderr.WriteString("loading core: " + err.Error() + "\n")
		os.Exit(2)
	}
	if os.Getenv("BEANS_CHECK_BROKEN_PARENT") == "1" {
		if err := testCore.Create(&item.Item{ID: "aaa1", Title: "A", Status: "todo", Parent: "nonexistent"}); err != nil {
			os.Stderr.WriteString("creating item: " + err.Error() + "\n")
			os.Exit(2)
		}
	}
	core, cfg = testCore, testCfg

	_ = checkCmd.RunE(checkCmd, nil) // may os.Exit(1); that's fine, we're a subprocess
}

// runCheckInTestStore runs `lif check` against a small empty store, with
// the theme currently active in the calling (parent) test process, in a
// subprocess -- see TestCheckHelperProcess for why. It returns whatever the
// subprocess wrote to stdout and stderr.
func runCheckInTestStore(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	itemsDir := filepath.Join(dir, ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("creating test .beans dir: %v", err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestCheckHelperProcess$")
	cmd.Env = append(os.Environ(),
		"BEANS_CHECK_HELPER=1",
		"BEANS_CHECK_DIR="+itemsDir,
		"BEANS_CHECK_THEME="+ui.ActiveTheme().Name,
		"BEANS_CHECK_BAD_STATUS_COLOR="+os.Getenv("BEANS_CHECK_BAD_STATUS_COLOR"),
		"BEANS_CHECK_BAD_TYPE_COLOR="+os.Getenv("BEANS_CHECK_BAD_TYPE_COLOR"),
		"BEANS_CHECK_BAD_THEME="+os.Getenv("BEANS_CHECK_BAD_THEME"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			t.Fatalf("running check subprocess: %v\noutput:\n%s", err, out)
		}
		// An *exec.ExitError just means checkCmd.RunE found issues and
		// exited 1, which is expected whenever the test deliberately feeds
		// it a bad colour/theme override; it is not a test failure.
	}
	return string(out)
}

// TestCheckInheritsTheThemePalette proves check's output actually follows
// ui.SetTheme, the way ui.Success/ui.Danger/ui.Bold/ui.Warning were always
// meant to: check.go renders exclusively through those shared styles, never
// a colour of its own, so a theme switch must change what it prints.
func TestCheckInheritsTheThemePalette(t *testing.T) {
	t.Cleanup(func() { ui.SetTheme("mocha") })

	ui.SetTheme("mocha")
	mocha := runCheckInTestStore(t)
	ui.SetTheme("latte")
	latte := runCheckInTestStore(t)

	if mocha == latte {
		t.Error("check does not follow the theme; it should through ui.Success and friends")
	}
}

// TestCheckStaysAReportNotATable guards the report shape: check prints tick
// lines, not an item listing with a TITLE column header.
func TestCheckStaysAReportNotATable(t *testing.T) {
	out := stripANSITest(runCheckInTestStore(t))
	if strings.Contains(out, "TITLE") {
		t.Errorf("check is a report of tick lines, not a bean listing:\n%s", out)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("check must keep its tick lines:\n%s", out)
	}
}

// TestCheckReportsNoConfigIssueForDefaultTables guards the regression this
// fix round exists for: config.DefaultTypes deliberately leaves "task"
// uncoloured (Color: "", task 3's ranked type scale), and an empty colour
// means "no explicit colour, fall back to the muted default" -- the same
// thing it means to ui.ResolveColor and to a StatusOverride's merge. Before
// this fix, check.go's colour-validation loops ran ui.IsValidColor("") and
// got false, so `lif check` reported "invalid color ” for type 'task'"
// and exited 1 on every repository, including a freshly created empty one.
func TestCheckReportsNoConfigIssueForDefaultTables(t *testing.T) {
	out := stripANSITest(runCheckInTestStore(t))
	if strings.Contains(out, "invalid color") {
		t.Errorf("check reported an invalid colour for the default tables:\n%s", out)
	}
	if !strings.Contains(out, "All checks passed") {
		t.Errorf("expected a clean check for a fresh store, got:\n%s", out)
	}
}

// AC3: check's section heading and tick line name the noun as item.
func TestCheckReportNamesItems(t *testing.T) {
	out := stripANSITest(runCheckInTestStore(t))
	for _, want := range []string{"Item Links", "Every item carries a known type"} {
		if !strings.Contains(out, want) {
			t.Errorf("check report lacks %q:\n%s", want, out)
		}
	}
}

func TestUnknownTypeItemsFindsItemsTheConfigDoesNotCover(t *testing.T) {
	prev := cfg
	cfg = &config.Config{}
	defer func() { cfg = prev }()

	items := []*item.Item{
		{ID: "a1", Type: "task"},
		{ID: "a2", Type: "chore"},
		{ID: "a3", Type: "bug"},
		{ID: "a4", Type: ""},
	}

	got := unknownTypeItems(items)

	if len(got) != 1 {
		t.Fatalf("got %d beans, want 1", len(got))
	}
	if got[0].ID != "a2" {
		t.Errorf("flagged %q, want \"a2\"", got[0].ID)
	}
}

func TestUnknownTypeItemsAcceptsAConfiguredType(t *testing.T) {
	prev := cfg
	cfg = &config.Config{Types: []config.TypeOverride{{Name: "chore"}}}
	defer func() { cfg = prev }()

	if got := unknownTypeItems([]*item.Item{{ID: "a2", Type: "chore"}}); len(got) != 0 {
		t.Errorf("got %d beans, want 0 — chore is configured", len(got))
	}
}

func TestItemFieldIssuesFlagsHandEditedFields(t *testing.T) {
	prev := cfg
	cfg = &config.Config{}
	defer func() { cfg = prev }()

	items := []*item.Item{
		{ID: "m1", Type: "milestone", Status: "todo"},
		{ID: "ok", Type: "task", Status: "todo", Priority: "normal", Parent: "m1", Tags: []string{"fine"}},
		{ID: "rank", Type: "milestone", Status: "todo", Parent: "ok"},
		{ID: "st", Type: "task", Status: "bogus"},
		{ID: "pr", Type: "task", Status: "todo", Priority: "urgentest"},
		{ID: "tg", Type: "task", Status: "todo", Tags: []string{"Bad Tag!"}},
	}

	got := itemFieldIssues(items)
	if len(got) != 4 {
		t.Fatalf("got %d issues, want 4: %v", len(got), got)
	}
}

// TestCheckValidatesConfiguredColoursNotOnlyDefaults pins the fix for the
// final review's config finding. The colour checks iterated
// config.DefaultStatuses and config.DefaultTypes -- compile-time constants
// this repository writes itself -- so they could only ever report a defect
// we had shipped, never one a user configured. That made the entire
// override surface introduced by this branch unvalidated, and made the
// guard structurally incapable of failing.
//
// Mutation: point the loops back at config.DefaultStatuses/DefaultTypes and
// this goes green again while the user's broken colour stays unreported.
func TestCheckValidatesConfiguredColoursNotOnlyDefaults(t *testing.T) {
	t.Setenv("BEANS_CHECK_BAD_STATUS_COLOR", "not-a-tone")
	t.Setenv("BEANS_CHECK_BAD_TYPE_COLOR", "chartreuse")

	out := stripANSITest(runCheckInTestStore(t))

	if !strings.Contains(out, "invalid color 'not-a-tone' for status 'todo'") {
		t.Errorf("check did not report the user's invalid status colour:\n%s", out)
	}
	if !strings.Contains(out, "invalid color 'chartreuse' for type 'bug'") {
		t.Errorf("check did not report the user's invalid type colour:\n%s", out)
	}
}

// TestCheckReportsAnUnknownTheme covers the one place a display.theme typo
// can surface. ui.SetTheme keeps the current palette on an unknown name by
// design, so `display: {theme: moccha}` renders in mocha and says nothing --
// the user never learns their configuration was ignored.
//
// Mutation: drop the IsValidTheme branch in check.go and this goes red.
func TestCheckReportsAnUnknownTheme(t *testing.T) {
	t.Setenv("BEANS_CHECK_BAD_THEME", "moccha")

	out := stripANSITest(runCheckInTestStore(t))

	if !strings.Contains(out, "unknown display.theme 'moccha'") {
		t.Errorf("check did not report the unknown theme:\n%s", out)
	}
}

// TestCheckJSONNamesLinkIssuesWithItemKeys runs `check --json` against a store
// holding one broken parent link and asserts the product-defined keys use the
// item vocabulary: `item_issues` for the link-check block, `item_id` inside
// each broken-link entry.
func TestCheckJSONNamesLinkIssuesWithItemKeys(t *testing.T) {
	dir := t.TempDir()
	itemsDir := filepath.Join(dir, ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("creating test store dir: %v", err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestCheckHelperProcess$")
	cmd.Env = append(os.Environ(),
		"BEANS_CHECK_HELPER=1",
		"BEANS_CHECK_DIR="+itemsDir,
		"BEANS_CHECK_THEME="+ui.ActiveTheme().Name,
		"BEANS_CHECK_JSON=1",
		"BEANS_CHECK_BROKEN_PARENT=1",
	)
	out, err := cmd.Output() // exit 1 (issues found) is expected
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			t.Fatalf("running check subprocess: %v", err)
		}
	}

	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("decoding JSON: %v; output = %s", err, out)
	}
	issues, ok := doc["item_issues"].(map[string]any)
	if !ok {
		t.Fatalf(`doc["item_issues"] missing or not an object; output = %s`, out)
	}
	if _, ok := doc["bean_issues"]; ok {
		t.Errorf("output still carries legacy key bean_issues; output = %s", out)
	}
	broken, _ := issues["broken_links"].([]any)
	if len(broken) != 1 {
		t.Fatalf(`item_issues.broken_links = %v, want one entry; output = %s`, issues["broken_links"], out)
	}
	entry, _ := broken[0].(map[string]any)
	if entry["item_id"] != "aaa1" {
		t.Errorf(`broken_links[0].item_id = %v, want "aaa1"; entry = %v`, entry["item_id"], entry)
	}
	if _, ok := entry["bean_id"]; ok {
		t.Errorf("broken-link entry still carries legacy key bean_id: %v", entry)
	}
}
