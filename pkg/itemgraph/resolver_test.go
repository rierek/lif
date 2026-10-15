package itemgraph

import (
	"errors"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
)

// TestValidateETagReturnsCoreErrorTypes pins the ETag error types to the domain
// layer. itemgraph used to declare its own copies, so a caller matching on
// itemcore's types silently missed the resolver's errors (and vice versa).
func TestValidateETagReturnsCoreErrorTypes(t *testing.T) {
	r := newTestResolver(t)

	b := &item.Item{
		ID:     "beans-etag1",
		Slug:   item.Slugify("Etag types"),
		Title:  "Etag types",
		Status: "todo",
		Type:   "task",
	}
	if err := r.Core.Create(b); err != nil {
		t.Fatalf("core.Create: %v", err)
	}

	t.Run("mismatch", func(t *testing.T) {
		wrong := "not-the-current-etag"
		err := r.validateETag(b, &wrong)

		var mismatch *itemcore.ETagMismatchError
		if !errors.As(err, &mismatch) {
			t.Fatalf("validateETag() = %T (%v), want *itemcore.ETagMismatchError", err, err)
		}
		if mismatch.Provided != wrong {
			t.Errorf("Provided = %q, want %q", mismatch.Provided, wrong)
		}
		if mismatch.Current != b.ETag() {
			t.Errorf("Current = %q, want %q", mismatch.Current, b.ETag())
		}
	})

	t.Run("required", func(t *testing.T) {
		cfg := config.Default()
		cfg.Items.RequireIfMatch = true
		strict := &CoreResolver{Core: itemcore.New(t.TempDir(), cfg)}

		err := strict.validateETag(b, nil)

		var required *itemcore.ETagRequiredError
		if !errors.As(err, &required) {
			t.Fatalf("validateETag() = %T (%v), want *itemcore.ETagRequiredError", err, err)
		}
	})

	t.Run("match", func(t *testing.T) {
		current := b.ETag()
		if err := r.validateETag(b, &current); err != nil {
			t.Fatalf("validateETag() with current etag = %v, want nil", err)
		}
	})
}
