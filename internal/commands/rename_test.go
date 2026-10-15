package commands

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/spf13/cobra"
)

func renameTestCore(t *testing.T, prefix string, files map[string]string) *itemcore.Core {
	t.Helper()
	repo := t.TempDir()
	itemsDir := filepath.Join(repo, ".beans")
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(itemsDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.DefaultWithPrefix(prefix)
	cfg.SetConfigDir(repo)
	c := itemcore.New(itemsDir, cfg)
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestBuildRenamePlan_dispatch(t *testing.T) {
	seed := map[string]string{
		"tp-aaaa--a.md": "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
	}
	tests := []struct {
		name     string
		args     []string
		flags    renameFlags
		wantMode string
		wantErr  bool
	}{
		{"slug", []string{"tp-aaaa"}, renameFlags{slug: "x", slugSet: true}, "slug", false},
		{"noSlug", []string{"tp-aaaa"}, renameFlags{noSlug: true}, "slug", false},
		{"reslug", []string{"tp-aaaa"}, renameFlags{reslug: true}, "slug", false},
		{"newID", []string{"tp-aaaa", "tp-zzzz"}, renameFlags{}, "id", false},
		{"suffix", []string{"tp-aaaa"}, renameFlags{suffix: "k7x2", suffixSet: true}, "id", false},
		{"prefix", nil, renameFlags{prefix: "op-", prefixSet: true}, "prefix", false},
		{"mutual-excl slug+newid", []string{"tp-aaaa", "tp-zzzz"}, renameFlags{slug: "x", slugSet: true}, "", true},
		{"prefix with positional", []string{"tp-aaaa"}, renameFlags{prefix: "op-", prefixSet: true}, "", true},
		{"no args no prefix", nil, renameFlags{}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := renameTestCore(t, "tp-", seed)
			plan, err := buildRenamePlan(c, tt.args, tt.flags)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got plan %+v", plan)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if plan.Mode != tt.wantMode {
				t.Errorf("mode = %q, want %q", plan.Mode, tt.wantMode)
			}
		})
	}
}

func TestBuildRenamePlan_suffixWrongPrefixErrors(t *testing.T) {
	c := renameTestCore(t, "tp-", map[string]string{
		"other-aaaa--a.md": "---\n# other-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
	})
	if _, err := buildRenamePlan(c, []string{"other-aaaa"}, renameFlags{suffix: "k7x2", suffixSet: true}); err == nil {
		t.Fatal("expected error: --suffix on an id lacking the configured prefix")
	}
}

// TestApplyRenameWithGuards_reChecksGuardsBeforeApply is the I01 regression
// test: for a "prefix" rebrand, the server/worktree guards must be
// re-checked immediately before the real apply/staging (not only at Plan
// time), closing the TOCTOU window between building the plan (e.g. while a
// user considers an interactive --yes confirm) and the actual cascade swap.
// We simulate "a server started after planning" by binding the configured
// port AFTER a valid plan was built, then assert applyRenameWithGuards
// refuses instead of applying.
func TestApplyRenameWithGuards_reChecksGuardsBeforeApply(t *testing.T) {
	c := renameTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md": "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
	})

	plan, err := buildRenamePlan(c, nil, renameFlags{prefix: "op-", prefixSet: true})
	if err != nil {
		t.Fatal(err)
	}

	// Simulate a server starting between plan and apply by occupying the
	// configured port now.
	port := c.Config().GetServerPort()
	ln, lerr := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if lerr != nil {
		t.Skipf("cannot bind configured port %d: %v", port, lerr)
	}
	defer ln.Close()

	if err := applyRenameWithGuards(c, plan); err == nil {
		t.Fatal("expected refusal: guard must be re-checked immediately before apply (I01)")
	}

	// Item must NOT have been rebranded on disk (guard closed the TOCTOU window).
	if _, err := c.Get("op-aaaa"); err == nil {
		t.Error("apply proceeded despite guard re-check — TOCTOU window not closed")
	}
	if _, err := c.Get("tp-aaaa"); err != nil {
		t.Error("original bean missing after refused apply — plan must not have mutated state")
	}
}

// TestApplyRenameWithGuards_nonPrefixModeSkipsGuards verifies that id/slug
// modes are not subject to the prefix-only D05 guards (they apply cleanly
// even while a listener occupies the configured port).
func TestApplyRenameWithGuards_nonPrefixModeSkipsGuards(t *testing.T) {
	c := renameTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md": "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
	})
	plan, err := buildRenamePlan(c, []string{"tp-aaaa"}, renameFlags{slug: "new-slug", slugSet: true})
	if err != nil {
		t.Fatal(err)
	}

	port := c.Config().GetServerPort()
	ln, lerr := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if lerr == nil {
		defer ln.Close()
	}

	if err := applyRenameWithGuards(c, plan); err != nil {
		t.Fatalf("slug rename should not be blocked by prefix-only guards: %v", err)
	}
}

