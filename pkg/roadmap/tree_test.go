package roadmap

import (
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
)

// shape renders nodes as "ID[child,child]" so a whole tree compares as one string.
func shape(nodes []*Node) string {
	parts := make([]string, len(nodes))
	for i, n := range nodes {
		parts[i] = n.Item.ID
		if len(n.Children) > 0 {
			parts[i] += "[" + shape(n.Children) + "]"
		}
	}
	return strings.Join(parts, ",")
}

func TestTreeOrder(t *testing.T) {
	cfg := config.Default()
	items := []*item.Item{
		{ID: "M", Type: "milestone", Title: "M", Status: "todo"},
		{ID: "E", Type: "epic", Title: "E", Status: "todo", Parent: "M"},
		{ID: "F", Type: "feature", Title: "F", Status: "todo", Parent: "E"},
		{ID: "T1", Type: "task", Title: "T1", Status: "todo", Parent: "F"},
		{ID: "L", Type: "task", Title: "L", Status: "todo", Parent: "M"},
		{ID: "E2", Type: "epic", Title: "E2", Status: "todo"},
		{ID: "T2", Type: "task", Title: "T2", Status: "todo", Parent: "E2"},
		{ID: "T3", Type: "task", Title: "T3", Status: "todo"},
	}

	scheduled, unscheduled := Build(cfg, items, true, nil, nil).Tree()

	if got, want := shape(scheduled), "M[E[F[T1]],L]"; got != want {
		t.Errorf("scheduled = %s, want %s", got, want)
	}
	if got, want := shape(unscheduled), "E2[T2],T3"; got != want {
		t.Errorf("unscheduled = %s, want %s", got, want)
	}
}

func TestTreeScopedRootHasNoUnscheduled(t *testing.T) {
	cfg := config.Default()
	e := &item.Item{ID: "E", Type: "epic", Title: "E", Status: "todo"}
	items := []*item.Item{
		e,
		{ID: "T1", Type: "task", Title: "T1", Status: "todo", Parent: "E"},
	}

	scheduled, unscheduled := BuildScoped(cfg, items, true, e).Tree()

	if got, want := shape(scheduled), "E[T1]"; got != want {
		t.Errorf("scheduled = %s, want %s", got, want)
	}
	if unscheduled != nil {
		t.Errorf("unscheduled = %v, want nil", unscheduled)
	}
}
