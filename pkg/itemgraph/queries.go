package itemgraph

import (
	"context"

	"github.com/rierek/lif/internal/gitutil"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/itemgraph/model"
)

// Item returns a single item by ID, or nil if not found.
func (r *CoreResolver) Item(ctx context.Context, id string) (*item.Item, error) {
	b, err := r.Core.Get(id)
	if err == itemcore.ErrNotFound {
		return nil, nil
	}
	return b, err
}

// Items returns a filtered, sorted list of items.
func (r *CoreResolver) Items(ctx context.Context, filter *model.ItemFilter) ([]*item.Item, error) {
	var items []*item.Item

	// If search filter is provided, start with search results
	if filter != nil && filter.Search != nil && *filter.Search != "" {
		searchResults, err := r.Core.Search(*filter.Search)
		if err != nil {
			return nil, err
		}
		items = searchResults
	} else {
		items = r.Core.All()
	}

	result := ApplyFilter(items, filter, r.Core)

	// Sort using the same logic as CLI and TUI
	cfg := r.Core.Config()
	item.SortByStatusPriorityAndType(result, cfg.StatusNames(), cfg.PriorityNames(), cfg.TypeNames())

	return result, nil
}

// ProjectName returns the configured project name.
func (r *CoreResolver) ProjectName(ctx context.Context) (string, error) {
	cfg := r.Core.Config()
	if cfg == nil {
		return "", nil
	}
	return cfg.GetProjectName(), nil
}

// MainBranch returns the current branch of the main repository.
func (r *CoreResolver) MainBranch(ctx context.Context, projectRoot string) (string, error) {
	if branch, ok := gitutil.CurrentBranch(projectRoot); ok {
		return branch, nil
	}
	return "main", nil
}
