package commands

import (
	"reflect"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/roadmap"
)

// TestContainerTypesOfTheDefaultConfig pins beans-v2ox's shared container-rank
// helper by the names it gives for the default configuration. The expected
// names are spelled out, not rebuilt from cfg.TypesAtRank: a test that loops
// over the ranks the way Config.ContainerTypeNames does would repeat the
// production code and agree with it whatever it returned. Every reader of the
// list -- pick.go's roadmapScopeTypes, roadmap.go's roadmap.IsContainerRank and
// roadmap.ValidateRootType, and the ready filter -- goes through it.
func TestContainerTypesOfTheDefaultConfig(t *testing.T) {
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	want := []string{"milestone", "epic", "feature"}
	if got := cfg.ContainerTypeNames(); !reflect.DeepEqual(got, want) {
		t.Errorf("ContainerTypeNames() = %v, want %v", got, want)
	}

	scope := roadmapScopeTypes()
	if len(scope) != len(want) {
		t.Errorf("roadmapScopeTypes() = %v, want exactly %v", scope, want)
	}
	for _, name := range want {
		if !scope[name] {
			t.Errorf("roadmapScopeTypes() lacks the container type %q: %v", name, scope)
		}
		if !roadmap.IsContainerRank(cfg, &item.Item{Type: name}) {
			t.Errorf("roadmap.IsContainerRank(%q) = false, want true", name)
		}
	}
}

// TestIsContainerRankFalseForLeafRank pins the behavioural half of
// beans-v2ox: an item whose type sits on the leaf rank is never a container,
// whatever bound roadmap.IsContainerRank reads from config.MaxContainerRank.
func TestIsContainerRankFalseForLeafRank(t *testing.T) {
	oldCfg := cfg
	defer func() { cfg = oldCfg }()
	cfg = config.Default()

	for _, leaf := range []string{"task", "bug"} {
		if roadmap.IsContainerRank(cfg, &item.Item{Type: leaf}) {
			t.Errorf("roadmap.IsContainerRank(%q) = true, want false", leaf)
		}
		if roadmapScopeTypes()[leaf] {
			t.Errorf("roadmapScopeTypes() holds the leaf type %q", leaf)
		}
	}
}
