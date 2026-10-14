package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xRiErOS/lif/internal/output"
	"github.com/xRiErOS/lif/internal/ui"
	"github.com/xRiErOS/lif/pkg/item"
)

const (
	keyEnter = "\r"
	keyDown  = "\x1b[B"
	keyEsc   = "\x1b"
	keyCtrlC = "\x03"
)

// feedCreatePrograms returns the input source of a test run. programs[N] is
// the key script of the N-th huh program: call N returns a fresh reader that
// delivers only those keys, one key per read, each after a pause that lets
// the program process the previous key (a burst loses Enter before the
// parent options load). A call beyond the scripted programs fails the test.
func feedCreatePrograms(t *testing.T, programs ...[]string) createInput {
	t.Helper()
	t.Setenv("TERM", "xterm-256color") // TERM=dumb pins huh to width 80
	next := 0
	return func() io.Reader {
		if next >= len(programs) {
			t.Fatalf("run opened program %d, only %d scripted", next+1, len(programs))
		}
		keys := programs[next]
		next++
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("os.Pipe() error = %v", err)
		}
		done := make(chan struct{})
		go func() {
			for _, k := range keys {
				select {
				case <-done:
					return
				case <-time.After(120 * time.Millisecond):
				}
				if _, err := w.WriteString(k); err != nil {
					return
				}
			}
			// Watchdog: a script that leaves the form open aborts it, so a
			// broken script fails the test instead of hanging it.
			select {
			case <-done:
			case <-time.After(4 * time.Second):
				t.Logf("program %d still open after its script; sending Ctrl+C", next)
				_, _ = w.WriteString(keyCtrlC)
				<-done
			}
		}()
		t.Cleanup(func() { close(done); w.Close(); r.Close() })
		return r
	}
}

// ciFixture installs the three-level fixture store as the command's core.
func ciFixture(t *testing.T) {
	t.Helper()
	resolver, c := ppFixture(t)
	oldCore, oldCfg := core, cfg
	core, cfg = resolver.Core, c
	t.Cleanup(func() { core, cfg = oldCore, oldCfg })
	resetCreateFlags(t)
}

// ciRun runs the interactive core and returns the new item (nil on error),
// command stdout and the error.
// The menu between the parent step and the write is one more program: Enter
// on "create". A run that never reaches it never opens it.
func ciRun(t *testing.T, args []string, programs ...[]string) (*item.Item, []byte, *bytes.Buffer, error) {
	t.Helper()
	return ciRunRaw(t, args, append(programs, []string{keyEnter})...)
}

// ciRunRaw runs the core with exactly the given programs.
func ciRunRaw(t *testing.T, args []string, programs ...[]string) (*item.Item, []byte, *bytes.Buffer, error) {
	t.Helper()
	before := map[string]bool{}
	for _, it := range core.All() {
		before[it.ID] = true
	}
	var chrome bytes.Buffer
	var runErr error
	stdout := captureCreateStdout(t, func() {
		runErr = runCreateInteractiveWith(createCmd, args, feedCreatePrograms(t, programs...), &chrome)
	})
	var added []*item.Item
	for _, it := range core.All() {
		if !before[it.ID] {
			added = append(added, it)
		}
	}
	if runErr != nil && len(added) != 0 {
		t.Fatalf("error %v but %d items written", runErr, len(added))
	}
	if len(added) > 1 {
		t.Fatalf("%d items written, want at most 1", len(added))
	}
	if len(added) == 1 {
		return added[0], stdout, &chrome, runErr
	}
	return nil, stdout, &chrome, runErr
}

func TestCreateInteractiveStepsWriteOneItem(t *testing.T) {
	ciFixture(t)
	cfg.Items.DefaultType = "" // the type select then starts on its first option
	got, stdout, chrome, err := ciRun(t, nil, []string{
		"Hello form", keyEnter, // title
		keyDown, keyDown, keyEnter, // type: milestone, epic, feature
		keyDown, keyDown, keyEnter, // priority: (none), critical, high
		"/", "e1", keyEnter, keyEnter, // parent: filter, end filter, pick
	})
	if err != nil {
		t.Fatalf("runCreateInteractiveWith error = %v", err)
	}
	if got.Title != "Hello form" || got.Type != "feature" || got.Priority != "high" || got.Parent != "pp-e1" {
		t.Errorf("item = title %q type %q priority %q parent %q", got.Title, got.Type, got.Priority, got.Parent)
	}
	if !strings.Contains(string(stdout), "Created") || !strings.Contains(string(stdout), got.ID) {
		t.Errorf("stdout = %q, want Created line with the id", stdout)
	}
	if strings.Contains(string(stdout), "\x1b[?") {
		t.Errorf("stdout holds terminal control sequences: %q", stdout)
	}
	view := ciText(chrome.String())
	for _, step := range []string{"Title", "Parent"} {
		if !strings.Contains(view, step) {
			t.Errorf("chrome never showed step %q", step)
		}
	}
	if i, j := strings.Index(view, "Title"), strings.Index(view, "Parent"); i > j {
		t.Errorf("Title step after Parent step")
	}
}

