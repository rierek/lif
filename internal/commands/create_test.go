package commands

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/xRiErOS/lif/pkg/config"
	"github.com/xRiErOS/lif/pkg/item"
	"github.com/xRiErOS/lif/pkg/itemcore"
)

// createCmdWithOrderFlag returns a throwaway *cobra.Command carrying only an
// --order flag bound to the package-level createOrder var. createCmd.RunE
// reads flag state off whatever *cobra.Command it's called with (not off the
// shared createCmd singleton), so tests can drive cmd.Flags().Changed("order")
// without touching createCmd's own FlagSet — which RegisterCreateCmd/
// RegisterCoreCommands populate exactly once elsewhere (see path_test.go);
// registering "order" a second time there would panic with "flag redefined".
func createCmdWithOrderFlag() *cobra.Command {
	c := &cobra.Command{Use: "create"}
	c.Flags().StringVar(&createOrder, "order", "", "Explicit fractional-index order value")
	return c
}

// setupCreateTest installs a throwaway core and default config into the
// package globals createCmd.RunE reads, and restores both afterwards.
func setupCreateTest(t *testing.T) {
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

// resetCreateFlags clears every create* package global the tests below touch
// and restores the pre-test values afterwards, so tests stay isolated despite
// sharing the createCmd package-level flag vars.
func resetCreateFlags(t *testing.T) {
	t.Helper()
	oldType, oldSet, oldUnset, oldOrder, oldJSON := createType, createSet, createUnset, createOrder, createJSON
	createType, createSet, createUnset, createOrder, createJSON = "", nil, nil, "", false
	t.Cleanup(func() {
		createType, createSet, createUnset, createOrder, createJSON = oldType, oldSet, oldUnset, oldOrder, oldJSON
	})
}

// D05/D12: `create --json` returns the bare item document directly, not a
// {success,item,message} envelope -- same shape as `show`/`update`/`tag`.
func TestCreateCmdJSONReturnsBareItem(t *testing.T) {
	setupCreateTest(t)
	resetCreateFlags(t)
	createJSON = true

	out := captureCreateStdout(t, func() {
		if err := createCmd.RunE(createCmd, []string{"A", "new", "bean"}); err != nil {
			t.Fatalf("createCmd.RunE() error = %v", err)
		}
	})

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decoding JSON: %v; output = %s", err, out)
	}
	for _, envelopeKey := range []string{"success", "message", "item"} {
		if _, ok := got[envelopeKey]; ok {
			t.Errorf("captured JSON has envelope key %q; want the raw bean", envelopeKey)
		}
	}
	if _, ok := got["id"]; !ok {
		t.Errorf("captured JSON = %s, want top-level bean fields like id", out)
	}
	if got["title"] != "A new bean" {
		t.Errorf("title = %v, want %q", got["title"], "A new bean")
	}
}

// captureCreateStdout redirects os.Stdout for the duration of fn and returns
// everything written to it.
func captureCreateStdout(t *testing.T, fn func()) []byte {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = orig
	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe write end: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}
	return data
}

// AC1: --order writes the given value to the new item's order field.
func TestCreateCmdOrderWritesValue(t *testing.T) {
	setupCreateTest(t)
	resetCreateFlags(t)

	createType = "task"
	cmd := createCmdWithOrderFlag()
	if err := cmd.Flags().Set("order", "V"); err != nil {
		t.Fatalf("failed to set --order flag: %v", err)
	}

	if err := createCmd.RunE(cmd, []string{"x"}); err != nil {
		t.Fatalf("createCmd.RunE() error = %v", err)
	}

	list := core.All()
	if len(list) != 1 {
		t.Fatalf("expected 1 bean, got %d", len(list))
	}
	if list[0].Order != "V" {
		t.Errorf("Order = %q, want %q", list[0].Order, "V")
	}
}

