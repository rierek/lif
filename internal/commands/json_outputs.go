package commands

import (
	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/version"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
)

// JSONOutputs maps the name of a published JSON Schema file to a value of the
// Go type that a --json command encodes. cmd/lif-schemagen generates
// schemas/<name>.schema.json from it. A command that gains a --json shape
// adds the shape here, or its output has no schema.
//
// "item" is the object of show, create, update, list and the other item
// verbs; "response" is the envelope of archive, next, init, a partial batch
// failure and every error.
func JSONOutputs() map[string]any {
	return map[string]any{
		"item":        item.Item{},
		"items":       []*item.Item{},
		"response":    output.Response{},
		"check":       checkResult{},
		"graph":       graphModel{},
		"milestones":  []milestoneEntry{},
		"progress":    progressResult{},
		"rename":      renameResult{},
		"rename-plan": itemcore.RenamePlan{},
		"roadmap":     roadmapData{},
		"version":     version.Info{},
	}
}
