package itemgraph

import (
	"context"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph/model"
)

func strptr(s string) *string { return &s }

// TestCreateItemRejectsUnknownStatus pins the enum check on create: without it
// an arbitrary string is persisted as a status and every consumer that maps a
// status to colour, sort rank or archive behaviour silently falls through.
func TestCreateItemRejectsUnknownStatus(t *testing.T) {
	r := newTestResolver(t)

	_, err := r.CreateItem(context.Background(), model.CreateItemInput{
		Title:  "Bad status",
		Status: strptr("nonsense"),
	})
	if err == nil {
		t.Fatal("CreateBean accepted an unknown status")
	}
	if !strings.Contains(err.Error(), "status") {
		t.Fatalf("error should name the offending field, got %v", err)
	}
}

func TestCreateItemRejectsUnknownType(t *testing.T) {
	r := newTestResolver(t)

	_, err := r.CreateItem(context.Background(), model.CreateItemInput{
		Title: "Bad type",
		Type:  strptr("nonsense"),
	})
	if err == nil {
		t.Fatal("CreateBean accepted an unknown type")
	}
	if !strings.Contains(err.Error(), "type") {
		t.Fatalf("error should name the offending field, got %v", err)
	}
}

func TestCreateItemRejectsUnknownPriority(t *testing.T) {
	r := newTestResolver(t)

	_, err := r.CreateItem(context.Background(), model.CreateItemInput{
		Title:    "Bad priority",
		Priority: strptr("nonsense"),
	})
	if err == nil {
		t.Fatal("CreateBean accepted an unknown priority")
	}
	if !strings.Contains(err.Error(), "priority") {
		t.Fatalf("error should name the offending field, got %v", err)
	}
}

// TestCreateItemAcceptsConfiguredEnumValues guards the other direction: the
// enum check must read the configured tables, not a hand-rolled list, so every
// value the config declares still passes.
func TestCreateItemAcceptsConfiguredEnumValues(t *testing.T) {
	r := newTestResolver(t)
	cfg := r.Core.Config()

	for _, status := range cfg.StatusNames() {
		for _, typeName := range cfg.TypeNames() {
			b, err := r.CreateItem(context.Background(), model.CreateItemInput{
				Title:  "Configured " + status + " " + typeName,
				Status: strptr(status),
				Type:   strptr(typeName),
			})
			if err != nil {
				t.Fatalf("CreateBean(status=%q, type=%q): %v", status, typeName, err)
			}
			if b.Status != status || b.Type != typeName {
				t.Fatalf("got status=%q type=%q, want %q/%q", b.Status, b.Type, status, typeName)
			}
		}
	}
	for _, priority := range cfg.PriorityNames() {
		if _, err := r.CreateItem(context.Background(), model.CreateItemInput{
			Title:    "Configured priority " + priority,
			Priority: strptr(priority),
		}); err != nil {
			t.Fatalf("CreateBean(priority=%q): %v", priority, err)
		}
	}
}

func TestCreateItemRejectsOverlongTitle(t *testing.T) {
	r := newTestResolver(t)

	_, err := r.CreateItem(context.Background(), model.CreateItemInput{
		Title: strings.Repeat("a", MaxTitleLength+1),
	})
	if err == nil {
		t.Fatal("CreateBean accepted a title above the limit")
	}
	if !strings.Contains(err.Error(), "title") {
		t.Fatalf("error should name the offending field, got %v", err)
	}
}

// TestCreateItemAcceptsTitleAtLimit pins the boundary: an off-by-one in the
// comparison would reject a title that is exactly at the documented maximum.
func TestCreateItemAcceptsTitleAtLimit(t *testing.T) {
	r := newTestResolver(t)

	if _, err := r.CreateItem(context.Background(), model.CreateItemInput{
		Title: strings.Repeat("a", MaxTitleLength),
	}); err != nil {
		t.Fatalf("CreateBean rejected a title at the limit: %v", err)
	}
}

