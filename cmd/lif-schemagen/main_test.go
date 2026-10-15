package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/version"
	"github.com/rierek/lif/pkg/item"
)

// committed is the directory of the published schema files, relative to the
// repository root.
const committed = "schemas"

// The committed files must equal what the generator renders from the Go types
// now. A changed --json shape therefore fails here until `just schemas` has
// been run and its result committed.
func TestCommittedSchemasAreCurrent(t *testing.T) {
	t.Chdir("../..")
	want, err := schemas()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(committed)
	if err != nil {
		t.Fatalf("reading %s: %v (run `just schemas`)", committed, err)
	}
	have := map[string]bool{}
	for _, e := range entries {
		have[e.Name()] = true
		if _, ok := want[e.Name()]; !ok {
			t.Errorf("%s/%s has no output type any more: delete it (run `just schemas`)", committed, e.Name())
		}
	}
	for name, data := range want {
		got, err := os.ReadFile(filepath.Join(committed, name))
		if err != nil {
			t.Errorf("%s/%s is missing: run `just schemas`", committed, name)
			continue
		}
		if string(got) != string(data) {
			t.Errorf("%s/%s is stale: run `just schemas` and commit the result", committed, name)
		}
	}
}

// compile loads one committed schema file.
func compile(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	path := filepath.Join("..", "..", committed, name+".schema.json")
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(name+".schema.json", doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(name + ".schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// validate checks the real JSON encoding of v against the named schema. This
// is what keeps the schema honest where reflection cannot see the encoding,
// e.g. the computed etag Item.MarshalJSON adds.
func validate(t *testing.T, name string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	if err := compile(t, name).Validate(inst); err != nil {
		t.Errorf("%s does not match %s schema: %v", data, name, err)
	}
}

func TestEncodedOutputMatchesSchema(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	it := &item.Item{
		ID: "abcd", Slug: "x", Path: "abcd--x.md", Title: "X", Status: "todo", Type: "task",
		Priority: "normal", Tags: []string{"a"}, CreatedAt: &now, UpdatedAt: &now,
		Body: "body", Parent: "wxyz", Blocking: []string{"b"}, BlockedBy: []string{"c"},
		Extra: map[string]any{"release": "0.15"},
	}
	validate(t, "item", it)
	validate(t, "items", []*item.Item{it, it})
	validate(t, "items", []*item.Item(nil)) // list with no match prints null
	validate(t, "version", version.JSON())
	validate(t, "response", output.Response{Success: true, Message: "no ready items found"})
	validate(t, "response", output.Response{Success: false, Error: "item not found: nope", Code: "NOT_FOUND"})
	validate(t, "response", output.Response{Success: false, Items: []*item.Item{it}, Code: "CONFLICT", Error: "etag mismatch"})
}

// A schema for an item must reject an item without the computed etag: the
// reflected struct alone would not require it.
func TestItemSchemaRequiresETag(t *testing.T) {
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(`{"id":"a","path":"a.md","title":"t","status":"todo"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := compile(t, "item").Validate(inst); err == nil {
		t.Error("item without etag validates, want it rejected")
	}
}