func TestCreateInteractiveFormOptions(t *testing.T) {
	ciFixture(t)
	_, _, chrome, _ := ciRun(t, nil, []string{keyEnter, keyEnter, keyCtrlC})
	view := ciText(chrome.String())
	for _, want := range []string{"milestone", "epic", "feature", "bug", "task", "(none)", "critical", "deferred"} {
		if !strings.Contains(view, want) {
			t.Errorf("chrome lacks option %q", want)
		}
	}
	if i, j := strings.Index(view, "(none)"), strings.Index(view, "critical"); i < 0 || i > j {
		t.Errorf("(none) must come before critical")
	}
}

func TestCreateInteractivePrefillOrder(t *testing.T) {
	enters := []string{keyEnter, keyEnter, keyEnter, keyEnter}

	t.Run("flag beats config default", func(t *testing.T) {
		ciFixture(t)
		cfg.Items.DefaultType = "task"
		createType = "bug"
		got, _, _, err := ciRun(t, []string{"T"}, enters)
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != "T" || got.Type != "bug" {
			t.Errorf("title %q type %q, want T/bug", got.Title, got.Type)
		}
	})

	t.Run("config defaults without flags", func(t *testing.T) {
		ciFixture(t)
		cfg.Items.DefaultType = "task"
		cfg.Items.DefaultPriority = "high"
		cfg.Items.DefaultTags = []string{"a", "b"}
		got, _, _, err := ciRun(t, []string{"T"}, enters)
		if err != nil {
			t.Fatal(err)
		}
		if got.Type != "task" || got.Priority != "high" {
			t.Errorf("type %q priority %q, want task/high", got.Type, got.Priority)
		}
		if strings.Join(got.Tags, ",") != "a,b" {
			t.Errorf("tags = %v, want [a b]", got.Tags)
		}
	})

	t.Run("form overrides pre-fill", func(t *testing.T) {
		ciFixture(t)
		cfg.Items.DefaultType = "task"
		cfg.Items.DefaultPriority = "high"
		got, _, _, err := ciRun(t, []string{"T"}, []string{keyEnter, keyEnter, keyDown, keyEnter, keyEnter})
		if err != nil {
			t.Fatal(err)
		}
		if got.Priority != "normal" {
			t.Errorf("priority = %q, want normal from the form", got.Priority)
		}
	})

	t.Run("untouched none equals plain create", func(t *testing.T) {
		ciFixture(t)
		cfg.Items.DefaultType = "task"
		got, _, _, err := ciRun(t, []string{"T"}, enters)
		if err != nil {
			t.Fatal(err)
		}
		createType = "task"
		var plain *item.Item
		captureCreateStdout(t, func() {
			if err := createCmd.RunE(createCmd, []string{"P"}); err != nil {
				t.Fatal(err)
			}
		})
		for _, it := range core.All() {
			if it.Title == "P" {
				plain = it
			}
		}
		if plain == nil || got.Priority != plain.Priority {
			t.Errorf("-i priority %q, plain priority %v", got.Priority, plain)
		}
	})
}

func TestCreateInteractiveCtrlCAtEveryStepWritesNothing(t *testing.T) {
	steps := [][]string{
		{keyCtrlC},
		{keyEnter, keyCtrlC},
		{keyEnter, keyEnter, keyCtrlC},
		{keyEnter, keyEnter, keyEnter, keyCtrlC},
	}
	for i, keys := range steps {
		ciFixture(t)
		cfg.Items.DefaultType = "task"
		got, stdout, _, err := ciRun(t, []string{"T"}, keys)
		if got != nil {
			t.Errorf("step %d wrote an item", i+1)
		}
		if !output.IsSilent(err) {
			t.Errorf("step %d error = %v, want silent", i+1, err)
		}
		if len(stdout) != 0 {
			t.Errorf("step %d stdout = %q, want empty", i+1, stdout)
		}
	}
}

func TestCreateInteractiveEscInParentFilterKeepsFormOpen(t *testing.T) {
	ciFixture(t)
	cfg.Items.DefaultType = "feature"
	got, _, _, err := ciRun(t, []string{"T"}, []string{
		keyEnter, keyEnter, keyEnter, // title, type, priority
		"/", "e1", keyEsc, // Esc must not cancel
		keyEnter, // form still open: submits the parent step
	})
	if err != nil {
		t.Fatalf("Esc cancelled or failed the form: %v", err)
	}
	if got == nil {
		t.Fatal("no item written after completing the form")
	}
}

