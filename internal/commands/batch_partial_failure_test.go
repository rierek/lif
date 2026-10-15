package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// denyWrite makes the on-disk file for an item unwritable (0o444) so that the
// next write against it (pkg/itemcore/core.go:952, saveToDisk's
// os.WriteFile) fails with a permission error. That failure is the only way
// to reach emitBatchFailure's partial-failure branch (batch.go:94-116): the
// preflight rules out unknown IDs, repeated IDs and policy violations before
// the first write, so nothing short of an actual I/O failure gets there.
//
// A read-only .beans *directory* was measured and rejected for this seam
// (contract beans-bxss, Risks): saveToDisk never needs directory write
// access for a file that already exists, so it does not reproduce the
// failure. Only a file-level chmod does.
//
// denyWrite also verifies its own effectiveness before returning: chmod
// 0o444 is a no-op for root, which would let every assertion below pass
// vacuously against a full success instead of the partial-failure path
// under test. t.Cleanup restores the original mode, and verifies the
// restore happened, so the temp store tears down cleanly.
func denyWrite(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s before chmod: %v", path, err)
	}
	original := info.Mode()

	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatalf("chmod 0o444 %s: %v", path, err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(path, original); err != nil {
			t.Fatalf("restoring mode on %s: %v", path, err)
		}
		got, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s after restore: %v", path, err)
		}
		if got.Mode().Perm() != original.Perm() {
			t.Fatalf("mode on %s = %v after restore, want %v", path, got.Mode().Perm(), original.Perm())
		}
	})

	// Seam self-check: a direct probe open for writing must be denied. If it
	// is not, the chmod above did nothing — most likely because the test is
	// running as root, where file-mode denial does not apply — and this
	// names that cause instead of letting the test continue to an envelope
	// assertion it never earned.
	f, openErr := os.OpenFile(path, os.O_WRONLY, 0)
	if openErr == nil {
		f.Close()
		t.Fatalf("seam ineffective: chmod 0o444 on %s did not block os.OpenFile(O_WRONLY) — euid=%d, most likely running as root", path, os.Geteuid())
	}
}

