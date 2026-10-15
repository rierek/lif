package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"

	"github.com/rierek/lif/internal/graph"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/itemgraph"
	"github.com/vektah/gqlparser/v2"
)

// primeCmdOutput runs the real primeCmd.RunE against a fresh temp directory
// holding cfg's saved .beans.yml, and returns everything it writes to
// stdout. Like runInitCmd (see init_test.go), it drives the package-level
// flag variables RunE reads directly rather than going through cobra's flag
// parsing, and captures stdout the same way captureListStdout does for
// listCmd's --json path.
func primeCmdOutput(t *testing.T, cfg *config.Config) string {
	t.Helper()

	dir := t.TempDir()
	cfg.SetConfigDir(dir)
	if err := cfg.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}
	return primeCmdOutputIn(t, dir)
}

// primeCmdOutputIn runs the real primeCmd.RunE inside a prepared project
// directory and returns everything it writes to stdout.
func primeCmdOutputIn(t *testing.T, dir string) string {
	t.Helper()
	t.Chdir(dir)

	origItemsPath, origConfigPath := storePath, configPath
	storePath, configPath = "", ""
	t.Cleanup(func() { storePath, configPath = origItemsPath, origConfigPath })

	out := captureListStdout(t, func() {
		if err := primeCmd.RunE(primeCmd, nil); err != nil {
			t.Fatalf("primeCmd.RunE: %v", err)
		}
	})
	return string(out)
}

// renderPrimeTemplate executes the embedded agent prompt template with a
// minimal but representative promptData, the same shape primeCmd.RunE
// builds, and returns the rendered text.
func renderPrimeTemplate(t *testing.T) string {
	t.Helper()
	tmpl, err := template.New("prompt").Parse(agentPromptTemplate)
	if err != nil {
		t.Fatalf("parsing agentPromptTemplate: %v", err)
	}
	data := promptData{
		Types:      config.DefaultTypes,
		Statuses:   config.DefaultStatuses,
		Priorities: config.DefaultPriorities,
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("executing agentPromptTemplate: %v", err)
	}
	return buf.String()
}

// The prime prompt is the only interface most agents ever read before using
// lif -- a feature that exists only in --help output is, for that
// audience, undiscoverable. beans-xej5 shipped --set/--unset/--where and
// lif order/--sort order/--order without ever touching this template.
func TestPrimeCmdDocumentsCustomFrontMatter(t *testing.T) {
	out := renderPrimeTemplate(t)
	for _, want := range []string{"--set", "--unset", "--where"} {
		if !strings.Contains(out, want) {
			t.Errorf("prime output missing %q (custom front matter undocumented)", want)
		}
	}
}

func TestPrimeCmdDocumentsManualOrdering(t *testing.T) {
	out := renderPrimeTemplate(t)
	for _, want := range []string{"lif order", "--sort order", "--order"} {
		if !strings.Contains(out, want) {
			t.Errorf("prime output missing %q (manual ordering undocumented)", want)
		}
	}
}

// beans-spo2 added the "## Review Findings and Attachments" section as the
// single authoritative source for the attachments convention, the real
// `lif promote` invocation, and rename's attachment carry-along -- every
// other surface (starting with skill:use-items) cites this heading instead
// of restating its content.
//
// beans-q2g5 revised what the directory may hold. The prior wording sent
// human knowledge to `docs/archive/<slug>/` and forbade it here; the sink
// is now the container's own attachment directory, so the ban is gone and
// the two consequences an agent cannot guess -- no front matter key, and
// delete taking the directory with it -- are pinned instead.
func TestPrimeCmdDocumentsReviewFindingsAndAttachments(t *testing.T) {
	out := renderPrimeTemplate(t)
	for _, want := range []string{
		"## Review Findings and Attachments",
		"attachments/<item-id>/",
		"Attach to the container, not to a leaf",
		"No front matter key lists attachments",
		"There is no read verb and no read flag",
		"lif promote <artifact> <finding-id> [<finding-id>...]",
		"review=<artifact-path>",
		"finding=<finding-id>",
		"lif list --where review=<path>",
		"lif rename <id> <new-id>",
		"--suffix <suffix>",
		"lif rename --prefix <prefix>",
		"--dry-run",
		"Archiving an item leaves its attachment directory in place",
		"`lif archive` itself is a batch verb with no `<id>` argument",
		"`lif delete` removes the attachment directory together with the item",
		"`lif check` reports an attachment directory whose item no longer resolves, or, if the directory itself cannot be read, that read failure",
		"moving every item already in an archive status",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("prime output missing %q (review findings/attachments undocumented)", want)
		}
	}
	// The revised sink replaces the old one; leaving the retired sentence
	// in place would have the canon state both answers at once.
	if strings.Contains(out, "never duplicated into the attachments directory") {
		t.Error("prime output still bans human knowledge from the attachments directory")
	}
}