func TestCreateInteractiveParentGuard(t *testing.T) {
	enters := []string{keyEnter, keyEnter, keyEnter, keyEnter}

	t.Run("milestone offers only no parent", func(t *testing.T) {
		ciFixture(t)
		createType = "milestone"
		got, _, chrome, err := ciRun(t, []string{"T"}, enters)
		if err != nil {
			t.Fatal(err)
		}
		if got.Parent != "" {
			t.Errorf("parent = %q, want none", got.Parent)
		}
		if strings.Contains(ciText(chrome.String()), "pp-m1") {
			t.Errorf("parent step listed a candidate for a milestone")
		}
	})

	t.Run("prefilled non-candidate is dropped", func(t *testing.T) {
		ciFixture(t)
		createType = "milestone"
		createParent = "pp-e1"
		got, _, _, err := ciRun(t, []string{"T"}, enters)
		if err != nil {
			t.Fatal(err)
		}
		if got.Parent != "" {
			t.Errorf("parent = %q, want dropped", got.Parent)
		}
	})

	t.Run("short id normalised and kept", func(t *testing.T) {
		ciFixture(t)
		createType = "feature"
		createParent = "e1"
		got, _, _, err := ciRun(t, []string{"T"}, enters)
		if err != nil {
			t.Fatal(err)
		}
		if got.Parent != "pp-e1" {
			t.Errorf("parent = %q, want pp-e1", got.Parent)
		}
	})
}

func TestCreateInteractiveDefaultStatus(t *testing.T) {
	ciFixture(t)
	cfg.Items.DefaultType = "task"
	cfg.Items.DefaultStatus = "draft"
	got, _, _, err := ciRun(t, []string{"T"}, []string{keyEnter, keyEnter, keyEnter, keyEnter})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "draft" {
		t.Errorf("status = %q, want draft", got.Status)
	}
}

func jsonKeys(t *testing.T, b []byte) []string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("stdout is not one JSON object: %v; %q", err, b)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestCreateInteractiveJSONMatchesPlainShape(t *testing.T) {
	ciFixture(t)
	cfg.Items.DefaultType = "task"
	createJSON = true
	_, stdout, _, err := ciRun(t, []string{"T"}, []string{keyEnter, keyEnter, keyEnter, keyEnter})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.IndexByte(stdout, 0x1b) >= 0 {
		t.Errorf("stdout holds an escape byte: %q", stdout)
	}
	plain := captureCreateStdout(t, func() {
		if err := createCmd.RunE(createCmd, []string{"P"}); err != nil {
			t.Fatal(err)
		}
	})
	if a, b := strings.Join(jsonKeys(t, stdout), ","), strings.Join(jsonKeys(t, plain), ","); a != b {
		t.Errorf("keys differ:\n -i:    %s\n plain: %s", a, b)
	}
}

func TestCreateInteractiveRequiresTerminal(t *testing.T) {
	ciFixture(t)
	devnull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close()
	oldStdin := os.Stdin
	os.Stdin = devnull
	defer func() { os.Stdin = oldStdin }()
	createInteractive = true
	createBody = "-"

	start := time.Now()
	runErr := createCmd.RunE(createCmd, []string{"T"})
	if runErr == nil || !strings.Contains(runErr.Error(), "terminal") {
		t.Fatalf("error = %v, want a message naming the terminal", runErr)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("guard took %v", d)
	}
	if len(core.All()) != 10 {
		t.Errorf("guard wrote an item")
	}
}

func TestCreateInteractiveColoursComeFromFlavour(t *testing.T) {
	withTrueColor(t)
	hexRe := regexp.MustCompile(`\x1b\[[0-9;]*?(?:38|48);2;(\d+);(\d+);(\d+)m`)
	out := map[string]string{}
	for _, flavour := range []string{"mocha", "latte"} {
		t.Run(flavour, func(t *testing.T) {
			ui.SetTheme(flavour)
			t.Cleanup(func() { ui.SetTheme("mocha") })
			ciFixture(t)
			_, _, chrome, _ := ciRun(t, nil, []string{keyEnter, keyEnter, keyEnter, keyCtrlC})
			out[flavour] = chrome.String()
			allowed := map[string]bool{}
			for _, hex := range ui.ActiveTheme().Colors {
				allowed[strings.ToLower(hex)] = true
			}
			ms := hexRe.FindAllStringSubmatch(chrome.String(), -1)
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
		})
	}
	if out["mocha"] == out["latte"] {
		t.Error("mocha and latte render identically")
	}
}

var ciCSI = regexp.MustCompile("\x1b\\[[0-9;?]*[a-zA-Z]")

// ciText drops every CSI sequence (colour and cursor) from captured frames.
func ciText(s string) string { return ciCSI.ReplaceAllString(s, "") }