// AC2: without --order, the new item's order field is left empty.
func TestCreateCmdWithoutOrderLeavesEmpty(t *testing.T) {
	setupCreateTest(t)
	resetCreateFlags(t)

	createType = "task"

	if err := createCmd.RunE(createCmd, []string{"x"}); err != nil {
		t.Fatalf("createCmd.RunE() error = %v", err)
	}

	list := core.All()
	if len(list) != 1 {
		t.Fatalf("expected 1 bean, got %d", len(list))
	}
	if list[0].Order != "" {
		t.Errorf("Order = %q, want empty", list[0].Order)
	}
}

// AC3: an invalid fractional index passed via --order exits non-zero and
// creates no item.
func TestCreateCmdOrderInvalidValueFails(t *testing.T) {
	setupCreateTest(t)
	resetCreateFlags(t)

	createType = "task"
	cmd := createCmdWithOrderFlag()
	if err := cmd.Flags().Set("order", "not valid!"); err != nil {
		t.Fatalf("failed to set --order flag: %v", err)
	}

	err := createCmd.RunE(cmd, []string{"x"})
	if err == nil {
		t.Fatal("expected error for invalid --order value")
	}

	list := core.All()
	if len(list) != 0 {
		t.Errorf("expected no bean to be created, got %d", len(list))
	}
}

// SC-01: five items created with ascending explicit order values come back
// from a sort-by-order in exactly that sequence.
func TestCreateCmdOrderAscendingSortOrder(t *testing.T) {
	setupCreateTest(t)
	resetCreateFlags(t)

	orders := []string{"1", "3", "5", "7", "9"}
	for _, o := range orders {
		createType = "task"
		cmd := createCmdWithOrderFlag()
		if err := cmd.Flags().Set("order", o); err != nil {
			t.Fatalf("failed to set --order flag: %v", err)
		}
		if err := createCmd.RunE(cmd, []string{o}); err != nil {
			t.Fatalf("createCmd.RunE() error = %v", err)
		}
	}

	list := core.All()
	if len(list) != len(orders) {
		t.Fatalf("expected %d beans, got %d", len(orders), len(list))
	}

	item.SortByOrder(list)

	for i, b := range list {
		if b.Order != orders[i] {
			t.Errorf("position %d: Order = %q, want %q", i, b.Order, orders[i])
		}
	}
}

// AC1/AC2/SC-01: --set writes extra front matter keys, and repeats accumulate.
func TestCreateCmdSetWritesExtraKeys(t *testing.T) {
	setupCreateTest(t)
	resetCreateFlags(t)

	createType = "task"
	createSet = []string{"release=0-4-1", "klasse=bugfix"}

	if err := createCmd.RunE(createCmd, []string{"x"}); err != nil {
		t.Fatalf("createCmd.RunE() error = %v", err)
	}

	list := core.All()
	if len(list) != 1 {
		t.Fatalf("expected 1 bean, got %d", len(list))
	}
	b := list[0]
	if b.Extra["release"] != "0-4-1" {
		t.Errorf("Extra[release] = %v, want %q", b.Extra["release"], "0-4-1")
	}
	if b.Extra["klasse"] != "bugfix" {
		t.Errorf("Extra[klasse] = %v, want %q", b.Extra["klasse"], "bugfix")
	}
}

// AC4/SC-01: --set on a reserved key fails naming the native flag, and no
// item is created.
func TestCreateCmdSetReservedKeyFails(t *testing.T) {
	setupCreateTest(t)
	resetCreateFlags(t)

	createType = "task"
	createSet = []string{"status=done"}

	err := createCmd.RunE(createCmd, []string{"x"})
	if err == nil {
		t.Fatal("expected error for reserved key")
	}
	if !contains(err.Error(), "--status") {
		t.Errorf("expected error to name --status, got %q", err.Error())
	}

	list := core.All()
	if len(list) != 0 {
		t.Errorf("expected no bean to be created, got %d", len(list))
	}
}