// beans-8zlv: the archive sentence added by 7f1ffb8 named completed/scrapped as a
// fixed literal although archiveCmd selects items via the per-status
// Archive config flag (Config.IsArchiveStatus, pkg/config/config.go), not
// an invariant -- a project with a different archive-status profile would
// have been told the wrong statuses. Guard against that literal creeping
// back in.
func TestPrimeReviewFindingsSectionDoesNotNameArchiveStatusesAsFixed(t *testing.T) {
	out := renderPrimeTemplate(t)
	section := sectionOf(t, out, "## Review Findings and Attachments")
	if strings.Contains(section, "status `completed`/`scrapped`") {
		t.Errorf("Review Findings and Attachments section names archive statuses as a fixed literal:\n%s", section)
	}
}

// sectionOf extracts the section starting at heading (up to the next "\n## "
// heading) from a rendered or raw prime prompt, so a check confined to one
// section cannot false-positive on unrelated template syntax or prose
// elsewhere in the document.
func sectionOf(t *testing.T, out, heading string) string {
	t.Helper()
	start := strings.Index(out, heading)
	if start == -1 {
		t.Fatalf("prompt has no %s section:\n%s", heading, out)
	}
	rest := out[start+len(heading):]
	if end := strings.Index(rest, "\n## "); end != -1 {
		rest = rest[:end]
	}
	return rest
}

// AC7: beans-spo2's Scope rules out a new promptData field or a new
// {{...}} directive for these facts, since none of them vary by project
// config -- the section renders as static prose only. Scanning the raw
// embedded template (rather than rendered output) matters: Execute
// consumes and removes any real {{...}} directive from its output, so a
// check against rendered text passes whether or not the section carries
// one.
func TestPrimeReviewFindingsSectionHasNoTemplateDirective(t *testing.T) {
	section := sectionOf(t, agentPromptTemplate, "## Review Findings and Attachments")
	if strings.Contains(section, "{{") {
		t.Errorf("Review Findings and Attachments section contains a template directive:\n%s", section)
	}
}

// Task 7 (beans-9m5y) replaced the old lif update -s <status> lifecycle
// prose with dedicated wrapper commands. The prime template is the only
// interface most agents read, so each recipe must be documented by name
// with its defining flag -- undocumented commands are undiscoverable.
func TestPrimeCmdDocumentsWorkflowRecipes(t *testing.T) {
	out := renderPrimeTemplate(t)
	for _, want := range []string{
		"lif start <id>",
		"lif complete <id>",
		// The exact signature is pinned by TestPrimeCmdDocumentsBatchAndFilters;
		// here it is only the recipe and its defining flag that must be present.
		"lif scrap",
		"--reason",
		"lif next",
		"lif milestones",
		"lif progress",
		"list --ready",
		"list --is-blocked",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("prime output missing %q (workflow recipe undocumented)", want)
		}
	}
}

// The batch signatures and the tag verb exist only for agents that know to
// reach for them. next's filters are the same case: an agent that has only
// ever seen bare `lif next` will keep pulling the wrong item out of a
// large store.
func TestPrimeCmdDocumentsBatchAndFilters(t *testing.T) {
	out := renderPrimeTemplate(t)
	for _, want := range []string{
		"lif complete <id> [id...]",
		"lif scrap <id> [id...]",
		"lif start <id> [id...]",
		"lif tag <id> [id...]",
		"--remove-tag",
		"lif next --type",
		"--desc",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("prime output missing %q (batch/filter capability undocumented)", want)
		}
	}
}

// A failure is the one output shape an agent cannot afford to guess at: it
// decides whether the agent reads stdout or stderr, and whether it treats a
// missing document as a crash. beans-ra75 shipped the shape without touching
// this template.
func TestPrimeCmdDocumentsFailureShape(t *testing.T) {
	out := renderPrimeTemplate(t)
	for _, want := range []string{
		"Run '<command> --help' for usage.",
		`{"success": false`,
		"exits 1",
		"stderr stays empty",
		`"items": [`,
		"`NO_LIF_DIR`",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("prime output missing %q (failure shape undocumented)", want)
		}
	}
}

