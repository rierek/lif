package commands

import (
	"context"
	"fmt"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph"
	"github.com/spf13/cobra"
)

var (
	orderAfter  string
	orderBefore string
	orderFirst  bool
	orderLast   bool
	orderJSON   bool
)

var orderCmd = &cobra.Command{
	Use:   "order <id>",
	Short: "Place an item relative to its siblings",
	Long: `Sets an item's manual order key relative to its siblings under the same
parent, using a fractional index. This means a move only ever writes the one
item file being placed, never the neighbours around it.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		resolver := newResolver()

		if !orderFirst && !orderLast && orderAfter == "" && orderBefore == "" {
			return cmdError(orderJSON, output.ErrValidation,
				"one of --after, --before, --first, or --last is required")
		}

		b, err := resolver.Item(ctx, args[0])
		if err != nil {
			return cmdError(orderJSON, output.ErrNotFound, "failed to find item: %v", err)
		}
		if b == nil {
			return cmdError(orderJSON, output.ErrNotFound, "item not found: %s", args[0])
		}

		siblings, err := orderSiblingsOf(resolver, ctx, b)
		if err != nil {
			return cmdError(orderJSON, output.ErrValidation, "%s", err)
		}

		newOrder, err := computeOrderPlacement(resolver, ctx, b, siblings)
		if err != nil {
			return cmdError(orderJSON, output.ErrValidation, "%s", err)
		}

		// Pass the item's own etag as ifMatch instead of nil: nil bypasses
		// optimistic concurrency control entirely -- silently ignored under a
		// normal config, and under require_if_match:true it makes the write
		// fail outright. b was loaded via resolver.Item/Core.Load, so its
		// ETag() is the etag of the bytes actually on disk (cached at parse
		// time), not a re-render of the in-memory struct -- see item.ETag.
		etag := b.ETag()
		b.Order = newOrder
		if err := core.Update(b, &etag); err != nil {
			return mutationError(orderJSON, err)
		}

		if orderJSON {
			return output.SuccessSingle(b)
		}
		fmt.Println(ui.Success.Render("Ordered ") + ui.ID.Render(b.ID) + " " + ui.Muted.Render(b.Path))
		return nil
	},
}

// orderSiblingsOf returns b's siblings (same Parent, b itself excluded),
// sorted by their current Order. Filtering happens manually rather than via
// model.ItemFilter.ParentID because that filter treats an empty ParentID as
// "no filter" — which would return every item instead of just the top-level
// ones for a parent-less item.
func orderSiblingsOf(resolver *itemgraph.CoreResolver, ctx context.Context, b *item.Item) ([]*item.Item, error) {
	all, err := resolver.Items(ctx, nil)
	if err != nil {
		return nil, err
	}

	var siblings []*item.Item
	for _, s := range all {
		if s.ID == b.ID {
			continue
		}
		if s.Parent == b.Parent {
			siblings = append(siblings, s)
		}
	}
	item.SortByOrder(siblings)
	return siblings, nil
}

// computeOrderPlacement resolves the requested placement flag into a
// concrete fractional-index Order value for b, given its sorted siblings.
func computeOrderPlacement(resolver *itemgraph.CoreResolver, ctx context.Context, b *item.Item, siblings []*item.Item) (string, error) {
	switch {
	case orderFirst:
		if len(siblings) == 0 {
			return item.OrderBetween("", ""), nil
		}
		return item.OrderBetween("", siblings[0].Order), nil

	case orderLast:
		last := lastOrderedSibling(siblings)
		if last == "" && len(siblings) > 0 {
			return "", fmt.Errorf("--last needs at least one sibling with an order to anchor on, and none has one yet; order the siblings in the wanted sequence first (e.g. `lif order <id> --first`, then `--after`), then use --last")
		}
		return item.OrderBetween(last, ""), nil

	case orderAfter != "":
		return orderRelativeTo(resolver, ctx, b, siblings, orderAfter, true)

	case orderBefore != "":
		return orderRelativeTo(resolver, ctx, b, siblings, orderBefore, false)
	}

	return "", fmt.Errorf("one of --after, --before, --first, or --last is required")
}

// lastOrderedSibling returns the Order of the last sibling (in siblings'
// SortByOrder order) that actually carries one, or "" if none do.
// SortByOrder sorts Order-having siblings before Order-less ones, so
// siblings[len(siblings)-1] is only a safe stand-in for "the sibling with
// the greatest Order" when every sibling happens to have one -- on a store
// where some don't (the normal starting state, since nothing wrote Order
// before this feature existed), it silently picks an Order-less sibling and
// yields "" as if there were no ordered siblings at all yet, which is wrong
// once at least one actually has an Order.
func lastOrderedSibling(siblings []*item.Item) string {
	for i := len(siblings) - 1; i >= 0; i-- {
		if siblings[i].Order != "" {
			return siblings[i].Order
		}
	}
	return ""
}

// orderRelativeTo resolves --after/--before: it looks up the named sibling,
// validates it shares b's parent (R-12: order is scoped per parent), and
// returns a key between that sibling and its neighbour on the requested
// side.
func orderRelativeTo(resolver *itemgraph.CoreResolver, ctx context.Context, b *item.Item, siblings []*item.Item, refID string, after bool) (string, error) {
	ref, err := resolver.Item(ctx, refID)
	if err != nil {
		return "", fmt.Errorf("failed to find item %s: %w", refID, err)
	}
	if ref == nil {
		return "", fmt.Errorf("item not found: %s", refID)
	}
	if ref.ID == b.ID {
		return "", fmt.Errorf("cannot place an item relative to itself")
	}
	if ref.Parent != b.Parent {
		return "", fmt.Errorf("order is scoped per parent: %s has a different parent than %s", ref.ID, b.ID)
	}
	if ref.Order == "" {
		return "", fmt.Errorf("item %s has no explicit order yet; place it first (e.g. `lif order %s --first`) before placing another item relative to it", ref.ID, ref.ID)
	}

	idx := -1
	for i, s := range siblings {
		if s.ID == ref.ID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return "", fmt.Errorf("item not found: %s", refID)
	}

	if after {
		next := ""
		if idx+1 < len(siblings) {
			next = siblings[idx+1].Order
		}
		return item.OrderBetween(ref.Order, next), nil
	}

	prev := ""
	if idx > 0 {
		prev = siblings[idx-1].Order
	}
	return item.OrderBetween(prev, ref.Order), nil
}

// RegisterOrderCmd adds the order command to root. Flag registration is
// idempotent (guarded by a Lookup check) because orderCmd is a package-level
// singleton and tests that want to exercise cobra's flag-group validation
// (MarkFlagsMutuallyExclusive fires during Execute(), not when calling RunE
// directly) need to register it into a throwaway root without panicking on
// a second flag definition.
func RegisterOrderCmd(root *cobra.Command) {
	if orderCmd.Flags().Lookup("after") == nil {
		orderCmd.Flags().StringVar(&orderAfter, "after", "", "Place after this sibling item")
		orderCmd.Flags().StringVar(&orderBefore, "before", "", "Place before this sibling item")
		orderCmd.Flags().BoolVar(&orderFirst, "first", false, "Place before all siblings")
		orderCmd.Flags().BoolVar(&orderLast, "last", false, "Place after all siblings (needs at least one sibling that already has an order)")
		orderCmd.Flags().BoolVar(&orderJSON, "json", false, "Output as JSON")
		orderCmd.MarkFlagsMutuallyExclusive("after", "before", "first", "last")
	}
	orderCmd.ValidArgsFunction = completionUpTo(1)
	root.AddCommand(orderCmd)
}