// AC6: --set without "=" is a usage error.
func TestCreateCmdSetWithoutEqualsFails(t *testing.T) {
	setupCreateTest(t)
	resetCreateFlags(t)

	createType = "task"
	createSet = []string{"release"}

	if err := createCmd.RunE(createCmd, []string{"x"}); err == nil {
		t.Fatal("expected error for --set without '='")
	}
}

// Conflict resolution: a key named by both --set and --unset in the same
// invocation ends up unset (see applyExtraOps).
func TestCreateCmdSetAndUnsetSameKey(t *testing.T) {
	setupCreateTest(t)
	resetCreateFlags(t)

	createType = "task"
	createSet = []string{"release=0-4-1"}
	createUnset = []string{"release"}

	if err := createCmd.RunE(createCmd, []string{"x"}); err != nil {
		t.Fatalf("createCmd.RunE() error = %v", err)
	}

	list := core.All()
	if len(list) != 1 {
		t.Fatalf("expected 1 bean, got %d", len(list))
	}
	if _, ok := list[0].Extra["release"]; ok {
		t.Errorf("expected release to be unset, Extra = %#v", list[0].Extra)
	}
}

// B01 regression: the second write that persists --set/--unset used to pass
// ifMatch=nil to core.Update, which under require_if_match:true makes the
// write fail outright -- leaving the item created on disk WITHOUT the extra
// keys the caller just asked for (a half-written state, not just an error).
// Passing the item's own freshly-computed etag instead satisfies
// require_if_match and lets the write through.
func TestCreateCmdSetSucceedsUnderRequireIfMatch(t *testing.T) {
	tmpDir := t.TempDir()
	itemsDir := tmpDir + "/.beans"
	if err := os.MkdirAll(itemsDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}
	testCfg := config.Default()
	testCfg.Items.RequireIfMatch = true
	testCore := itemcore.New(itemsDir, testCfg)
	if err := testCore.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}
	oldCore, oldCfg := core, cfg
	core, cfg = testCore, testCfg
	t.Cleanup(func() { core, cfg = oldCore, oldCfg })

	resetCreateFlags(t)
	createType = "task"
	createSet = []string{"release=0-4-1"}

	if err := createCmd.RunE(createCmd, []string{"x"}); err != nil {
		t.Fatalf("createCmd.RunE() error = %v (extra-key write should succeed under require_if_match:true, not fail)", err)
	}

	list := core.All()
	if len(list) != 1 {
		t.Fatalf("expected 1 bean, got %d", len(list))
	}
	if list[0].Extra["release"] != "0-4-1" {
		t.Errorf("Extra[release] = %v, want %q (bean must not be left half-written without its extra keys)", list[0].Extra["release"], "0-4-1")
	}
}

// Plain create ignores lif.default_priority and lif.default_tags (only
// `create -i` pre-fills them). Asserts on the written file: the loader fills
// priority "normal" on read, so a loaded item would look as if a default applied.
func TestCreateCmdIgnoresDefaultPriorityAndTags(t *testing.T) {
	setupCreateTest(t)
	resetCreateFlags(t)
	cfg.Items.DefaultPriority = "high"
	cfg.Items.DefaultTags = []string{"backend", "api"}
	oldPriority, oldTag := createPriority, createTag
	createPriority, createTag = "", nil
	t.Cleanup(func() { createPriority, createTag = oldPriority, oldTag })

	if err := createCmd.RunE(createCmd, []string{"x"}); err != nil {
		t.Fatalf("createCmd.RunE() error = %v", err)
	}

	files, err := filepath.Glob(filepath.Join(core.Root(), "*.md"))
	if err != nil || len(files) != 1 {
		t.Fatalf("expected 1 item file, got %v (err %v)", files, err)
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"priority:", "tags:"} {
		if strings.Contains(string(data), "\n"+key) {
			t.Errorf("item file contains %q front matter key:\n%s", key, data)
		}
	}
}
