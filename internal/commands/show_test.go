package commands

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
)

// setupShowTest installs a throwaway core and the default config into the
// package globals that showStyledItem reads (cfg.GetStatus, core.ImplicitStatus)
// and restores both afterwards.
func setupShowTest(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	itemsDir := filepath.Join(tmpDir, ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}

	testCfg := config.Default()
	testCore := itemcore.New(itemsDir, testCfg)
	if err := testCore.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}

	oldCore, oldCfg := core, cfg
	core, cfg = testCore, testCfg
	t.Cleanup(func() { core, cfg = oldCore, oldCfg })
}

func showTestItem(id, title, body string) *item.Item {
	return &item.Item{
		ID:     id,
		Slug:   item.Slugify(title),
		Title:  title,
		Status: "todo",
		Type:   "task",
		Body:   body,
	}
}

func mustRender(t *testing.T, b *item.Item) string {
	t.Helper()
	content, err := b.Render()
	if err != nil {
		t.Fatalf("item.Render() error = %v", err)
	}
	return string(content)
}

// REQ-01 / AC-01.1: without a terminal on stdout, the default output path must
// be the byte-identical raw rendering -- the same text `--raw` produces. A
// mutation that keeps the glamour path for the non-TTY branch turns this red.
func TestShowOutputSwitchesOnTTY(t *testing.T) {
	setupShowTest(t)
	b := showTestItem("beans-test1", "A test bean", "# Heading\n\nSome body text.\n")

	t.Run("non-tty is byte-identical to raw", func(t *testing.T) {
		got, err := showOutput(b, false, false, 110)
		if err != nil {
			t.Fatalf("showOutput() error = %v", err)
		}
		want := mustRender(t, b)
		if got != want {
			t.Errorf("non-TTY output differs from item.Render()\n got: %q\nwant: %q", got, want)
		}
	})

	// REQ-02 / AC-02.1: the terminal branch keeps the styled rendering. ANSI is
	// deliberately not asserted (D04) -- under `go test` stdout is not a
	// terminal, so lipgloss and glamour degrade their colour profile and emit
	// no escape sequences. The horizontal rule is the stable marker.
	t.Run("tty renders the styled representation", func(t *testing.T) {
		got, err := showOutput(b, true, false, 110)
		if err != nil {
			t.Fatalf("showOutput() error = %v", err)
		}
		rule := strings.Repeat("─", 50)
		if !strings.Contains(got, rule) {
			t.Errorf("expected TTY output to contain the 50-char horizontal rule, got %q", got)
		}
		raw := mustRender(t, b)
		if got == raw {
			t.Error("TTY output must differ from the raw rendering")
		}
	})
}

// REQ-04 / AC-04.1: without a terminal, consecutive items are joined by
// "\n---\n\n". The assertion is an equality against the expected concatenation
// rather than a substring count: item.Render emits "\n---\n\n" around its own
// closing front-matter fence, so a global count would not isolate the joint.
func TestShowOutputAllSeparatorNonTTY(t *testing.T) {
	setupShowTest(t)
	b1 := showTestItem("beans-test4", "First bean", "First body.\n")
	b2 := showTestItem("beans-test5", "Second bean", "Second body.\n")

	got, err := showOutputAll([]*item.Item{b1, b2}, false, false, 110)
	if err != nil {
		t.Fatalf("showOutputAll() error = %v", err)
	}

	want := mustRender(t, b1) + "\n---\n\n" + mustRender(t, b2)
	if got != want {
		t.Errorf("non-TTY join differs\n got: %q\nwant: %q", got, want)
	}
}

// REQ-02 / AC-02.2: with a terminal, consecutive items are joined by the
// separator showCmd.RunE builds today from three fmt.Println calls -- a blank
// line, a rule of 60 U+2550, and another blank line.
func TestShowOutputAllSeparatorTTY(t *testing.T) {
	setupShowTest(t)
	b1 := showTestItem("beans-test6", "First bean", "First body.\n")
	b2 := showTestItem("beans-test7", "Second bean", "Second body.\n")

	got, err := showOutputAll([]*item.Item{b1, b2}, true, false, 110)
	if err != nil {
		t.Fatalf("showOutputAll() error = %v", err)
	}

	sep := "\n" + ui.Muted.Render(strings.Repeat("═", 60)) + "\n\n"
	if n := strings.Count(got, sep); n != 1 {
		t.Errorf("expected exactly 1 occurrence of the TTY separator, got %d", n)
	}

	first, err := showOutput(b1, true, false, 110)
	if err != nil {
		t.Fatalf("showOutput() error = %v", err)
	}
	second, err := showOutput(b2, true, false, 110)
	if err != nil {
		t.Fatalf("showOutput() error = %v", err)
	}
	if want := first + sep + second; got != want {
		t.Errorf("TTY join differs\n got: %q\nwant: %q", got, want)
	}
}

// REQ-01 / AC-01.3: an item without a body still round-trips byte-identically,
// including the trailing POSIX newline item.Render appends after the closing
// front-matter fence.
func TestShowOutputEmptyBodyNonTTY(t *testing.T) {
	setupShowTest(t)
	b := showTestItem("beans-test2", "Bean without body", "")

	got, err := showOutput(b, false, false, 110)
	if err != nil {
		t.Fatalf("showOutput() error = %v", err)
	}
	if want := mustRender(t, b); got != want {
		t.Errorf("non-TTY output differs from item.Render()\n got: %q\nwant: %q", got, want)
	}
	if !strings.HasSuffix(got, "---\n\n") {
		t.Errorf("expected output to end with %q, got tail %q", "---\n\n", got[max(0, len(got)-10):])
	}
}

// REQ-01: guards the wiring, not just the branch. The unit tests above call
// showOutput directly, so they stay green even if showCmd.RunE stops consulting
// term.IsTerminal and hardcodes the styled path. This test runs the command
// itself with os.Stdout replaced by a pipe -- which is exactly the non-TTY
// case -- and pins the output to item.Render().
func TestShowCmdNonTTYWiring(t *testing.T) {
	setupShowTest(t)

	b := showTestItem("beans-test8", "Wired bean", "# Heading\n\nSome body text.\n")
	if err := core.Create(b); err != nil {
		t.Fatalf("failed to create test bean: %v", err)
	}

	oldJSON, oldRaw, oldBody, oldETag := showJSON, showRaw, showBodyOnly, showETagOnly
	showJSON, showRaw, showBodyOnly, showETagOnly = false, false, false, false
	t.Cleanup(func() {
		showJSON, showRaw, showBodyOnly, showETagOnly = oldJSON, oldRaw, oldBody, oldETag
	})

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	runErr := showCmd.RunE(showCmd, []string{b.ID})

	os.Stdout = oldStdout
	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe write end: %v", err)
	}
	captured, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}
	if runErr != nil {
		t.Fatalf("showCmd.RunE() error = %v", runErr)
	}

	stored, err := core.Get(b.ID)
	if err != nil {
		t.Fatalf("core.Get() error = %v", err)
	}
	if want := mustRender(t, stored); string(captured) != want {
		t.Errorf("piped command output differs from item.Render()\n got: %q\nwant: %q", string(captured), want)
	}
}

// REQ-05 / AC-05.1: the non-TTY path must not hard-wrap. A 300-character
// paragraph occupying one source line stays one output line; the glamour
// renderer would break it at 80 columns.
func TestShowNonTTYPreservesLineStructure(t *testing.T) {
	setupShowTest(t)
	longLine := strings.Repeat("lorem ipsum dolor sit amet ", 12) // 324 chars
	b := showTestItem("beans-test3", "Bean with a long paragraph", longLine+"\n")

	got, err := showOutput(b, false, false, 110)
	if err != nil {
		t.Fatalf("showOutput() error = %v", err)
	}

	var found int
	for _, line := range strings.Split(got, "\n") {
		if line == longLine {
			found++
		}
	}
	if found != 1 {
		t.Errorf("expected the %d-character paragraph as exactly 1 line, found %d", len(longLine), found)
	}
}

// runShowInTestStore returns the styled (TTY) show output for one item
// carrying the given body. It calls showOutput directly with isTTY forced
// true, the same way the other styled-output tests in this file do --
// routing through showCmd.RunE would capture stdout via an os.Pipe, which is
// itself not a terminal and would silently select the non-TTY branch instead.
func runShowInTestStore(t *testing.T, body string) string {
	t.Helper()
	setupShowTest(t)
	b := showTestItem("beans-detail1", "A detail bean", body)
	out, err := showOutput(b, true, false, 110)
	if err != nil {
		t.Fatalf("showOutput() error = %v", err)
	}
	return out
}

