package itemgraph

import (
	"context"
	"testing"

	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph/model"
)

// AC3: the relationship errors name the noun as item. Each case drives the
// production resolver and compares the returned text.
func TestRelationshipErrorsNameItems(t *testing.T) {
	ctx := context.Background()
	r := newTestResolver(t)
	one := &item.Item{ID: "t-1", Slug: "one", Title: "One", Status: "todo", Type: "task"}
	if err := r.Core.Create(one); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		call func() error
		want string
	}{
		{"create: unknown blocking target", func() error {
			_, err := r.CreateItem(ctx, model.CreateItemInput{Title: "x", Blocking: []string{"nope"}})
			return err
		}, "target item not found: nope"},
		{"create: unknown blocker", func() error {
			_, err := r.CreateItem(ctx, model.CreateItemInput{Title: "x", BlockedBy: []string{"nope"}})
			return err
		}, "blocker item not found: nope"},
		{"create: blocks and is blocked by the same item", func() error {
			_, err := r.CreateItem(ctx, model.CreateItemInput{Title: "x", Blocking: []string{"t-1"}, BlockedBy: []string{"t-1"}})
			return err
		}, "would create cycle: new item both blocks and is blocked by t-1"},
		{"add blocking: self", func() error {
			_, err := r.AddBlocking(ctx, "t-1", "t-1", nil)
			return err
		}, "item cannot block itself"},
		{"add blocking: unknown target", func() error {
			_, err := r.AddBlocking(ctx, "t-1", "nope", nil)
			return err
		}, "target item not found: nope"},
		{"add blocked-by: self", func() error {
			_, err := r.AddBlockedBy(ctx, "t-1", "t-1", nil)
			return err
		}, "item cannot be blocked by itself"},
		{"add blocked-by: unknown blocker", func() error {
			_, err := r.AddBlockedBy(ctx, "t-1", "nope", nil)
			return err
		}, "blocker item not found: nope"},
		{"validate blocking: self", func() error {
			return r.ValidateAndAddBlocking(one, []string{"t-1"})
		}, "item cannot block itself"},
		{"validate blocking: unknown target", func() error {
			return r.ValidateAndAddBlocking(one, []string{"nope"})
		}, "blocking target item not found: nope"},
		{"validate blocked-by: self", func() error {
			return r.ValidateAndAddBlockedBy(one, []string{"t-1"})
		}, "item cannot be blocked by itself"},
		{"validate blocked-by: unknown blocker", func() error {
			return r.ValidateAndAddBlockedBy(one, []string{"nope"})
		}, "blocker item not found: nope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil || err.Error() != tc.want {
				t.Errorf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
