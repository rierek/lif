package main

import (
	"os"

	"github.com/rierek/lif/internal/commands"
)

func main() {
	root := commands.NewRootCmd()
	root.Use = "lif-serve"
	commands.RegisterServeCmd(root)

	// Default to "serve" when no subcommand is given
	if len(os.Args) < 2 || os.Args[1][0] == '-' {
		os.Args = append([]string{os.Args[0], "serve"}, os.Args[1:]...)
	}

	commands.Execute(root)
}
