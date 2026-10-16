package itemgraph

import (
	"maps"
	"reflect"
	"slices"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
)

// Mutation describes an item change that hooks react to.
type Mutation struct {
	Event   string     // "created" or "updated"
	Item    *item.Item // state after the change (clone); for a gate: the state about to be written
	Prev    *item.Item // state before (clone); nil for "created"
	Changed []string   // watched fields that differ: built-ins in HookBuiltinWatched order, then extra keys sorted
}

// ChangedFields compares the watched fields of prev and next; tags as a set,
// extra values with reflect.DeepEqual. A nil prev counts as an empty item.
// Extra keys that fail item.IsWatchableKey are skipped.
func ChangedFields(prev, next *item.Item) []string {
	if prev == nil {
		prev = &item.Item{}
	}
	var changed []string
	for _, field := range config.HookBuiltinWatched {
		if builtinDiffers(field, prev, next) {
			changed = append(changed, field)
		}
	}
	return append(changed, changedExtras(prev.Extra, next.Extra)...)
}

func builtinDiffers(field string, prev, next *item.Item) bool {
	switch field {
	case "status":
		return prev.Status != next.Status
	case "type":
		return prev.Type != next.Type
	case "parent":
		return prev.Parent != next.Parent
	case "tags":
		// The list compare keeps the common case, tags untouched, free of allocation.
		return !slices.Equal(prev.Tags, next.Tags) && !maps.Equal(tagSet(prev.Tags), tagSet(next.Tags))
	}
	return false
}

func tagSet(tags []string) map[string]struct{} {
	set := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		set[tag] = struct{}{}
	}
	return set
}

// changedExtras returns the watchable keys, sorted, that only one of the two
// maps holds or whose values differ.
func changedExtras(prev, next map[string]any) []string {
	var keys []string
	for key, pv := range prev {
		if nv, ok := next[key]; (!ok || !reflect.DeepEqual(pv, nv)) && item.IsWatchableKey(key) {
			keys = append(keys, key)
		}
	}
	for key := range next {
		if _, ok := prev[key]; !ok && item.IsWatchableKey(key) {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

// after hands a written change to the after hooks; without AfterMutation it
// does nothing. A hook that ran is remembered: its task may have written
// items behind the back of this process (see beforeUpdate).
func (r *CoreResolver) after(m Mutation) {
	if r.AfterMutation != nil && r.AfterMutation(m) {
		r.hooksRan.Store(true)
	}
}

// afterUpdate fires the after hooks for the update that took prev to b and
// that Core.Update has written. It fires only when a watched field changed.
func (r *CoreResolver) afterUpdate(prev, b *item.Item) {
	if ch := ChangedFields(prev, b); len(ch) > 0 {
		r.after(Mutation{Event: "updated", Item: b.Clone(), Prev: prev, Changed: ch})
	}
}

// beforeUpdate is the gate point of an update that takes prev to b: all input
// is applied to b, nothing is written yet. It returns the options for the
// Core.Update call of the caller, or the error that stops the write.
//
// When a hook task ran through this resolver and the caller holds no If-Match,
// the write also expects the etag that prev was read with. A task that wrote
// the item meanwhile then makes the write fail with an etag mismatch instead
// of being overwritten. Only a hook that ran triggers this, never the mere
// presence of BeforeMutation: an implicit If-Match on every write would
// satisfy require_if_match and break the batch contract of last write wins.
func (r *CoreResolver) beforeUpdate(prev, b *item.Item, ifMatch *string, opts []itemcore.UpdateOption) ([]itemcore.UpdateOption, error) {
	if err := r.gate(prev, b, ifMatch, opts); err != nil {
		return nil, err
	}
	if (ifMatch == nil || *ifMatch == "") && r.hooksRan.Load() {
		// Clip: appending must not write into the backing array of the caller's opts.
		opts = append(slices.Clip(opts), itemcore.WithExpectedETag(prev.ETag()))
	}
	return opts, nil
}

// gate runs BeforeMutation for the update of prev to b when a watched field
// changes. The checks that Core.Update makes anyway come first: a gate must
// not run for a write that would fail with ETagRequiredError or
// PolicyViolationError.
func (r *CoreResolver) gate(prev, b *item.Item, ifMatch *string, opts []itemcore.UpdateOption) error {
	if r.BeforeMutation == nil {
		return nil
	}
	// The extra fields of this call are written inside Core.Update only: the
	// gate would not see them in b.
	next := itemcore.PreviewExtras(b, opts...)
	ch := ChangedFields(prev, next)
	if len(ch) == 0 {
		return nil
	}
	if err := r.requireETag(ifMatch); err != nil {
		return err
	}
	if slices.Contains(ch, "status") {
		if missing := itemcore.MissingRequiredFields(next, r.Core.Config().RequiredFieldsFor(next.Status)); len(missing) > 0 {
			return &itemcore.PolicyViolationError{ItemID: b.ID, Status: next.Status, Missing: missing}
		}
	}
	ran, err := r.BeforeMutation(Mutation{Event: "updated", Item: next, Prev: prev, Changed: ch})
	if ran {
		r.hooksRan.Store(true)
	}
	return err
}