// TestShowEmitsNoGlamourPadding guards against glamour's signature defect:
// trailing spaces -- often colour-painted -- running to the right margin of
// every line. withTrueColor forces a real colour profile so stripANSITest
// has ANSI codes to strip; without it, `go test` has no controlling tty, no
// colour is ever emitted, and colour-painted padding would disappear before
// the trailing-space check could see it.
func TestShowEmitsNoGlamourPadding(t *testing.T) {
	withTrueColor(t)
	out := runShowInTestStore(t, "a bean with a body")
	for _, line := range strings.Split(out, "\n") {
		plain := stripANSITest(line)
		if plain != strings.TrimRight(plain, " ") {
			t.Errorf("line carries trailing padding: %q", plain)
		}
	}
}

// TestShowHeaderCarriesTypeIDAndStatus pins the minimum content of the new
// attribute header: the item's type and status must both still be visible
// once colour is stripped.
func TestShowHeaderCarriesTypeIDAndStatus(t *testing.T) {
	withTrueColor(t)
	out := stripANSITest(runShowInTestStore(t, "a bean with a body"))
	for _, want := range []string{"task", "todo"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail header is missing %q:\n%s", want, out)
		}
	}
}

// TestShowHeaderOrdersTypeIDTitleThenStatus pins the vertical reading order
// the task requires: type, id, then title, then status (and priority, when
// present) -- the same order the items table reads across, read down
// instead. It asserts on index positions rather than a fixed line layout, so
// it survives spacing changes but still catches the order flipping.
func TestShowHeaderOrdersTypeIDTitleThenStatus(t *testing.T) {
	withTrueColor(t)
	out := stripANSITest(runShowInTestStore(t, "a bean with a body"))

	typeIdx := strings.Index(out, "task")
	idIdx := strings.Index(out, "beans-detail1")
	titleIdx := strings.Index(out, "A detail bean")
	statusIdx := strings.Index(out, "todo")

	if typeIdx < 0 || idIdx < 0 || titleIdx < 0 || statusIdx < 0 {
		t.Fatalf("expected all of type/id/title/status present, got indices %d/%d/%d/%d in:\n%s",
			typeIdx, idIdx, titleIdx, statusIdx, out)
	}
	if !(typeIdx < idIdx && idIdx < titleIdx && titleIdx < statusIdx) {
		t.Errorf("header order wrong: type=%d id=%d title=%d status=%d, want type < id < title < status\n%s",
			typeIdx, idIdx, titleIdx, statusIdx, out)
	}
}

// TestShowUsesNoBackgroundBadges guards against glamour's other signature
// defect: background-painted badges that don't survive next to a flat
// raster. withTrueColor forces a real colour profile so this failure mode --
// a Background() call slipping back into the attribute header -- would
// actually emit the escape sequence for the assertion to catch.
func TestShowUsesNoBackgroundBadges(t *testing.T) {
	withTrueColor(t)
	out := runShowInTestStore(t, "a bean with a body")
	if strings.Contains(out, "\x1b[48;") {
		t.Error("detail view still paints background badges")
	}
}

// TestShowHeaderCarriesCreatedAndUpdatedTimestamps guards against silently
// dropping user-visible information under this rewrite: the plan's only
// authorised behaviour change is removing glamour, everything else is
// presentation. The old header carried created/updated timestamps as muted
// text; the new attribute header must still carry them somewhere sensible.
func TestShowHeaderCarriesCreatedAndUpdatedTimestamps(t *testing.T) {
	setupShowTest(t)
	created := time.Date(2026, 1, 2, 15, 4, 0, 0, time.UTC)
	updated := time.Date(2026, 3, 4, 9, 30, 0, 0, time.UTC)
	b := showTestItem("beans-detail2", "A timestamped bean", "body text")
	b.CreatedAt = &created
	b.UpdatedAt = &updated

	out, err := showOutput(b, true, false, 110)
	if err != nil {
		t.Fatalf("showOutput() error = %v", err)
	}
	plain := stripANSITest(out)

	for _, want := range []string{
		"created 2026-01-02 15:04 UTC",
		"updated 2026-03-04 09:30 UTC",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("detail header is missing %q:\n%s", want, plain)
		}
	}
}

// TestShowDetailShowsNormalPriority guards against a second unauthorised
// behaviour change: hiding a "normal" priority in the detail view. Hiding it
// is correct in the table (Task 10's prioCell), where it removes a repeated
// word from every row and buys density; a detail view shows exactly one
// item, has no density to buy, and previously showed the badge for it. The
// title and body deliberately avoid the word "normal" so the only possible
// source of the substring is the priority attribute itself.
func TestShowDetailShowsNormalPriority(t *testing.T) {
	setupShowTest(t)
	b := showTestItem("beans-detail3", "Detail priority bean", "body text")
	b.Priority = "normal"

	out, err := showOutput(b, true, false, 110)
	if err != nil {
		t.Fatalf("showOutput() error = %v", err)
	}
	plain := stripANSITest(out)
	if !strings.Contains(plain, "normal") {
		t.Errorf("detail view hides the normal priority:\n%s", plain)
	}
}

// TestShowDetailShowsUnknownStatus guards against the attribute vanishing
// entirely when the config has no matching entry -- the one case where a
// reader most needs to see the raw value, not less of it. The old code fell
// back to the "gray" legacy colour alias in that case; the new code must
// keep rendering the value even though its colour resolution has nothing to
// key off.
func TestShowDetailShowsUnknownStatus(t *testing.T) {
	setupShowTest(t)
	b := showTestItem("beans-detail5", "Bean with an odd status", "body text")
	b.Status = "wibble"

	out, err := showOutput(b, true, false, 110)
	if err != nil {
		t.Fatalf("showOutput() error = %v", err)
	}
	plain := stripANSITest(out)
	if !strings.Contains(plain, "wibble") {
		t.Errorf("detail view hides an unconfigured status:\n%s", plain)
	}
}

// showFullItem returns an item carrying every front matter field the format
// allows -- tags, both blocking directions, order and unknown ("extra")
// keys -- so a detail-view test can assert on the whole front matter rather
// than the subset the header happened to render.
func showFullItem(body string) *item.Item {
	created := time.Date(2026, 9, 5, 10, 15, 5, 0, time.UTC)
	updated := time.Date(2026, 9, 5, 20, 27, 9, 0, time.UTC)
	return &item.Item{
		ID:        "beans-full1",
		Slug:      "a-full-bean",
		Title:     "A full bean",
		Status:    "todo",
		Type:      "task",
		Priority:  "high",
		Tags:      []string{"reviewed", "backend"},
		CreatedAt: &created,
		UpdatedAt: &updated,
		Order:     "a0",
		Parent:    "beans-paren",
		Blocking:  []string{"beans-block1"},
		BlockedBy: []string{"beans-blkby1"},
		Extra: map[string]any{
			"branch":  "feature/beans-full1-a-full-bean",
			"release": "0-9-0",
			"reviews": []any{"beans-rev1", "beans-rev2"},
			"gate":    map[string]any{"suite": "green", "reviewer": "ReviewSix"},
		},
		Body: body,
	}
}

// TestShowHeaderCarriesWholeFrontMatter pins that the styled detail view
// withholds nothing the file holds, and that it carries it *above* the body:
// tags used to be printed after the rendered markdown, which put them below
// a screenful of text on any real item, and blocked_by and unknown keys were
// not rendered at all. Splitting on the horizontal rule is what makes this a
// placement assertion and not just a "appears somewhere" one.
func TestShowHeaderCarriesWholeFrontMatter(t *testing.T) {
	setupShowTest(t)
	b := showFullItem("Body text that mentions nothing else.\n")

	out, err := showOutput(b, true, false, 110)
	if err != nil {
		t.Fatalf("showOutput() error = %v", err)
	}
	plain := stripANSITest(out)

	rule := strings.Index(plain, strings.Repeat("─", 10))
	if rule < 0 {
		t.Fatalf("no horizontal rule in styled output:\n%s", plain)
	}
	header := plain[:rule]

	for _, want := range []string{
		"#reviewed", "#backend",
		"parent: beans-paren",
		"blocking: beans-block1",
		"blocked by: beans-blkby1",
		"branch: feature/beans-full1-a-full-bean",
		"release: 0-9-0",
		"gate: {reviewer: ReviewSix, suite: green}",
		"reviews: [beans-rev1, beans-rev2]",
		"order a0",
	} {
		if !strings.Contains(header, want) {
			t.Errorf("header is missing %q\nheader:\n%s", want, header)
		}
	}
}

// TestShowMetaDropsBodyOnATerminal: --meta answers "what is this item" for a
// reader who does not want to page through the body. The header must survive
// whole; the body and the rule that introduces it must be gone.
func TestShowMetaDropsBodyOnATerminal(t *testing.T) {
	setupShowTest(t)
	b := showFullItem("Distinctive body sentinel.\n")

	out, err := showOutput(b, true, true, 110)
	if err != nil {
		t.Fatalf("showOutput() error = %v", err)
	}
	plain := stripANSITest(out)

	if strings.Contains(plain, "Distinctive body sentinel") {
		t.Errorf("--meta printed the body:\n%s", plain)
	}
	if strings.Contains(plain, strings.Repeat("─", 10)) {
		t.Errorf("--meta printed the body rule:\n%s", plain)
	}
	for _, want := range []string{"beans-full1", "A full bean", "#reviewed", "blocked by: beans-blkby1"} {
		if !strings.Contains(plain, want) {
			t.Errorf("--meta is missing %q\ngot:\n%s", want, plain)
		}
	}
}

