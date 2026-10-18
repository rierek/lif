package hooks

import (
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph"
)

// created is the creation of next; the before state is empty (Prev nil).
func created(next *item.Item, changed ...string) itemgraph.Mutation {
	return itemgraph.Mutation{Event: "created", Item: next, Changed: changed}
}

func wantMatch(t *testing.T, h config.Hook, m itemgraph.Mutation, want bool) {
	t.Helper()
	if got := match(h, m); got != want {
		t.Errorf("match = %v, want %v", got, want)
	}
}

func TestM01_EventMismatchIsNoMatch(t *testing.T) {
	h := config.Hook{Event: []string{"created"}}
	wantMatch(t, h, upd(it("todo"), it("in-progress"), "status"), false)
}

func TestM02_ChangedMatchesOnAnyListedField(t *testing.T) {
	h := config.Hook{Event: []string{"updated"}, Changed: []string{"status", "tags"}}
	wantMatch(t, h, upd(it("todo"), it("todo", "x"), "tags"), true)
}

func TestM03_ChangedWithoutAListedFieldIsNoMatch(t *testing.T) {
	h := config.Hook{Event: []string{"updated"}, Changed: []string{"status"}}
	next := it("todo")
	next.Parent = "m1"
	wantMatch(t, h, upd(it("todo"), next, "parent"), false)
}

func TestM04_FromAndToFilterTheStatuses(t *testing.T) {
	h := config.Hook{Event: []string{"updated"}, From: []string{"todo"}, To: []string{"in-progress"}}
	wantMatch(t, h, upd(it("todo"), it("in-progress"), "status"), true)
	wantMatch(t, h, upd(it("draft"), it("in-progress"), "status"), false)
}

func TestM05_ToAlsoMatchesAnUnrelatedChangeOfACompletedItem(t *testing.T) {
	h := config.Hook{Event: []string{"updated"}, To: []string{"completed"}}
	wantMatch(t, h, upd(it("completed"), it("completed", "x"), "tags"), true)
}

func TestM06_TypeMismatchIsNoMatch(t *testing.T) {
	h := config.Hook{Event: []string{"updated"}, Type: []string{"feature"}}
	wantMatch(t, h, upd(it("todo"), it("todo", "a"), "tags"), false) // it() is a task
}

func TestM07_TagsMatchWhenTheItemCarriesOneOfThem(t *testing.T) {
	h := config.Hook{Event: []string{"updated"}, Tags: []string{"x", "y"}}
	wantMatch(t, h, upd(it("todo"), it("todo", "y", "z"), "tags"), true)
	wantMatch(t, h, upd(it("todo"), it("todo", "z"), "tags"), false) // control: none of x, y
}

func TestM08_TagsAddedNeedsATagThatWasNotThereBefore(t *testing.T) {
	h := config.Hook{Event: []string{"updated"}, TagsAdded: []string{"accepted"}}
	wantMatch(t, h, upd(it("todo", "a"), it("todo", "a", "accepted"), "tags"), true)
	wantMatch(t, h, upd(it("todo", "accepted"), it("todo", "accepted", "b"), "tags"), false)
}

func TestM09_CreatedCountsEveryTagAsAdded(t *testing.T) {
	h := config.Hook{Event: []string{"created"}, TagsAdded: []string{"accepted"}}
	wantMatch(t, h, created(it("todo", "accepted"), "status", "type", "tags"), true)
}

func TestM10_TagsRemovedNeedsATagThatIsGoneNow(t *testing.T) {
	h := config.Hook{Event: []string{"created", "updated"}, TagsRemoved: []string{"blocked"}}
	wantMatch(t, h, created(it("todo", "blocked"), "status", "type", "tags"), false)
	wantMatch(t, h, upd(it("todo", "blocked"), it("todo"), "tags"), true)
}

func TestM11_ChangedMatchesAnExtraKey(t *testing.T) {
	h := config.Hook{Event: []string{"updated"}, Changed: []string{"target_entry"}}
	wantMatch(t, h, upd(it("todo"), it("todo"), "target_entry"), true)
}

func TestM12_CreatedAndUpdatedBothEnterAStatus(t *testing.T) {
	h := config.Hook{Event: []string{"created", "updated"}, Changed: []string{"status"}, To: []string{"in-progress"}}
	wantMatch(t, h, created(it("in-progress"), "status", "type"), true)
	wantMatch(t, h, upd(it("todo"), it("in-progress"), "status"), true)
}

// GATES-01: a gate with type: or tags: still holds when the change takes the
// item out of that type or tag; an after hook tests the after state only.
func TestGateTypeAndTagsMatchTheBeforeStateToo(t *testing.T) {
	feature := func(status string, tags ...string) *item.Item {
		i := it(status, tags...)
		i.Type = "feature"
		return i
	}
	gate := config.Hook{Stage: "before", Event: []string{"updated"}, Type: []string{"feature"}}
	wantMatch(t, gate, upd(feature("todo"), it("completed"), "type", "status"), true)
	wantMatch(t, gate, upd(it("todo"), feature("todo"), "type"), true)
	wantMatch(t, gate, upd(it("todo"), it("completed"), "status"), false) // control: a task all along
	after := gate
	after.Stage = ""
	wantMatch(t, after, upd(feature("todo"), it("completed"), "type", "status"), false)

	tagged := config.Hook{Stage: "before", Event: []string{"updated"}, Tags: []string{"locked"}}
	wantMatch(t, tagged, upd(it("completed", "locked"), it("todo"), "tags", "status"), true)
	wantMatch(t, tagged, upd(it("completed"), it("todo"), "status"), false)
}
