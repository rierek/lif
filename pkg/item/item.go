package item

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/adrg/frontmatter"
	"gopkg.in/yaml.v3"
)

// tagPattern matches valid tags: lowercase letters, numbers, and hyphens.
// Must start with a letter, can contain hyphens but not consecutively or at the end.
var tagPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

// ValidateTag checks if a tag is valid (lowercase, URL-safe, single word).
func ValidateTag(tag string) error {
	if tag == "" {
		return fmt.Errorf("tag cannot be empty")
	}
	if !tagPattern.MatchString(tag) {
		return fmt.Errorf("invalid tag %q: must be lowercase, start with a letter, and contain only letters, numbers, and hyphens", tag)
	}
	return nil
}

// NormalizeTag converts a tag to its canonical form (lowercase).
func NormalizeTag(tag string) string {
	return strings.ToLower(strings.TrimSpace(tag))
}

// HasTag returns true if the item has the specified tag.
func (b *Item) HasTag(tag string) bool {
	normalized := NormalizeTag(tag)
	for _, t := range b.Tags {
		if t == normalized {
			return true
		}
	}
	return false
}

// AddTag adds a tag to the item if it doesn't already exist.
// Returns an error if the tag is invalid.
func (b *Item) AddTag(tag string) error {
	normalized := NormalizeTag(tag)
	if err := ValidateTag(normalized); err != nil {
		return err
	}
	if !b.HasTag(normalized) {
		b.Tags = append(b.Tags, normalized)
	}
	return nil
}

// RemoveTag removes a tag from the item.
func (b *Item) RemoveTag(tag string) {
	normalized := NormalizeTag(tag)
	result := make([]string, 0, len(b.Tags))
	for _, t := range b.Tags {
		if t != normalized {
			result = append(result, t)
		}
	}
	b.Tags = result
}

// HasParent returns true if the item has a parent.
func (b *Item) HasParent() bool {
	return b.Parent != ""
}

// IsBlocking returns true if this item is blocking the given item ID.
func (b *Item) IsBlocking(id string) bool {
	for _, target := range b.Blocking {
		if target == id {
			return true
		}
	}
	return false
}

// AddBlocking adds an item ID to the blocking list if not already present.
func (b *Item) AddBlocking(id string) {
	if !b.IsBlocking(id) {
		b.Blocking = append(b.Blocking, id)
	}
}

// RemoveBlocking removes an item ID from the blocking list.
func (b *Item) RemoveBlocking(id string) {
	result := make([]string, 0, len(b.Blocking))
	for _, target := range b.Blocking {
		if target != id {
			result = append(result, target)
		}
	}
	b.Blocking = result
}

// IsBlockedBy returns true if this item is blocked by the given item ID.
func (b *Item) IsBlockedBy(id string) bool {
	for _, blocker := range b.BlockedBy {
		if blocker == id {
			return true
		}
	}
	return false
}

// AddBlockedBy adds an item ID to the blocked-by list if not already present.
func (b *Item) AddBlockedBy(id string) {
	if !b.IsBlockedBy(id) {
		b.BlockedBy = append(b.BlockedBy, id)
	}
}

// RemoveBlockedBy removes an item ID from the blocked-by list.
func (b *Item) RemoveBlockedBy(id string) {
	result := make([]string, 0, len(b.BlockedBy))
	for _, blocker := range b.BlockedBy {
		if blocker != id {
			result = append(result, blocker)
		}
	}
	b.BlockedBy = result
}

// Item represents an issue stored as a markdown file with front matter.
type Item struct {
	// ID is the unique NanoID identifier (from filename).
	ID string `yaml:"-" json:"id"`
	// Slug is the optional human-readable part of the filename.
	Slug string `yaml:"-" json:"slug,omitempty"`
	// Path is the relative path from .beans/ root (e.g., "epic-auth/abc123-login.md").
	Path string `yaml:"-" json:"path"`

	// Front matter fields

	// Title is the human-readable title.
	Title     string     `yaml:"title" json:"title"`
	Status    string     `yaml:"status" json:"status"`
	Type      string     `yaml:"type,omitempty" json:"type,omitempty"`
	Priority  string     `yaml:"priority,omitempty" json:"priority,omitempty"`
	Tags      []string   `yaml:"tags,omitempty" json:"tags,omitempty"`
	CreatedAt *time.Time `yaml:"created_at,omitempty" json:"created_at,omitempty"`
	UpdatedAt *time.Time `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`

	// Order is a fractional index string for manual sorting.
	Order string `yaml:"order,omitempty" json:"order,omitempty"`

	// Body is the markdown content after the front matter.
	Body string `yaml:"-" json:"body,omitempty"`

	// Parent is the optional parent item ID (milestone, epic, or feature).
	Parent string `yaml:"parent,omitempty" json:"parent,omitempty"`

	// Blocking is a list of item IDs that this item is blocking.
	Blocking []string `yaml:"blocking,omitempty" json:"blocking,omitempty"`

	// BlockedBy is a list of item IDs that are blocking this item.
	BlockedBy []string `yaml:"blocked_by,omitempty" json:"blocked_by,omitempty"`

	// Extra holds front matter keys not covered by any typed field above.
	Extra map[string]any `yaml:"-" json:"extra,omitempty"`

	// RecordedID is the ID the file names itself in the "# <id>" comment on
	// its first front matter line, or "" when it has none. ID comes from the
	// file name, so the two differ after a hand rename; lif check reports
	// that. It follows the bytes that back the item (see SetContentETag) and
	// is never written or serialized.
	RecordedID string `yaml:"-" json:"-"`

	// contentETag is the hash of the bytes this item was parsed from, or of
	// the bytes last written for it. It is the single source of the etag:
	// see ETag. Unexported so neither YAML nor JSON ever carries it.
	contentETag string
}

