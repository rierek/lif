// Generates the JSON Schema files for the --json output of lif from the Go
// types the commands encode. Run it from the repository root: `just schemas`.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/invopop/jsonschema"

	"github.com/rierek/lif/internal/commands"
	"github.com/rierek/lif/pkg/item"
)

const (
	// modulePath is the import path the comment extraction keys types by.
	modulePath = "github.com/rierek/lif"

	// itemDef names the shared definition of an item in every schema file.
	itemDef = "Item"
)

// commentDirs are the packages whose Go doc comments become "description".
var commentDirs = []string{"pkg/item", "pkg/itemcore", "internal/output", "internal/version"}

// itemJSON is the JSON shape of an item. item.Item.MarshalJSON adds the
// computed "etag" to the struct fields; reflection cannot see that, so the
// shape is spelled out here. The generator test marshals a real item and
// validates it against the generated schema, which fails when the two drift.
type itemJSON struct {
	item.Item
	ETag string `json:"etag" jsonschema_description:"Hash of the item file; the value --if-match expects."`
}

func main() {
	out := "schemas"
	if len(os.Args) == 2 {
		out = os.Args[1]
	} else if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: lif-schemagen [output-dir]")
		os.Exit(2)
	}
	if err := generate(out); err != nil {
		fmt.Fprintln(os.Stderr, "lif-schemagen:", err)
		os.Exit(1)
	}
}

// generate writes one <name>.schema.json per entry of commands.JSONOutputs
// into out. The working directory must be the repository root, because the
// doc comments are read from source.
func generate(out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	files, err := schemas()
	if err != nil {
		return err
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(out, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// schemas renders every schema file, keyed by file name.
func schemas() (map[string][]byte, error) {
	itemType := reflect.TypeOf(item.Item{})
	newReflector := func() (*jsonschema.Reflector, error) {
		r := &jsonschema.Reflector{
			Anonymous:                 true,
			AllowAdditionalProperties: true, // new fields are additive, not breaking
			Mapper: func(t reflect.Type) *jsonschema.Schema {
				if t == itemType {
					return &jsonschema.Schema{Ref: "#/$defs/" + itemDef}
				}
				return nil
			},
		}
		for _, dir := range commentDirs {
			if err := r.AddGoComments(modulePath, dir); err != nil {
				return nil, err
			}
		}
		return r, nil
	}

	// The item definition is reflected from itemJSON, with the item mapping
	// switched off so the root of that run is the object itself.
	ir, err := newReflector()
	if err != nil {
		return nil, err
	}
	ir.Mapper = nil
	ir.ExpandedStruct = true
	itemSchema := ir.Reflect(itemJSON{})
	itemSchema.Version = ""
	itemSchema.Definitions = nil
	itemSchema.Title = "Item"
	itemSchema.Description = "An item as printed by show, create, update, list and the other item verbs."

	files := map[string][]byte{}
	for name, value := range commands.JSONOutputs() {
		r, err := newReflector()
		if err != nil {
			return nil, err
		}
		var s *jsonschema.Schema
		if name == "item" {
			s = &jsonschema.Schema{Version: jsonschema.Version, Ref: "#/$defs/" + itemDef}
		} else {
			s = r.Reflect(value)
		}
		if s.Definitions == nil {
			s.Definitions = jsonschema.Definitions{}
		}
		s.Definitions[itemDef] = itemSchema
		if name != "item" && !usesItem(s) {
			delete(s.Definitions, itemDef)
		}
		allowNullCollections(s)
		if name == "items" {
			// `lif list --json` prints null, not [], when nothing matches.
			s = &jsonschema.Schema{
				Version:     s.Version,
				Definitions: s.Definitions,
				AnyOf:       []*jsonschema.Schema{{Type: "array", Items: s.Items}, {Type: "null"}},
			}
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false) // keep "->" readable in descriptions
		enc.SetIndent("", "  ")
		if err := enc.Encode(s); err != nil {
			return nil, err
		}
		files[name+".schema.json"] = buf.Bytes()
	}
	return files, nil
}

// usesItem reports whether the schema refers to the shared item definition.
func usesItem(s *jsonschema.Schema) bool {
	data, _ := json.Marshal(s)
	return strings.Contains(string(data), `"$ref":"#/$defs/`+itemDef+`"`)
}

// allowNullCollections lets every required array and map property also be
// null. Go encodes a nil slice or map as null, and the commands do not
// normalise that (for example `lif check --json` prints "config_errors": null
// when there are none).
func allowNullCollections(s *jsonschema.Schema) {
	for _, def := range s.Definitions {
		if def.Properties == nil {
			continue
		}
		var names []string
		for p := def.Properties.Oldest(); p != nil; p = p.Next() {
			if slices.Contains(def.Required, p.Key) && (p.Value.Type == "array" || p.Value.AdditionalProperties != nil && p.Value.Type == "object") {
				names = append(names, p.Key)
			}
		}
		for _, name := range names {
			inner, _ := def.Properties.Get(name)
			wrapped := &jsonschema.Schema{
				Description: inner.Description,
				AnyOf:       []*jsonschema.Schema{inner, {Type: "null"}},
			}
			inner.Description = ""
			def.Properties.Set(name, wrapped)
		}
	}
}