// AC3: rename's errors, plan summary, prompt and result line name the noun
// as item.
func TestRenameTextsNameItems(t *testing.T) {
	seed := map[string]string{
		"tp-aaaa--a.md": "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
		"zz-bbbb--b.md": "---\n# zz-bbbb\ntitle: B\nstatus: todo\ntype: task\n---\n",
	}

	t.Run("no id", func(t *testing.T) {
		c := renameTestCore(t, "tp-", seed)
		_, err := buildRenamePlan(c, nil, renameFlags{})
		if err == nil || err.Error() != "provide an item id, or use --prefix" {
			t.Errorf("error = %v, want %q", err, "provide an item id, or use --prefix")
		}
	})

	t.Run("suffix on a foreign prefix", func(t *testing.T) {
		c := renameTestCore(t, "tp-", seed)
		want := `item "zz-bbbb" does not start with the configured prefix "tp-"; cannot apply --suffix`
		_, err := buildRenamePlan(c, []string{"zz-bbbb"}, renameFlags{suffix: "k7x2", suffixSet: true})
		if err == nil || err.Error() != want {
			t.Errorf("error = %v, want %q", err, want)
		}
	})

	t.Run("ref updates line", func(t *testing.T) {
		cmd := &cobra.Command{}
		var out bytes.Buffer
		cmd.SetOut(&out)
		plan := &itemcore.RenamePlan{Mode: "id", RefUpdates: map[string]int{"tp-bbbb": 2, "tp-cccc": 1}}
		if err := renderRenamePlan(cmd, plan, false); err != nil {
			t.Fatal(err)
		}
		if want := "ref updates: 3 across 2 item(s)\n"; !strings.Contains(out.String(), want) {
			t.Errorf("summary = %q, want it to contain %q", out.String(), want)
		}
	})

	runRename := func(t *testing.T, files map[string]string, stdin string, flagKV map[string]string, args ...string) string {
		t.Helper()
		sharedTestRoot(t) // registers renameCmd's flags
		oldCore, oldCfg := core, cfg
		core = renameTestCore(t, "tp-", files)
		cfg = core.Config()
		t.Cleanup(func() {
			core, cfg = oldCore, oldCfg
			resetFlags(renameCmd)
			renameCmd.SetOut(nil)
			renameCmd.SetIn(nil)
		})
		var out bytes.Buffer
		renameCmd.SetOut(&out)
		renameCmd.SetIn(strings.NewReader(stdin))
		for k, v := range flagKV {
			if err := renameCmd.Flags().Set(k, v); err != nil {
				t.Fatalf("setting --%s: %v", k, err)
			}
		}
		if err := renameCmd.RunE(renameCmd, args); err != nil {
			t.Fatalf("rename error = %v", err)
		}
		return out.String()
	}

	t.Run("result line", func(t *testing.T) {
		out := runRename(t, seed, "", map[string]string{"slug": "new-slug"}, "tp-aaaa")
		if want := "Renamed 1 item(s).\n"; !strings.Contains(out, want) {
			t.Errorf("output = %q, want it to contain %q", out, want)
		}
	})

	t.Run("rebrand prompt", func(t *testing.T) {
		out := runRename(t, map[string]string{"tp-aaaa--a.md": seed["tp-aaaa--a.md"]}, "n\n", map[string]string{"prefix": "op-"})
		if want := `Rebrand 1 item(s) to prefix "op-"?`; !strings.Contains(out, want) {
			t.Errorf("output = %q, want it to contain %q", out, want)
		}
	})
}

// beans-gbl5: a prefix dry run reports the plan and every refusal.
func TestBuildRenamePlan_prefixDryRunListsAllRefusals(t *testing.T) {
	item := func(id string) string { return "---\n# " + id + "\ntitle: T\nstatus: todo\ntype: task\n---\n" }
	c := renameTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md": item("tp-aaaa"),
		"tp-dddd--d.md": item("tp-dddd"),
		"zz-bbbb--b.md": item("zz-bbbb"),
		"op-dddd--d.md": item("op-dddd"),
	})

	plan, err := buildRenamePlan(c, nil, renameFlags{prefix: "op-", prefixSet: true, dryRun: true})
	if err != nil {
		t.Fatalf("dry run must return the plan, got error %v", err)
	}
	if len(plan.Refusals) != 3 {
		t.Fatalf("refusals = %q, want 3 (two foreign prefixes, one collision)", plan.Refusals)
	}
	if len(plan.Changes) != 1 || plan.Changes[0].NewID != "op-aaaa" {
		t.Errorf("changes = %+v, want only tp-aaaa -> op-aaaa", plan.Changes)
	}

	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	if err := renderRenamePlan(cmd, plan, false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"tp-aaaa -> op-aaaa", "refusals (3):", `"zz-bbbb"`, `"op-dddd" already exists`} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}

	// The real run keeps failing at once and never applies a refused plan.
	if _, err := buildRenamePlan(c, nil, renameFlags{prefix: "op-", prefixSet: true}); err == nil {
		t.Error("real rename must still fail on a refusal")
	}
	if err := c.ApplyRename(plan); err == nil {
		t.Error("ApplyRename must reject a plan that carries refusals")
	}
}

// beans-gbl5: through the real command, a refused dry run prints the plan and
// the refusal and exits non-zero.
func TestRenamePrefixDryRunExitsNonZeroWithRefusals(t *testing.T) {
	store := filepath.Join(t.TempDir(), ".lif")
	item := func(id string) string { return "---\n# " + id + "\ntitle: T\nstatus: todo\ntype: task\n---\n" }
	writeFixtureFile(t, filepath.Join(store, "aaaa--a.md"), item("aaaa"))
	writeFixtureFile(t, filepath.Join(store, "op-aaaa--b.md"), item("op-aaaa"))

	stdout, _, err := runRootIn(t, t.TempDir(), "--lif-path", store, "rename", "--prefix", "op-", "--dry-run")
	if err == nil {
		t.Fatalf("want non-zero exit, got none; stdout:\n%s", stdout)
	}
	for _, want := range []string{"mode: prefix", "refusals (1):", `"op-aaaa" already exists`} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout lacks %q:\n%s", want, stdout)
		}
	}
}
