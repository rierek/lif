package hooks

import (
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemgraph"
)

// itemFields returns the item fields that a hook can name in args, as the
// strings that the task gets: the built-ins id, title, status, type, parent,
// tags, prev_status, prev_type, prev_parent, prev_tags and changed, plus
// <key> and prev_<key> for every watchable extra key. A built-in name wins
// over an extra key of the same name.
func itemFields(m itemgraph.Mutation) map[string]string {
	cur, prev := orEmpty(m.Item), orEmpty(m.Prev)
	fields := make(map[string]string)
	for _, key := range extraKeys(cur, prev) {
		fields[key] = extraValue(cur.Extra, key)
		fields["prev_"+key] = extraValue(prev.Extra, key)
	}
	maps.Copy(fields, map[string]string{
		"id":          cur.ID,
		"title":       cur.Title,
		"status":      cur.Status,
		"type":        cur.Type,
		"parent":      cur.Parent,
		"tags":        strings.Join(cur.Tags, " "),
		"prev_status": prev.Status,
		"prev_type":   prev.Type,
		"prev_parent": prev.Parent,
		"prev_tags":   strings.Join(prev.Tags, " "),
		"changed":     strings.Join(m.Changed, " "),
	})
	return fields
}

// env returns the environment that the task of a hook gets on top of the
// one of tasks.Run. LIF_NO_HOOKS=1 keeps every lif call inside the task from
// firing hooks. All values come from itemFields or share its formatting, so
// that args and environment agree.
func (d *Dispatcher) env(hook, stage string, m itemgraph.Mutation) []string {
	cur, prev := orEmpty(m.Item), orEmpty(m.Prev)
	f := itemFields(m)
	env := []string{
		"LIF_NO_HOOKS=1",
		"LIF_HOOK_DEPTH=" + strconv.Itoa(hookDepth()+1),
		"LIF_HOOK=" + hook,
		"LIF_STAGE=" + stage,
		"LIF_EVENT=" + m.Event,
		"LIF_CHANGED=" + f["changed"],
		"LIF_ITEM_ID=" + f["id"],
		"LIF_ITEM_PATH=" + filepath.Join(d.storeRoot, cur.Path),
		"LIF_STATUS=" + f["status"],
		"LIF_TYPE=" + f["type"],
		"LIF_PARENT=" + f["parent"],
		"LIF_TAGS=" + f["tags"],
		"LIF_PREV_STATUS=" + f["prev_status"],
		"LIF_PREV_TYPE=" + f["prev_type"],
		"LIF_PREV_PARENT=" + f["prev_parent"],
		"LIF_PREV_TAGS=" + f["prev_tags"],
	}
	owner := make(map[string]string) // variable suffix -> the extra key that holds it
	for _, key := range extraKeys(cur, prev) {
		suffix := nonEnvName.ReplaceAllString(strings.ToUpper(key), "_")
		if first, taken := owner[suffix]; taken {
			// Once per process and key, not once per hook and event.
			if id := suffix + "/" + key; !d.clashed[id] {
				if d.clashed == nil {
					d.clashed = make(map[string]bool)
				}
				d.clashed[id] = true
				fmt.Fprintf(d.warn, "warning: hook %q: extra key %q maps to LIF_FIELD_%s, taken by %q; ignored (args use the real key and have no such clash)\n", hook, key, suffix, first)
			}
			continue
		}
		owner[suffix] = key
		env = append(env,
			"LIF_FIELD_"+suffix+"="+extraValue(cur.Extra, key),
			"LIF_PREV_FIELD_"+suffix+"="+extraValue(prev.Extra, key))
	}
	return env
}

var nonEnvName = regexp.MustCompile(`[^A-Z0-9]`)

// extraKeys returns the watchable extra keys that cur or prev holds, sorted.
func extraKeys(cur, prev *item.Item) []string {
	keys := make(map[string]bool)
	for _, extra := range []map[string]any{cur.Extra, prev.Extra} {
		for key := range extra {
			if item.IsWatchableKey(key) {
				keys[key] = true
			}
		}
	}
	return slices.Sorted(maps.Keys(keys))
}

// extraValue formats the value of key; a missing key is "".
func extraValue(extra map[string]any, key string) string {
	if v, ok := extra[key]; ok {
		return formatValue(v)
	}
	return ""
}

// formatValue renders an extra value: a string as is, a number or bool via
// fmt.Sprint, a list of scalars space-separated, anything else as JSON.
func formatValue(v any) string {
	if s, ok := scalar(v); ok {
		return s
	}
	if list := reflect.ValueOf(v); list.Kind() == reflect.Slice {
		parts := make([]string, list.Len())
		for i := range parts {
			s, ok := scalar(list.Index(i).Interface())
			if !ok {
				return asJSON(v)
			}
			parts[i] = s
		}
		return strings.Join(parts, " ")
	}
	return asJSON(v)
}

// scalar renders a string, number or bool.
func scalar(v any) (string, bool) {
	switch rv := reflect.ValueOf(v); rv.Kind() {
	case reflect.String:
		return rv.String(), true
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return fmt.Sprint(v), true
	}
	return "", false
}

func asJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}