// TestShowMetaOffATerminalParsesAsAnItem: off a terminal, show emits source
// markdown a parser can read; --meta must not break that contract by
// emitting a styled or truncated form. The output has to parse back into the
// same front matter with an empty body.
func TestShowMetaOffATerminalParsesAsAnItem(t *testing.T) {
	setupShowTest(t)
	b := showFullItem("Distinctive body sentinel.\n")

	out, err := showOutput(b, false, true, 110)
	if err != nil {
		t.Fatalf("showOutput() error = %v", err)
	}
	if strings.Contains(out, "Distinctive body sentinel") {
		t.Errorf("--meta printed the body:\n%s", out)
	}

	got, err := item.Parse(strings.NewReader(out))
	if err != nil {
		t.Fatalf("item.Parse() error = %v, output:\n%s", err, out)
	}
	if strings.TrimSpace(got.Body) != "" {
		t.Errorf("parsed body = %q, want empty", got.Body)
	}
	if strings.Join(got.Tags, ",") != "reviewed,backend" {
		t.Errorf("parsed tags = %v, want [reviewed backend]", got.Tags)
	}
	if strings.Join(got.BlockedBy, ",") != "beans-blkby1" {
		t.Errorf("parsed blocked_by = %v, want [beans-blkby1]", got.BlockedBy)
	}
	if got.Extra["release"] != "0-9-0" {
		t.Errorf("parsed extra[release] = %v, want %q", got.Extra["release"], "0-9-0")
	}

	// The source item must be untouched: --meta reads, it does not edit.
	if b.Body != "Distinctive body sentinel.\n" {
		t.Errorf("--meta mutated the bean's body to %q", b.Body)
	}
}

// TestShowMetaOffATerminalEmitsNoEmptyDocument: each --meta block ends with
// the front matter's own closing "---", so the raw separator on top of it
// produced an empty document between every pair of items -- a stream that
// still "looks like" markdown but hands a parser an item with no fields.
func TestShowMetaOffATerminalEmitsNoEmptyDocument(t *testing.T) {
	setupShowTest(t)
	b1 := showFullItem("First body.\n")
	b2 := showTestItem("beans-second", "A second bean", "Second body.\n")

	got, err := showOutputAll([]*item.Item{b1, b2}, false, true, 110)
	if err != nil {
		t.Fatalf("showOutputAll() error = %v", err)
	}

	// Two items are exactly four delimiter lines: an opening and a closing
	// "---" each. A fifth is the stray separator, and the empty document it
	// opens is what a parser then reads as an item with no fields.
	var delimiters int
	for _, line := range strings.Split(got, "\n") {
		if line == "---" {
			delimiters++
		}
	}
	if delimiters != 4 {
		t.Errorf("got %d \"---\" lines, want 4 (two per bean):\n%s", delimiters, got)
	}
	if !strings.Contains(got, "beans-full1") || !strings.Contains(got, "beans-second") {
		t.Errorf("both beans must appear:\n%s", got)
	}
}

// TestExtraValueStaysOnOneLine pins the header's one-line-per-key contract
// against the YAML emitter's line-breaking. yaml.v3 does not break flow
// output today -- yaml_emitter_initialize sets best_width to -1 and
// emitterc.go raises that to MaxInt32, while the 80-column default only
// applies when the width is set explicitly through the private
// yaml_emitter_set_width -- so no whitespace collapsing is needed. That is a
// property of the dependency, not of this code, which is exactly why it
// belongs in a test: a yaml upgrade that starts wrapping at 80 columns would
// silently tear one header line into several without one.
//
// The space-bearing values matter: a plain scalar containing spaces is the
// only place the emitter has a break candidate at all (emitterc.go:1620).
func TestExtraValueStaysOnOneLine(t *testing.T) {
	wide := make([]any, 40)
	for i := range wide {
		wide[i] = fmt.Sprintf("beans-r%03d", i)
	}

	cases := map[string]any{
		"wide sequence": wide,
		"wide mapping": map[string]any{
			"reviewer": "ReviewSix", "suite": "green", "commit": "8c08a5a",
			"branch": "feature/beans-full1-a-rather-long-branch-name",
			"note":   "a fairly long sentence value that contains spaces",
		},
		"nested":            map[string]any{"gate": map[string]any{"suite": "green"}, "reviews": wide},
		"space-bearing seq": []any{"a fairly long sentence value that contains spaces and more", "and a second one to push well past eighty columns"},
	}

	for name, v := range cases {
		got := formatExtraValue(v)
		if strings.Contains(got, "\n") {
			t.Errorf("formatExtraValue(%s) spans more than one line:\n%s", name, got)
		}
	}

	// Flow style is what keeps it readable on that one line: block markers
	// folded onto a single line ("- a - b") are not the value's notation.
	if got := formatExtraValue([]any{"a", "b"}); got != "[a, b]" {
		t.Errorf("formatExtraValue([a b]) = %q, want %q", got, "[a, b]")
	}
	if got := formatExtraValue(map[string]any{"k": "v"}); got != "{k: v}" {
		t.Errorf("formatExtraValue({k: v}) = %q, want %q", got, "{k: v}")
	}
}

// tableLabels returns the label of every row in a rendered table, in order.
// Every table assertion goes through this rather than matching raw substrings:
// the point of the view is that a label sits in its own left column, and a
// substring match would pass just as happily on the flowing header.
func tableLabels(out string) []string {
	var labels []string
	for _, line := range strings.Split(out, "\n") {
		if !tableIsFieldRow(line) {
			continue
		}
		cells := strings.Split(stripANSI(line), "│")
		if label := strings.TrimSpace(cells[1]); label != "" {
			labels = append(labels, label)
		}
	}
	return labels
}

// tableIsFieldRow tells a two-column field row from a full-width band. Both
// carry three bars; only a field row has its second bar at an interior
// column rather than at the right edge, so the width of the first cell is
// what separates them.
func tableIsFieldRow(line string) bool {
	plain := stripANSI(line)
	cells := strings.Split(plain, "│")
	if len(cells) != 4 {
		return false
	}
	return ui.DisplayWidth(cells[1]) < ui.DisplayWidth(plain)-4
}

// TestTableCarriesEveryFrontMatterField is the table view's half of
// TestShowHeaderCarriesWholeFrontMatter: the grid is a different arrangement
// of the whole front matter, not a smaller selection of it. Without this the
// renderer could quietly drop blocked_by or an extra key and only the flowing
// header would notice.
func TestTableCarriesEveryFrontMatterField(t *testing.T) {
	setupShowTest(t)
	b := showFullItem("Body text.\n")

	out := renderItemTable(b, cfg, 110, itemTableLabelWidth([]*item.Item{b}, cfg))

	for _, want := range []string{
		"title:", "id:", "type:", "status:", "priority:", "tags:",
		"parent:", "blocked by:", "blocking:",
		"branch:", "gate:", "release:", "reviews:",
		"created:", "updated:", "order:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("table is missing label %q\n%s", want, out)
		}
	}
	for _, want := range []string{
		"A full bean", "beans-full1", "high", "#reviewed", "#backend",
		"beans-paren", "beans-block1", "beans-blkby1",
		"feature/beans-full1-a-full-bean", "0-9-0", "a0",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("table is missing value %q\n%s", want, out)
		}
	}
}

// TestTableBlockOrderFollowsTheSketch pins the PO's arrangement: an
// identity band, then one ruled field per front matter entry in a fixed
// order, then the stamps band. A grid whose rows move between items buys
// nothing over the flowing header -- reading a column only works when the
// same label sits on the same row every time.
//
// It asserts on the block model rather than the rendered text, because the
// order is a property of the layout and not of the box drawing.
func TestTableBlockOrderFollowsTheSketch(t *testing.T) {
	setupShowTest(t)
	b := showFullItem("Body.\n")

	blocks := itemTableBlocks(b, cfg)
	if len(blocks) < 3 {
		t.Fatalf("got %d blocks, want an identity band, fields and a stamps band", len(blocks))
	}
	if !blocks[0].band {
		t.Errorf("first block is not the identity band: %+v", blocks[0])
	}
	if !blocks[len(blocks)-1].band {
		t.Errorf("last block is not the stamps band: %+v", blocks[len(blocks)-1])
	}

	var got []string
	for _, bl := range blocks[1 : len(blocks)-1] {
		if bl.band {
			t.Errorf("unexpected band between the two: %+v", bl)
		}
		got = append(got, bl.label)
	}
	want := []string{
		"title:", "tags:", "parent:", "blocked by:", "blocking:",
		"branch:", "gate:", "release:", "reviews:",
	}
	if len(got) != len(want) {
		t.Fatalf("field labels = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("field %d = %q, want %q (all: %v)", i, got[i], want[i], got)
		}
	}
}

