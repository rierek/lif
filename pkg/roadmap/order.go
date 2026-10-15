// Package roadmap holds the sibling order shared by lif roadmap and the
// interactive create parent tree.
package roadmap

import (
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// SortSiblings sorts one slice of sibling items in place: rank-1 containers
// first, containers of rank 2..config.MaxContainerRank second as one mixed
// sequence, everything else (leaf rank) last. Container groups follow
// item.SortRoadmapContainers; leaf-rank items follow item.SortRoadmapLeaves.
func SortSiblings(cfg *config.Config, items []*item.Item) {
	var top, mid, leaves []*item.Item
	for _, it := range items {
		switch r := cfg.RankOf(it.Type); {
		case r == 1:
			top = append(top, it)
		case r > 1 && r <= config.MaxContainerRank:
			mid = append(mid, it)
		default:
			leaves = append(leaves, it)
		}
	}
	statuses, priorities := cfg.StatusNames(), cfg.PriorityNames()
	item.SortRoadmapContainers(top, statuses, priorities)
	item.SortRoadmapContainers(mid, statuses, priorities)
	item.SortRoadmapLeaves(leaves, statuses, priorities, cfg.TypeNames())
	n := copy(items, top)
	n += copy(items[n:], mid)
	copy(items[n:], leaves)
}
