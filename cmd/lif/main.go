package main

import (
	"os"

	"github.com/rierek/lif/internal/commands"
	"github.com/rierek/lif/internal/tasks"
)

func main() {
	root := commands.NewRootCmd()
	commands.RegisterCoreCommands(root)
	tasks.LifPath, _ = os.Executable()
	commands.Execute(root)
}
