// Generates Starlight-ready Markdown for every command from the Cobra tree.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rierek/lif/internal/commands"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// treeFile is the generated partial with the command hierarchy. The underscore keeps Starlight from treating it
// as a page; reference/commands/index.mdx imports it.
const treeFile = "_command-tree.md"

// supplementDir is the directory below the output dir with optional hand-written supplements (D24). The
// underscore in the directory and in every file name keeps Starlight (whose page glob tests only the file name) from treating the files as pages. A supplement named after a command page
// (_lif_create.md for `lif create`) is inserted before that command's "SEE ALSO" section.
const supplementDir = "_supplements"

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: lif-docgen <output-dir>")
		os.Exit(2)
	}
	if err := generate(newRoot(), os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "lif-docgen:", err)
		os.Exit(1)
	}
}

// newRoot builds the same command tree as the lif binary.
func newRoot() *cobra.Command {
	root := commands.NewRootCmd()
	commands.RegisterCoreCommands(root)
	return root
}

// documented reports whether c gets a page: every command that `lif --help` lists as usable (D25). Hidden and
// deprecated commands are out, and so is Cobra's built-in help, which IsAvailableCommand excludes (D57).
func documented(c *cobra.Command) bool {
	return c.IsAvailableCommand() && !c.IsAdditionalHelpTopicCommand()
}

// supplement returns the hand-written supplement of c, or "" when there is none.
func supplement(out string, c *cobra.Command) (string, error) {
	b, err := os.ReadFile(filepath.Join(out, supplementDir, "_"+strings.ReplaceAll(c.CommandPath(), " ", "_")+".md"))
	if os.IsNotExist(err) {
		return "", nil
	}
	return strings.TrimSpace(string(b)), err
}

// withSupplement inserts sup before the "### SEE ALSO" heading of a generated page, or appends it.
func withSupplement(md, sup string) string {
	if sup == "" {
		return md
	}
	if i := strings.Index(md, "### SEE ALSO"); i >= 0 {
		return md[:i] + sup + "\n\n" + md[i:]
	}
	return md + "\n" + sup + "\n"
}

// generate writes one page per documented command below root into out, plus the command tree partial.
func generate(root *cobra.Command, out string) error {
	root.DisableAutoGenTag = true
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	// Every generated page starts with "lif"; drop pages of commands that no longer exist (hand-written
	// pages such as index.md do not match).
	stale, err := filepath.Glob(filepath.Join(out, "lif*.md"))
	if err != nil {
		return err
	}
	for _, p := range stale {
		if err := os.Remove(p); err != nil {
			return err
		}
	}
	if err := writeFile(filepath.Join(out, treeFile), func(w io.Writer) error { return writeTree(w, root, 0) }); err != nil {
		return err
	}
	return writePages(root, out)
}

func writeFile(path string, fill func(io.Writer) error) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	return fill(f)
}

// fenceIndentedComments turns help-text example blocks into code. Cobra copies the long help verbatim, and a line
// like "  # List all items" is an ATX heading in Markdown. Every block that starts with such a line, up to the next
// unindented line, becomes a fenced block with the indent removed. Existing fences are left alone.
func fenceIndentedComments(md string) string {
	var out []string
	lines := strings.Split(md, "\n")
	inFence := false
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		if strings.HasPrefix(l, "```") {
			inFence = !inFence
		}
		if inFence || !isIndentedHeading(l) {
			out = append(out, l)
			continue
		}
		j := i
		for j < len(lines) && (lines[j] == "" || strings.HasPrefix(lines[j], " ")) {
			j++
		}
		end := j
		for end > i && lines[end-1] == "" {
			end--
		}
		out = append(out, "```")
		for _, b := range lines[i:end] {
			out = append(out, strings.TrimPrefix(b, "  "))
		}
		out = append(out, "```")
		out = append(out, lines[end:j]...)
		i = j - 1
	}
	return strings.Join(out, "\n")
}

func isIndentedHeading(l string) bool {
	t := strings.TrimLeft(l, " ")
	return len(l)-len(t) >= 1 && len(l)-len(t) <= 3 && strings.HasPrefix(t, "# ")
}

func slug(s string) string { return strings.TrimSuffix(strings.ReplaceAll(s, "_", "-"), ".md") }

func pageName(c *cobra.Command) string { return strings.ReplaceAll(c.CommandPath(), " ", "-") }

func usableChildren(c *cobra.Command) []*cobra.Command {
	var out []*cobra.Command
	for _, s := range c.Commands() {
		if documented(s) {
			out = append(out, s)
		}
	}
	return out
}

func writePages(c *cobra.Command, out string) error {
	if !documented(c) {
		return nil
	}
	err := writeFile(filepath.Join(out, pageName(c)+".md"), func(w io.Writer) error {
		fmt.Fprintf(w, "---\ntitle: %q\ndescription: %q\n---\n\n", c.CommandPath(), c.Short)
		// D25: the completion shells share the page of `completion`; links between them become anchors.
		shared := c.Name() == "completion"
		link := func(s string) string {
			if shared && strings.HasPrefix(s, "lif_completion") {
				return "#" + slug(s)
			}
			return "../" + slug(s) + "/"
		}
		sections := []*cobra.Command{c}
		if shared {
			sections = append(sections, usableChildren(c)...)
		}
		var buf bytes.Buffer
		for _, s := range sections {
			var page bytes.Buffer
			if err := doc.GenMarkdownCustom(s, &page, link); err != nil {
				return err
			}
			sup, err := supplement(out, s)
			if err != nil {
				return err
			}
			buf.WriteString(withSupplement(page.String(), sup))
		}
		_, err := io.WriteString(w, fenceIndentedComments(buf.String()))
		return err
	})
	if err != nil || c.Name() == "completion" {
		return err
	}
	for _, s := range c.Commands() {
		if err := writePages(s, out); err != nil {
			return err
		}
	}
	return nil
}

// writeTree writes c and its documented subcommands as a nested list: one level per parent. Links are relative to
// the page that imports the partial (the command overview, next to the generated pages); the shells of
// `completion` point to anchors on its shared page.
func writeTree(w io.Writer, c *cobra.Command, depth int) error {
	href := pageName(c) + "/"
	if c.Parent() != nil && c.Parent().Name() == "completion" {
		href = pageName(c.Parent()) + "/#" + pageName(c)
	}
	if _, err := fmt.Fprintf(w, "%s- [`%s`](%s): %s\n", strings.Repeat("  ", depth), c.CommandPath(), href, c.Short); err != nil {
		return err
	}
	for _, s := range usableChildren(c) {
		if err := writeTree(w, s, depth+1); err != nil {
			return err
		}
	}
	return nil
}
