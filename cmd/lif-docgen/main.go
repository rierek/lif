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
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			return
		}
		name := strings.ReplaceAll(c.CommandPath(), " ", "-") + ".md"
		f, err := os.Create(filepath.Join(out, name))
		if err != nil {
			panic(err)
		}
		defer f.Close()
		fmt.Fprintf(f, "---\ntitle: %q\ndescription: %q\n---\n\n", c.CommandPath(), c.Short)
		link := func(s string) string { return "../" + strings.TrimSuffix(strings.ReplaceAll(s, "_", "-"), ".md") + "/" }
		if err := doc.GenMarkdownCustom(c, f, link); err != nil {
			panic(err)
		}
		for _, s := range c.Commands() {
			walk(s)
		}
	}
	walk(root)
}