// TestTablePairsShareOneRow guards the sketch's paired rows: type, status and
// priority belong on one line, and so do created, updated and order. Three
// rows each would push the interesting fields off the first screen, which is
// the density this view exists to buy.
func TestTablePairsShareOneRow(t *testing.T) {
	setupShowTest(t)
	b := showFullItem("Body.\n")

	out := renderItemTable(b, cfg, 110, itemTableLabelWidth([]*item.Item{b}, cfg))
	for _, want := range []struct{ label, mate string }{
		{"type:", "status:"},
		{"type:", "priority:"},
		{"created:", "updated:"},
		{"created:", "order:"},
	} {
		var found bool
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, want.label) && strings.Contains(line, want.mate) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s and %s are not on one row\n%s", want.label, want.mate, out)
		}
	}
}

// TestTableRasterIsIdenticalAcrossItems is the whole promise of the view: two
// items with wildly different content produce the same column geometry, so a
// reader scans down one column instead of reading every line. A renderer that
// sized its columns from the values it happens to hold would fail here while
// looking perfectly fine on a single item.
//
// The promise holds for items whose labels stay inside the vocabulary, which
// is what the width is floored at. A longer custom front matter key widens
// the column on purpose -- see TestTableLongCustomKeyWidensTheColumn -- so
// the claim here is stability across ordinary items, not a fixed width.
func TestTableRasterIsIdenticalAcrossItems(t *testing.T) {
	setupShowTest(t)

	narrow := &item.Item{ID: "beans-n1", Title: "x", Status: "todo", Type: "task"}
	wide := showFullItem("Body.\n")

	// The measurement is the *position of the column separator*, not the
	// total row width: a renderer that sizes the label column from its own
	// data still produces rows of the requested total width, because the
	// value column absorbs the difference. Only the boundary moves, and
	// only the boundary is what a reader's eye follows down the page.
	geometry := func(out string) []int {
		var boundaries []int
		for _, line := range strings.Split(out, "\n") {
			if !tableIsFieldRow(line) {
				continue
			}
			inner := strings.TrimPrefix(stripANSI(line), "│")
			boundaries = append(boundaries, ui.DisplayWidth(inner[:strings.Index(inner, "│")]))
		}
		return boundaries
	}

	gotNarrow, gotWide := geometry(renderItemTable(narrow, cfg, 110, itemTableLabelWidth([]*item.Item{narrow}, cfg))), geometry(renderItemTable(wide, cfg, 110, itemTableLabelWidth([]*item.Item{wide}, cfg)))
	if len(gotNarrow) == 0 || len(gotWide) == 0 {
		t.Fatalf("no bordered rows rendered")
	}
	for _, b := range append(gotNarrow, gotWide...) {
		if b != gotWide[0] {
			t.Errorf("label column boundary moves: narrow %v vs wide %v", gotNarrow, gotWide)
			break
		}
	}
}

// TestTableWrapsLongValuesInsideTheColumn is the case that motivated the
// view: a customer_value of a few sentences must fold inside its cell. A
// renderer that let it run would push the right border off the screen and
// destroy the raster the other tests pin.
func TestTableWrapsLongValuesInsideTheColumn(t *testing.T) {
	setupShowTest(t)
	b := showFullItem("Body.\n")
	b.Extra = map[string]any{"goal": strings.Repeat("Lorem ipsum dolor sit amet. ", 12)}

	out := renderItemTable(b, cfg, 72, itemTableLabelWidth([]*item.Item{b}, cfg))

	var rows int
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "│") {
			continue
		}
		rows++
		if w := ui.DisplayWidth(line); w > 72 {
			t.Errorf("row is %d cells wide, want <= 72: %q", w, line)
		}
	}
	if rows < 3 {
		t.Fatalf("expected the long value to occupy several rows, got %d\n%s", rows, out)
	}
	// The continuation rows carry no label -- the value keeps flowing in
	// its own column instead of restating "goal:" on every line.
	labels := tableLabels(out)
	var goals int
	for _, l := range labels {
		if l == "goal:" {
			goals++
		}
	}
	if goals != 1 {
		t.Errorf("label goal: appears %d times, want 1\n%s", goals, out)
	}
}

// TestTableMaxWidthCapsTheGrid is --max-width's own guard. resolveWidth is
// already tested for list; what is untested is that show's table actually
// honours the number instead of rendering at the default and letting the
// terminal wrap.
func TestTableMaxWidthCapsTheGrid(t *testing.T) {
	setupShowTest(t)
	b := showFullItem("Body.\n")

	for _, width := range []int{40, 60, 100} {
		out := renderItemTable(b, cfg, width, itemTableLabelWidth([]*item.Item{b}, cfg))
		for _, line := range strings.Split(out, "\n") {
			if !strings.Contains(line, "│") {
				continue
			}
			if w := ui.DisplayWidth(line); w != width {
				t.Errorf("at max-width %d a row is %d cells: %q", width, w, line)
				break
			}
		}
	}
}

// TestTableForcesTheGridOffATerminal pins that --table is a forcing flag in
// both directions, the way --raw forces raw markdown on a terminal. Piping
// the grid into less or a file is exactly what a reader comparing items does.
//
// It runs the command rather than the renderer, because the forcing decision
// lives in RunE: showOutputTable itself cannot tell a pipe from a terminal,
// and a unit-level call would assert nothing about the dispatch. Test stdout
// is a pipe, so term.IsTerminal is genuinely false here.
func TestTableForcesTheGridOffATerminal(t *testing.T) {
	setupShowTest(t)

	b := &item.Item{
		ID:     "beans-grid1",
		Slug:   item.Slugify("A gridded bean"),
		Title:  "A gridded bean",
		Status: "todo",
		Type:   "task",
		Body:   "Body text.\n",
	}
	if err := core.Create(b); err != nil {
		t.Fatalf("core.Create() error = %v", err)
	}

	oldTable, oldMeta := showTable, showMeta
	showTable, showMeta = true, true
	t.Cleanup(func() { showTable, showMeta = oldTable, oldMeta })

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	runErr := showCmd.RunE(showCmd, []string{b.ID})
	os.Stdout = oldStdout
	w.Close()
	captured, _ := io.ReadAll(r)
	if runErr != nil {
		t.Fatalf("showCmd.RunE() error = %v", runErr)
	}

	out := string(captured)
	if !strings.Contains(out, "│") {
		t.Errorf("--table off a terminal did not render the grid:\n%s", out)
	}
	if strings.Contains(out, "Body text.") {
		t.Errorf("--table --meta kept the body:\n%s", out)
	}
}

// TestTableWithoutMetaKeepsTheBody guards the other combination: --table on
// its own replaces the header with the grid and still renders the body, so
// the flag is an arrangement of the front matter, not a body switch.
func TestTableWithoutMetaKeepsTheBody(t *testing.T) {
	setupShowTest(t)
	b := showFullItem("Body text that is unmistakable.\n")

	out, err := showOutputTable(b, false, 110, itemTableLabelWidth([]*item.Item{b}, cfg))
	if err != nil {
		t.Fatalf("showOutputTable() error = %v", err)
	}
	if !strings.Contains(out, "│") {
		t.Errorf("no grid rendered:\n%s", out)
	}
	if !strings.Contains(out, "unmistakable") {
		t.Errorf("body missing:\n%s", out)
	}
}

