package commands

import (
	_ "embed"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"

	"github.com/rierek/lif/pkg/config"
	"github.com/spf13/cobra"
)

//go:embed prompt.tmpl
var agentPromptTemplate string

// requiredFieldsEntry is one status's required-field list, for deterministic
// (sorted-by-status) template rendering of a map.
type requiredFieldsEntry struct {
	Status string
	Fields []string
}

// promptData holds all data needed to render the prompt template.
type promptData struct {
	Types      []config.TypeConfig
	Statuses   []config.StatusConfig
	Priorities []config.PriorityConfig

	// RequiredFields lists this project's beans.require_fields_on policy,
	// sorted by status; empty when no policy is configured.
	RequiredFields []requiredFieldsEntry
	// CommitField is the configured beans.commit_field (or its default).
	CommitField string
	// CommitFieldGated is true when CommitField is among some status's
	// required fields, i.e. `--commit` resolution applies somewhere in
	// this project (used by the generic policy section).
	CommitFieldGated bool
	// CompletionCommitGated is true specifically when CommitField is
	// required on the "completed" status, i.e. `lif complete` itself
	// needs `--commit`. A project can gate CommitField on another status
	// (e.g. "accepted") without gating completion at all.
	CompletionCommitGated bool
	// TypeRanks lists one formatted line per occupied hierarchy rank, e.g.
	// "rank 1: milestone", reflecting this project's actual config.
	TypeRanks []string
}

// rankLines renders one line per occupied hierarchy rank. The prompt template
// is parsed without a FuncMap, so the joining happens here rather than there.
func rankLines(c *config.Config) []string {
	var out []string
	for rank := 1; rank <= config.LeafRank; rank++ {
		names := c.TypesAtRank(rank)
		if len(names) == 0 {
			continue
		}
		out = append(out, fmt.Sprintf("rank %d: %s", rank, strings.Join(names, ", ")))
	}
	return out
}

var primeCmd = &cobra.Command{
	Use:   "prime",
	Short: "Output instructions for AI coding agents",
	Long:  `Outputs a prompt that primes AI coding agents on how to use the lif CLI to manage project items.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load the project's actual config so the required-fields policy
		// section reflects what's really configured here, not static prose.
		// If no explicit path is given and no project can be found, stay
		// silent (agents outside a lif project get no prompt).
		var loadedCfg *config.Config
		switch {
		case configPath != "":
			c, err := config.Load(configPath)
			if err != nil {
				return nil // Silently exit on error
			}
			loadedCfg = c
		case storePath != "":
			cwd, err := os.Getwd()
			if err != nil {
				return nil // Silently exit on error
			}
			c, err := config.LoadFromDirectory(cwd)
			if err != nil {
				return nil // Silently exit on error
			}
			loadedCfg = c
		default:
			cwd, err := os.Getwd()
			if err != nil {
				return nil // Silently exit on error
			}
			configFile, err := config.FindConfig(cwd)
			if err != nil || configFile == "" {
				// No config file found - silently exit
				return nil
			}
			c, err := config.LoadFromDirectory(cwd)
			if err != nil {
				return nil // Silently exit on error
			}
			loadedCfg = c
		}

		tmpl, err := template.New("prompt").Parse(agentPromptTemplate)
		if err != nil {
			return err
		}

		commitField := loadedCfg.GetCommitField()
		commitFieldGated := false

		statuses := make([]string, 0, len(loadedCfg.Items.RequireFieldsOn))
		for status := range loadedCfg.Items.RequireFieldsOn {
			statuses = append(statuses, status)
		}
		sort.Strings(statuses)

		requiredFields := make([]requiredFieldsEntry, 0, len(statuses))
		for _, status := range statuses {
			fields := loadedCfg.Items.RequireFieldsOn[status]
			requiredFields = append(requiredFields, requiredFieldsEntry{Status: status, Fields: fields})
			for _, f := range fields {
				if f == commitField {
					commitFieldGated = true
				}
			}
		}

		completionCommitGated := false
		for _, f := range loadedCfg.RequiredFieldsFor("completed") {
			if f == commitField {
				completionCommitGated = true
				break
			}
		}

		data := promptData{
			Types:                 loadedCfg.TypeList(),
			Statuses:              loadedCfg.StatusList(),
			Priorities:            loadedCfg.PriorityList(),
			RequiredFields:        requiredFields,
			CommitField:           commitField,
			CommitFieldGated:      commitFieldGated,
			CompletionCommitGated: completionCommitGated,
			TypeRanks:             rankLines(loadedCfg),
		}

		return tmpl.Execute(os.Stdout, data)
	},
}

func RegisterPrimeCmd(root *cobra.Command) {
	root.AddCommand(primeCmd)
}