func TestCreateItemRejectsEmptyTitle(t *testing.T) {
	r := newTestResolver(t)

	if _, err := r.CreateItem(context.Background(), model.CreateItemInput{Title: "  "}); err == nil {
		t.Fatal("CreateBean accepted a blank title")
	}
}

func TestCreateItemRejectsOversizedBody(t *testing.T) {
	r := newTestResolver(t)

	_, err := r.CreateItem(context.Background(), model.CreateItemInput{
		Title: "Huge body",
		Body:  strptr(strings.Repeat("x", MaxBodyBytes+1)),
	})
	if err == nil {
		t.Fatal("CreateBean accepted a body above the limit")
	}
	if !strings.Contains(err.Error(), "body") {
		t.Fatalf("error should name the offending field, got %v", err)
	}
}

// TestCreateItemRejectsInvalidTag routes the create path through the same
// ValidateTag the item package already enforces on AddTag — before this the
// GraphQL path was the one way to write a tag that no CLI command would accept.
func TestCreateItemRejectsInvalidTag(t *testing.T) {
	r := newTestResolver(t)

	_, err := r.CreateItem(context.Background(), model.CreateItemInput{
		Title: "Bad tag",
		Tags:  []string{"fine", "Not A Tag"},
	})
	if err == nil {
		t.Fatal("CreateBean accepted an invalid tag")
	}
	if !strings.Contains(err.Error(), "tag") {
		t.Fatalf("error should name the offending field, got %v", err)
	}
}

// TestCreateItemRejectsPathTraversalPrefix is the path-traversal guard: the
// prefix goes straight into the generated ID and from there into the item's
// filename, so a prefix carrying a separator escapes the store directory.
// "a--b" is in here for a second reason: ParseFilename splits an ID from its
// slug on the first "--", so a prefix containing one makes the file unreadable
// back into the ID it was written for.
func TestCreateItemRejectsPathTraversalPrefix(t *testing.T) {
	for _, prefix := range []string{"../../etc/", "a/b", `a\b`, "a.b", "1lead", "a b", "-lead", "a--b"} {
		r := newTestResolver(t)
		_, err := r.CreateItem(context.Background(), model.CreateItemInput{
			Title:  "Bad prefix",
			Prefix: strptr(prefix),
		})
		if err == nil {
			t.Fatalf("CreateBean accepted prefix %q", prefix)
		}
		if !strings.Contains(err.Error(), "prefix") {
			t.Fatalf("error for %q should name the offending field, got %v", prefix, err)
		}
	}
}

// TestCreateItemAcceptsHyphenatedPrefix keeps the real-world shapes working:
// stores in the wild use a trailing hyphen ("beans-abc1"), and prefixes mirrored
// from an external tracker are upper case ("SYNC-TASK-"). The guard may only
// exclude separators, not hyphens or case.
func TestCreateItemAcceptsHyphenatedPrefix(t *testing.T) {
	for _, prefix := range []string{"beans-", "b", "my-project-", "proj2-", "SYNC-TASK-"} {
		r := newTestResolver(t)
		b, err := r.CreateItem(context.Background(), model.CreateItemInput{
			Title:  "Good prefix",
			Prefix: strptr(prefix),
		})
		if err != nil {
			t.Fatalf("CreateBean rejected prefix %q: %v", prefix, err)
		}
		if !strings.HasPrefix(b.ID, prefix) {
			t.Fatalf("ID %q does not carry prefix %q", b.ID, prefix)
		}
	}
}

func TestUpdateItemRejectsUnknownStatus(t *testing.T) {
	r := newTestResolver(t)
	b := mustCreate(t, r, "Update target")

	if _, err := r.UpdateItem(context.Background(), b.ID, model.UpdateItemInput{
		Status: strptr("nonsense"),
	}); err == nil {
		t.Fatal("UpdateBean accepted an unknown status")
	}
}