// TestTableRelationsNameTypeAndTitle covers the third leaf: a parent shows as
// type and title, and an unresolvable id degrades to the bare id rather than
// erroring or blanking the cell.
func TestTableRelationsNameTypeAndTitle(t *testing.T) {
	setupShowTest(t)

	parent := &item.Item{
		ID:     "beans-pare1",
		Slug:   item.Slugify("The parent epic"),
		Title:  "The parent epic",
		Status: "todo",
		Type:   "epic",
	}
	if err := core.Create(parent); err != nil {
		t.Fatalf("core.Create() error = %v", err)
	}

	b := showFullItem("Body.\n")
	b.Parent = parent.ID
	b.BlockedBy = []string{"beans-gone1"}

	out := renderItemTable(b, cfg, 110, itemTableLabelWidth([]*item.Item{b}, cfg))
	if !strings.Contains(out, "The parent epic") {
		t.Errorf("parent row does not name the title:\n%s", out)
	}
	if !strings.Contains(out, "epic") {
		t.Errorf("parent row does not name the type:\n%s", out)
	}
	if !strings.Contains(out, "beans-gone1") {
		t.Errorf("unresolvable id was dropped instead of shown bare:\n%s", out)
	}

	// The id sits in the label column on the row beneath its label, not
	// inside the value cell: the label column is where a reader looks for
	// what a row is, and keeping the id out of the value column leaves
	// type and title the full width. Right-aligning it in the value cell,
	// which this replaced, made every relation cell a dozen cells narrower
	// than every other cell in the grid.
	labelCell := func(line string) string {
		cells := strings.Split(stripANSI(line), "│")
		if len(cells) != 4 {
			return ""
		}
		return strings.TrimSpace(cells[1])
	}

	lines := strings.Split(out, "\n")
	var checked bool
	for i, line := range lines {
		if labelCell(line) != "parent:" {
			continue
		}
		checked = true
		if i+1 >= len(lines) {
			t.Fatalf("parent row has no continuation row to carry the id:\n%s", out)
		}
		if got := labelCell(lines[i+1]); got != parent.ID {
			t.Errorf("label column beneath parent: is %q, want the id %q:\n%s", got, parent.ID, out)
		}
		if strings.Contains(stripANSI(line), parent.ID) {
			t.Errorf("id is still inside the value cell: %q", stripANSI(line))
		}
	}
	if !checked {
		t.Fatalf("no parent row in output:\n%s", out)
	}
}

// TestTableWrapsStyledValuesAtVisibleWidth is the defect the first TTY run
// showed: a styled value (a relation carries the related type's colour and a
// muted id) wrapped several cells early, because ui.WrapText measures the
// string it is given and a styled string carries ANSI bytes that occupy no
// cells. The grid stayed aligned -- padding is computed on visible width --
// so only a side-by-side comparison of a styled and an unstyled row of the
// same text reveals it.
func TestTableWrapsStyledValuesAtVisibleWidth(t *testing.T) {
	setupShowTest(t)

	// The escape sequences are written out rather than taken from
	// ui.Muted.Render: the test environment sets NO_COLOR, which makes
	// lipgloss render plain text and would leave this test asserting
	// nothing. What the renderer has to survive is ANSI in its input,
	// whoever produced it.
	dim := func(s string) string { return "\x1b[38;5;245m" + s + "\x1b[0m" }
	text := "alpha bravo charlie delta echo foxtrot golf hotel india juliett kilo lima mike"
	styled := dim("alpha") + " bravo charlie delta echo foxtrot golf " +
		dim("hotel") + " india juliett kilo lima " + dim("mike")

	plainLines := wrapVisible(text, 40)
	styledLines := wrapVisible(styled, 40)

	if len(plainLines) != len(styledLines) {
		t.Fatalf("styled value wrapped into %d lines, plain into %d:\n%q\n%q",
			len(styledLines), len(plainLines), styledLines, plainLines)
	}
	for i := range plainLines {
		if got, want := stripANSI(styledLines[i]), plainLines[i]; got != want {
			t.Errorf("line %d: styled wrapped to %q, plain to %q", i, got, want)
		}
	}
	// Every sequence must survive the wrap paired: a line that opens a
	// colour and never closes it bleeds into the border and beyond.
	for i, line := range styledLines {
		if opens, closes := strings.Count(line, "\x1b[38"), strings.Count(line, "\x1b[0m"); opens != closes {
			t.Errorf("line %d has %d colour starts and %d resets: %q", i, opens, closes, line)
		}
	}
}

// TestShowMaxWidthAppliesWithoutTable pins that --max-width is a property of
// the command, not of --table. styledItemOutput hard-wired resolveWidth(0,
// false, cfg), so `lif show --max-width 60` silently rendered at the
// default while `lif show --table --max-width 60` obeyed -- a flag that
// works in one combination and is ignored in another is worse than no flag.
//
// The horizontal rule between header and body is the measurement, because
// its width *is* the resolved width.
func TestShowMaxWidthAppliesWithoutTable(t *testing.T) {
	setupShowTest(t)
	b := showFullItem("Body text.\n")

	for _, width := range []int{60, 80} {
		out, err := showOutput(b, true, false, width)
		if err != nil {
			t.Fatalf("showOutput() error = %v", err)
		}
		var found bool
		for _, line := range strings.Split(out, "\n") {
			plain := stripANSI(line)
			if strings.Count(plain, "─") < 3 {
				continue
			}
			found = true
			if got := ui.DisplayWidth(plain); got != width {
				t.Errorf("at --max-width %d the rule is %d cells wide", width, got)
			}
		}
		if !found {
			t.Fatalf("no horizontal rule in output:\n%s", out)
		}
	}
}

// TestShowHeaderWrapsAtTheResolvedWidth pins that --max-width governs the
// header too. renderItemHeader emitted every label/value pair as one
// unbroken line, so an item with a few sentences in an extra key ran to the
// terminal's own width while the rule below it obeyed the cap -- the flag
// looked honoured because the only measurable element, the rule, was.
//
// The continuation lines are checked for their hanging indent as well: a
// value that resumes in column 0 reads as a new field rather than as the
// remainder of the one above it.
func TestShowHeaderWrapsAtTheResolvedWidth(t *testing.T) {
	setupShowTest(t)
	b := showFullItem("Body.\n")
	b.Extra = map[string]any{
		"customer_value": "Entries that are still queued, that failed, or that look " +
			"like a double capture are recognisable without relying on colour, and " +
			"each carries the gesture that resolves it.",
	}
	b.Title = "Offline states and duplicate detection: pending, failed and suspicious are visible"

	for _, width := range []int{80, 100} {
		out := renderItemHeader(b, cfg, width)
		for _, line := range strings.Split(out, "\n") {
			if got := ui.DisplayWidth(stripANSI(line)); got > width {
				t.Errorf("at width %d a header line is %d cells: %q", width, got, stripANSI(line))
			}
		}

		var continuations int
		for _, line := range strings.Split(out, "\n") {
			plain := stripANSI(line)
			if !strings.HasPrefix(plain, " ") || strings.TrimSpace(plain) == "" {
				continue
			}
			continuations++
			if !strings.HasPrefix(plain, strings.Repeat(" ", len("customer_value: "))) {
				t.Errorf("continuation line is not aligned under its value: %q", plain)
			}
		}
		if continuations == 0 {
			t.Errorf("at width %d nothing wrapped, so the test proves nothing:\n%s", width, out)
		}
	}
}

// TestBandIDWidthFollowsTheConfiguredPrefix pins that the id cell in the top
// band is budgeted from the store's own id shape -- prefix plus suffix
// length -- rather than from a constant.
//
// It asserts the exact column "type:" starts at, not merely that a longer
// prefix moves it: a constant that only ever pads moves the column too, so a
// comparative assertion passes under the defect. With "SPF-" ids the derived
// budget is eight cells and a constant of twelve wasted four, which is
// exactly the kind of drift a band's fixed columns exist to avoid.
func TestBandIDWidthFollowsTheConfiguredPrefix(t *testing.T) {
	setupShowTest(t)

	for _, tc := range []struct {
		prefix   string
		idLength int
	}{
		{"SPF-", 4},
		{"beans-", 4},
		{"a-very-long-prefix-", 6},
	} {
		old := cfg.Items
		cfg.Items.Prefix, cfg.Items.IDLength = tc.prefix, tc.idLength

		b := showFullItem("Body.\n")
		b.ID = tc.prefix + strings.Repeat("z", tc.idLength)
		band := stripANSI(strings.Split(renderItemTable(b, cfg, 110, itemTableLabelWidth([]*item.Item{b}, cfg)), "\n")[1])
		cfg.Items = old

		// "│ " + "id: " + <id cell> + "    " + "type:"
		want := len("│ id: ") + len(tc.prefix) + tc.idLength + 4
		if got := strings.Index(band, "type:"); got != want {
			t.Errorf("prefix %q: type: starts at column %d, want %d\n%s",
				tc.prefix, got, want, band)
		}
	}

	// Two items of one store keep the column: an id shorter than the
	// configured shape is padded up to it, which is what a width derived
	// from the data at hand would not do.
	old := cfg.Items
	cfg.Items.Prefix, cfg.Items.IDLength = "beans-", 4
	t.Cleanup(func() { cfg.Items = old })

	b1, b2 := showFullItem("x\n"), showFullItem("y\n")
	b1.ID, b2.ID = "beans-aaaa", "beans-b"
	col := func(b *item.Item) int {
		return strings.Index(stripANSI(strings.Split(renderItemTable(b, cfg, 110, itemTableLabelWidth([]*item.Item{b}, cfg)), "\n")[1]), "type:")
	}
	if col(b1) != col(b2) {
		t.Errorf("type: moves between beans of one store: %d vs %d", col(b1), col(b2))
	}
}

