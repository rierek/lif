package main

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/rierek/lif/internal/commands"
	"github.com/rierek/lif/internal/tasks"
)

// lifBinary returns the lif binary that hook tasks get as $LIF. This binary
// registers only serve, so its own path would make a hook that calls
// "$LIF" update fail with an unknown command. The lif beside exe wins, then
// the first one on PATH. Empty means there is none: a hook then fails with
// "lif binary not found".
func lifBinary(exe string) string {
	candidates := []string{"lif"}
	if exe != "" {
		candidates = []string{filepath.Join(filepath.Dir(exe), "lif"), "lif"}
	}
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func main() {
	root := commands.NewRootCmd()
	root.Use = "lif-serve"
	commands.RegisterServeCmd(root)
	exe, _ := os.Executable()
	tasks.LifPath = lifBinary(exe)

	// Default to "serve" when no subcommand is given
	if len(os.Args) < 2 || os.Args[1][0] == '-' {
		os.Args = append([]string{os.Args[0], "serve"}, os.Args[1:]...)
	}

	commands.Execute(root)
}