// TestTagBatchPartialFailure covers emitBatchFailure's write-loop failure
// branch through the tag verb. tag is the cheapest of the four batch verbs
// (start, complete, scrap, tag) that call it: unlike the other three, it
// never calls preflightStatusPolicy, so no --set/required-field scaffolding
// is needed to get past the preflight. emitBatchFailure's own behavior
// depends only on its three parameters (jsonMode, done, err), not on which
// verb called it, so covering it once here covers all four callers of the
// shared helper. delete is a fifth batch-shaped command but does not call
// emitBatchFailure at all — a mid-loop failure there returns a plain
// cmdError and discards done on purpose (delete.go:68-72), a different,
// deliberate path this container does not touch.
func TestTagBatchPartialFailure(t *testing.T) {
	t.Run("json/second target fails, first already written", func(t *testing.T) {
		first := setupTagTest(t)
		resetTagFlags(t)
		second := mkTagItem(t, "beans-tgt2", "Second bean", nil)
		denyWrite(t, filepath.Join(core.Root(), second.Path))

		tagAdd = []string{"seamtest"}
		tagJSON = true

		var runErr error
		out := captureTagStdout(t, func() {
			runErr = tagCmd.RunE(tagCmd, []string{first.ID, second.ID})
		})
		if runErr == nil {
			t.Fatalf("tagCmd.RunE() error = nil, want the second write's permission failure")
		}

		var doc map[string]any
		if err := json.Unmarshal(out, &doc); err != nil {
			t.Fatalf("decoding JSON: %v; output = %s", err, out)
		}

		if success, _ := doc["success"].(bool); success {
			t.Errorf("success = %v, want false", doc["success"])
		}
		items, ok := doc["items"].([]any)
		if !ok || len(items) != 1 {
			t.Fatalf("items = %v, want a one-element array naming %s", doc["items"], first.ID)
		}
		gotID, _ := items[0].(map[string]any)["id"].(string)
		if gotID != first.ID {
			t.Errorf("items[0].id = %q, want %q (the target written before the failure)", gotID, first.ID)
		}
		count, _ := doc["count"].(float64)
		if int(count) != 1 {
			t.Errorf("count = %v, want 1", doc["count"])
		}
		if code, _ := doc["code"].(string); code != "FILE_ERROR" {
			t.Errorf("code = %q, want FILE_ERROR (a permission failure is a store failure, not a refused request)", code)
		}
		if errMsg, _ := doc["error"].(string); errMsg == "" {
			t.Errorf("error = %q, want a non-empty permission-denied message", errMsg)
		}
	})

	t.Run("json/first target fails, nothing written yet", func(t *testing.T) {
		first := setupTagTest(t)
		resetTagFlags(t)
		second := mkTagItem(t, "beans-tgt3", "Second bean", nil)
		denyWrite(t, filepath.Join(core.Root(), first.Path))

		tagAdd = []string{"seamtest"}
		tagJSON = true

		var runErr error
		out := captureTagStdout(t, func() {
			runErr = tagCmd.RunE(tagCmd, []string{first.ID, second.ID})
		})
		if runErr == nil {
			t.Fatalf("tagCmd.RunE() error = nil, want the first write's permission failure")
		}

		var doc map[string]any
		if err := json.Unmarshal(out, &doc); err != nil {
			t.Fatalf("decoding JSON: %v; output = %s", err, out)
		}

		if success, _ := doc["success"].(bool); success {
			t.Errorf("success = %v, want false", doc["success"])
		}
		// output.Response tags Items and Count with `omitempty` (output.go:26-27):
		// an empty slice and a zero int both drop their key entirely, so "nothing
		// written yet" is observable as key absence, not as an empty array or 0.
		if v, ok := doc["items"]; ok {
			t.Errorf(`"items" key = %v, want the key absent (empty done, omitempty)`, v)
		}
		if v, ok := doc["count"]; ok {
			t.Errorf(`"count" key = %v, want the key absent (zero count, omitempty)`, v)
		}
		if code, _ := doc["code"].(string); code != "FILE_ERROR" {
			t.Errorf("code = %q, want FILE_ERROR", code)
		}
		if errMsg, _ := doc["error"].(string); errMsg == "" {
			t.Errorf("error = %q, want a non-empty permission-denied message", errMsg)
		}
	})

	t.Run("plain/second target fails, first already written", func(t *testing.T) {
		first := setupTagTest(t)
		resetTagFlags(t)
		second := mkTagItem(t, "beans-tgt5", "Second bean", nil)
		denyWrite(t, filepath.Join(core.Root(), second.Path))

		tagAdd = []string{"seamtest"}
		tagJSON = false

		var runErr error
		out := captureTagStdout(t, func() {
			runErr = tagCmd.RunE(tagCmd, []string{first.ID, second.ID})
		})

		if runErr == nil {
			t.Fatalf("tagCmd.RunE() error = nil, want the second write's permission failure")
		}
		if !strings.Contains(runErr.Error(), first.ID) {
			t.Errorf("error = %q, want it to name the already-written id %q", runErr.Error(), first.ID)
		}
		if len(out) != 0 {
			t.Errorf("stdout = %q, want empty — the non-json path must not print a JSON document", out)
		}
	})
}

// runRootInDir mirrors runRootWithArgs (error_shape_test.go) for a scenario
// that needs the same on-disk store across more than one command: a setup
// step writes state through the package's core/cfg globals (setupTagTest,
// mkTagItem), then the command under test is reached through the real
// ExecuteC + reportExecutionError path so its stderr is observable.
// Duplicated here rather than folded into runRootWithArgs itself, because a
// parallel container (beans-iw5j) writes promote.go/promote_test.go/
// register.go in this same package and may depend on error_shape_test.go's
// helpers staying exactly as they are — not because it edits that file.
//
// stdout is captured via os.Pipe, not cobra's SetOut: output.JSON (and
// PartialFailure through it) writes straight to os.Stdout rather than
// through the command's own out stream, the same reason tag_test.go's
// captureTagStdout exists instead of reading cmd.OutOrStdout().
func runRootInDir(t *testing.T, itemsDir string, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	root := sharedTestRoot(t)
	resetFlags(root)

	var errBuf bytes.Buffer
	root.SetErr(&errBuf)
	root.SetArgs(append([]string{"--lif-path", itemsDir}, args...))

	out := captureTagStdout(t, func() {
		var cmd *cobra.Command
		cmd, err = root.ExecuteC()
		if err != nil {
			reportExecutionError(cmd, err)
		}
	})
	return string(out), errBuf.String(), err
}