// TestHeaderHangIndentSurvivesColour covers the path the suite otherwise
// cannot reach: the environment sets NO_COLOR, so ui.Muted.Render returns
// plain text in every other test and a hang indent derived from the styled
// string would look correct here while being wrong on a real terminal.
//
// wrapHeaderLines is called directly with escape sequences written out, so
// the assertion holds regardless of the colour profile.
func TestHeaderHangIndentSurvivesColour(t *testing.T) {
	dim := func(s string) string { return "\x1b[38;5;245m" + s + "\x1b[0m" }
	line := dim("customer_value:") + " " +
		"Entries that are still queued, that failed, or that look like a double capture " +
		"are recognisable without relying on colour."

	out := wrapHeaderLines(line+"\n", 60)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("nothing wrapped, so the test proves nothing:\n%s", out)
	}
	for i, l := range lines {
		if got := ui.DisplayWidth(stripANSI(l)); got > 60 {
			t.Errorf("line %d is %d cells wide: %q", i, got, stripANSI(l))
		}
	}
	for i, l := range lines[1:] {
		if want := strings.Repeat(" ", len("customer_value: ")); !strings.HasPrefix(stripANSI(l), want) {
			t.Errorf("continuation %d is not hung under the value: %q", i, stripANSI(l))
		}
	}
}

// TestTableRulesConnectTheColumn pins that every horizontal rule carries the
// connector matching what the column line does at that height: it begins,
// continues, ends, or is absent. Drawing every rule straight left a visible
// gap wherever the column line arrived at a rule with nothing to meet it,
// and the misalignment is invisible in a width check -- every row was the
// right width, the corners were simply not joined.
func TestTableRulesConnectTheColumn(t *testing.T) {
	setupShowTest(t)
	b := showFullItem("Body.\n")

	lines := strings.Split(strings.TrimRight(renderItemTable(b, cfg, 110, itemTableLabelWidth([]*item.Item{b}, cfg)), "\n"), "\n")
	if len(lines) < 5 {
		t.Fatalf("grid too small to have interior rules:\n%s", strings.Join(lines, "\n"))
	}

	// The column sits wherever a field row puts its second bar.
	column := -1
	for _, line := range lines {
		if !tableIsFieldRow(line) {
			continue
		}
		runes := []rune(stripANSI(line))
		for i := 1; i < len(runes)-1; i++ {
			if runes[i] == '│' {
				column = i
				break
			}
		}
		break
	}
	if column < 1 {
		t.Fatalf("no column found in any field row")
	}

	isRule := func(line string) bool { return strings.Contains(stripANSI(line), "───") }
	for i, line := range lines {
		if !isRule(line) {
			continue
		}
		runes := []rune(stripANSI(line))
		got := runes[column]

		above := i > 0 && tableIsFieldRow(lines[i-1])
		below := i+1 < len(lines) && tableIsFieldRow(lines[i+1])
		want := '─'
		switch {
		case above && below:
			want = '┼'
		case below:
			want = '┬'
		case above:
			want = '┴'
		}
		if got != want {
			t.Errorf("rule on line %d has %q at the column, want %q\n%s",
				i+1, string(got), string(want), strings.Join(lines, "\n"))
		}
	}
}

// TestTableTitleIsBold pins the weight on the title, the one field a reader
// looks for first. It asserts on the block model because the environment
// sets NO_COLOR, which makes lipgloss emit plain text -- a rendered-string
// assertion would pass on an unstyled title.
func TestTableTitleIsBold(t *testing.T) {
	setupShowTest(t)
	b := showFullItem("Body.\n")
	b.Title = "A full bean"

	var title *tableBlock
	for i, bl := range itemTableBlocks(b, cfg) {
		if bl.label == "title:" {
			title = &itemTableBlocks(b, cfg)[i]
		}
	}
	if title == nil {
		t.Fatalf("no title block")
	}
	if !title.bold {
		t.Errorf("title block is not marked bold: %+v", *title)
	}
	// The value stays plain text: the weight is applied per wrapped line at
	// render time, because a style spanning a wrap leaks into the border.
	if title.value != b.Title {
		t.Errorf("title value = %q, want the plain title %q", title.value, b.Title)
	}
}

// TestTableStaysAlignedWithColour is the defect a NO_COLOR test run cannot
// see: the id in the label column and the type tint in the band are styled,
// and ui.PadRight measures the string it is handed, so under a real colour
// profile every styled cell was padded by however many bytes its escape
// sequences occupied -- which is to say not at all. Every row still had the
// nominally correct width in a byte count while the borders visibly stepped
// left, exactly what the terminal showed.
//
// Forcing the profile is what gives the test teeth; the suite otherwise runs
// with NO_COLOR set and lipgloss emits plain text.
func TestTableStaysAlignedWithColour(t *testing.T) {
	setupShowTest(t)
	withTrueColorCommands(t)

	parent := &item.Item{
		ID:     "beans-pare1",
		Slug:   item.Slugify("The parent epic"),
		Title:  "The parent epic",
		Status: "todo",
		Type:   "epic",
	}
	if err := core.Create(parent); err != nil {
		t.Fatalf("core.Create() error = %v", err)
	}

	b := showFullItem("Body.\n")
	b.Parent = parent.ID
	// A label longer than the id is what exposes the defect: with
	// "blocked by:" as the widest label and an eleven-character id there
	// is nothing to pad, and the missing padding is invisible. Real stores
	// carry keys like customer_value.
	b.Extra["customer_value"] = "Recognisable without relying on colour."

	out := renderItemTable(b, cfg, 100, itemTableLabelWidth([]*item.Item{b}, cfg))
	if !strings.Contains(out, "\x1b[") {
		t.Fatalf("no colour in output, so the test proves nothing")
	}

	var column = -1
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		plain := stripANSI(line)
		if got := ui.DisplayWidth(plain); got != 100 {
			t.Errorf("row is %d cells wide, want 100: %q", got, plain)
		}
		if !tableIsFieldRow(line) {
			continue
		}
		runes := []rune(plain)
		for i := 1; i < len(runes)-1; i++ {
			if runes[i] == '│' {
				if column == -1 {
					column = i
				} else if i != column {
					t.Errorf("column moved from %d to %d: %q", column, i, plain)
				}
				break
			}
		}
	}
}

// withTrueColorCommands forces lipgloss to TrueColor for one test, mirroring
// internal/ui's own helper: without it `go test` has no controlling tty,
// lipgloss emits no escapes, and any alignment assertion about styled cells
// is vacuously true.
func withTrueColorCommands(t *testing.T) {
	t.Helper()
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(old) })
}

// TestTableBoldTitleClosesOnEveryLine pins that a styled value that wraps
// carries its start and its reset on each of its lines. Styling the whole
// title and wrapping afterwards put "\x1b[1m" on the first line and its
// reset on the last, so every line between them, and the border to their
// right, inherited the weight -- visible in a terminal as a bold box edge.
func TestTableBoldTitleClosesOnEveryLine(t *testing.T) {
	setupShowTest(t)
	withTrueColorCommands(t)

	b := showFullItem("Body.\n")
	b.Title = "Offline states and duplicate detection: pending, failed and suspicious are visible"

	// The width has to force the title to wrap: on one line lipgloss's own
	// reset lands before the border and the defect cannot appear.
	rendered := renderItemTable(b, cfg, 80, itemTableLabelWidth([]*item.Item{b}, cfg))
	if !strings.Contains(rendered, "visible") || len(strings.Split(rendered, "\n")) < 6 {
		t.Fatalf("title did not wrap, so the test proves nothing:\n%s", stripANSI(rendered))
	}

	// Without this the test is silently vacuous: it only inspects lines
	// that carry the bold sequence, so a title rendered plain -- because
	// the flag was dropped, or because the profile degraded -- would find
	// nothing to inspect and pass. The sequence has to be there first.
	if !strings.Contains(rendered, "\x1b[1m") {
		t.Fatalf("no bold sequence around the title:\n%s", rendered)
	}

	for _, line := range strings.Split(rendered, "\n") {
		if !strings.Contains(line, "\x1b[1m") {
			continue
		}
		// Counting starts against resets is not enough: the reset may
		// well arrive, but *after* the closing border, which is exactly
		// what a bold box edge is. The assertion is positional -- no
		// border character may sit inside an open bold run.
		bold := false
		for i := 0; i < len(line); {
			switch {
			case strings.HasPrefix(line[i:], "\x1b[1m"):
				bold, i = true, i+len("\x1b[1m")
			case strings.HasPrefix(line[i:], "\x1b[0m"):
				bold, i = false, i+len("\x1b[0m")
			case strings.HasPrefix(line[i:], "│"):
				if bold {
					t.Errorf("border sits inside an open bold run: %q", line)
				}
				i += len("│")
			default:
				i++
			}
		}
	}
}