// frontMatter is the subset of Item that gets serialized to YAML front matter.
type frontMatter struct {
	Title     string     `yaml:"title"`
	Status    string     `yaml:"status"`
	Type      string     `yaml:"type,omitempty"`
	Priority  string     `yaml:"priority,omitempty"`
	Tags      []string   `yaml:"tags,omitempty"`
	CreatedAt *time.Time `yaml:"created_at,omitempty"`
	UpdatedAt *time.Time `yaml:"updated_at,omitempty"`
	Order     string     `yaml:"order,omitempty"`
	Parent    string     `yaml:"parent,omitempty"`
	Blocking  []string   `yaml:"blocking,omitempty"`
	BlockedBy []string   `yaml:"blocked_by,omitempty"`
}

// knownFrontMatterKeys are the YAML keys owned by a typed field on frontMatter.
var knownFrontMatterKeys = map[string]bool{
	"title":      true,
	"status":     true,
	"type":       true,
	"priority":   true,
	"tags":       true,
	"created_at": true,
	"updated_at": true,
	"order":      true,
	"parent":     true,
	"blocking":   true,
	"blocked_by": true,
}

// reservedKeyFlags maps a known front matter key to the native CLI flag that
// writes it. Keys with no direct write flag (created_at, updated_at, order)
// map to "" and get a generic "managed field" error instead of a flag name.
var reservedKeyFlags = map[string]string{
	"title":      "--title",
	"status":     "--status",
	"type":       "--type",
	"priority":   "--priority",
	"tags":       "--tag",
	"created_at": "",
	"updated_at": "",
	"order":      "",
	"parent":     "--parent",
	"blocking":   "--blocking",
	"blocked_by": "--blocked-by",
}

// ReservedKeyFlag reports whether key is a known front matter field and, if
// so, the native CLI flag that writes it. A known key with no direct write
// flag (e.g. "order") returns known=true and flag="" -- callers should fall
// back to a "managed field" message rather than inventing a flag name.
func ReservedKeyFlag(key string) (flag string, known bool) {
	flag, known = reservedKeyFlags[key]
	return flag, known
}

// normalizeYAMLValue converts the map[interface{}]interface{} produced by the
// yaml.v2 decoder (used internally by the frontmatter library) into
// map[string]any, recursively, so Extra stays JSON-marshalable. It refuses
// NaN and infinite numbers, the one kind of value a decoder accepts and JSON
// cannot carry; path (meta.ratio, scores[1]) names the key in the error.
// It works in place: the decoder output is owned by the caller.
func normalizeYAMLValue(path string, v any) (any, error) {
	switch typed := v.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]any, len(typed))
		for k, val := range typed {
			m[fmt.Sprintf("%v", k)] = val
		}
		return normalizeMap(path, m)
	case map[string]interface{}: // TOML and JSON front matter decode to this
		return normalizeMap(path, typed)
	case []map[string]interface{}: // a TOML array of tables
		for i, m := range typed {
			if _, err := normalizeMap(fmt.Sprintf("%s[%d]", path, i), m); err != nil {
				return nil, err
			}
		}
		return typed, nil
	case []interface{}:
		for i, val := range typed {
			n, err := normalizeYAMLValue(fmt.Sprintf("%s[%d]", path, i), val)
			if err != nil {
				return nil, err
			}
			typed[i] = n
		}
		return typed, nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return nil, fmt.Errorf("key %q holds %v: only finite numbers are supported, quote the value to keep it as text", path, typed)
		}
	}
	return v, nil
}

func normalizeMap(path string, m map[string]any) (any, error) {
	for k, val := range m {
		n, err := normalizeYAMLValue(path+"."+k, val)
		if err != nil {
			return nil, err
		}
		m[k] = n
	}
	return m, nil
}

