package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/item"
)

// Scripted keys of a create-next run. Form 1 starts on the title "A" given as
// argument; the first type is milestone (DefaultType is cleared), so two Downs
// pick feature; two Downs pick priority high; the parent is picked by filter.
var (
	cnForm1 = []string{keyEnter, keyDown, keyDown, keyEnter, keyDown, keyDown, keyEnter, "/", "e1", keyEnter, keyEnter}
	// Follow-up form with every field carried: only the title is typed.
	cnForm2Carried = []string{"B", keyEnter, keyEnter, keyEnter, keyEnter}
)

// cnFixture installs the fixture with a type cursor on the first option and
// one default tag.
func cnFixture(t *testing.T) {
	t.Helper()
	ciFixture(t)
	cfg.Items.DefaultType = ""
	cfg.Items.DefaultTags = []string{"dflt"}
}

// cnRun runs the whole core and returns every item it wrote by title, stdout,
// chrome and the error. Config and store file names are checked on the way:
// only item files may be new.
func cnRun(t *testing.T, args []string, programs ...[]string) (map[string]*item.Item, []byte, *bytes.Buffer, error) {
	t.Helper()
	before := map[string]bool{}
	for _, it := range core.All() {
		before[it.ID] = true
	}
	files := func() []string {
		var fs []string
		_ = filepath.WalkDir(core.Root(), func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				fs = append(fs, p)
			}
			return nil
		})
		return fs
	}
	filesBefore := len(files())
	var chrome bytes.Buffer
	var runErr error
	stdout := captureCreateStdout(t, func() {
		runErr = runCreateInteractiveWith(createCmd, args, feedCreatePrograms(t, programs...), &chrome)
	})
	added := map[string]*item.Item{}
	for _, it := range core.All() {
		if !before[it.ID] {
			added[it.Title] = it
		}
	}
	if got := len(files()) - filesBefore; got != len(added) {
		t.Errorf("%d new files for %d new items", got, len(added))
	}
	return added, stdout, &chrome, runErr
}

func TestCreateInteractiveCreateNextCarriesTicked(t *testing.T) {
	cnFixture(t)
	got, stdout, _, err := cnRun(t, []string{"A"},
		cnForm1,
		cmBody, []string{"body of A", keyEnter},
		cmStatus, []string{keyDown, keyEnter},
		cmTags, []string{",taga", keyEnter},
		cmNext,
		[]string{keyEnter}, // checklist: all four ticked on first open
		cnForm2Carried,
		[]string{keyEnter}, // menu: create
	)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	a, b := got["A"], got["B"]
	if a == nil || b == nil || len(got) != 2 {
		t.Fatalf("written = %v, want A and B", got)
	}
	if a.Type != "feature" || a.Priority != "high" || a.Parent != "pp-e1" || !slices.Equal(a.Tags, []string{"dflt", "taga"}) {
		t.Fatalf("item A = %s %s %s %v", a.Type, a.Priority, a.Parent, a.Tags)
	}
	if b.Type != a.Type || b.Priority != a.Priority || b.Parent != a.Parent || !slices.Equal(b.Tags, a.Tags) {
		t.Errorf("item B = %s %s %s %v, want carried from A", b.Type, b.Priority, b.Parent, b.Tags)
	}
	if b.Body != "" {
		t.Errorf("body of B = %q, want empty", b.Body)
	}
	if a.Status == cfg.GetDefaultStatus() {
		t.Fatal("test setup: A must have a non-default status")
	}
	if b.Status != cfg.GetDefaultStatus() {
		t.Errorf("status of B = %q, want default %q", b.Status, cfg.GetDefaultStatus())
	}
	if n := strings.Count(string(stdout), "Created"); n != 2 {
		t.Errorf("stdout has %d Created lines, want 2: %q", n, stdout)
	}
}

func TestCreateInteractiveCreateNextUntickedFallsBackAndChecklistRemembers(t *testing.T) {
	cnFixture(t)
	got, _, _, err := cnRun(t, []string{"A"},
		cnForm1,
		cmTags, []string{",taga", keyEnter},
		cmNext,
		// untick priority and tags
		[]string{keyDown, "x", keyDown, keyDown, "x", keyEnter},
		// B: priority high again, tag tagb on top of the default tag
		[]string{"B", keyEnter, keyEnter, keyDown, keyDown, keyEnter, keyEnter},
		cmTags, []string{",tagb", keyEnter},
		cmNext,
		// second checklist must start with type and parent ticked only
		[]string{keyEnter},
		[]string{"C", keyEnter, keyEnter, keyEnter, keyEnter},
		[]string{keyEnter},
	)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	a, b, c := got["A"], got["B"], got["C"]
	if a == nil || b == nil || c == nil {
		t.Fatalf("written = %v", got)
	}
	if b.Priority != "high" || !slices.Equal(b.Tags, []string{"dflt", "tagb"}) || b.Type != "feature" || b.Parent != "pp-e1" {
		t.Fatalf("item B = %s %s %s %v", b.Type, b.Priority, b.Parent, b.Tags)
	}
	if c.Type != "feature" || c.Parent != "pp-e1" {
		t.Errorf("item C type %q parent %q, want carried", c.Type, c.Parent)
	}
	if c.Priority != cfg.GetDefaultPriority() || !slices.Equal(c.Tags, []string{"dflt"}) {
		t.Errorf("item C priority %q tags %v: second checklist did not start with the last choice", c.Priority, c.Tags)
	}
}