// TestTagBatchPartialFailureJSONSuppressesStderr pins D06 on the batch
// partial-failure path: the json document PartialFailure already wrote to
// stdout must be the ONLY artifact, so reportExecutionError must not also
// print a plain-text copy to stderr. TestTagBatchPartialFailure's json
// subtests drive tagCmd.RunE directly and so cannot observe stderr at all —
// that only happens through the real ExecuteC path, hence runRootInDir
// instead of captureTagStdout.
//
// This test asserts stdout's shape and stderr's emptiness independently, on
// purpose: it must go red only when stderr regains its stray line (the
// _ = output.PartialFailure(...) regression), and stay green if PartialFailure
// itself regresses to returning the encoder's own result — that second
// regression already turns TestTagBatchPartialFailure's json subtests red
// (their runErr-must-be-non-nil assertion), and a test that also asserted
// runErr != nil here would double-count that failure instead of leaving this
// test's own target isolated.
func TestTagBatchPartialFailureJSONSuppressesStderr(t *testing.T) {
	first := setupTagTest(t)
	resetTagFlags(t)
	itemsDir := core.Root()
	second := mkTagItem(t, "beans-tgt9", "Second bean", nil)
	denyWrite(t, filepath.Join(itemsDir, second.Path))

	stdout, stderr, _ := runRootInDir(t, itemsDir, "tag", "--json", "--tag", "seamtest", first.ID, second.ID)

	if stderr != "" {
		t.Errorf("stderr = %q, want empty — the json document on stdout already reported the failure", stderr)
	}

	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("decoding stdout as JSON: %v; stdout = %s", err, stdout)
	}
	if success, _ := doc["success"].(bool); success {
		t.Errorf("success = %v, want false", success)
	}
	items, ok := doc["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("items = %v, want a one-element array naming %s", doc["items"], first.ID)
	}
	gotID, _ := items[0].(map[string]any)["id"].(string)
	if gotID != first.ID {
		t.Errorf("items[0].id = %q, want %q (the target written before the failure)", gotID, first.ID)
	}
}

// TestTagBatchPartialFailurePlainKeepsStderr is the counterpart to
// TestTagBatchPartialFailureJSONSuppressesStderr: the plain-text branch
// never wrote a document, so it is the one path whose stderr line must
// survive. Without this, a fix that silenced stderr on both branches instead
// of only the json one would pass every other test in this file — the json
// subtests above assert stdout, and TestTagBatchPartialFailure's plain
// subtest calls tagCmd.RunE directly, which never reaches stderr at all.
func TestTagBatchPartialFailurePlainKeepsStderr(t *testing.T) {
	first := setupTagTest(t)
	resetTagFlags(t)
	itemsDir := core.Root()
	second := mkTagItem(t, "beans-tgt11", "Second bean", nil)
	denyWrite(t, filepath.Join(itemsDir, second.Path))

	stdout, stderr, err := runRootInDir(t, itemsDir, "tag", "--tag", "seamtest", first.ID, second.ID)

	if err == nil {
		t.Fatal("root.ExecuteC() error = nil, want the second write's permission failure")
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty — the plain path never writes a document", stdout)
	}
	if stderr == "" {
		t.Fatal("stderr is empty, want the permission failure reported (naming the already-written id)")
	}
	if !strings.Contains(stderr, first.ID) {
		t.Errorf("stderr = %q, want it to name the already-written id %q", stderr, first.ID)
	}
}
