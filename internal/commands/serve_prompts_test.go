package commands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/rierek/lif/internal/graph"
	"github.com/rierek/lif/pkg/item"
)

// The web UI hands these two prompts to the agent CLI. Both name the product
// as Lif and the noun as item.
func TestServeAgentPromptsNameLifAndItem(t *testing.T) {
	t.Run("worker context", func(t *testing.T) {
		got := itemAgentContext(&item.Item{ID: "t-1", Title: "Fix it", Type: "task", Status: "todo", Priority: "high", Body: "Details"})
		for _, want := range []string{
			"You are already working inside a lif-managed worktree.",
			`You are working on item t-1: "Fix it"`,
			"Type: task | Status: todo | Priority: high",
			"Description:\nDetails",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("worker context lacks %q:\n%s", want, got)
			}
		}
	})

	t.Run("planning system prompt", func(t *testing.T) {
		got := centralPlanningPrompt("/work/proj")
		for _, want := range []string{
			"You are the central planning agent for the Lif project.",
			"main repository at: /work/proj\n",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("planning prompt lacks %q:\n%s", want, got)
			}
		}
	})
}

// The central agent prompt cites GraphQL mutations by name. Every cited
// mutation must exist in the served schema, otherwise the agent gets a
// GraphQL error (beans-xw7j: startWork never existed).
func TestCentralAgentPromptCitesOnlyRealMutations(t *testing.T) {
	schema := graph.NewExecutableSchema(graph.Config{}).Schema()
	cited := regexp.MustCompile(`mutation \{ (\w+)\(`).FindAllStringSubmatch(centralAgentPrompt, -1)
	if len(cited) == 0 {
		t.Fatal("centralAgentPrompt cites no mutation example")
	}
	for _, m := range cited {
		if schema.Mutation.Fields.ForName(m[1]) == nil {
			t.Errorf("centralAgentPrompt cites mutation %q, not in schema", m[1])
		}
	}
	if strings.Contains(centralAgentPrompt, "startWork") {
		t.Error("centralAgentPrompt still mentions nonexistent startWork")
	}
}
