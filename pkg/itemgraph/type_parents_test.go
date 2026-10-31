package itemgraph

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/itemgraph/model"
)

// parentsResolver: feature only under epic, task only under feature, milestone
// at the top level only. The rank rule alone would allow task under epic.
func parentsResolver(t *testing.T) *CoreResolver {
	t.Helper()
	itemsDir := filepath.Join(t.TempDir(), ".beans")
	if err := os.MkdirAll(itemsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	epic, feature, none := []string{"epic"}, []string{"feature"}, []string{}
	cfg := config.Default()
	cfg.Types = []config.TypeOverride{
		{Name: "feature", Parents: &epic},
		{Name: "task", Parents: &feature},
		{Name: "milestone", Parents: &none},
	}
	c := itemcore.New(itemsDir, cfg)
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}
	return &CoreResolver{Core: c}
}

func TestParentsListIsEnforcedOnGraphQLWritePaths(t *testing.T) {
	ctx := context.Background()
	r := parentsResolver(t)
	createTyped(t, r, "beans-ms01", "milestone", "")
	createTyped(t, r, "beans-ep01", "epic", "beans-ms01")
	createTyped(t, r, "beans-ft01", "feature", "beans-ep01")
	createTyped(t, r, "beans-tk01", "task", "beans-ft01")
	before := len(r.Core.All())

	t.Run("createItem", func(t *testing.T) {
		_, err := r.CreateItem(ctx, model.CreateItemInput{Title: "x", Type: strptr("task"), Parent: strptr("beans-ep01")})
		if err == nil || !strings.Contains(err.Error(), "feature") {
			t.Fatalf("err = %v, want rejection naming feature", err)
		}
		if got := len(r.Core.All()); got != before {
			t.Fatalf("rejected create wrote an item: %d -> %d", before, got)
		}
	})
	t.Run("setParent", func(t *testing.T) {
		if _, err := r.SetParent(ctx, "beans-tk01", strptr("beans-ep01"), nil); err == nil {
			t.Fatal("task under epic accepted despite parents: [feature]")
		}
		if got, _ := r.Core.Get("beans-tk01"); got.Parent != "beans-ft01" {
			t.Fatalf("parent = %q after rejected setParent", got.Parent)
		}
	})
	t.Run("empty list refuses every parent", func(t *testing.T) {
		_, err := r.SetParent(ctx, "beans-ms01", strptr("beans-ep01"), nil)
		if err == nil || !strings.Contains(err.Error(), "cannot have a parent") {
			t.Fatalf("err = %v, want 'cannot have a parent'", err)
		}
	})
	t.Run("listed parent works", func(t *testing.T) {
		if _, err := r.CreateItem(ctx, model.CreateItemInput{Title: "y", Type: strptr("task"), Parent: strptr("beans-ft01")}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("type change names a child its list no longer allows", func(t *testing.T) {
		createTyped(t, r, "beans-ft02", "feature", "")
		createTyped(t, r, "beans-tk02", "task", "beans-ft02")
		_, err := r.UpdateItem(ctx, "beans-ft02", model.UpdateItemInput{Type: strptr("epic")})
		if err == nil || !strings.Contains(err.Error(), "beans-tk02") {
			t.Fatalf("err = %v, want rejection naming beans-tk02", err)
		}
		if got, _ := r.Core.Get("beans-ft02"); got.Type != "feature" {
			t.Fatalf("type = %q after rejected change", got.Type)
		}
	})
}
