package itemgraph

import (
	"slices"
	"testing"

	"github.com/rierek/lif/pkg/item"
)

// wantChanged fails unless got lists exactly the fields in want, in order. A
// nil and an empty slice both mean that no watched field changed.
func wantChanged(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("ChangedFields = %v, want %v", got, want)
	}
}

func TestC01_NilPrevCountsAsEmptyItem(t *testing.T) {
	next := &item.Item{Status: "todo", Type: "task", Parent: "m1"}
	wantChanged(t, ChangedFields(nil, next), []string{"status", "type", "parent"})
}

func TestC02_StatusAndTagsChange(t *testing.T) {
	prev := &item.Item{Status: "todo", Type: "task", Tags: []string{"a"}}
	next := &item.Item{Status: "in-progress", Type: "task", Tags: []string{"a", "b"}}
	wantChanged(t, ChangedFields(prev, next), []string{"status", "tags"})
}

func TestC03_TagReorderIsNoChange(t *testing.T) {
	prev := &item.Item{Status: "todo", Tags: []string{"a", "b"}}
	next := &item.Item{Status: "todo", Tags: []string{"b", "a"}}
	wantChanged(t, ChangedFields(prev, next), nil)
}

func TestC04_TitleOnlyIsNoChange(t *testing.T) {
	prev := &item.Item{Title: "Old", Status: "todo"}
	next := &item.Item{Title: "New", Status: "todo"}
	wantChanged(t, ChangedFields(prev, next), nil)
}

func TestC05_ExtraAdded(t *testing.T) {
	prev := &item.Item{Status: "todo"}
	next := &item.Item{Status: "todo", Extra: map[string]any{"target_entry": "x"}}
	wantChanged(t, ChangedFields(prev, next), []string{"target_entry"})
}

func TestC06_ExtraEqualValuesAreNoChange(t *testing.T) {
	// The list and the map are separate values on each side: only a deep
	// comparison sees them as equal, and a plain != on them panics.
	prev := &item.Item{Extra: map[string]any{
		"target_entry": "x",
		"list_key":     []any{"a", "b"},
		"map_key":      map[string]any{"k": 1},
	}}
	next := &item.Item{Extra: map[string]any{
		"target_entry": "x",
		"list_key":     []any{"a", "b"},
		"map_key":      map[string]any{"k": 1},
	}}
	wantChanged(t, ChangedFields(prev, next), nil)
}

func TestC07_ExtraRemoved(t *testing.T) {
	prev := &item.Item{Extra: map[string]any{"target_entry": "x"}}
	next := &item.Item{}
	wantChanged(t, ChangedFields(prev, next), []string{"target_entry"})
}

func TestC08_ExtrasFollowBuiltInsSorted(t *testing.T) {
	// b_key changes in place and a_key is new, so a result in the order the
	// keys are found would put b_key first. The list value also needs a deep
	// comparison.
	prev := &item.Item{Status: "todo", Extra: map[string]any{"b_key": []any{"x"}}}
	next := &item.Item{Status: "in-progress", Extra: map[string]any{"b_key": []any{"x", "y"}, "a_key": 1}}
	// Map iteration order is random: repeat, so an unsorted result shows.
	for range 50 {
		wantChanged(t, ChangedFields(prev, next), []string{"status", "a_key", "b_key"})
	}
}

func TestC09_UnwatchedFieldsAreNoChange(t *testing.T) {
	prev := &item.Item{Status: "todo", Type: "task", Priority: "normal", Order: "a", Body: "old"}
	next := &item.Item{
		Title:     "Other",
		Status:    "todo",
		Type:      "task",
		Priority:  "high",
		Order:     "b",
		Body:      "new",
		Blocking:  []string{"beans-b1"},
		BlockedBy: []string{"beans-b2"},
	}
	wantChanged(t, ChangedFields(prev, next), nil)
}

func TestC10_ExtraKeyWithSpaceIsNotWatched(t *testing.T) {
	prev := &item.Item{Status: "todo"}
	next := &item.Item{Status: "todo", Extra: map[string]any{"my key": "x"}}
	wantChanged(t, ChangedFields(prev, next), nil)
}