// TestTableSharesTheLabelColumnAcrossItems is the promise the view exists
// for, applied to the case that actually breaks it: several ids in one call.
// The label column was sized per item, so an item carrying customer_value got
// a 16-cell column and its neighbour an 11-cell one -- rendered one after
// the other, the grids stepped and a reader could not follow a column down
// the page, which is the whole reason for a grid over the flowing header.
func TestTableSharesTheLabelColumnAcrossItems(t *testing.T) {
	setupShowTest(t)

	rich := showFullItem("Body.\n")
	rich.ID = "beans-rich1"
	rich.Extra = map[string]any{"customer_value": "A sentence."}

	plain := &item.Item{ID: "beans-plai1", Title: "Plain", Status: "todo", Type: "task"}

	labelWidth := itemTableLabelWidth([]*item.Item{rich, plain}, cfg)
	boundary := func(b *item.Item) int {
		for _, line := range strings.Split(renderItemTable(b, cfg, 100, labelWidth), "\n") {
			if !tableIsFieldRow(line) {
				continue
			}
			runes := []rune(stripANSI(line))
			for i := 1; i < len(runes)-1; i++ {
				if runes[i] == '│' {
					return i
				}
			}
		}
		return -1
	}

	got, want := boundary(plain), boundary(rich)
	if want < 1 {
		t.Fatalf("no field row in the rich bean")
	}
	if got != want {
		t.Errorf("label column is %d cells for the plain bean and %d for the rich one; "+
			"beans shown in one call do not line up", got, want)
	}
	if want < len("customer_value:") {
		t.Errorf("shared column %d is narrower than the widest label", want)
	}
}

// TestTableCommandSharesTheColumnAcrossIDs covers the dispatch rather than
// the renderer: `lif show <a> <b> --table` is the call that exposed the
// stepping grids, and a renderer that accepts a shared width proves nothing
// if RunE does not compute one. Test stdout is a pipe, so this also
// exercises --table's forcing path.
func TestTableCommandSharesTheColumnAcrossIDs(t *testing.T) {
	setupShowTest(t)

	rich := &item.Item{
		ID: "beans-rich2", Slug: "rich", Title: "Rich", Status: "todo", Type: "task",
		Extra: map[string]any{"customer_value": "A sentence."},
	}
	plain := &item.Item{ID: "beans-plai2", Slug: "plain", Title: "Plain", Status: "todo", Type: "task"}
	for _, b := range []*item.Item{rich, plain} {
		if err := core.Create(b); err != nil {
			t.Fatalf("core.Create() error = %v", err)
		}
	}

	oldTable, oldMeta := showTable, showMeta
	showTable, showMeta = true, true
	t.Cleanup(func() { showTable, showMeta = oldTable, oldMeta })

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	runErr := showCmd.RunE(showCmd, []string{rich.ID, plain.ID})
	os.Stdout = oldStdout
	w.Close()
	captured, _ := io.ReadAll(r)
	if runErr != nil {
		t.Fatalf("showCmd.RunE() error = %v", runErr)
	}

	var boundaries []int
	for _, line := range strings.Split(string(captured), "\n") {
		if !tableIsFieldRow(line) {
			continue
		}
		runes := []rune(stripANSI(line))
		for i := 1; i < len(runes)-1; i++ {
			if runes[i] == '│' {
				boundaries = append(boundaries, i)
				break
			}
		}
	}
	if len(boundaries) < 2 {
		t.Fatalf("expected field rows from both beans:\n%s", captured)
	}
	for _, got := range boundaries {
		if got != boundaries[0] {
			t.Errorf("column boundaries differ across the two grids: %v\n%s", boundaries, captured)
			break
		}
	}
}

// showParentTestFamily creates a parent with three children and one
// unrelated item, and returns the parent.
func showParentTestFamily(t *testing.T) *item.Item {
	t.Helper()
	parent := &item.Item{ID: "beans-fam01", Slug: "family", Title: "Family", Status: "todo", Type: "epic"}
	if err := core.Create(parent); err != nil {
		t.Fatalf("core.Create() error = %v", err)
	}
	for _, id := range []string{"beans-kid01", "beans-kid02", "beans-kid03"} {
		child := &item.Item{ID: id, Slug: id, Title: id, Status: "todo", Type: "task", Parent: parent.ID}
		if err := core.Create(child); err != nil {
			t.Fatalf("core.Create() error = %v", err)
		}
	}
	stranger := &item.Item{ID: "beans-out01", Slug: "outsider", Title: "Outsider", Status: "todo", Type: "task"}
	if err := core.Create(stranger); err != nil {
		t.Fatalf("core.Create() error = %v", err)
	}
	return parent
}

// showIDsInOutput returns the item IDs the rendered output shows, in order,
// read off the id: cell so it counts items shown rather than mentions.
func showIDsInOutput(out string) []string {
	var ids []string
	for _, line := range strings.Split(out, "\n") {
		plain := stripANSI(line)
		if i := strings.Index(plain, "id: "); i >= 0 && strings.HasPrefix(strings.TrimSpace(plain), "│ id:") {
			rest := strings.Fields(plain[i+len("id: "):])
			if len(rest) > 0 {
				ids = append(ids, rest[0])
			}
		}
	}
	return ids
}

// TestShowParentResolvesChildren is the feature: the shell substitution over
// `list --parent --json` moves into the tool. --parent alone selects the
// children of that item, mirroring `list --parent`, and takes no ids.
func TestShowParentResolvesChildren(t *testing.T) {
	setupShowTest(t)
	parent := showParentTestFamily(t)

	out := runShowCommand(t, []string{}, func() {
		showTable, showMeta, showParent = true, true, parent.ID
	})

	got := showIDsInOutput(out)
	want := []string{"beans-kid01", "beans-kid02", "beans-kid03"}
	if len(got) != len(want) {
		t.Fatalf("showed %v, want the three children %v\n%s", got, want, out)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("shown ids = %v, want %v", got, want)
			break
		}
	}
}

// TestShowParentAppendsToGivenIDs keeps the case the feature was asked for:
// a milestone and its children on one page, without naming the milestone
// twice. Given ids come first, then the children, and the parent is not
// repeated when it is both an argument and the --parent item.
func TestShowParentAppendsToGivenIDs(t *testing.T) {
	setupShowTest(t)
	parent := showParentTestFamily(t)

	out := runShowCommand(t, []string{parent.ID}, func() {
		showTable, showMeta, showParent = true, true, parent.ID
	})

	got := showIDsInOutput(out)
	want := []string{parent.ID, "beans-kid01", "beans-kid02", "beans-kid03"}
	if len(got) != len(want) {
		t.Fatalf("showed %v, want the parent then its children %v\n%s", got, want, out)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("shown ids = %v, want %v", got, want)
			break
		}
	}
}

// TestShowParentRejectsUnknownItem keeps --parent as loud as a bad id: a typo
// must not silently render nothing.
func TestShowParentRejectsUnknownItem(t *testing.T) {
	setupShowTest(t)
	showParentTestFamily(t)

	oldParent := showParent
	showParent = "beans-nope1"
	t.Cleanup(func() { showParent = oldParent })

	err := showCmd.RunE(showCmd, []string{})
	if err == nil {
		t.Fatal("showCmd.RunE() error = nil, want a not-found error for an unknown --parent")
	}
	if !strings.Contains(err.Error(), "beans-nope1") {
		t.Errorf("error %q does not name the missing bean", err)
	}
}

// TestShowParentWithoutChildrenIsAnError distinguishes "no children" from
// "nothing to say": an empty page looks like a broken command.
func TestShowParentWithoutChildrenIsAnError(t *testing.T) {
	setupShowTest(t)
	showParentTestFamily(t)

	oldParent := showParent
	showParent = "beans-kid01"
	t.Cleanup(func() { showParent = oldParent })

	err := showCmd.RunE(showCmd, []string{})
	if err == nil {
		t.Fatal("showCmd.RunE() error = nil, want an error when --parent has no children")
	}
	if !strings.Contains(err.Error(), "beans-kid01") {
		t.Errorf("error %q does not name the childless bean", err)
	}
}

// TestShowRequiresIDsWithoutParent guards the other half of the arity change:
// dropping MinimumNArgs(1) must not make a bare `lif show` legal.
func TestShowRequiresIDsWithoutParent(t *testing.T) {
	setupShowTest(t)

	oldParent := showParent
	showParent = ""
	t.Cleanup(func() { showParent = oldParent })

	if err := showCmd.Args(showCmd, []string{}); err == nil {
		t.Error("showCmd.Args() error = nil for a bare `show`, want a usage error")
	}
	if err := showCmd.Args(showCmd, []string{"beans-kid01"}); err != nil {
		t.Errorf("showCmd.Args() error = %v for one id, want nil", err)
	}
	showParent = "beans-fam01"
	if err := showCmd.Args(showCmd, []string{}); err != nil {
		t.Errorf("showCmd.Args() error = %v with --parent and no ids, want nil", err)
	}
}

