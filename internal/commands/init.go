package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rierek/lif/internal/gitutil"
	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/spf13/cobra"
)

var initJSON bool
var initProfile string

var initCmd = &cobra.Command{
	Use:   "init",
	Args:  cobra.NoArgs,
	Short: "Initialize a Lif project",
	Long: `Creates a .lif directory and .lif.yml config file in the current directory.
A project that already uses the legacy names .beans or .beans.yml keeps them.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var projectDir string
		var itemsDir string
		var dirName string

		if storePath != "" && initProfile != "" {
			msg := "--profile writes a .lif.yml, which --lif-path skips; use one or the other"
			if initJSON {
				return output.Error(output.ErrValidation, msg)
			}
			return errors.New(msg)
		}

		// Resolve the profile before any side effect (Core.Init creates
		// the store directory and its .gitignore): a typo must not leave a half-initialised
		// project behind.
		var profileTypes []config.TypeOverride
		if initProfile != "" {
			types, ok := config.ProfileTypes(initProfile)
			if !ok {
				msg := fmt.Sprintf("unknown profile %q (must be %s)", initProfile, strings.Join(config.ProfileNames(), ", "))
				if initJSON {
					return output.Error(output.ErrValidation, msg)
				}
				return errors.New(msg)
			}
			profileTypes = types
		}

		if storePath != "" {
			// Use explicit path for items directory
			itemsDir = storePath
			// Create the directory using Core.Init to set up .gitignore
			core := itemcore.New(itemsDir, nil)
			if err := core.Init(); err != nil {
				if initJSON {
					return output.Error(output.ErrFileError, err.Error())
				}
				return fmt.Errorf("failed to create directory: %w", err)
			}
			// Skip creating a config file when --lif-path is explicit:
			// the path is already known, and writing a config to the parent
			// directory could pollute unrelated locations (e.g. /tmp).
		} else {
			// Use current working directory
			dir, err := os.Getwd()
			if err != nil {
				if initJSON {
					return output.Error(output.ErrFileError, err.Error())
				}
				return err
			}

			// Resolve every name level that can fail -- store directory,
			// config file, section key -- before the first side effect: an
			// ambiguous project must be left exactly as it was.
			storeDirName, err := config.ResolveInitStoreDir(dir)
			if err != nil {
				if initJSON {
					return output.Error(output.ErrValidation, err.Error())
				}
				return err
			}

			projectDir = dir
			itemsDir = filepath.Join(dir, storeDirName)
			dirName = filepath.Base(dir)

			// Create default config file with directory name as prefix
			// Config is saved at project root (not inside the store directory)
			defaultCfg := config.DefaultWithPrefix(dirName + "-")
			defaultCfg.Project.Name = dirName
			defaultCfg.Items.Path = storeDirName
			if initProfile != "" {
				defaultCfg.Types = profileTypes
				// A profile gives a project exactly its own types: switch off
				// the merge onto the built-in defaults, not just override them.
				defaultCfg.TypesExclusive = true
				// The default type must exist in the chosen profile: todo has no
				// milestone, complex has no plain feature leaf.
				defaultCfg.Items.DefaultType = "task"
			}
			defaultCfg.SetConfigDir(projectDir)

			// Auto-detect the remote's default branch if we're in a git repo
			if baseRef, ok := gitutil.DefaultRemoteBranch(projectDir, "origin"); ok {
				defaultCfg.Worktree.BaseRef = baseRef
			}
			if _, _, err := defaultCfg.SaveTarget(projectDir); err != nil {
				if initJSON {
					return output.Error(output.ErrValidation, err.Error())
				}
				return err
			}

			if err := itemcore.New(itemsDir, nil).Init(); err != nil {
				if initJSON {
					return output.Error(output.ErrFileError, err.Error())
				}
				return fmt.Errorf("failed to initialize: %w", err)
			}
			if err := defaultCfg.Save(projectDir); err != nil {
				if initJSON {
					return output.Error(output.ErrFileError, err.Error())
				}
				return fmt.Errorf("failed to create config: %w", err)
			}
		}

		if initJSON {
			return output.SuccessInit(itemsDir)
		}

		fmt.Println("Initialized Lif project")
		return nil
	},
}

func RegisterInitCmd(root *cobra.Command) {
	initCmd.Flags().BoolVar(&initJSON, "json", false, "Output as JSON")
	initCmd.Flags().StringVar(&initProfile, "profile", "",
		fmt.Sprintf("Type profile to write out (%s)", strings.Join(config.ProfileNames(), ", ")))
	root.AddCommand(initCmd)
}
