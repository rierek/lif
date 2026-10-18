package config

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/rierek/lif/pkg/item"
	"gopkg.in/yaml.v3"
)

// Hook runs a task when an item event matches all conditions.
type Hook struct {
	Stage       string   `yaml:"stage,omitempty"` // "" means after
	Event       []string `yaml:"event"`
	Changed     []string `yaml:"changed,omitempty"`
	From        []string `yaml:"from,omitempty"`
	To          []string `yaml:"to,omitempty"`
	Type        []string `yaml:"type,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
	TagsAdded   []string `yaml:"tags_added,omitempty"`
	TagsRemoved []string `yaml:"tags_removed,omitempty"`
	Run         string   `yaml:"run"`
	Args        []string `yaml:"args,omitempty"`
	Message     string   `yaml:"message,omitempty"`
	Timeout     *int     `yaml:"timeout,omitempty" json:",omitempty"` // seconds; nil means DefaultHookTimeout, 0 means none
}

// DefaultHookTimeout bounds a hook or gate task that sets no timeout.
const DefaultHookTimeout = 60 * time.Second

// TimeoutDuration returns the time that the task of h may run; 0 means no limit.
func (h Hook) TimeoutDuration() time.Duration {
	if h.Timeout == nil {
		return DefaultHookTimeout
	}
	return time.Duration(*h.Timeout) * time.Second
}

// IsGate reports whether the hook runs before the write and can block it.
func (h Hook) IsGate() bool { return h.Stage == "before" }

var HookEvents = []string{"created", "updated"}
var HookStages = []string{"after", "before"}

// HookBuiltinWatched are the built-in watched fields, in Changed order; extra keys follow, sorted.
var HookBuiltinWatched = []string{"status", "type", "parent", "tags"}

var HookBuiltinItemFields = []string{"id", "title", "status", "type", "parent", "tags",
	"prev_status", "prev_type", "prev_parent", "prev_tags", "changed"}

// UnmarshalYAML decodes a Hook and rejects non-mappings and unknown keys, so a
// typo such as "on:" fails loudly instead of yielding a hook without event.
func (h *Hook) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: a hook must be a mapping with event and run", value.Line)
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		keyNode := value.Content[i]
		switch keyNode.Value {
		case "stage", "event", "changed", "from", "to", "type", "tags", "tags_added", "tags_removed", "run", "args", "message", "timeout":
		default:
			return fmt.Errorf("line %d: unknown hook key %q (known: stage, event, changed, from, to, type, tags, tags_added, tags_removed, run, args, message, timeout)", keyNode.Line, keyNode.Value)
		}
	}
	type plain Hook
	var raw plain
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*h = Hook(raw)
	return nil
}

// validateHooks returns the first problem in hooks, visiting names sorted. c
// is nil for the global file: statuses and types differ per project, so those
// two checks are skipped there. Whether Run names a task resolves at fire time.
func validateHooks(hooks map[string]Hook, c *Config) error {
	names := make([]string, 0, len(hooks))
	for name := range hooks {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		h := hooks[name]
		if !taskNameRe.MatchString(name) {
			return fmt.Errorf(`hooks.%s: invalid hook name (use lowercase letters, digits, "-" and "_")`, name)
		}
		if h.Stage != "" && !slices.Contains(HookStages, h.Stage) {
			return fmt.Errorf("hooks.%s.stage: unknown stage %q (known: after, before)", name, h.Stage)
		}
		if len(h.Event) == 0 {
			return fmt.Errorf("hooks.%s: no event", name)
		}
		for _, ev := range h.Event {
			if !slices.Contains(HookEvents, ev) {
				return fmt.Errorf("hooks.%s.event: unknown event %q (known: created, updated)", name, ev)
			}
		}
		onlyUpdated := len(h.Event) == 1 && h.Event[0] == "updated"
		if h.IsGate() && !onlyUpdated {
			return fmt.Errorf("hooks.%s.stage: before is only valid with event: [updated]", name)
		}
		if h.Message != "" && !h.IsGate() {
			return fmt.Errorf("hooks.%s.message: only valid with stage: before", name)
		}
		if len(h.From) > 0 && !onlyUpdated {
			return fmt.Errorf("hooks.%s.from: only valid with event: [updated]", name)
		}
		for _, key := range h.Changed {
			if !slices.Contains(HookBuiltinWatched, key) && !item.IsWatchableKey(key) {
				return fmt.Errorf("hooks.%s.changed: %q is not watchable (use status, type, parent, tags or an extra front matter key)", name, key)
			}
		}
		for _, arg := range h.Args {
			if !isHookItemField(arg) {
				return fmt.Errorf("hooks.%s.args: unknown item field %q (built-in: %s; or an extra front matter key)", name, arg, strings.Join(HookBuiltinItemFields, ", "))
			}
		}
		if c != nil {
			for _, f := range []struct {
				key  string
				list []string
			}{{"from", h.From}, {"to", h.To}} {
				for _, s := range f.list {
					if !c.IsValidStatus(s) {
						return fmt.Errorf("hooks.%s.%s: unknown status %q (valid: %s)", name, f.key, s, strings.Join(c.StatusNames(), ", "))
					}
				}
			}
			for _, ty := range h.Type {
				if !c.IsValidType(ty) {
					return fmt.Errorf("hooks.%s.type: unknown type %q (valid: %s)", name, ty, strings.Join(c.TypeNames(), ", "))
				}
			}
		}
		for _, f := range []struct {
			key  string
			list []string
		}{{"tags", h.Tags}, {"tags_added", h.TagsAdded}, {"tags_removed", h.TagsRemoved}} {
			for _, tag := range f.list {
				if err := item.ValidateTag(tag); err != nil {
					return fmt.Errorf("hooks.%s.%s: %v", name, f.key, err)
				}
			}
		}
		if h.Timeout != nil && *h.Timeout < 0 {
			return fmt.Errorf("hooks.%s.timeout: must be 0 (no limit) or more seconds", name)
		}
		if h.Run == "" {
			return fmt.Errorf("hooks.%s: no run task", name)
		}
		if !taskNameRe.MatchString(h.Run) {
			return fmt.Errorf(`hooks.%s.run: invalid task name %q (use lowercase letters, digits, "-" and "_")`, name, h.Run)
		}
	}
	return nil
}

// isHookItemField reports whether arg is a built-in item field or <key> /
// prev_<key> of a watchable extra key.
func isHookItemField(arg string) bool {
	return slices.Contains(HookBuiltinItemFields, arg) ||
		item.IsWatchableKey(arg) ||
		strings.HasPrefix(arg, "prev_") && item.IsWatchableKey(strings.TrimPrefix(arg, "prev_"))
}