// runShowCommand runs show's RunE with stdout captured, applying setFlags
// after saving every show flag the suite mutates.
func runShowCommand(t *testing.T, args []string, setFlags func()) string {
	t.Helper()
	oldTable, oldMeta, oldParent := showTable, showMeta, showParent
	t.Cleanup(func() { showTable, showMeta, showParent = oldTable, oldMeta, oldParent })
	setFlags()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = w
	runErr := showCmd.RunE(showCmd, args)
	os.Stdout = oldStdout
	w.Close()
	captured, _ := io.ReadAll(r)
	if runErr != nil {
		t.Fatalf("showCmd.RunE() error = %v", runErr)
	}
	return string(captured)
}

// TestShowParentDoesNotRepeatAGivenChild is the collision --parent actually
// produces: naming one child explicitly and asking for the family would
// otherwise render that child twice, once from args and once from the
// resolver. The parent can never collide with its own children, so this is
// the case that earns the deduplication.
func TestShowParentDoesNotRepeatAGivenChild(t *testing.T) {
	setupShowTest(t)
	parent := showParentTestFamily(t)

	out := runShowCommand(t, []string{"beans-kid02"}, func() {
		showTable, showMeta, showParent = true, true, parent.ID
	})

	got := showIDsInOutput(out)
	want := []string{"beans-kid02", "beans-kid01", "beans-kid03"}
	if len(got) != len(want) {
		t.Fatalf("showed %v, want each bean once %v\n%s", got, want, out)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("shown ids = %v, want %v", got, want)
			break
		}
	}
}

// TestTableLongCustomKeyWidensTheColumn pins the one case where the column is
// not the vocabulary's width. Truncating a front matter key a user chose
// would hide which field a row is, so the column grows instead, and the two
// tests together say exactly how far the raster promise reaches.
func TestTableLongCustomKeyWidensTheColumn(t *testing.T) {
	setupShowTest(t)

	plain := &item.Item{ID: "beans-vocb1", Title: "Plain", Status: "todo", Type: "task"}
	longKey := "acceptance_criteria_reference"
	wide := &item.Item{
		ID: "beans-vocb2", Title: "Wide", Status: "todo", Type: "task",
		Extra: map[string]any{longKey: "yes"},
	}

	vocabulary := itemTableLabelWidth([]*item.Item{plain}, cfg)
	widened := itemTableLabelWidth([]*item.Item{wide}, cfg)

	if widened <= vocabulary {
		t.Errorf("label width %d did not grow past the vocabulary width %d for key %q",
			widened, vocabulary, longKey)
	}
	if widened < len(longKey+":") {
		t.Errorf("label width %d cannot hold %q in full", widened, longKey+":")
	}
	if got := itemTableLabelWidth([]*item.Item{plain, wide}, cfg); got != widened {
		t.Errorf("shared width %d for both beans, want the widened %d", got, widened)
	}
	rendered := renderItemTable(wide, cfg, 110, widened)
	if !strings.Contains(stripANSI(rendered), longKey+":") {
		t.Errorf("grid does not show the long key in full:\n%s", rendered)
	}
}

// writeShowAttachment drops a file into the throwaway core's attachment
// directory for the item id.
func writeShowAttachment(t *testing.T, id, name string) {
	t.Helper()
	dir := filepath.Join(core.Root(), itemcore.AttachmentsDir, id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

// The attachment directory is where a closed container's design documents
// live, so the detail view has to name them: D16 recorded "no show section"
// as the reason the sink could not carry them, and this is that section.
// The names come from the directory, not from front matter -- there is no
// key to drift.
func TestHeaderNamesAttachments(t *testing.T) {
	setupShowTest(t)
	writeShowAttachment(t, "beans-aaaa", "DESIGN.md")
	writeShowAttachment(t, "beans-aaaa", "REQUIREMENTS.md")
	b := showTestItem("beans-aaaa", "Host Bean", "Body.\n")

	out := stripANSI(renderItemHeader(b, cfg, 110))

	if !strings.Contains(out, "attachments:") {
		t.Fatalf("header carries no attachments row:\n%s", out)
	}
	for _, want := range []string{"DESIGN.md", "REQUIREMENTS.md"} {
		if !strings.Contains(out, want) {
			t.Errorf("header does not name %q:\n%s", want, out)
		}
	}
}

// Most items have no attachments, and an empty row would cost every one of
// them a line to say nothing.
func TestHeaderOmitsEmptyAttachments(t *testing.T) {
	setupShowTest(t)
	b := showTestItem("beans-aaaa", "Host Bean", "Body.\n")

	if out := stripANSI(renderItemHeader(b, cfg, 110)); strings.Contains(out, "attachments:") {
		t.Fatalf("header invents an attachments row:\n%s", out)
	}
}

// The grid is an arrangement of the same fields the header prints, not a
// selection of them (TestTableCarriesEveryFrontMatterField); a row the
// header gained and the grid did not is exactly the drift that guard exists
// for.
func TestTableNamesAttachments(t *testing.T) {
	setupShowTest(t)
	writeShowAttachment(t, "beans-aaaa", "DESIGN.md")
	b := showTestItem("beans-aaaa", "Host Bean", "Body.\n")

	out := stripANSI(renderItemTable(b, cfg, 110, itemTableLabelWidth([]*item.Item{b}, cfg)))

	if !strings.Contains(out, "attachments:") || !strings.Contains(out, "DESIGN.md") {
		t.Fatalf("grid does not name the attachment:\n%s", out)
	}
}

// The piped representation is the item file itself and "still parses as a
// item file" is its contract, so the derived row must not reach it: a
// reader that round-trips the output would otherwise write the row back as
// front matter and create exactly the second truth this design avoids.
func TestPipedOutputHasNoAttachmentsRow(t *testing.T) {
	setupShowTest(t)
	writeShowAttachment(t, "beans-aaaa", "DESIGN.md")
	b := showTestItem("beans-aaaa", "Host Bean", "Body.\n")

	out, err := showOutput(b, false, false, 110)
	if err != nil {
		t.Fatalf("showOutput: %v", err)
	}
	if strings.Contains(out, "attachments:") {
		t.Fatalf("piped output carries the derived row:\n%s", out)
	}
}

func TestShowFiltersParentChildrenByStatus(t *testing.T) {
	setupShowTest(t)
	seedStatusMix(t)
	t.Cleanup(func() { resetFlags(sharedTestRoot(t)) })

	stdout, _, err := runRootInDir(t, core.Root(), "show", "--meta", "--table", "--parent", "beans-mix00", "--status", "todo,draft")
	if err != nil {
		t.Fatalf("show error = %v", err)
	}
	if got, want := showIDsInOutput(stdout), []string{"beans-mixtd", "beans-mixdr"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ids = %v, want %v", got, want)
	}
}

func TestShowFiltersApplyToGivenIDs(t *testing.T) {
	setupShowTest(t)
	seedStatusMix(t)
	t.Cleanup(func() { resetFlags(sharedTestRoot(t)) })

	stdout, _, err := runRootInDir(t, core.Root(), "show", "--meta", "--table", "beans-mixip", "beans-mixtd", "--no-status", "in-progress")
	if err != nil {
		t.Fatalf("show error = %v", err)
	}
	if got, want := showIDsInOutput(stdout), []string{"beans-mixtd"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ids = %v, want %v", got, want)
	}
}

func TestShowFilterLeavingNothingIsAnError(t *testing.T) {
	setupShowTest(t)
	seedStatusMix(t)
	t.Cleanup(func() { resetFlags(sharedTestRoot(t)) })

	_, stderr, err := runRootInDir(t, core.Root(), "show", "--parent", "beans-mix00", "--status", "completed")
	if err == nil {
		t.Fatal("show error = nil, want error")
	}
	if !strings.Contains(stderr, "no item matches the filters") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "no item matches the filters")
	}
}

// AC3: show's lookup errors name the noun as item, in plain and in JSON mode.
func TestShowErrorsNameItems(t *testing.T) {
	setupShowTest(t)
	seedStatusMix(t)
	t.Cleanup(func() { resetFlags(sharedTestRoot(t)) })

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"unknown id", []string{"show", "beans-nope9"}, "item not found: beans-nope9"},
		{"unknown id as json", []string{"show", "--json", "beans-nope9"}, "item not found: beans-nope9"},
		{"unknown parent", []string{"show", "--parent", "beans-nope9"}, "item not found: beans-nope9"},
		{"parent without children", []string{"show", "--parent", "beans-mixtd"}, "item has no children: beans-mixtd"},
		{"filter leaves nothing as json", []string{"show", "--json", "--parent", "beans-mix00", "--status", "completed"}, "no item matches the filters"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, err := runRootInDir(t, core.Root(), tc.args...)
			if err == nil {
				t.Fatal("show error = nil, want error")
			}
			if got := stdout + stderr + err.Error(); !strings.Contains(got, tc.want) {
				t.Errorf("output = %q, want it to contain %q", got, tc.want)
			}
		})
	}
}
