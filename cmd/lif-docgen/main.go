// Generates Starlight-ready Markdown for every command from the Cobra tree.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/xRiErOS/lif/internal/commands"
)

func main() {
	out := os.Args[1]
	root := commands.NewRootCmd()
	commands.RegisterCoreCommands(root)
	root.DisableAutoGenTag = true
	if err := os.MkdirAll(out, 0o755); err != nil {
		panic(err)
	}
	// Every generated page starts with "lif"; drop pages of commands that no longer exist.
	stale, _ := filepath.Glob(filepath.Join(out, "lif*.md"))
	for _, p := range stale {
		if err := os.Remove(p); err != nil {
			panic(err)
		}
	}
	slug := func(s string) string { return strings.TrimSuffix(strings.ReplaceAll(s, "_", "-"), ".md") }
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		// D25: every command listed by `lif --help` gets a page. Cobra's built-in help is listed there,
		// but IsAvailableCommand excludes it.
		if (!c.IsAvailableCommand() && c.Name() != "help") || c.IsAdditionalHelpTopicCommand() {
			return
		}
		f, err := os.Create(filepath.Join(out, strings.ReplaceAll(c.CommandPath(), " ", "-")+".md"))
		if err != nil {
			panic(err)
		}
		defer f.Close()
		fmt.Fprintf(f, "---\ntitle: %q\ndescription: %q\n---\n\n", c.CommandPath(), c.Short)
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
			for _, s := range c.Commands() {
				if s.IsAvailableCommand() {
					sections = append(sections, s)
				}
			}
		}
		for _, s := range sections {
			if err := doc.GenMarkdownCustom(s, f, link); err != nil {
				panic(err)
			}
		}
		if shared {
			return
		}
		for _, s := range c.Commands() {
			walk(s)
		}
	}
	walk(root)
}
