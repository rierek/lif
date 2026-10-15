package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rierek/lif/pkg/itemcore"
	"github.com/spf13/cobra"
)

// R-23: the operator has to see the attachment move before consenting. The
// measured gap was that `--dry-run` listed the item file move and the config
// write but said nothing about the directory it was about to strand.
func TestRenderRenamePlan_showsAttachmentMoves(t *testing.T) {
	c := renameTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md": "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
	})
	dir := filepath.Join(c.Root(), itemcore.AttachmentsDir, "tp-aaaa")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "review.json"), []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	plan, err := c.PlanRenameID("tp-aaaa", "tp-bbbb")
	if err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := renderRenamePlan(cmd, plan, false); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "attachments/tp-aaaa") || !strings.Contains(got, "attachments/tp-bbbb") {
		t.Errorf("plan does not name the attachment move:\n%s", got)
	}
}

// A plan without attachments must stay byte-identical to the established
// output, so the added line cannot become noise on the common path.
func TestRenderRenamePlan_silentWithoutAttachments(t *testing.T) {
	c := renameTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md": "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
	})
	plan, err := c.PlanRenameID("tp-aaaa", "tp-bbbb")
	if err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := renderRenamePlan(cmd, plan, false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "attachments") {
		t.Errorf("plan mentions attachments although there are none:\n%s", out.String())
	}
}

// R-23/T02: check has to see the orphan, otherwise the carry-along
// invariant is asserted rather than verified. Measured before the fix:
// "All checks passed" over a directory whose item had been rebranded away.
func TestAttachmentOrphanIssues(t *testing.T) {
	c := renameTestCore(t, "tp-", map[string]string{
		"tp-aaaa--a.md": "---\n# tp-aaaa\ntitle: A\nstatus: todo\ntype: task\n---\n",
	})
	for _, id := range []string{"tp-aaaa", "tp-9999"} {
		dir := filepath.Join(c.Root(), itemcore.AttachmentsDir, id)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.Load(); err != nil {
		t.Fatal(err)
	}

	issues := attachmentOrphanIssues(c)
	if len(issues) != 1 {
		t.Fatalf("issues = %v, want exactly one", issues)
	}
	if want := "attachments/tp-9999 has no item tp-9999"; issues[0] != want {
		t.Errorf("issue = %q, want %q", issues[0], want)
	}
	if strings.Contains(issues[0], "tp-aaaa") {
		t.Errorf("issue wrongly names a live bean's attachment: %q", issues[0])
	}
}
