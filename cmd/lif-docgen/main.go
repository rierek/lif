// Generates Starlight-ready Markdown for every command from the Cobra tree.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/xRiErOS/lif/internal/commands"
)

// treeFile is the generated partial with the command hierarchy. The underscore keeps Starlight from treating it
// as a page; reference/commands/index.mdx imports it.
const treeFile = "_command-tree.md"

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
		for _, s := range sections {
			if err := doc.GenMarkdownCustom(s, w, link); err != nil {
				return err
			}
		}
		return nil
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
