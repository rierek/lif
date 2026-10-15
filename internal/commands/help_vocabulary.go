package commands

import (
	"os"

	"github.com/rierek/lif/pkg/config"
	"github.com/spf13/cobra"
)

// withConfiguredVocabulary makes cmd's help render the status, type and
// priority names of the config found for the current directory. Help runs
// without a loaded store (it is bootstrap-skip), so the global cfg is not
// set; the flag texts built at registration only know the built-in names.
// refresh rewrites those texts from the config it is given.
func withConfiguredVocabulary(cmd *cobra.Command, refresh func(*config.Config)) {
	defaultHelp := cmd.HelpFunc()
	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		refresh(helpConfig())
		defaultHelp(c, args)
	})
}

// helpConfig loads the config that --config or the working directory names.
// Outside a project, or when the config does not load, it returns nil, which
// the config accessors treat as the built-in defaults.
func helpConfig() *config.Config {
	var c *config.Config
	var err error
	if configPath != "" {
		c, err = config.Load(configPath)
	} else if cwd, wdErr := os.Getwd(); wdErr == nil {
		c, err = config.LoadFromDirectory(cwd)
	}
	if err != nil {
		return nil
	}
	return c
}
