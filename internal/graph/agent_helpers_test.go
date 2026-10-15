package graph

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/rierek/lif/internal/worktree"
	"github.com/rierek/lif/pkg/forge"
)

func TestPrPrompt_NoPR(t *testing.T) {
	ctx := actionContext{ForgeCLI: "gh", PullRequest: nil}
	prompt := prPrompt(ctx)

	if !strings.Contains(prompt, "Create a pull request") {
		t.Error("expected 'Create a pull request' in prompt for no-PR state")
	}
	if strings.Contains(prompt, "Merge") {
		t.Error("prompt should not mention merging when no PR exists")
	}
}

func TestPrPrompt_HasLocalChanges(t *testing.T) {
	ctx := actionContext{
		ForgeCLI:    "gh",
		PullRequest: &forge.PullRequest{Checks: forge.CheckStatusPass},
		HasChanges:  true,
	}
	prompt := prPrompt(ctx)

	if !strings.Contains(prompt, "Update the existing pull request") {
		t.Error("expected 'Update the existing pull request' in prompt")
	}
	if !strings.Contains(prompt, "Do NOT merge the PR") {
		t.Error("expected 'Do NOT merge' guardrail in update prompt")
	}
}

func TestPrPrompt_HasUnpushedCommits(t *testing.T) {
	ctx := actionContext{
		ForgeCLI:           "gh",
		PullRequest:        &forge.PullRequest{Checks: forge.CheckStatusPass},
		HasUnpushedCommits: true,
	}
	prompt := prPrompt(ctx)

	if !strings.Contains(prompt, "Update the existing pull request") {
		t.Error("expected 'Update the existing pull request' in prompt")
	}
	if !strings.Contains(prompt, "Do NOT merge the PR") {
		t.Error("expected 'Do NOT merge' guardrail in update prompt")
	}
}

func TestPrPrompt_ChecksFailing(t *testing.T) {
	ctx := actionContext{
		ForgeCLI:    "gh",
		PullRequest: &forge.PullRequest{Checks: forge.CheckStatusFail},
	}
	prompt := prPrompt(ctx)

	if !strings.Contains(prompt, "CI checks on this PR are failing") {
		t.Error("expected failing checks prompt")
	}
	if !strings.Contains(prompt, "Do NOT merge the PR") {
		t.Error("expected 'Do NOT merge' guardrail in fix-tests prompt")
	}
}

func TestPrPrompt_Mergeable(t *testing.T) {
	ctx := actionContext{
		ForgeCLI:    "gh",
		PullRequest: &forge.PullRequest{Checks: forge.CheckStatusPass, Mergeable: true},
	}
	prompt := prPrompt(ctx)

	if !strings.Contains(prompt, "Merge this pull request") {
		t.Error("expected 'Merge this pull request' in prompt")
	}
}

func TestPrPrompt_UsesForgeCLI(t *testing.T) {
	for _, cli := range []string{"gh", "glab"} {
		ctx := actionContext{ForgeCLI: cli, PullRequest: nil}
		prompt := prPrompt(ctx)
		if !strings.Contains(prompt, cli+" pr create") {
			t.Errorf("expected prompt to use %s CLI", cli)
		}
	}
}

var productWord = regexp.MustCompile(`(?i)\bbeans?\b`)

// beans-oaex: labels, descriptions and prompts of every agent action name the
// item, never the old noun, in every state the action context can take.
func TestAgentActions_UseItemVocabulary(t *testing.T) {
	contexts := []actionContext{
		{},
		{ForgeCLI: "gh", HasChanges: true, HasNewCommits: true, MainRepoPath: "/repo"},
		{ForgeCLI: "gh", PullRequest: &forge.PullRequest{Checks: forge.CheckStatusPass}},
		{ForgeCLI: "glab", PullRequest: &forge.PullRequest{Checks: forge.CheckStatusFail}, HasUnpushedCommits: true},
	}
	var all strings.Builder
	for _, a := range agentActions {
		all.WriteString(a.Label + "\n" + a.Description + "\n")
		for _, ctx := range contexts {
			if a.LabelFunc != nil {
				all.WriteString(a.LabelFunc(ctx) + "\n")
			}
			if a.PromptFunc != nil {
				all.WriteString(a.PromptFunc(ctx) + "\n")
			}
		}
	}
	text := all.String()
	if hit := productWord.FindString(text); hit != "" {
		t.Errorf("agent actions still name %q", hit)
	}
	for _, want := range []string{
		"complete any associated items",
		"include relevant item IDs",
		"non-item changes",
		"describe the item updates",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("agent action texts missing %q", want)
		}
	}
}

func TestFindWorktreePath_ErrorNamesItem(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	r := &Resolver{WorktreeMgr: worktree.NewManager(repo, t.TempDir(), "main", "")}
	_, err := r.findWorktreePath("abc1")
	if err == nil {
		t.Fatal("expected an error for an unknown worktree")
	}
	if productWord.MatchString(err.Error()) || !strings.Contains(err.Error(), "no worktree found for item abc1") {
		t.Errorf("error = %q, want it to name the item", err)
	}
}