// utf8BOM is the byte order mark some editors put in front of the first line
// of a file. The front matter reader does not know it and would treat the
// whole file as body.
var utf8BOM = []byte("\xEF\xBB\xBF")

// Parse reads an item from a reader (markdown with YAML front matter). A
// leading UTF-8 byte order mark is ignored; the etag still hashes the bytes
// as they are on disk, because Core.Update hashes the file the same way.
func Parse(r io.Reader) (*Item, error) {
	onDisk, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading item: %w", err)
	}
	data := bytes.TrimPrefix(onDisk, utf8BOM)

	var fm frontMatter
	body, err := frontmatter.Parse(bytes.NewReader(data), &fm)
	if err != nil {
		return nil, fmt.Errorf("parsing front matter: %w", err)
	}

	raw := map[string]any{}
	if _, err := frontmatter.Parse(bytes.NewReader(data), &raw); err != nil {
		return nil, fmt.Errorf("parsing front matter: %w", err)
	}
	var extra map[string]any
	for key, value := range raw {
		if knownFrontMatterKeys[key] {
			continue
		}
		if extra == nil {
			extra = make(map[string]any, len(raw))
		}
		normalized, err := normalizeYAMLValue(key, value)
		if err != nil {
			return nil, fmt.Errorf("parsing front matter: %w", err)
		}
		extra[key] = normalized
	}

	// The trailing newline of a POSIX file and the blank separator line after
	// the front matter are not content. Without the strip, a parsed body would
	// differ from the body of an item just created in memory.
	bodyStr := strings.TrimSuffix(string(body), "\n")
	if rest, ok := strings.CutPrefix(bodyStr, "\r\n"); ok {
		bodyStr = rest
	} else {
		bodyStr = strings.TrimPrefix(bodyStr, "\n")
	}

	return &Item{
		Title:       fm.Title,
		Status:      fm.Status,
		Type:        fm.Type,
		Priority:    fm.Priority,
		Tags:        fm.Tags,
		CreatedAt:   fm.CreatedAt,
		UpdatedAt:   fm.UpdatedAt,
		Order:       fm.Order,
		Body:        bodyStr,
		Parent:      fm.Parent,
		Blocking:    fm.Blocking,
		BlockedBy:   fm.BlockedBy,
		Extra:       extra,
		RecordedID:  recordedID(data),
		contentETag: ETagOf(onDisk),
	}, nil
}

// recordedID returns the ID that the first front matter line of data records
// as a "# <id>" comment (see Render), or "" when there is none. It finds the
// opening delimiter the way the front matter reader does, skipping blank
// lines before it. A comment of several words is a note, not an ID.
func recordedID(data []byte) string {
	next := func() (string, bool) {
		if len(data) == 0 {
			return "", false
		}
		var line []byte
		line, data, _ = bytes.Cut(data, []byte("\n"))
		return string(bytes.TrimSpace(line)), true
	}
	line, ok := next()
	for ok && line == "" {
		line, ok = next()
	}
	if !ok || line != "---" {
		return ""
	}
	if line, ok = next(); !ok {
		return ""
	}
	id, found := strings.CutPrefix(line, "#")
	id = strings.TrimSpace(id)
	if !found || id == "" || strings.ContainsAny(id, " \t") {
		return ""
	}
	return id
}

// renderFrontMatter is used for YAML output with yaml.v3 (supports custom marshalers).
type renderFrontMatter struct {
	Title     string     `yaml:"title"`
	Status    string     `yaml:"status"`
	Type      string     `yaml:"type,omitempty"`
	Priority  string     `yaml:"priority,omitempty"`
	Tags      []string   `yaml:"tags,omitempty"`
	CreatedAt *time.Time `yaml:"created_at,omitempty"`
	UpdatedAt *time.Time `yaml:"updated_at,omitempty"`
	Order     string     `yaml:"order,omitempty"`
	Parent    string     `yaml:"parent,omitempty"`
	Blocking  []string   `yaml:"blocking,omitempty"`
	BlockedBy []string   `yaml:"blocked_by,omitempty"`
}

