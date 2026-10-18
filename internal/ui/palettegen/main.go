// Command palettegen writes the Catppuccin palette as CSS for the web UI.
// It runs through `go generate ./internal/ui`.
package main

import (
	"fmt"
	"os"

	"github.com/rierek/lif/internal/ui"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: palettegen <output.css>")
		os.Exit(2)
	}
	if err := os.WriteFile(os.Args[1], []byte(ui.PaletteCSS()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
