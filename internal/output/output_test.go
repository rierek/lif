package output

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/rierek/lif/pkg/item"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it. SuccessSingle/SuccessMultiple encode directly
// to os.Stdout, so this is the only way to observe their output.
func captureStdout(t *testing.T, fn func()) []byte {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	orig := os.Stdout
	os.Stdout = w
	fn()
	os.Stdout = orig
	w.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading captured stdout: %v", err)
	}
	return data
}

func TestSuccessSingleAndMultipleAgreeOnExtraField(t *testing.T) {
	// list --json uses SuccessMultiple, show --json uses SuccessSingle.
	// Both must expose extra front matter keys under the same field name.
	withExtra := &item.Item{
		ID:     "test-1",
		Title:  "With Extra",
		Status: "todo",
		Extra:  map[string]any{"release": "0-4-1"},
	}
	withoutExtra := &item.Item{
		ID:     "test-2",
		Title:  "Without Extra",
		Status: "todo",
	}

	singleOut := captureStdout(t, func() {
		if err := SuccessSingle(withExtra); err != nil {
			t.Fatalf("SuccessSingle: %v", err)
		}
	})
	multipleOut := captureStdout(t, func() {
		if err := SuccessMultiple([]*item.Item{withExtra, withoutExtra}); err != nil {
			t.Fatalf("SuccessMultiple: %v", err)
		}
	})

	var single map[string]interface{}
	if err := json.Unmarshal(singleOut, &single); err != nil {
		t.Fatalf("unmarshal SuccessSingle output: %v", err)
	}
	singleExtra, ok := single["extra"].(map[string]interface{})
	if !ok || singleExtra["release"] != "0-4-1" {
		t.Errorf("SuccessSingle output missing extra.release, got: %s", singleOut)
	}

	var multiple []map[string]interface{}
	if err := json.Unmarshal(multipleOut, &multiple); err != nil {
		t.Fatalf("unmarshal SuccessMultiple output: %v", err)
	}
	if len(multiple) != 2 {
		t.Fatalf("len(multiple) = %d, want 2", len(multiple))
	}
	multiExtra, ok := multiple[0]["extra"].(map[string]interface{})
	if !ok || multiExtra["release"] != "0-4-1" {
		t.Errorf("SuccessMultiple output missing extra.release for first bean, got: %s", multipleOut)
	}
	if _, ok := multiple[1]["extra"]; ok {
		t.Errorf("SuccessMultiple output should omit extra for bean without extra keys, got: %s", multipleOut)
	}
}

// The envelope names its payload keys with the item vocabulary: `item` for a
// single item, `items` for a list. The old `bean`/`beans` keys must not appear.
func TestJSONEnvelopeUsesItemKeys(t *testing.T) {
	one := &item.Item{ID: "aaa1", Title: "One"}
	out := captureStdout(t, func() {
		if err := JSON(Response{Success: true, Item: one, Items: []*item.Item{one}}); err != nil {
			t.Fatalf("JSON() error = %v", err)
		}
	})

	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("decoding JSON: %v; output = %s", err, out)
	}
	itemObj, _ := doc["item"].(map[string]any)
	if got, _ := itemObj["id"].(string); got != "aaa1" {
		t.Errorf(`doc["item"].id = %q, want "aaa1"; output = %s`, got, out)
	}
	if arr, _ := doc["items"].([]any); len(arr) != 1 {
		t.Errorf(`doc["items"] = %v, want a one-element array; output = %s`, doc["items"], out)
	}
	for _, old := range []string{"bean", "beans"} {
		if _, ok := doc[old]; ok {
			t.Errorf("envelope still carries legacy key %q; output = %s", old, out)
		}
	}
}