func TestUpdateItemRejectsOverlongTitle(t *testing.T) {
	r := newTestResolver(t)
	b := mustCreate(t, r, "Update target")

	if _, err := r.UpdateItem(context.Background(), b.ID, model.UpdateItemInput{
		Title: strptr(strings.Repeat("a", MaxTitleLength+1)),
	}); err == nil {
		t.Fatal("UpdateBean accepted a title above the limit")
	}
}

func TestUpdateItemRejectsInvalidAddedTag(t *testing.T) {
	r := newTestResolver(t)
	b := mustCreate(t, r, "Update target")

	if _, err := r.UpdateItem(context.Background(), b.ID, model.UpdateItemInput{
		AddTags: []string{"Not A Tag"},
	}); err == nil {
		t.Fatal("UpdateBean accepted an invalid tag")
	}
}

// TestUpdateItemRejectsOversizedBodyMod checks the grown body, not the input:
// bodyMod appends to what is already stored, so a size check that only looks at
// input.Body never sees the result that gets written.
func TestUpdateItemRejectsOversizedBodyMod(t *testing.T) {
	r := newTestResolver(t)
	b := mustCreate(t, r, "Update target")

	if _, err := r.UpdateItem(context.Background(), b.ID, model.UpdateItemInput{
		BodyMod: &model.BodyModification{Append: strptr(strings.Repeat("x", MaxBodyBytes+1))},
	}); err == nil {
		t.Fatal("UpdateBean accepted a bodyMod growing the body past the limit")
	}
}

// TestUpdateItemLeavesUntouchedInvalidFieldsAlone keeps the validation scoped to
// the input. An item written before the guard existed may carry a status this
// build no longer knows; editing its body must not become impossible.
func TestUpdateItemLeavesUntouchedInvalidFieldsAlone(t *testing.T) {
	r := newTestResolver(t)

	legacy := &item.Item{
		ID:     "beans-old1",
		Slug:   item.Slugify("Legacy bean"),
		Title:  "Legacy bean",
		Status: "retired-status",
		Type:   "task",
	}
	if err := r.Core.Create(legacy); err != nil {
		t.Fatalf("core.Create: %v", err)
	}

	got, err := r.UpdateItem(context.Background(), legacy.ID, model.UpdateItemInput{
		Body: strptr("still editable"),
	})
	if err != nil {
		t.Fatalf("UpdateBean on a legacy bean: %v", err)
	}
	if got.Body != "still editable" {
		t.Fatalf("body = %q, want %q", got.Body, "still editable")
	}
}

// TestCreateItemRejectionWritesNothing pins that a rejected create leaves no
// file behind: validation has to run before Core.Create, not after.
func TestCreateItemRejectionWritesNothing(t *testing.T) {
	r := newTestResolver(t)

	before := len(r.Core.All())
	if _, err := r.CreateItem(context.Background(), model.CreateItemInput{
		Title:  "Rejected",
		Status: strptr("nonsense"),
	}); err == nil {
		t.Fatal("CreateBean accepted an unknown status")
	}
	after := len(r.Core.All())
	if after != before {
		t.Fatalf("store grew from %d to %d beans on a rejected create", before, after)
	}
}

func mustCreate(t *testing.T, r *CoreResolver, title string) *item.Item {
	t.Helper()
	b, err := r.CreateItem(context.Background(), model.CreateItemInput{Title: title})
	if err != nil {
		t.Fatalf("CreateBean(%q): %v", title, err)
	}
	return b
}

func TestValidateTitleRejectsControlCharacters(t *testing.T) {
	for _, title := range []string{"nul\x00byte", "two\nlines", "esc\x1b[2J", "del\x7f", "c1\u0085"} {
		if err := ValidateTitle(title); err == nil {
			t.Errorf("ValidateTitle(%q) = nil, want an error", title)
		}
	}
	if err := ValidateTitle("Zeichen: ä ö — 日本"); err != nil {
		t.Errorf("ValidateTitle rejected a printable title: %v", err)
	}
}
