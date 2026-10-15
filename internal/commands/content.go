package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rierek/lif/internal/gitutil"
	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/pkg/item"
)

// resolveContent returns content from a direct value or file flag.
// If value is "-", reads from stdin.
func resolveContent(value, file string) (string, error) {
	if value != "" && file != "" {
		return "", fmt.Errorf("cannot use both --body and --body-file")
	}

	if value == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return string(data), nil
	}

	if value != "" {
		return item.UnescapeBody(value), nil
	}

	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("reading file: %w", err)
		}
		return string(data), nil
	}

	return "", nil
}

// applyTags adds tags to an item, returning an error if any tag is invalid.
func applyTags(b *item.Item, tags []string) error {
	for _, tag := range tags {
		if err := b.AddTag(tag); err != nil {
			return err
		}
	}
	return nil
}

// formatCycle formats a cycle path for display.
func formatCycle(path []string) string {
	return strings.Join(path, " → ")
}

// cmdError returns an appropriate error for JSON or text mode.
// Note: Use %v instead of %w for error arguments - wrapping is not preserved in JSON mode.
func cmdError(jsonMode bool, code string, format string, args ...any) error {
	if jsonMode {
		return output.Error(code, fmt.Sprintf(format, args...))
	}
	return fmt.Errorf(format, args...)
}

// mergeTags combines existing tags with additions and removals.
func mergeTags(existing, add, remove []string) []string {
	tags := make(map[string]bool)
	for _, t := range existing {
		tags[t] = true
	}
	for _, t := range add {
		tags[t] = true
	}
	for _, t := range remove {
		delete(tags, t)
	}
	result := make([]string, 0, len(tags))
	for t := range tags {
		result = append(result, t)
	}
	return result
}

// applyBodyReplace replaces exactly one occurrence of old with new.
// Returns an error if old is not found or found multiple times.
func applyBodyReplace(body, old, new string) (string, error) {
	return item.ReplaceOnce(body, old, new)
}

// applyBodyAppend appends text to the body with a newline separator.
func applyBodyAppend(body, text string) string {
	return item.AppendWithSeparator(body, text)
}

// parseSetPair splits a flag-value argument on the first "=". Only the
// first "=" is significant, so values may contain their own "=" (e.g. URLs).
// flagName should be the actual flag the user typed (e.g. "--set", "--where").
func parseSetPair(arg string, flagName string) (key, value string, err error) {
	idx := strings.Index(arg, "=")
	if idx < 0 {
		return "", "", fmt.Errorf("invalid %s value %q: expected key=value", flagName, arg)
	}
	key = arg[:idx]
	if key == "" {
		return "", "", fmt.Errorf("invalid %s value %q: key cannot be empty", flagName, arg)
	}
	return key, arg[idx+1:], nil
}

// checkReservedKey returns an error naming the native flag for key if key is
// a field of the known item schema, nil otherwise. Known keys without a
// direct write flag (created_at, updated_at, order) get a "managed field"
// error instead of a fabricated flag name.
func checkReservedKey(key string) error {
	flag, known := item.ReservedKeyFlag(key)
	if !known {
		return nil
	}
	if flag == "" {
		return fmt.Errorf("%q is a managed field and cannot be set directly", key)
	}
	return fmt.Errorf("%q is a reserved field; use %s instead", key, flag)
}

// validateExtraKeys checks every --set and --unset argument up front: --set
// entries must carry "=" (a usage error otherwise) and neither --set nor
// --unset may name a reserved schema field.
func validateExtraKeys(sets []string, unsets []string) error {
	for _, s := range sets {
		key, _, err := parseSetPair(s, "--set")
		if err != nil {
			return err
		}
		if err := checkReservedKey(key); err != nil {
			return err
		}
	}
	for _, key := range unsets {
		if key == "" {
			return fmt.Errorf("invalid --unset value: key cannot be empty")
		}
		if err := checkReservedKey(key); err != nil {
			return err
		}
	}
	return nil
}

// applyExtraOps applies --set and --unset operations to an item's Extra map.
// --set operations are applied first, then --unset removes keys -- a key
// named by both --set and --unset in the same invocation ends up unset. This
// keeps --unset predictable as the more destructive, last-word operation.
// Callers must run validateExtraKeys first; a malformed --set here returns
// an error but any already-applied --set writes are not rolled back.
func applyExtraOps(b *item.Item, sets []string, unsets []string) error {
	for _, s := range sets {
		key, value, err := parseSetPair(s, "--set")
		if err != nil {
			return err
		}
		if b.Extra == nil {
			b.Extra = make(map[string]any)
		}
		b.Extra[key] = value
	}
	for _, key := range unsets {
		delete(b.Extra, key)
	}
	return nil
}

// extraSetMap parses validated --set pairs into the map itemcore.WithExtraOps
// takes. Callers must run validateExtraKeys first.
func extraSetMap(sets []string) (map[string]any, error) {
	m := make(map[string]any, len(sets))
	for _, s := range sets {
		key, value, err := parseSetPair(s, "--set")
		if err != nil {
			return nil, err
		}
		m[key] = value
	}
	return m, nil
}

// normalizeCommitSets rewrites every --set pair naming cfg.GetCommitField() so
// the stored value is the full 40-hex SHA, rejecting refs that do not resolve
// in the current working directory's repository.
func normalizeCommitSets(sets []string) ([]string, error) {
	field := cfg.GetCommitField()
	result := make([]string, len(sets))
	for i, s := range sets {
		key, value, err := parseSetPair(s, "--set")
		if err != nil {
			return nil, err
		}
		if key != field {
			result[i] = s
			continue
		}
		dir, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		sha, err := gitutil.ResolveCommit(dir, value)
		if err != nil {
			return nil, err
		}
		result[i] = key + "=" + sha
	}
	return result, nil
}

// resolveAppendContent handles --append value, supporting stdin with "-".
func resolveAppendContent(value string) (string, error) {
	if value == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return strings.TrimRight(string(data), "\n"), nil
	}
	return item.UnescapeBody(value), nil
}
