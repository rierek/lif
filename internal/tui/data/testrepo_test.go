package data

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const testLifYML = `project:
    name: test-repo

lif:
    path: .lif
    prefix: tt-
    id_length: 4
    default_status: todo
    default_type: task
`

// newTestRepo creates a temporary beans repo (tmp dir with .beans.yml,
// prefix "tt-") containing 3 fixture beans — a milestone, an epic (child of
// the milestone), and a task (child of the epic) — written as real .md
// files with YAML frontmatter, matching the on-disk format beans itself
// produces. The milestone carries "blocking: [tt-task]", the epic carries
// "tags", and the task carries "blocked_by: [tt-mlst]", so List() callers
// exercise the full JSON contract (slice fields), not just the scalar ones.
//
// Deliberate placement quirk: "tags" specifically sits on the epic, not the
// task, because of a real beans 0.4.2 CLI bug found while building this
// fixture -- for a bean whose on-disk frontmatter includes a hand-authored
// "tags:" block, the ETag `list`/`show` report does not match the ETag
// `update --if-match`'s conflict check computes internally (verified in
// isolation: "blocked_by"/"blocking"/"parent" do NOT have this divergence,
// only "tags" does; a bean's ETag becomes consistent again once beans
// itself has rewritten the file once, e.g. via any successful update). The
// task fixture is what client_mut_test.go drives through SetStatus/
// SetPriority/AppendBody/Delete round-trips using a List()-obtained ETag,
// so it must stay free of hand-authored "tags" to keep those tests
// deterministic. See bt-tknb concerns for the upstream bug report.
//
// It returns the repo root directory.
func newTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".lif.yml"), []byte(testLifYML), 0o644); err != nil {
		t.Fatalf("write .beans.yml: %v", err)
	}

	beansDir := filepath.Join(dir, ".lif")
	if err := os.MkdirAll(beansDir, 0o755); err != nil {
		t.Fatalf("mkdir .lif: %v", err)
	}

	fixtures := map[string]string{
		"tt-mlst--test-milestone.md": `---
# tt-mlst
title: Test Milestone
status: todo
type: milestone
priority: high
created_at: 2026-01-01T00:00:00Z
updated_at: 2026-01-01T00:00:00Z
blocking:
    - tt-task
---

Milestone fixture body.
`,
		"tt-epic--test-epic.md": `---
# tt-epic
title: Test Epic
status: in-progress
type: epic
priority: normal
created_at: 2026-01-01T00:00:00Z
updated_at: 2026-01-01T00:00:00Z
parent: tt-mlst
tags:
    - urgent
    - backend
---

Epic fixture body.
`,
		"tt-task--test-task.md": `---
# tt-task
title: Test Task
status: todo
type: task
priority: normal
created_at: 2026-01-01T00:00:00Z
updated_at: 2026-01-01T00:00:00Z
parent: tt-epic
blocked_by:
    - tt-mlst
---

Task fixture body.
`,
	}

	for name, content := range fixtures {
		if err := os.WriteFile(filepath.Join(beansDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}

	return dir
}

// newTestRepoN creates a temporary beans repo (same .beans.yml/frontmatter
// schema as newTestRepo, prefix "tt-") sized for volume testing: 1 milestone
// + 10 epics (children of the milestone) + n-11 tasks, evenly distributed
// (round-robin) across the 10 epics via `parent`. A realistic tree depth
// (milestone -> epic -> task) rather than a flat list, per E6 T1 design
// decision b (epic-E6-plan.md) -- the US-01 performance smoke measures
// Open+Items() against this, the exact calls runTUI and loadCmd
// issues on startup (`beans list --json --full`).
//
// n must be >= 11 (1 milestone + 10 epics minimum). IDs are generated
// directly (not through the beans CLI) as "tt-0001".."tt-NNNN" to keep
// fixture generation a pure filesystem write, matching newTestRepo's own
// no-CLI-dependency-for-fixtures approach -- requireBeansBinary is still the
// caller's job for the List() call itself.
func newTestRepoN(t *testing.T, n int) string {
	t.Helper()

	if n < 11 {
		t.Fatalf("newTestRepoN: n must be >= 11 (1 milestone + 10 epics), got %d", n)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".lif.yml"), []byte(testLifYML), 0o644); err != nil {
		t.Fatalf("write .beans.yml: %v", err)
	}

	beansDir := filepath.Join(dir, ".lif")
	if err := os.MkdirAll(beansDir, 0o755); err != nil {
		t.Fatalf("mkdir .lif: %v", err)
	}

	writeBean := func(id, title, beanType, parent string) {
		t.Helper()
		parentLine := ""
		if parent != "" {
			parentLine = fmt.Sprintf("parent: %s\n", parent)
		}
		content := fmt.Sprintf(`---
# %s
title: %s
status: todo
type: %s
priority: normal
created_at: 2026-01-01T00:00:00Z
updated_at: 2026-01-01T00:00:00Z
%s---

%s fixture body.
`, id, title, beanType, parentLine, title)
		name := fmt.Sprintf("%s--%s.md", id, id)
		if err := os.WriteFile(filepath.Join(beansDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write fixture %s: %v", id, err)
		}
	}

	nextID := 1
	genID := func() string {
		id := fmt.Sprintf("tt-%04d", nextID)
		nextID++
		return id
	}

	milestoneID := genID()
	writeBean(milestoneID, "Perf Milestone", "milestone", "")

	epicIDs := make([]string, 10)
	for i := 0; i < 10; i++ {
		epicIDs[i] = genID()
		writeBean(epicIDs[i], fmt.Sprintf("Perf Epic %d", i+1), "epic", milestoneID)
	}

	taskCount := n - 11
	for i := 0; i < taskCount; i++ {
		id := genID()
		parent := epicIDs[i%10]
		writeBean(id, fmt.Sprintf("Perf Task %d", i+1), "task", parent)
	}

	return dir
}
