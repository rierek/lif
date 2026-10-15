package commands

import (
	"encoding/json"
	"testing"
)

// jsonRun runs one lif command with --json over the store of the current test
// and returns stdout.
func jsonRun(t *testing.T, args ...string) []byte {
	t.Helper()
	stdout, _, _ := runRootInDir(t, core.Root(), append(args, "--json")...)
	return []byte(stdout)
}

// jsonItemID creates an item through the CLI and returns its ID.
func jsonItemID(t *testing.T, args ...string) string {
	t.Helper()
	var got struct {
		ID string `json:"id"`
	}
	out := jsonRun(t, append([]string{"create"}, args...)...)
	if err := json.Unmarshal(out, &got); err != nil || got.ID == "" {
		t.Fatalf("create %v: no item in output %q (%v)", args, out, err)
	}
	return got.ID
}

// ju1q: a request the hierarchy or the link graph refuses is a validation
// failure, not an I/O failure, on create and on update.
func TestJSONRefusalsAreValidationErrors(t *testing.T) {
	setupListTest(t)
	t.Cleanup(func() { resetFlags(sharedTestRoot(t)) })

	task := jsonItemID(t, "A task", "--type", "task")
	other := jsonItemID(t, "Other task", "--type", "task")
	blocked := jsonItemID(t, "Blocked task", "--type", "task", "--blocked-by", other)

	cases := []struct {
		name string
		args []string
	}{
		{"create refused parent", []string{"create", "An epic", "--type", "epic", "--parent", task}},
		{"create unknown parent", []string{"create", "Child", "--parent", "nope"}},
		{"create unknown blocker", []string{"create", "Child", "--blocked-by", "nope"}},
		{"create cycle blocks and blocked by", []string{"create", "Loop", "--blocking", task, "--blocked-by", task}},
		{"create cycle through the graph", []string{"create", "Loop", "--blocking", other, "--blocked-by", blocked}},
		{"update cycle", []string{"update", other, "--blocked-by", blocked}},
		{"update refused parent", []string{"update", task, "--type", "epic", "--parent", other}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got struct {
				Success bool   `json:"success"`
				Code    string `json:"code"`
				Error   string `json:"error"`
			}
			out := jsonRun(t, tc.args...)
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("output %q is not JSON: %v", out, err)
			}
			if got.Success || got.Code != "VALIDATION_ERROR" {
				t.Errorf("got success=%v code=%q error=%q, want VALIDATION_ERROR", got.Success, got.Code, got.Error)
			}
		})
	}
}