func TestCreateInteractiveCreateNextCarriedParentDroppedForOtherType(t *testing.T) {
	cnFixture(t)
	got, _, _, err := cnRun(t, []string{"A"},
		cnForm1,
		cmNext,
		[]string{keyEnter},
		// B: type feature -> epic (Up once); feature parent pp-e1 is no epic parent
		[]string{"B", keyEnter, keyUp, keyEnter, keyEnter, keyEnter},
		[]string{keyEnter},
	)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if b := got["B"]; b == nil || b.Type != "epic" || b.Parent == "pp-e1" {
		t.Fatalf("item B = %+v, want epic without the carried feature parent", b)
	}
}

func TestCreateInteractiveCreateNextEscInChecklistFilterKeepsItOpen(t *testing.T) {
	cnFixture(t)
	got, _, _, err := cnRun(t, []string{"A"},
		cnForm1,
		cmNext,
		// filter, Esc ends the filter, x unticks priority, Enter confirms
		[]string{"/", "pri", keyEsc, "x", keyEnter},
		[]string{"B", keyEnter, keyEnter, keyEnter, keyEnter},
		[]string{keyEnter},
	)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("written = %v: Esc closed the checklist", got)
	}
	if b := got["B"]; b.Priority != cfg.GetDefaultPriority() || b.Type != "feature" {
		t.Errorf("item B priority %q type %q: the x after Esc did not reach the checklist", b.Priority, b.Type)
	}
}

func TestCreateInteractiveCreateNextCtrlCEndsSeriesWithExitZero(t *testing.T) {
	cases := map[string][][]string{
		"checklist":      {{keyCtrlC}},
		"form title":     {{keyEnter}, {keyCtrlC}},
		"form later":     {{keyEnter}, {"B", keyEnter, keyEnter, keyCtrlC}},
		"menu":           {{keyEnter}, cnForm2Carried, {keyCtrlC}},
		"body of form 2": {{keyEnter}, cnForm2Carried, cmBody, {keyCtrlC}},
	}
	for name, tail := range cases {
		t.Run(name, func(t *testing.T) {
			cnFixture(t)
			programs := append([][]string{cnForm1, cmNext}, tail...)
			got, stdout, _, err := cnRun(t, []string{"A"}, programs...)
			if err != nil {
				t.Fatalf("err = %v, want nil after a written item", err)
			}
			if len(got) != 1 || got["A"] == nil {
				t.Errorf("written = %v, want only A", got)
			}
			if n := strings.Count(string(stdout), "Created"); n != 1 {
				t.Errorf("stdout = %q, want one Created line", stdout)
			}
		})
	}
}

func TestCreateInteractiveCtrlCBeforeFirstWriteStaysSilentError(t *testing.T) {
	cnFixture(t)
	got, stdout, _, err := cnRun(t, []string{"A"}, cnForm1, []string{keyCtrlC})
	if err != errCreateAborted || !output.IsSilent(err) {
		t.Fatalf("err = %v, want errCreateAborted", err)
	}
	if len(got) != 0 || len(stdout) != 0 {
		t.Errorf("written %v stdout %q", got, stdout)
	}
}

func TestCreateInteractiveJSONRunHasNoCreateNext(t *testing.T) {
	cnFixture(t)
	createJSON = true
	t.Cleanup(func() { createJSON = false })
	got, stdout, _, err := cnRun(t, []string{"A"},
		[]string{keyEnter, keyEnter, keyEnter, keyEnter},
		[]string{keyDown, keyEnter}, // second entry is set tags, not create next
		[]string{",jt", keyEnter},
		[]string{keyEnter},
	)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if a := got["A"]; a == nil || !slices.Contains(a.Tags, "jt") {
		t.Fatalf("written = %v", got)
	}
	if strings.Count(string(stdout), `"id"`) != 1 {
		t.Errorf("stdout = %q, want one JSON object", stdout)
	}
}

func TestRunCreateCarryStepColoursComeFromFlavour(t *testing.T) {
	withTrueColor(t)
	hexRe := regexp.MustCompile(`\x1b\[[0-9;]*?(?:38|48);2;(\d+);(\d+);(\d+)m`)
	out := map[string]string{}
	for _, flavour := range []string{"mocha", "latte"} {
		t.Run(flavour, func(t *testing.T) {
			ui.SetTheme(flavour)
			t.Cleanup(func() { ui.SetTheme("mocha") })
			ciFixture(t)
			carry := newCreateCarry()
			var chrome bytes.Buffer
			if err := runCreateCarryStep(&carry, feedCreatePrograms(t, []string{keyEnter}), &chrome); err != nil {
				t.Fatal(err)
			}
			view := chrome.String()
			out[flavour] = view
			allowed := map[string]bool{}
			for _, hex := range ui.ActiveTheme().Colors {
				allowed[strings.ToLower(hex)] = true
			}
			ms := hexRe.FindAllStringSubmatch(view, -1)
			if len(ms) == 0 {
				t.Fatal("no colour escapes under forced TrueColor")
			}
			for _, m := range ms {
				r, _ := strconv.Atoi(m[1])
				g, _ := strconv.Atoi(m[2])
				b, _ := strconv.Atoi(m[3])
				if !ppNearFlavour(allowed, r, g, b) {
					t.Errorf("colour #%02x%02x%02x is not in flavour %s", r, g, b, flavour)
				}
			}
			text := ansi.Strip(view)
			for _, label := range []string{"type", "priority", "parent", "tags", "ctrl+c cancel"} {
				if !strings.Contains(text, label) {
					t.Errorf("checklist lacks %q in %q", label, text)
				}
			}
		})
	}
	if out["mocha"] == out["latte"] {
		t.Error("mocha and latte render identically")
	}
}
