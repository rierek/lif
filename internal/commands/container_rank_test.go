package commands

import (
	"reflect"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// TestContainerTypeNamesMatchesTypesAtRank pins beans-v2ox's shared
// container-rank helper against cfg.TypesAtRank itself, for every
// configured rank up to config.MaxContainerRank: ContainerTypeNames is the
// one place that loops 1..MaxContainerRank, and pick.go's roadmapScopeTypes,
// roadmap.go's validateRoadmapRootType and the ready filter all route
// through it.
func TestContainerTypeNamesMatchesTypesAtRank(t *testing.T) {
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	var want []string
	for rank := 1; rank <= config.MaxContainerRank; rank++ {
		want = append(want, cfg.TypesAtRank(rank)...)
	}

	got := cfg.ContainerTypeNames()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ContainerTypeNames() = %v, want %v (cfg.TypesAtRank(1..MaxContainerRank))", got, want)
	}
	if len(got) == 0 {
		t.Fatal("expected at least one container-rank type from the default config")
	}
}

// TestIsContainerRankFalseForLeafRank pins the behavioral half of
// beans-v2ox: an item whose type sits at config.LeafRank is never a
// container, regardless of the exact bound isContainerRank now reads from
// config.MaxContainerRank instead of a repeated literal.
func TestIsContainerRankFalseForLeafRank(t *testing.T) {
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	leafType := ""
	for _, name := range cfg.TypeNames() {
		if cfg.RankOf(name) == config.LeafRank {
			leafType = name
			break
		}
	}
	if leafType == "" {
		t.Fatal("expected the default config to have a leaf-rank type")
	}

	b := &item.Item{Type: leafType}
	if isContainerRank(b) {
		t.Errorf("isContainerRank(%q) = true, want false", leafType)
	}
}
