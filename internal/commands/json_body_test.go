package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// 0g6b: show, create and list return the same body bytes.
func TestJSONBodyIdenticalAcrossCommands(t *testing.T) {
	setupListTest(t)
	t.Cleanup(func() { resetFlags(sharedTestRoot(t)) })

	var created struct {
		ID   string `json:"id"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal(jsonRun(t, "create", "With body", "--body", "Hello"), &created); err != nil {
		t.Fatal(err)
	}
	var shown struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal(jsonRun(t, "show", created.ID), &shown); err != nil {
		t.Fatal(err)
	}
	var listed []struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal(jsonRun(t, "list", "--full"), &listed); err != nil || len(listed) != 1 {
		t.Fatalf("list output: %v", err)
	}
	for name, body := range map[string]string{"create": created.Body, "show": shown.Body, "list": listed[0].Body} {
		if body != "Hello" {
			t.Errorf("%s body = %q, want %q", name, body, "Hello")
		}
	}
	if strings.HasPrefix(shown.Body, "\n") {
		t.Errorf("show body has leading newline")
	}
}