func TestRankLinesListEveryOccupiedRank(t *testing.T) {
	got := rankLines(&config.Config{})
	want := []string{
		"rank 1: milestone",
		"rank 2: epic",
		"rank 3: feature",
		"rank 4: bug, task",
	}
	if len(got) != len(want) {
		t.Fatalf("rankLines() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("rankLines()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// A rank left genuinely unoccupied (TypesExclusive turns the merge with
// DefaultTypes off, so ranks 2 and 3 have nothing on them here) must produce
// no line at all -- not an empty "rank N: " line -- while the ranks that are
// occupied still render, in order, on either side of the gap.
func TestRankLinesSkipAnEmptyRank(t *testing.T) {
	milestoneRank, taskRank := 1, config.LeafRank
	c := &config.Config{
		TypesExclusive: true,
		Types: []config.TypeOverride{
			{Name: "milestone", Rank: &milestoneRank},
			{Name: "task", Rank: &taskRank},
		},
	}
	got := rankLines(c)
	want := []string{"rank 1: milestone", "rank 4: task"}
	if len(got) != len(want) {
		t.Fatalf("rankLines() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("rankLines()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPromptTemplateNoLongerCarriesTheLinearChain(t *testing.T) {
	if strings.Contains(agentPromptTemplate, "milestone → epic → feature → task/bug") {
		t.Error("the template still hands agents the misleading linear chain")
	}
	if !strings.Contains(agentPromptTemplate, "higher rank") {
		t.Error("the template does not explain the rank rule")
	}
}

// An exclusive config (lif init --profile todo, say) is its own complete
// type table: DefaultTypes' milestone/epic/feature/bug must not leak into
// the "## Item Types" section, and that section must name exactly the same
// types as the "## Relationships" rank lines -- both are derived from the
// one live config an agent is actually working in.
func TestPrimeExclusiveConfigTypesAndRanksAgree(t *testing.T) {
	rank := config.LeafRank
	cfg := config.DefaultWithPrefix("demo-")
	cfg.TypesExclusive = true
	cfg.Types = []config.TypeOverride{{Name: "task", Rank: &rank, Short: "T"}}

	out := primeCmdOutput(t, cfg)
	issueTypes := sectionOf(t, out, "## Item Types")

	for _, name := range []string{"milestone", "epic", "feature", "bug"} {
		if strings.Contains(issueTypes, name) {
			t.Errorf("exclusive config's item types section still mentions %q:\n%s", name, issueTypes)
		}
	}
	if !strings.Contains(out, "rank 4: task") {
		t.Errorf("rank section does not name the only configured type:\n%s", out)
	}
	if !strings.Contains(issueTypes, "**task**") {
		t.Errorf("item types section does not name the only configured type:\n%s", issueTypes)
	}
}

// The ordinary (non-exclusive) path stays pinned: a project that never
// touches types_exclusive still gets the full built-in table in both
// sections.
func TestPrimeDefaultConfigTypesAndRanksAgree(t *testing.T) {
	cfg := config.DefaultWithPrefix("demo-")

	out := primeCmdOutput(t, cfg)

	for _, name := range []string{"milestone", "epic", "feature", "bug", "task"} {
		if !strings.Contains(out, name) {
			t.Errorf("default config's prompt is missing built-in type %q:\n%s", name, out)
		}
	}
	if !strings.Contains(out, "rank 1: milestone") {
		t.Errorf("rank section does not list the default rank-1 type:\n%s", out)
	}
}

// prime is the authoritative documentation surface for agents; a status or
// priority appended in .beans.yml must show up in its output the same way
// an appended type already does (see TestPrimeExclusiveConfigTypesAndRanksAgree).
// Before this fix, promptData.Statuses/Priorities were built straight from
// config.DefaultStatuses/DefaultPriorities, so a project-defined status or
// priority never reached the rendered prompt at all.
func TestPrimeReflectsConfiguredStatusesAndPriorities(t *testing.T) {
	cfg := config.DefaultWithPrefix("demo-")
	cfg.Statuses = []config.StatusOverride{{Name: "in-review", Description: "Awaiting review"}}
	cfg.Priorities = []config.PriorityOverride{{Name: "urgent", Description: "Drop everything"}}

	out := primeCmdOutput(t, cfg)

	if !strings.Contains(out, "**in-review**: Awaiting review") {
		t.Errorf("prime output does not reflect the configured status:\n%s", out)
	}
	if !strings.Contains(out, "**urgent**: Drop everything") {
		t.Errorf("prime output does not reflect the configured priority:\n%s", out)
	}
}

// beans-oaex: the product and the noun are renamed in every text an agent
// reads. The word search covers the real prime output, the prime and graphql
// help and the planning-agent prompt.
var productWord = regexp.MustCompile(`(?i)\bbeans?\b`)

func TestPrimeSurfacesNameNeitherBeanNorBeans(t *testing.T) {
	surfaces := map[string]string{
		"lif prime":          primeCmdOutput(t, config.DefaultWithPrefix("demo-")),
		"lif prime --help":   primeCmd.Short + "\n" + primeCmd.Long,
		"lif graphql --help": graphqlCmd.Short + "\n" + graphqlCmd.Long,
		"centralAgentPrompt": centralAgentPrompt,
	}
	for name, text := range surfaces {
		if loc := productWord.FindStringIndex(text); loc != nil {
			t.Errorf("%s still names the old product or noun: ...%s...", name, text[max(0, loc[0]-20):min(len(text), loc[1]+20)])
		}
	}
}

// Every command line in a fenced block of the prime output must start a verb
// the CLI registers: an agent copies these lines verbatim.
var fencedLifVerb = regexp.MustCompile(`(?:^|\|\s*|\$\()(lif\w*) (\S+)`)

func TestPrimeFencedCommandsInvokeRegisteredLifVerbs(t *testing.T) {
	out := primeCmdOutput(t, config.DefaultWithPrefix("demo-"))
	root := sharedTestRoot(t)

	inFence, seen := false, 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if !inFence {
			continue
		}
		m := fencedLifVerb.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		seen++
		if m[1] != "lif" {
			t.Errorf("prime fenced line %q invokes binary %q, want lif", line, m[1])
			continue
		}
		if cmd, _, err := root.Find([]string{m[2]}); err != nil || cmd == root {
			t.Errorf("prime fenced line %q invokes unknown verb %q", line, m[2])
		}
	}
	if seen < 20 {
		t.Errorf("only %d fenced lif command lines found, expected the full command list", seen)
	}
	if strings.Contains(out, "\nbeans ") {
		t.Error("prime output has a line starting with the old binary name")
	}
}

// The prompt names neither the config file nor the store directory as a
// fixed literal: an agent in a legacy project must not create `.lif/` next to
// `.beans/`, and an agent in a new project must not look for `.beans/`.
func TestPrimeNamesNoFixedConfigOrStoreLiteral(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"legacy-only":   {".beans.yml": "beans:\n    path: .beans\n"},
		"new-only":      {".lif.yml": "lif:\n    path: .lif\n"},
		"explicit path": {".lif.yml": "lif:\n    path: custom-store\n"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			for f, content := range files {
				if err := os.WriteFile(filepath.Join(dir, f), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			out := primeCmdOutputIn(t, dir)
			if out == "" {
				t.Fatal("prime printed nothing for a project fixture")
			}
			for _, lit := range []string{".lif", ".beans"} {
				if i := strings.Index(out, lit); i >= 0 {
					t.Errorf("prime output carries fixed layout literal %q: ...%s...", lit, out[max(0, i-30):min(len(out), i+30)])
				}
			}
			if !strings.Contains(out, "`lif path`") {
				t.Error("prime output does not name `lif path` as the way to find the store directory")
			}
		})
	}
}

// Criterion 3: every query in the prime output and in `lif graphql --help`
// validates against the schema `lif graphql --schema` prints.
var quotedQuery = regexp.MustCompile(`'([^']*)'`)

func TestPrimeAndGraphqlExamplesValidateAgainstSchema(t *testing.T) {
	schema := graph.NewExecutableSchema(graph.Config{
		Resolvers: &graph.Resolver{CoreResolver: &itemgraph.CoreResolver{Core: core}},
	}).Schema()

	n := 0
	for name, text := range map[string]string{
		"lif prime":          primeCmdOutput(t, config.DefaultWithPrefix("demo-")),
		"lif graphql --help": graphqlCmd.Long,
	} {
		for _, m := range quotedQuery.FindAllStringSubmatch(text, -1) {
			q := strings.TrimSpace(m[1])
			if !strings.HasPrefix(q, "{ ") && !strings.HasPrefix(q, "query ") && !strings.HasPrefix(q, "mutation ") {
				continue
			}
			n++
			if _, perr := gqlparser.LoadQuery(schema, q); perr != nil {
				t.Errorf("%s example does not validate: %v\n%s", name, perr, q)
			}
		}
	}
	if n < 11 {
		t.Errorf("only %d GraphQL examples found, expected the prime and help examples", n)
	}
}

func TestPrimeGraphqlSectionNamesRegisteredCommand(t *testing.T) {
	out := primeCmdOutput(t, config.DefaultWithPrefix("demo-"))
	section := sectionOf(t, out, "## GraphQL Queries")
	for _, want := range []string{"`lif graphql`", "lif graphql --help", "lif graphql --schema", "`query`"} {
		if !strings.Contains(section, want) {
			t.Errorf("GraphQL section missing %q", want)
		}
	}
}
