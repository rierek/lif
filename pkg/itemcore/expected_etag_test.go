package itemcore

import (
	"errors"
	"testing"

	"github.com/rierek/lif/pkg/item"
)

// WithExpectedETag is compared like an If-Match: a file that changed since the
// etag was taken fails the write, a file that did not change takes it.
func TestWithExpectedETagComparesLikeIfMatch(t *testing.T) {
	core, _ := setupTestCore(t)
	createTestItem(t, core, "test-xe01", "Expected etag", "todo")
	stale, err := core.Get("test-xe01")
	if err != nil {
		t.Fatal(err)
	}

	// Another writer changes the item after the etag was taken.
	other, _ := core.Get("test-xe01")
	other.Body = "other writer"
	if err := core.Update(other, nil); err != nil {
		t.Fatal(err)
	}

	stale.Status = "in-progress"
	err = core.Update(stale, nil, WithExpectedETag(stale.ETag()))
	var mismatch *ETagMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("stale etag: error = %v, want ETagMismatchError", err)
	}

	current, _ := core.Get("test-xe01")
	current.Status = "in-progress"
	if err := core.Update(current, nil, WithExpectedETag(current.ETag())); err != nil {
		t.Fatalf("current etag: %v", err)
	}
}

// WithExpectedETag never stands in for the If-Match that require_if_match
// demands: an implicit check on every write would switch that rule off.
func TestWithExpectedETagDoesNotSatisfyRequireIfMatch(t *testing.T) {
	core, _ := setupTestCoreWithRequireIfMatch(t)
	createTestItem(t, core, "test-xe02", "Expected etag", "todo")
	b, _ := core.Get("test-xe02")
	b.Status = "in-progress"

	err := core.Update(b, nil, WithExpectedETag(b.ETag()))

	var required *ETagRequiredError
	if !errors.As(err, &required) {
		t.Fatalf("error = %v, want ETagRequiredError", err)
	}
}

// An explicit If-Match wins over the expected etag.
func TestWithExpectedETagYieldsToIfMatch(t *testing.T) {
	core, _ := setupTestCore(t)
	createTestItem(t, core, "test-xe03", "Expected etag", "todo")
	b, _ := core.Get("test-xe03")
	b.Status = "in-progress"
	ifMatch := b.ETag()

	if err := core.Update(b, &ifMatch, WithExpectedETag("not-the-etag")); err != nil {
		t.Fatalf("error = %v, want the If-Match to decide", err)
	}
}

// PreviewExtras returns the state Update would write for the extra ops and
// leaves the argument alone.
func TestPreviewExtrasAppliesExtraOpsToAClone(t *testing.T) {
	b := &item.Item{ID: "test-pe01", Status: "todo", Extra: map[string]any{"keep": 1, "drop": 2}}

	got := PreviewExtras(b, WithExtraOps(map[string]any{"set": "v"}, []string{"drop"}))

	if got == b {
		t.Fatal("PreviewExtras returned its argument, want a clone")
	}
	if got.Extra["set"] != "v" || got.Extra["keep"] != 1 {
		t.Errorf("preview Extra = %v, want set and keep", got.Extra)
	}
	if _, has := got.Extra["drop"]; has {
		t.Errorf("preview Extra = %v, want drop removed", got.Extra)
	}
	if _, has := b.Extra["set"]; has || b.Extra["drop"] != 2 {
		t.Errorf("argument Extra = %v, want it untouched", b.Extra)
	}
}

func TestPreviewExtrasWithoutOpsIsAClone(t *testing.T) {
	b := &item.Item{ID: "test-pe02", Status: "todo", Tags: []string{"a"}}

	got := PreviewExtras(b)

	if got == b || got.ID != b.ID || got.Status != b.Status {
		t.Fatalf("PreviewExtras = %+v, want a clone of %+v", got, b)
	}
	got.Tags[0] = "changed"
	if b.Tags[0] != "a" {
		t.Error("the clone shares its tags with the argument")
	}
}