// Render serializes the item back to markdown with YAML front matter.
func (b *Item) Render() ([]byte, error) {
	fm := renderFrontMatter{
		Title:     b.Title,
		Status:    b.Status,
		Type:      b.Type,
		Priority:  b.Priority,
		Tags:      b.Tags,
		CreatedAt: b.CreatedAt,
		UpdatedAt: b.UpdatedAt,
		Order:     b.Order,
		Parent:    b.Parent,
		Blocking:  b.Blocking,
		BlockedBy: b.BlockedBy,
	}

	var node yaml.Node
	if err := node.Encode(&fm); err != nil {
		return nil, fmt.Errorf("encoding front matter: %w", err)
	}

	if len(b.Extra) > 0 {
		keys := make([]string, 0, len(b.Extra))
		for k := range b.Extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			var keyNode, valueNode yaml.Node
			if err := keyNode.Encode(k); err != nil {
				return nil, fmt.Errorf("encoding extra key %q: %w", k, err)
			}
			if err := valueNode.Encode(b.Extra[k]); err != nil {
				return nil, fmt.Errorf("encoding extra value for key %q: %w", k, err)
			}
			node.Content = append(node.Content, &keyNode, &valueNode)
		}
	}

	fmBytes, err := yaml.Marshal(&node)
	if err != nil {
		return nil, fmt.Errorf("marshaling front matter: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString("---\n")
	if b.ID != "" {
		buf.WriteString("# ")
		buf.WriteString(b.ID)
		buf.WriteString("\n")
	}
	buf.Write(fmBytes)
	buf.WriteString("---\n")
	if b.Body != "" {
		// The blank line after the front matter is a separator, not content;
		// Parse strips exactly one, so a body that starts with a blank line
		// still needs its own separator to survive the round trip.
		buf.WriteString("\n")
		buf.WriteString(b.Body)
		// Ensure trailing newline if body doesn't end with one
		if !strings.HasSuffix(b.Body, "\n") {
			buf.WriteString("\n")
		}
	} else {
		// Even without body, add trailing newline for POSIX compliance
		buf.WriteString("\n")
	}

	return buf.Bytes(), nil
}

// ETag returns the item's etag for optimistic concurrency control: the FNV-1a
// 64-bit hash of its serialized bytes, as a 16-character hex string.
//
// The bytes are the ones the item was parsed from, or the ones last written
// for it -- never a fresh Render() of the in-memory struct, which diverges
// from the file in two ways that made this token useless: a loader default
// applied in memory (an absent priority becoming "normal") adds a key the
// file never had, and any in-memory mutation before the write would report
// the etag of the new state instead of the one being replaced.
//
// An item that has never touched a file -- freshly constructed in memory or in
// a test -- has no such bytes; for it the render is the only definition
// available and is used as the fallback.
func (b *Item) ETag() string {
	if b.contentETag != "" {
		return b.contentETag
	}
	content, err := b.Render()
	if err != nil {
		// Return a sentinel value that will never match a real ETag,
		// ensuring validation will fail rather than silently passing.
		return "0000000000000000"
	}
	return ETagOf(content)
}

// ETagOf hashes raw content the way ETag does.
func ETagOf(content []byte) string {
	h := fnv.New64a()
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// MarshalJSON implements json.Marshaler to include computed etag field.
func (b *Item) MarshalJSON() ([]byte, error) {
	type ItemAlias Item // Avoid infinite recursion
	return json.Marshal(&struct {
		*ItemAlias
		ETag string `json:"etag"`
	}{
		ItemAlias: (*ItemAlias)(b),
		ETag:      b.ETag(),
	})
}

// SetContentETag records the bytes now backing this item, so that ETag keeps
// reporting the token the write path validates against, and RecordedID the ID
// those bytes record. Callers that persist a item MUST call this with the
// exact bytes they wrote.
func (b *Item) SetContentETag(content []byte) {
	b.contentETag = ETagOf(content)
	b.RecordedID = recordedID(bytes.TrimPrefix(content, utf8BOM))
}

// Clone returns an independent copy of the item, safe to hand to a reader
// that outlives the lock the original is protected by. The struct copy
// carries every scalar field including the unexported content etag; the
// slices and the Extra map are rebuilt so a caller cannot write through
// them into the original.
//
// Values nested inside Extra are copied by reference: front matter parses
// into scalars, slices and maps, and a caller that reaches into a nested
// container still shares it with the original. Callers that mutate Extra
// replace whole keys, which the rebuilt map already isolates.
func (b *Item) Clone() *Item {
	if b == nil {
		return nil
	}

	clone := *b

	if b.Tags != nil {
		clone.Tags = append([]string(nil), b.Tags...)
	}
	if b.Blocking != nil {
		clone.Blocking = append([]string(nil), b.Blocking...)
	}
	if b.BlockedBy != nil {
		clone.BlockedBy = append([]string(nil), b.BlockedBy...)
	}
	if b.Extra != nil {
		clone.Extra = make(map[string]any, len(b.Extra))
		for k, v := range b.Extra {
			clone.Extra[k] = v
		}
	}
	if b.CreatedAt != nil {
		createdAt := *b.CreatedAt
		clone.CreatedAt = &createdAt
	}
	if b.UpdatedAt != nil {
		updatedAt := *b.UpdatedAt
		clone.UpdatedAt = &updatedAt
	}

	return &clone
}
