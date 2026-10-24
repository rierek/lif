package commands

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/itemcore"
)

// runCheckOnStore runs `lif check` in a subprocess (see TestCheckHelperProcess)
// against itemsDir. The first run seeds scenario; later runs pass seed=false
// and see what the earlier ones left on disk. flags are the env switches
// BEANS_CHECK_FIX, BEANS_CHECK_STRICT and BEANS_CHECK_JSON, as "NAME=1".
func runCheckOnStore(t *testing.T, itemsDir, scenario string, seed bool, flags ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestCheckHelperProcess$")
	cmd.Env = append(os.Environ(),
		"BEANS_CHECK_HELPER=1",
		"BEANS_CHECK_DIR="+itemsDir,
		"BEANS_CHECK_THEME="+ui.ActiveTheme().Name,
		"BEANS_CHECK_CYCLES="+scenario,
	)
	if !seed {
		cmd.Env = append(cmd.Env, "BEANS_CHECK_NOSEED=1")
	}
	cmd.Env = append(cmd.Env, flags...)
	out, err := cmd.Output()
	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running check subprocess: %v", err)
		}
		code = exitErr.ExitCode()
	}
	return stripANSITest(string(out)), code
}

func newCheckStoreDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), ".beans")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("creating test store dir: %v", err)
	}
	return dir
}

// storeFiles returns every file of the store by name, for byte comparison.
func storeFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		files[e.Name()] = string(b)
	}
	return files
}

func loadStore(t *testing.T, dir string) *itemcore.Core {
	t.Helper()
	c := itemcore.New(dir, config.Default())
	c.SetWarnWriter(nil)
	if err := c.Load(); err != nil {
		t.Fatalf("Load(): %v", err)
	}
	return c
}

// SC-01: one warning, exit 0 without --strict, 1 with it, files untouched.
func TestCheckWarnsOfARedundantEdgeAndLeavesTheFilesAlone(t *testing.T) {
	dir := newCheckStoreDir(t)
	out, code := runCheckOnStore(t, dir, "redundant", true)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 without --strict; output:\n%s", code, out)
	}
	const edge = "aaa1 → ccc3 (holds via bbb2)"
	if got := strings.Count(out, edge); got != 1 {
		t.Errorf("edge %q appears %d times, want 1; output:\n%s", edge, got, out)
	}
	if !strings.Contains(out, "Policy") {
		t.Errorf("output has no Policy section without require_fields_on:\n%s", out)
	}
	before := storeFiles(t, dir)

	out, code = runCheckOnStore(t, dir, "redundant", false, "BEANS_CHECK_STRICT=1")
	if code != 1 {
		t.Errorf("exit code = %d, want 1 with --strict; output:\n%s", code, out)
	}
	if !strings.Contains(out, "1 issue found") {
		t.Errorf("output lacks %q:\n%s", "1 issue found", out)
	}
	if after := storeFiles(t, dir); !reflect.DeepEqual(before, after) {
		t.Errorf("check without --fix changed the store")
	}
}

func TestCheckJSONListsTheRedundantEdgeAmongThePolicyWarnings(t *testing.T) {
	dir := newCheckStoreDir(t)
	out, code := runCheckOnStore(t, dir, "redundant", true, "BEANS_CHECK_JSON=1")
	if code != 0 {
		t.Errorf("exit code = %d, want 0; output:\n%s", code, out)
	}
	var doc struct {
		PolicyWarnings []string `json:"policy_warnings"`
		Fixed          int      `json:"fixed"`
	}
	if err := json.NewDecoder(strings.NewReader(out)).Decode(&doc); err != nil {
		t.Fatalf("decoding JSON: %v; output = %s", err, out)
	}
	if len(doc.PolicyWarnings) != 1 || !strings.Contains(doc.PolicyWarnings[0], "aaa1 → ccc3 (holds via bbb2)") {
		t.Errorf("policy_warnings = %q, want one warning naming aaa1 → ccc3 (holds via bbb2)", doc.PolicyWarnings)
	}
}

// SC-02: --fix drops a→c from both fields, keeps the rest, names the path;
// the next strict run is clean.
func TestCheckFixRemovesTheRedundantEdgeFromEveryDeclaration(t *testing.T) {
	dir := newCheckStoreDir(t)
	runCheckOnStore(t, dir, "redundant", true) // seeds the store

	out, code := runCheckOnStore(t, dir, "redundant", false, "BEANS_CHECK_FIX=1")
	if code != 0 {
		t.Errorf("exit code = %d, want 0; output:\n%s", code, out)
	}
	if !strings.Contains(out, "removed redundant edge aaa1 → ccc3 (holds via bbb2)") {
		t.Errorf("output does not name the removed edge and its path:\n%s", out)
	}
	if !strings.Contains(out, "Fixed 1 issue(s)") {
		t.Errorf("output lacks the fixed count:\n%s", out)
	}

	c := loadStore(t, dir)
	a, _ := c.Get("aaa1")
	cc, _ := c.Get("ccc3")
	if want := []string{"bbb2"}; !reflect.DeepEqual(a.Blocking, want) {
		t.Errorf("aaa1.Blocking = %v, want %v", a.Blocking, want)
	}
	if want := []string{"bbb2"}; !reflect.DeepEqual(cc.BlockedBy, want) {
		t.Errorf("ccc3.BlockedBy = %v, want %v", cc.BlockedBy, want)
	}
	if a.Parent != "ppp0" {
		t.Errorf("aaa1.Parent = %q, want ppp0", a.Parent)
	}

	out, code = runCheckOnStore(t, dir, "redundant", false, "BEANS_CHECK_STRICT=1")
	if code != 0 || strings.Contains(out, "redundant edge") {
		t.Errorf("second strict run: exit %d, output:\n%s", code, out)
	}
}

func TestCheckFixNamesTheWholePathOfEachRemovedEdge(t *testing.T) {
	out, code := runCheckOnStore(t, newCheckStoreDir(t), "longpath", true, "BEANS_CHECK_FIX=1")
	if code != 0 {
		t.Errorf("exit code = %d, want 0; output:\n%s", code, out)
	}
	for _, want := range []string{
		"removed redundant edge aaa1 → ddd4 (holds via bbb2 → ccc3)",
		"removed redundant edge bbb2 → ddd4 (holds via ccc3)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestCheckFixJSONCountsEachRemovedEdge(t *testing.T) {
	out, _ := runCheckOnStore(t, newCheckStoreDir(t), "longpath", true, "BEANS_CHECK_FIX=1", "BEANS_CHECK_JSON=1")
	var doc struct {
		Fixed          int      `json:"fixed"`
		PolicyWarnings []string `json:"policy_warnings"`
	}
	if err := json.NewDecoder(strings.NewReader(out)).Decode(&doc); err != nil {
		t.Fatalf("decoding JSON: %v; output = %s", err, out)
	}
	if doc.Fixed != 2 || len(doc.PolicyWarnings) != 0 {
		t.Errorf("fixed = %d, policy_warnings = %q, want 2 and none", doc.Fixed, doc.PolicyWarnings)
	}
}

// A cycle holds its own parallel paths: check reports nothing and keeps every edge.
func TestCheckKeepsTheEdgesOfACycle(t *testing.T) {
	out, _ := runCheckWithCycles(t, "ring")
	if strings.Contains(out, "redundant edge") {
		t.Errorf("check reported a redundant edge inside a cycle:\n%s", out)
	}
}
