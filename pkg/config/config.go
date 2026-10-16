package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rierek/lif/pkg/item"
	"gopkg.in/yaml.v3"
)

const (
	// ConfigFileName is the name of the config file at project root.
	ConfigFileName = ".lif.yml"
	// SectionKey is the top-level key of the items section in a config file.
	SectionKey = "lif"
	// DefaultStoreDir is the default directory for storing items.
	DefaultStoreDir = ".lif"
	// StoreEnvVar is the env var that names the store directory.
	StoreEnvVar = "LIF_PATH"

	// The four legacy names. A project made by the former tool keeps them and
	// they are read in place (never renamed). Every legacy name of these four
	// levels is defined here and only here, so dropping the fallback is one
	// deletion per level.
	//
	// LegacyConfigFileName is the former config file name.
	LegacyConfigFileName = ".beans.yml"
	// LegacySectionKey is the former key of the items section.
	LegacySectionKey = "beans"
	// LegacyDefaultStoreDir is the former default store directory.
	LegacyDefaultStoreDir = ".beans"
	// LegacyStoreEnvVar is the former env var that names the store directory.
	LegacyStoreEnvVar = "BEANS_PATH"

	// Legacy names of the worktree, crash-backup and port levels. Like the
	// four above they are read in place and never renamed, but these levels
	// have no ambiguity error: the legacy name works beside the new one.
	//
	// HomeDir is the directory in the user's home that holds the default
	// worktree roots (<home>/<HomeDir>/worktrees/<project>).
	HomeDir = ".lif"
	// LegacyHomeDir is the former home directory of the default worktree roots.
	LegacyHomeDir = ".beans"
	// WorktreeBranchPrefix is the branch prefix of every new worktree.
	WorktreeBranchPrefix = "lif/"
	// LegacyWorktreeBranchPrefix is the former worktree branch prefix. Worktrees
	// on such branches are still listed and used.
	LegacyWorktreeBranchPrefix = "beans/"
	// StagingDirPrefix starts the name of the staging directory of an atomic swap.
	StagingDirPrefix = ".lif-staging-"
	// BackupDirPrefix starts the name of the backup directory of an atomic swap.
	BackupDirPrefix = ".lif.bak-"
	// LegacyBackupDirPrefix starts the name of a backup directory left by the
	// former tool; the orphan scan matches it like BackupDirPrefix.
	LegacyBackupDirPrefix = ".beans.bak-"
	// WorkspacePortEnvVar names the port of a workspace in the terminal
	// environment of its shell and run sessions.
	WorkspacePortEnvVar = "LIF_WORKSPACE_PORT"
	// LegacyWorkspacePortEnvVar is the former name of the same variable; it is
	// exported with the same value.
	LegacyWorkspacePortEnvVar = "BEANS_WORKSPACE_PORT"

	// LegacyConfigFile is the old config file location (deprecated)
	LegacyConfigFile = "config.yaml"
	// DefaultServerPort is the default port for the web server
	DefaultServerPort = 8080
	// AnchorRepoRoot anchors the items directory at the main worktree's root
	// instead of at the config file's directory. Secondary worktrees then share
	// the main workdir's store rather than each carrying their own.
	AnchorRepoRoot = "repo-root"
	// DefaultCommitField is the front matter key used to record a commit SHA
	// when beans.commit_field is not configured.
	DefaultCommitField = "commit"
	// LeafRank is the rank of a type that carries no children. A type without
	// an explicit rank lands here, which keeps the old behaviour where an
	// unknown type was treated like "task".
	LeafRank = 4
	// MaxContainerRank is the highest rank a container type (one that can
	// hold children, as opposed to a leaf rendered inside one) sits on.
	// Ranks 1 through MaxContainerRank are container ranks; the single
	// source for that bound, read by internal/commands' isContainerRank,
	// validateRoadmapRootType, and roadmapScopeTypes instead of each
	// repeating the literal 3 (beans-v2ox).
	MaxContainerRank = 3
	// DefaultMaxWidth is the rendered width cap used when display.max_width
	// is unset. GetMaxWidth() and internal/commands' resolveWidth() both
	// need this exact number - it used to be a literal duplicated in both
	// packages, one edit away from drifting apart.
	DefaultMaxWidth = 110
)

// DefaultStatuses defines the built-in status table. Config.Statuses can
// override individual entries; see (*Config).StatusList.
// Order determines sort priority: in-progress first (active work), then todo, draft, and done states last.
var DefaultStatuses = []StatusConfig{
	{Name: "in-progress", Color: "peach", Short: "I", Description: "Currently being worked on"},
	{Name: "todo", Color: "green", Short: "T", Description: "Ready to be worked on"},
	{Name: "draft", Color: "overlay2", Short: "D", Description: "Needs refinement before it can be worked on"},
	{Name: "completed", Color: "overlay1", Archive: true, Short: "C", Description: "Finished successfully"},
	{Name: "scrapped", Color: "surface2", Archive: true, Short: "S", Description: "Will not be done"},
}

// DefaultTypes defines the built-in type table. Config.Types can override
// individual entries; see (*Config).TypeList.
var DefaultTypes = []TypeConfig{
	{Name: "milestone", Color: "mauve", Rank: 1, Short: "M", Emphasis: true, Description: "A target release or checkpoint; group work that should ship together"},
	{Name: "epic", Color: "blue", Rank: 2, Short: "E", Emphasis: true, Description: "A thematic container for related work; should have child items, not be worked on directly"},
	{Name: "feature", Color: "sapphire", Rank: 3, Short: "F", Description: "A user-facing capability or enhancement"},
	{Name: "bug", Color: "maroon", Rank: LeafRank, Short: "B", Description: "Something that is broken and needs fixing"},
	{Name: "task", Color: "", Rank: LeafRank, Short: "T", Description: "A concrete piece of work to complete (eg. a chore, or a sub-task for a feature)"},
}

// DefaultPriorities defines the built-in priority table. Config.Priorities
// can override individual entries; see (*Config).PriorityList.
// Priorities are ordered from highest to lowest urgency.
var DefaultPriorities = []PriorityConfig{
	{Name: "critical", Color: "red", Symbol: "\u203c", Description: "Urgent, blocking work. When possible, address immediately"},
	{Name: "high", Color: "yellow", Symbol: "!", Description: "Important, should be done before normal work"},
	{Name: "normal", Color: "", Description: "Standard priority"},
	{Name: "low", Color: "overlay0", Symbol: "\u2193", Description: "Less important, can be delayed"},
	{Name: "deferred", Color: "overlay0", Symbol: "\u2192", Description: "Explicitly pushed back, avoid doing unless necessary"},
}

// StatusConfig defines a single status with its display color.
type StatusConfig struct {
	Name    string `yaml:"name"`
	Color   string `yaml:"color"`
	Archive bool   `yaml:"archive,omitempty"`
	// Short is the single-character code the narrow list view renders. Empty
	// means "?" - see internal/ui.ShortStatus.
	Short       string `yaml:"short,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// TypeConfig defines a single item type with its display color.
type TypeConfig struct {
	Name  string `yaml:"name"`
	Color string `yaml:"color"`
	// Rank carries the hierarchy: a parent is valid when the child's rank is
	// strictly greater. Ranks 1 to 3 are container ranks, 4 is the leaf rank.
	// Ranks may be left unoccupied; the rule tolerates the gap.
	Rank int `yaml:"rank,omitempty"`
	// Emphasis renders this type bold across type word, id and title. It is
	// what carries the hierarchy where hue alone cannot: Catppuccin tones are
	// uniformly pastel, so a container would otherwise lose against a leaf.
	Emphasis bool `yaml:"emphasis,omitempty"`
	// Roadmap decides whether this type gets its own section in `lif roadmap`
	// and an entry in `lif milestones`. Nil means visible; only an explicit
	// roadmap: false hides the type. On a container rank (1-3) this also
	// hides its whole subtree; on the leaf rank it is inert, since a leaf is
	// never itself a container to hide beneath.
	Roadmap *bool `yaml:"roadmap,omitempty"`
	// Short is the single-character code the narrow list view renders. Empty
	// means the first letter of the name, upper-cased.
	Short       string `yaml:"short,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// PriorityConfig defines a single priority level with its display color.
type PriorityConfig struct {
	Name  string `yaml:"name"`
	Color string `yaml:"color"`
	// Symbol is the compact glyph the legend and narrow views render for
	// this priority. Empty means no symbol - see internal/ui.GetPrioritySymbol.
	Symbol      string `yaml:"symbol,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// StatusOverride is one entry of Config.Statuses: a status the config wants
// to override (matched by Name) or add. Archive is a pointer so an omitted
// key is structurally distinct from an explicit "archive: false" - a plain
// bool cannot make that distinction, and collapsing it silently flips a
// status like "completed" back to non-archiving on any recolour-only edit.
type StatusOverride struct {
	Name        string `yaml:"name"`
	Color       string `yaml:"color,omitempty"`
	Description string `yaml:"description,omitempty"`
	Archive     *bool  `yaml:"archive,omitempty"`
	// Short is the single-character code the narrow list view renders. Empty
	// means "?" - see internal/ui.ShortStatus.
	Short string `yaml:"short,omitempty"`

	// colorSet, descriptionSet and shortSet record whether "color"/
	// "description"/"short" were named in the YAML source at all, so a merge
	// can tell an explicit "color: ''" apart from color simply never being
	// named. These are not pointers like Archive because an empty string is
	// a legitimate value in their own right (an uncoloured/undescribed
	// status, or a status that deliberately renders no short code) - a
	// pointer would only move the ambiguity onto every Go-constructed
	// StatusOverride elsewhere in the codebase, which never populates the
	// field and would then look identically "unset". These flags are
	// populated by UnmarshalYAML below and default to false for any
	// override built directly in Go, which keeps that existing
	// non-empty-always-applies behaviour unchanged.
	colorSet, descriptionSet, shortSet bool
}

// UnmarshalYAML decodes a StatusOverride while recording which optional
// string keys the source named explicitly (see colorSet/descriptionSet).
func (o *StatusOverride) UnmarshalYAML(value *yaml.Node) error {
	type plain StatusOverride
	var raw plain
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*o = StatusOverride(raw)
	o.colorSet = yamlNodeHasKey(value, "color")
	o.descriptionSet = yamlNodeHasKey(value, "description")
	o.shortSet = yamlNodeHasKey(value, "short")
	return nil
}

// TypeOverride is one entry of Config.Types: a type the config wants to
// override (matched by Name) or add. Emphasis is a pointer for the same
// reason StatusOverride.Archive is: an omitted key must stay structurally
// distinct from an explicit "emphasis: false", or a colour-only recolour of
// e.g. "milestone" would silently clear its emphasis.
type TypeOverride struct {
	Name  string `yaml:"name"`
	Color string `yaml:"color,omitempty"`
	// Rank is a pointer for the same reason Emphasis is: an omitted key must
	// stay distinct from an explicit rank, or a colour-only override would
	// pull its type down to rank 0 and outrank every container.
	Rank        *int   `yaml:"rank,omitempty"`
	Description string `yaml:"description,omitempty"`
	Emphasis    *bool  `yaml:"emphasis,omitempty"`
	// Roadmap decides whether this type gets its own section in `lif roadmap`
	// and an entry in `lif milestones`. Nil means visible; only an explicit
	// roadmap: false hides the type. On a container rank (1-3) this also
	// hides its whole subtree; on the leaf rank it is inert, since a leaf is
	// never itself a container to hide beneath.
	Roadmap *bool `yaml:"roadmap,omitempty"`
	// Short is the single-character code the narrow list view renders. Empty
	// means the first letter of the name, upper-cased.
	Short string `yaml:"short,omitempty"`

	// colorSet, descriptionSet and shortSet record whether their YAML keys
	// were named at all - see StatusOverride.colorSet for why these three
	// stay plain strings (not pointers) and get this treatment instead.
	colorSet, descriptionSet, shortSet bool
}

// UnmarshalYAML decodes a TypeOverride while recording which optional
// string keys the source named explicitly (see colorSet/descriptionSet/shortSet).
func (o *TypeOverride) UnmarshalYAML(value *yaml.Node) error {
	type plain TypeOverride
	var raw plain
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*o = TypeOverride(raw)
	o.colorSet = yamlNodeHasKey(value, "color")
	o.descriptionSet = yamlNodeHasKey(value, "description")
	o.shortSet = yamlNodeHasKey(value, "short")
	return nil
}

// PriorityOverride is one entry of Config.Priorities: a priority the config
// wants to override (matched by Name) or add.
type PriorityOverride struct {
	Name        string `yaml:"name"`
	Color       string `yaml:"color,omitempty"`
	Description string `yaml:"description,omitempty"`
	// Symbol is the compact glyph the legend and narrow views render for
	// this priority. Empty means no symbol - see internal/ui.GetPrioritySymbol.
	Symbol string `yaml:"symbol,omitempty"`

	// colorSet, descriptionSet and symbolSet record whether their YAML keys
	// were named at all - see StatusOverride.colorSet.
	colorSet, descriptionSet, symbolSet bool
}

// UnmarshalYAML decodes a PriorityOverride while recording which optional
// string keys the source named explicitly (see colorSet/descriptionSet).
func (o *PriorityOverride) UnmarshalYAML(value *yaml.Node) error {
	type plain PriorityOverride
	var raw plain
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*o = PriorityOverride(raw)
	o.colorSet = yamlNodeHasKey(value, "color")
	o.descriptionSet = yamlNodeHasKey(value, "description")
	o.symbolSet = yamlNodeHasKey(value, "symbol")
	return nil
}

// yamlNodeHasKey reports whether a YAML mapping node names the given key,
// regardless of the value assigned to it. Presence, not value, is what
// distinguishes an omitted key from an explicit empty one.
func yamlNodeHasKey(value *yaml.Node, key string) bool {
	if value.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		if value.Content[i].Value == key {
			return true
		}
	}
	return false
}

// PermissionMode represents the default agent permission mode.
type PermissionMode string

const (
	PermissionModeAct  PermissionMode = "act"
	PermissionModePlan PermissionMode = "plan"
)

// IntegrateMode represents the worktree integration strategy.
type IntegrateMode string

const (
	IntegrateModeLocal IntegrateMode = "local"
	IntegrateModePR    IntegrateMode = "pr"
)

// WorktreeConfig defines settings for git worktree management.
type WorktreeConfig struct {
	// BaseRef is the git ref to use as the starting point for new worktree branches.
	// Default: "main"
	BaseRef string `yaml:"base_ref,omitempty"`

	// Path is the directory where worktrees are created.
	// Default: ~/.lif/worktrees/<project-name>/, or ~/.beans/worktrees/<project-name>/
	// when that directory already exists.
	// Supports ~ for home directory.
	Path string `yaml:"path,omitempty"`

	// Setup is a shell command to run inside a worktree after creation (e.g. "pnpm install").
	Setup string `yaml:"setup,omitempty"`

	// Run is a shell command to run the project (e.g. "mise dev").
	// When set, a "Run" button appears in the workspace toolbar.
	Run string `yaml:"run,omitempty"`

	// Integrate controls the worktree integration strategy.
	// "local" (default): squash-merge locally, hides PR buttons.
	// "pr": push and create PRs, hides the local Integrate button.
	Integrate IntegrateMode `yaml:"integrate,omitempty"`

	// FetchTimeout is the timeout in seconds for the git fetch that runs before
	// creating a new worktree. This fetch updates the base ref from the remote.
	// Set to 0 to disable the fetch entirely (useful for airgapped environments).
	// Default: 10 (seconds).
	FetchTimeout *int `yaml:"fetch_timeout,omitempty"`
}

// AgentConfig defines settings for agent sessions.
type AgentConfig struct {
	// Enabled controls whether agent functionality is available.
	// When false, the web UI hides agent chats, status panes, and worktree features.
	// Default: true
	Enabled *bool `yaml:"enabled,omitempty"`

	// DefaultMode is the default mode for new agent sessions.
	// Valid values: "act" (fully autonomous), "plan" (read-only).
	// Default: "act"
	DefaultMode PermissionMode `yaml:"default_mode,omitempty"`

	// DefaultEffort is the default thinking effort level for new agent sessions.
	// Valid values: "low", "medium", "high", "max".
	// When omitted, new sessions start with no effort override (uses CLI default).
	DefaultEffort string `yaml:"default_effort,omitempty"`
}

// ProjectConfig defines project-level settings.
type ProjectConfig struct {
	// Name is the human-readable project name, displayed in the UI.
	// Default: derived from directory name during `lif init`.
	Name string `yaml:"name,omitempty"`
}

// ServerConfig defines settings for the web server.
type ServerConfig struct {
	// Port is the port to listen on (default: 8080)
	Port int `yaml:"port,omitempty"`
	// CORSOrigins is the list of allowed origins for CORS and WebSocket.
	// Supports exact origins and port wildcards (e.g. "http://localhost:*").
	// Use "*" to send the CORS header for every origin so any page can read
	// the answers; it allows nothing else: changes and the terminal need a
	// listed origin, and the host check stays on.
	// Default: ["http://localhost:*", "http://127.0.0.1:*"]
	CORSOrigins []string `yaml:"cors_origins,omitempty"`
}

// DisplayConfig controls how the CLI renders to a terminal.
type DisplayConfig struct {
	// Theme names one of the bundled Catppuccin flavours: latte, frappe,
	// macchiato or mocha. Empty means mocha.
	Theme string `yaml:"theme,omitempty"`

	// MaxWidth caps the rendered width. 0 means unset and yields 110;
	// -1 disables the cap entirely.
	MaxWidth int `yaml:"max_width,omitempty"`
}

// Config holds the items configuration.
type Config struct {
	Project  ProjectConfig  `yaml:"project,omitempty"`
	Items    ItemsConfig    `yaml:"-"`
	Worktree WorktreeConfig `yaml:"worktree,omitempty"`
	Agent    AgentConfig    `yaml:"agent,omitempty"`
	Server   ServerConfig   `yaml:"server,omitempty"`
	Display  DisplayConfig  `yaml:"display,omitempty"`

	// Statuses, Types and Priorities override the built-in tables entry by
	// entry: a config naming only "bug" keeps the other four types as they are.
	// A name the defaults do not carry is appended.
	Statuses   []StatusOverride   `yaml:"statuses,omitempty"`
	Types      []TypeOverride     `yaml:"types,omitempty"`
	Priorities []PriorityOverride `yaml:"priorities,omitempty"`

	// Tasks are the named shell tasks that lif run executes.
	Tasks map[string]Task `yaml:"tasks,omitempty"`

	// tasksNode is the raw value node of the top-level tasks key. Saving it
	// back keeps task order and comments; without it a Save (lif rename)
	// would delete every task.
	tasksNode *yaml.Node `yaml:"-"`

	// Hooks run tasks when an item is created or a watched field changes.
	Hooks map[string]Hook `yaml:"hooks,omitempty"`

	// hooksNode is the raw value node of the top-level hooks key, kept for
	// Save like tasksNode.
	hooksNode *yaml.Node `yaml:"-"`

	// TypesExclusive switches the merge in TypeList() off: when true, Types is
	// the complete type table on its own, not an override list layered onto
	// DefaultTypes. lif init --profile sets this so a profile gives a
	// project exactly its own types; a hand-written config that never sets it
	// keeps the existing entry-by-entry override behaviour unchanged.
	TypesExclusive bool `yaml:"types_exclusive,omitempty"`

	// configDir is the directory containing the config file (not serialized)
	// Used to resolve relative paths
	configDir string `yaml:"-"`

	// configFile is the config file this config was read from (not serialized).
	// Empty when no file was found and the defaults are in play; callers use
	// that distinction to tell a repository's own declaration apart from a
	// fallback (see ConfigFile).
	configFile string `yaml:"-"`

	// sectionKey is the top-level key the items section was read under
	// ("lif" or "beans"). Empty means the new key (see SectionKey).
	sectionKey string `yaml:"-"`
}

// ItemsConfig defines settings for item creation.
type ItemsConfig struct {
	// Path is the path to the items directory (relative to config file location)
	Path string `yaml:"path,omitempty"`
	// Anchor decides what Path is relative to. Empty (the default) keeps the
	// config file's own directory, so a secondary worktree gets its own store.
	// AnchorRepoRoot resolves against the main worktree's root instead, so every
	// worktree of a repository shares one store.
	Anchor        string `yaml:"anchor,omitempty"`
	Prefix        string `yaml:"prefix"`
	IDLength      int    `yaml:"id_length"`
	DefaultStatus string `yaml:"default_status,omitempty"`
	DefaultType   string `yaml:"default_type,omitempty"`
	// DefaultPriority and DefaultTags are pre-filled by `lif create -i` only;
	// plain create, GraphQL createItem and the TUI form ignore them.
	DefaultPriority string   `yaml:"default_priority,omitempty"`
	DefaultTags     []string `yaml:"default_tags,omitempty"`
	RequireIfMatch  bool     `yaml:"require_if_match,omitempty"`
	// RequireFieldsOn maps a target status to front matter keys that must carry
	// a non-empty value whenever an item is written into that status.
	RequireFieldsOn map[string][]string `yaml:"require_fields_on,omitempty"`
	// CommitField names the extra front matter key that holds a git commit SHA.
	// Only this key is git-verified when written or checked.
	CommitField string `yaml:"commit_field,omitempty"`
}

// Default returns a Config with default values.
func Default() *Config {
	return &Config{
		Items: ItemsConfig{
			Path:          "",
			Prefix:        "",
			IDLength:      4,
			DefaultStatus: "todo",
			DefaultType:   "task",
		},
		Worktree: WorktreeConfig{
			BaseRef:   DefaultWorktreeBaseRef,
			Integrate: IntegrateModeLocal,
		},
		Agent: AgentConfig{
			Enabled:     boolPtr(true),
			DefaultMode: PermissionModeAct,
		},
		Server: ServerConfig{
			Port: DefaultServerPort,
		},
	}
}

// DefaultWithPrefix returns a Config with the given prefix.
func DefaultWithPrefix(prefix string) *Config {
	cfg := Default()
	cfg.Items.Prefix = prefix
	return cfg
}

// FindConfig searches upward from the given directory for a config file.
// Returns the absolute path to the config file, or empty string if not found.
func FindConfig(startDir string) (string, error) {
	return FindConfigWithin(startDir, "")
}

// FindConfigWithin searches upward from startDir for a config file (see ResolveConfigFile),
// stopping at rootDir (inclusive). If rootDir is empty, searches up to the
// filesystem root.
func FindConfigWithin(startDir, rootDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	var absRoot string
	if rootDir != "" {
		absRoot, err = filepath.Abs(rootDir)
		if err != nil {
			return "", err
		}
	}

	for {
		configPath, found, err := ResolveConfigFile(dir)
		if err != nil {
			return "", err
		}
		if found {
			return configPath, nil
		}

		// Stop if we've reached the root boundary
		if absRoot != "" && dir == absRoot {
			return "", nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "", nil
		}
		dir = parent
	}
}

// The four name levels -- config file, section key, default store directory
// and store-path env var -- follow one rule each: exactly one of the new and
// the legacy name present uses that one in place, both present is an error
// naming both, neither present uses the new name. Each level resolves in the
// function below; no caller compares the names itself.

// ResolveConfigFile resolves the config file inside dir. found reports whether
// a file exists; when none does, path is where the new-name file would go.
func ResolveConfigFile(dir string) (path string, found bool, err error) {
	newPath := filepath.Join(dir, ConfigFileName)
	legacyPath := filepath.Join(dir, LegacyConfigFileName)
	hasNew, hasLegacy := exists(newPath), exists(legacyPath)
	switch {
	case hasNew && hasLegacy:
		return "", false, fmt.Errorf("both %s and %s exist in %s; keep one of them", ConfigFileName, LegacyConfigFileName, dir)
	case hasLegacy:
		return legacyPath, true, nil
	case hasNew:
		return newPath, true, nil
	default:
		return newPath, false, nil
	}
}

// ResolveSectionKey resolves the section key of the config file at path. A
// missing file or a file with neither key yields the new key.
func ResolveSectionKey(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SectionKey, nil
		}
		return "", fmt.Errorf("%s: %w", path, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	_, key, err := itemsSection(&doc, path)
	return key, err
}

// itemsSection locates the items section of a parsed config document and
// returns its value node together with the key it sits under. The node is nil
// when neither key is present.
func itemsSection(doc *yaml.Node, path string) (section *yaml.Node, key string, err error) {
	key = SectionKey
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, key, nil
	}
	top := doc.Content[0]
	var newNode, legacyNode *yaml.Node
	for i := 0; i+1 < len(top.Content); i += 2 {
		switch top.Content[i].Value {
		case SectionKey:
			newNode = top.Content[i+1]
		case LegacySectionKey:
			legacyNode = top.Content[i+1]
		}
	}
	switch {
	case newNode != nil && legacyNode != nil:
		return nil, "", fmt.Errorf("%s: both sections %q and %q are present; keep one of them", path, SectionKey+":", LegacySectionKey+":")
	case legacyNode != nil:
		return legacyNode, LegacySectionKey, nil
	default:
		return newNode, SectionKey, nil
	}
}

// ResolveDefaultStoreDir resolves the default store directory name inside dir,
// for a config without an explicit path.
func ResolveDefaultStoreDir(dir string) (string, error) {
	hasNew := exists(filepath.Join(dir, DefaultStoreDir))
	hasLegacy := exists(filepath.Join(dir, LegacyDefaultStoreDir))
	switch {
	case hasNew && hasLegacy:
		return "", fmt.Errorf("both %s and %s exist in %s; keep one of them", DefaultStoreDir, LegacyDefaultStoreDir, dir)
	case hasLegacy:
		return LegacyDefaultStoreDir, nil
	default:
		return DefaultStoreDir, nil
	}
}

// ResolveInitStoreDir resolves the store directory `init` creates or
// re-initializes inside dir. It is ResolveDefaultStoreDir, except for a
// directory that holds a legacy config file and no store directory yet: that
// is a project with only legacy names, and init keeps it that way.
func ResolveInitStoreDir(dir string) (string, error) {
	name, err := ResolveDefaultStoreDir(dir)
	if err != nil || exists(filepath.Join(dir, DefaultStoreDir)) {
		return name, err
	}
	configFile, found, err := ResolveConfigFile(dir)
	if err != nil {
		return "", err
	}
	if found && IsLegacyConfigFile(configFile) {
		return LegacyDefaultStoreDir, nil
	}
	return name, nil
}

// ResolveStoreEnv resolves the store-path env var. A variable counts as
// present when it is non-empty. legacy reports that the value came from the
// legacy variable; path is empty when neither is set.
func ResolveStoreEnv() (path string, legacy bool, err error) {
	newValue, legacyValue := os.Getenv(StoreEnvVar), os.Getenv(LegacyStoreEnvVar)
	switch {
	case newValue != "" && legacyValue != "":
		return "", false, fmt.Errorf("both %s and %s are set; unset one of them", StoreEnvVar, LegacyStoreEnvVar)
	case legacyValue != "":
		return legacyValue, true, nil
	default:
		return newValue, false, nil
	}
}

// IsLegacyConfigFile reports whether path names the legacy config file.
func IsLegacyConfigFile(path string) bool {
	return filepath.Base(path) == LegacyConfigFileName
}

// IsLegacyStoreDir reports whether path names the legacy store directory.
func IsLegacyStoreDir(path string) bool {
	return filepath.Base(path) == LegacyDefaultStoreDir
}

// IsLegacySectionKey reports whether key is the legacy section key.
func IsLegacySectionKey(key string) bool {
	return key == LegacySectionKey
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Load reads configuration from the given config file path.
// Returns default config if the file doesn't exist.
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, err
	}

	// Decode in two steps: the items section sits under one of two keys, so
	// it is located first and decoded on its own; the rest of the document
	// decodes into the struct, where Items is tagged out.
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	var cfg Config
	if doc.Kind != 0 {
		if err := doc.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("%s: %w", configPath, err)
		}
		section, key, err := itemsSection(&doc, configPath)
		if err != nil {
			return nil, err
		}
		cfg.sectionKey = key
		cfg.tasksNode = topValueNode(&doc, "tasks")
		cfg.hooksNode = topValueNode(&doc, "hooks")
		if section != nil {
			if err := section.Decode(&cfg.Items); err != nil {
				return nil, err
			}
		}
	}

	// Store the config file and its directory: the directory resolves
	// relative paths, the file records that a declaration really exists.
	cfg.configDir = filepath.Dir(configPath)
	cfg.configFile = configPath

	// Apply defaults for missing values. A missing path stays empty: it means
	// "the default store directory", which is resolved against the disk when
	// the store is opened (ResolveItemsPath).
	if cfg.Items.IDLength == 0 {
		cfg.Items.IDLength = 4
	}
	if cfg.Items.DefaultStatus == "" {
		cfg.Items.DefaultStatus = "todo"
	}
	if cfg.Items.DefaultType == "" {
		// "task" is the canonical fallback: Default() (used by `lif init`
		// and whenever no config file exists) hardcodes it, and it is the
		// leaf/most common type for day-to-day work. Prefer it here too, so
		// a config that merely omits default_type behaves identically
		// whether or not a .beans.yml happens to exist yet - Save() used to
		// persist whichever value this branch picked, so disagreeing with
		// Default() meant a config file with no default_type at all would
		// get "milestone" baked in on first save.
		//
		// TypesExclusive can make TypeList() legitimately empty (an exclusive
		// config with no types of its own), or exclude "task" entirely (a
		// profile with its own type vocabulary); fall back to the first
		// merged type in that case, and leave DefaultType empty when there is
		// nothing to derive one from.
		types := cfg.TypeList()
		if len(types) > 0 {
			cfg.Items.DefaultType = types[0].Name
			for _, t := range types {
				if t.Name == "task" {
					cfg.Items.DefaultType = "task"
					break
				}
			}
		}
	}

	// A misspelt anchor must not degrade into the default: that would silently
	// resolve a different store than the file asks for.
	if err := ValidateAnchor(cfg.Items.Anchor, cfg.SectionKey()); err != nil {
		return nil, fmt.Errorf("%s: %w", configPath, err)
	}

	if err := cfg.validateRequireFieldsOn(); err != nil {
		return nil, fmt.Errorf("%s: %w", configPath, err)
	}

	if err := validateTasks(cfg.Tasks); err != nil {
		return nil, fmt.Errorf("%s: %w", configPath, err)
	}

	if err := validateHooks(cfg.Hooks, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", configPath, err)
	}

	return &cfg, nil
}

// ValidateAnchor rejects any anchor value the resolver cannot honour. key is the
// section key of the file the value came from, so the error names its key path.
func ValidateAnchor(anchor, key string) error {
	switch anchor {
	case "", AnchorRepoRoot:
		return nil
	default:
		return fmt.Errorf("unknown %s.anchor %q (expected %q or an empty value)", key, anchor, AnchorRepoRoot)
	}
}

// validateRequireFieldsOn rejects a require_fields_on configuration that
// names an unknown status, an empty field name, or a field items already
// manages as a schema field. Misconfiguration must not silently degrade into
// no policy at all.
func (c *Config) validateRequireFieldsOn() error {
	statuses := make([]string, 0, len(c.Items.RequireFieldsOn))
	for status := range c.Items.RequireFieldsOn {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	for _, status := range statuses {
		fields := c.Items.RequireFieldsOn[status]
		if !c.IsValidStatus(status) {
			return fmt.Errorf("unknown status %q in %s.require_fields_on (valid: %s)", status, c.SectionKey(), strings.Join(c.StatusNames(), ", "))
		}
		for _, field := range fields {
			if strings.TrimSpace(field) == "" {
				return fmt.Errorf("%s.require_fields_on[%q] contains an empty field name", c.SectionKey(), status)
			}
			if _, known := item.ReservedKeyFlag(field); known {
				return fmt.Errorf("%s.require_fields_on[%q] names reserved front matter field %q (managed by lif, not an extra key)", c.SectionKey(), status, field)
			}
		}
	}
	return nil
}

// LoadFromDirectory finds and loads the config file by searching upward from the given directory.
// If no config file is found, returns a default config anchored at the given directory.
func LoadFromDirectory(startDir string) (*Config, error) {
	return LoadFromDirectoryWithin(startDir, "")
}

// LoadFromDirectoryWithin is like LoadFromDirectory but limits the search to
// within rootDir (see FindConfigWithin).
func LoadFromDirectoryWithin(startDir, rootDir string) (*Config, error) {
	configPath, err := FindConfigWithin(startDir, rootDir)
	if err != nil {
		return nil, err
	}

	if configPath == "" {
		// No config found, return default anchored at startDir
		cfg := Default()
		cfg.configDir = startDir
		return cfg, nil
	}

	return Load(configPath)
}

// ResolveItemsPath returns the absolute path to the items directory,
// anchored at the config directory (the current directory when none is set).
func (c *Config) ResolveItemsPath() (string, error) {
	base := c.configDir
	if base == "" {
		// Fallback: use current directory
		base, _ = os.Getwd()
	}
	return c.ResolveItemsPathIn(base)
}

// ResolveItemsPathIn is ResolveItemsPath with an explicit anchor directory,
// the main worktree root for `anchor: repo-root`. An explicit path is joined
// verbatim; without one the default store directory is resolved inside base
// (see ResolveDefaultStoreDir), which is where both default names can clash.
func (c *Config) ResolveItemsPathIn(base string) (string, error) {
	if filepath.IsAbs(c.Items.Path) {
		return c.Items.Path, nil
	}
	if c.Items.Path != "" {
		return filepath.Join(base, c.Items.Path), nil
	}
	name, err := ResolveDefaultStoreDir(base)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, name), nil
}

// SectionKey returns the top-level key of the items section: the key of the
// file this config was loaded from, and the new key when no file was loaded.
func (c *Config) SectionKey() string {
	if c.sectionKey == "" {
		return SectionKey
	}
	return c.sectionKey
}

// SaveTarget decides which file Save writes and under which section key. A
// config read from a file writes that very file under its own key. Any other
// config writes into the one config file already present in the target
// directory and keeps that file's key; only a directory without a config file
// gets the new file with the new key.
func (c *Config) SaveTarget(dir string) (path, key string, err error) {
	if c.configFile != "" {
		return c.configFile, c.SectionKey(), nil
	}
	targetDir := c.configDir
	if targetDir == "" {
		targetDir = dir
	}
	path, found, err := ResolveConfigFile(targetDir)
	if err != nil {
		return "", "", err
	}
	key = SectionKey
	if found {
		if key, err = ResolveSectionKey(path); err != nil {
			return "", "", err
		}
	}
	return path, key, nil
}

// TargetFileName is the base name of the config file Save(dir) writes: the
// file this config was read from, the config file already in the directory,
// or the new file name when there is none. It falls back to the new name when
// the target cannot be decided, so a message can always name a file.
func (c *Config) TargetFileName(dir string) string {
	path, _, err := c.SaveTarget(dir)
	if err != nil {
		return ConfigFileName
	}
	return filepath.Base(path)
}

// ConfigDir returns the directory containing the config file.
func (c *Config) ConfigDir() string {
	return c.configDir
}

// ConfigFile returns the path of the config file this config was read from, or
// an empty string when no config file was found and the defaults apply. A
// non-empty result means the repository declared where its store lives, which
// ranks above the store-path env var during path resolution.
func (c *Config) ConfigFile() string {
	return c.configFile
}

// SetConfigDir sets the config directory (for testing or when creating new configs).
func (c *Config) SetConfigDir(dir string) {
	c.configDir = dir
}

// Save writes the configuration to the config file atomically with helpful comments.
// Writes to a temporary file in the same directory, fsyncs, and renames to the target.
// Preserves the existing file mode if the file already exists.
// If configDir is set, saves to that directory; otherwise saves to the given directory.
func (c *Config) Save(dir string) error {
	path, key, err := c.SaveTarget(dir)
	if err != nil {
		return err
	}
	targetDir := filepath.Dir(path)
	c.sectionKey = key

	doc := c.toYAMLNode()

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(4)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("closing encoder: %w", err)
	}

	// Get the existing file mode if the target file exists
	fileMode := os.FileMode(0644) // default mode
	if stat, err := os.Stat(path); err == nil {
		fileMode = stat.Mode()
	}

	// Write to a temporary file in the same directory to ensure atomicity.
	tmpFile, err := os.CreateTemp(targetDir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up temp file if it still exists

	if _, err := tmpFile.Write(buf.Bytes()); err != nil {
		tmpFile.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}

	// Fsync to ensure the data hits disk before we rename
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("syncing temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Atomic rename from temp to target
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming config file: %w", err)
	}

	// Restore the original file mode if it was different
	if err := os.Chmod(path, fileMode); err != nil {
		return fmt.Errorf("restoring file mode: %w", err)
	}

	return nil
}

// toYAMLNode builds a yaml.Node document tree with inline comments.
func (c *Config) toYAMLNode() *yaml.Node {
	// Helper to create a scalar node
	scalar := func(value string, tag string) *yaml.Node {
		return &yaml.Node{Kind: yaml.ScalarNode, Value: value, Tag: tag}
	}
	strNode := func(value string) *yaml.Node {
		return scalar(value, "!!str")
	}
	intNode := func(value int) *yaml.Node {
		return scalar(fmt.Sprintf("%d", value), "!!int")
	}

	// Build the project mapping
	projectMapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if c.Project.Name != "" {
		key := strNode("name")
		key.HeadComment = "Human-readable project name (displayed in the UI)"
		projectMapping.Content = append(projectMapping.Content, key, strNode(c.Project.Name))
	}

	// Build the items mapping
	itemsMapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}

	// An omitted path stays omitted: the default directory is resolved when
	// the store is opened (ResolveItemsPath), not frozen into the file.
	if c.Items.Path != "" {
		key := strNode("path")
		key.HeadComment = "Directory where item files are stored"
		itemsMapping.Content = append(itemsMapping.Content, key, strNode(c.Items.Path))
	}

	if c.Items.Anchor != "" {
		key := strNode("anchor")
		key.HeadComment = "What `path` is relative to: \"repo-root\" makes every worktree share the main workdir's store"
		itemsMapping.Content = append(itemsMapping.Content, key, strNode(c.Items.Anchor))
	}

	prefixKey := strNode("prefix")
	prefixKey.HeadComment = "Prefix for item IDs (e.g., \"myproject-abc1\")"
	itemsMapping.Content = append(itemsMapping.Content, prefixKey, strNode(c.Items.Prefix))

	idLenKey := strNode("id_length")
	idLenKey.HeadComment = "Length of the random ID suffix"
	itemsMapping.Content = append(itemsMapping.Content, idLenKey, intNode(c.Items.IDLength))

	if c.Items.DefaultStatus != "" {
		key := strNode("default_status")
		key.HeadComment = "Default status for new items"
		itemsMapping.Content = append(itemsMapping.Content, key, strNode(c.Items.DefaultStatus))
	}

	if c.Items.DefaultType != "" {
		key := strNode("default_type")
		key.HeadComment = "Default type for new items"
		itemsMapping.Content = append(itemsMapping.Content, key, strNode(c.Items.DefaultType))
	}

	if c.Items.DefaultPriority != "" {
		key := strNode("default_priority")
		key.HeadComment = "Default priority pre-filled by `lif create -i` (plain create ignores it)"
		itemsMapping.Content = append(itemsMapping.Content, key, strNode(c.Items.DefaultPriority))
	}

	if len(c.Items.DefaultTags) > 0 {
		key := strNode("default_tags")
		key.HeadComment = "Default tags pre-filled by `lif create -i` (plain create ignores them)"
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
		for _, tag := range c.Items.DefaultTags {
			seq.Content = append(seq.Content, strNode(tag))
		}
		itemsMapping.Content = append(itemsMapping.Content, key, seq)
	}

	if c.Items.RequireIfMatch {
		key := strNode("require_if_match")
		key.HeadComment = "Require ETag for updates (optimistic concurrency)"
		itemsMapping.Content = append(itemsMapping.Content, key, scalar("true", "!!bool"))
	}

	if len(c.Items.RequireFieldsOn) > 0 {
		key := strNode("require_fields_on")
		key.HeadComment = "Front matter keys that must be set when an item enters a status"
		mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		// Iterate the map's own keys, sorted for deterministic output -
		// looping over StatusList() instead would silently drop any status
		// name RequireFieldsOn carries that StatusList() doesn't (e.g. a
		// config built directly rather than through Load, which is the only
		// place that validates every key names a known status).
		statuses := make([]string, 0, len(c.Items.RequireFieldsOn))
		for s := range c.Items.RequireFieldsOn {
			statuses = append(statuses, s)
		}
		sort.Strings(statuses)
		for _, s := range statuses {
			fields := c.Items.RequireFieldsOn[s]
			if len(fields) == 0 {
				continue
			}
			seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
			for _, f := range fields {
				seq.Content = append(seq.Content, strNode(f))
			}
			mapping.Content = append(mapping.Content, strNode(s), seq)
		}
		itemsMapping.Content = append(itemsMapping.Content, key, mapping)
	}

	if c.Items.CommitField != "" {
		key := strNode("commit_field")
		key.HeadComment = "Extra front matter key holding the git commit SHA (default: commit)"
		itemsMapping.Content = append(itemsMapping.Content, key, strNode(c.Items.CommitField))
	}

	// Build the worktree mapping
	worktreeMapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if c.Worktree.BaseRef != "" {
		key := strNode("base_ref")
		key.HeadComment = "Git ref to use as the base for new worktree branches (default: main)"
		worktreeMapping.Content = append(worktreeMapping.Content, key, strNode(c.Worktree.BaseRef))
	}
	if c.Worktree.Path != "" {
		key := strNode("path")
		key.HeadComment = "Directory for worktrees (default: ~/.lif/worktrees/<project>/)"
		worktreeMapping.Content = append(worktreeMapping.Content, key, strNode(c.Worktree.Path))
	}
	setupKey := strNode("setup")
	setupKey.HeadComment = "Shell command to run inside a worktree after creation (e.g. \"pnpm install\")"
	worktreeMapping.Content = append(worktreeMapping.Content, setupKey, strNode(c.Worktree.Setup))

	runKey := strNode("run")
	runKey.HeadComment = "Shell command to run the project (adds a \"Run\" button to workspace toolbar)"
	worktreeMapping.Content = append(worktreeMapping.Content, runKey, strNode(c.Worktree.Run))

	integrateKey := strNode("integrate")
	integrateKey.HeadComment = "Integration strategy: \"local\" (squash-merge locally) or \"pr\" (push and create PRs)"
	worktreeMapping.Content = append(worktreeMapping.Content, integrateKey, strNode(string(c.GetWorktreeIntegrate())))

	if c.Worktree.FetchTimeout != nil {
		key := strNode("fetch_timeout")
		key.HeadComment = "Timeout in seconds for the fetch before creating a worktree (default: 10; 0 disables it)"
		worktreeMapping.Content = append(worktreeMapping.Content, key, intNode(*c.Worktree.FetchTimeout))
	}

	// Build the agent mapping
	agentMapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if c.Agent.Enabled != nil {
		key := strNode("enabled")
		key.HeadComment = "Enable agent functionality in the web UI (true, false)"
		agentMapping.Content = append(agentMapping.Content, key, scalar(fmt.Sprintf("%t", *c.Agent.Enabled), "!!bool"))
	}
	if c.Agent.DefaultMode != "" {
		key := strNode("default_mode")
		key.HeadComment = "Default mode for agent sessions (act, plan)"
		agentMapping.Content = append(agentMapping.Content, key, strNode(string(c.Agent.DefaultMode)))
	}
	if c.Agent.DefaultEffort != "" {
		key := strNode("default_effort")
		key.HeadComment = "Default thinking effort level for new agent sessions (low, medium, high, max)"
		agentMapping.Content = append(agentMapping.Content, key, strNode(c.Agent.DefaultEffort))
	}
	// Build the server mapping
	serverMapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if c.Server.Port != 0 {
		portKey := strNode("port")
		portKey.HeadComment = "Port for the web UI (used by `lif-serve`)"
		serverMapping.Content = append(serverMapping.Content, portKey, intNode(c.Server.Port))
	}
	if len(c.Server.CORSOrigins) > 0 {
		key := strNode("cors_origins")
		key.HeadComment = "Allowed origins for CORS and WebSocket (default: http://localhost:*, http://127.0.0.1:*)"
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: yaml.FlowStyle}
		for _, origin := range c.Server.CORSOrigins {
			seq.Content = append(seq.Content, strNode(origin))
		}
		serverMapping.Content = append(serverMapping.Content, key, seq)
	}

	// Build the display mapping. Like Statuses/Types/Priorities, this must be
	// round-tripped: Save() is called by `lif rename` on an already-loaded
	// Config, and before this a configured theme/max_width was silently
	// dropped on the next write.
	displayMapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	if c.Display.Theme != "" {
		key := strNode("theme")
		key.HeadComment = "Catppuccin flavour: latte, frappe, macchiato or mocha (default: mocha)"
		displayMapping.Content = append(displayMapping.Content, key, strNode(c.Display.Theme))
	}
	if c.Display.MaxWidth != 0 {
		key := strNode("max_width")
		key.HeadComment = "Caps the rendered width (default: 110); -1 disables the cap"
		displayMapping.Content = append(displayMapping.Content, key, intNode(c.Display.MaxWidth))
	}

	// Build the statuses/types/priorities override sequences. These round-trip
	// exactly what was configured (not the merged result) so that Save() -
	// called by `lif rename`, for instance - never silently drops a user's
	// overrides from the file.
	statusesSeq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, s := range c.Statuses {
		m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		m.Content = append(m.Content, strNode("name"), strNode(s.Name))
		if s.colorSet || s.Color != "" {
			m.Content = append(m.Content, strNode("color"), strNode(s.Color))
		}
		if s.Archive != nil {
			m.Content = append(m.Content, strNode("archive"), scalar(fmt.Sprintf("%t", *s.Archive), "!!bool"))
		}
		if s.descriptionSet || s.Description != "" {
			m.Content = append(m.Content, strNode("description"), strNode(s.Description))
		}
		statusesSeq.Content = append(statusesSeq.Content, m)
	}

	typesSeq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, t := range c.Types {
		m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		m.Content = append(m.Content, strNode("name"), strNode(t.Name))
		if t.colorSet || t.Color != "" {
			m.Content = append(m.Content, strNode("color"), strNode(t.Color))
		}
		if t.Rank != nil {
			m.Content = append(m.Content, strNode("rank"), scalar(fmt.Sprintf("%d", *t.Rank), "!!int"))
		}
		if t.Emphasis != nil {
			m.Content = append(m.Content, strNode("emphasis"), scalar(fmt.Sprintf("%t", *t.Emphasis), "!!bool"))
		}
		if t.Roadmap != nil {
			m.Content = append(m.Content, strNode("roadmap"), scalar(fmt.Sprintf("%t", *t.Roadmap), "!!bool"))
		}
		if t.shortSet || t.Short != "" {
			m.Content = append(m.Content, strNode("short"), strNode(t.Short))
		}
		if t.descriptionSet || t.Description != "" {
			m.Content = append(m.Content, strNode("description"), strNode(t.Description))
		}
		typesSeq.Content = append(typesSeq.Content, m)
	}

	prioritiesSeq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, p := range c.Priorities {
		m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		m.Content = append(m.Content, strNode("name"), strNode(p.Name))
		if p.colorSet || p.Color != "" {
			m.Content = append(m.Content, strNode("color"), strNode(p.Color))
		}
		if p.descriptionSet || p.Description != "" {
			m.Content = append(m.Content, strNode("description"), strNode(p.Description))
		}
		prioritiesSeq.Content = append(prioritiesSeq.Content, m)
	}

	// Build the top-level mapping
	topMapping := &yaml.Node{
		Kind:        yaml.MappingNode,
		Tag:         "!!map",
		HeadComment: "Lif configuration\nSee: https://github.com/rierek/lif",
	}
	if len(projectMapping.Content) > 0 {
		topMapping.Content = append(topMapping.Content, strNode("project"), projectMapping)
	}

	topMapping.Content = append(topMapping.Content, strNode(c.SectionKey()), itemsMapping)

	if len(worktreeMapping.Content) > 0 {
		topMapping.Content = append(topMapping.Content, strNode("worktree"), worktreeMapping)
	}

	if len(agentMapping.Content) > 0 {
		topMapping.Content = append(topMapping.Content, strNode("agent"), agentMapping)
	}

	if len(serverMapping.Content) > 0 {
		topMapping.Content = append(topMapping.Content, strNode("server"), serverMapping)
	}

	if len(displayMapping.Content) > 0 {
		topMapping.Content = append(topMapping.Content, strNode("display"), displayMapping)
	}

	if len(statusesSeq.Content) > 0 {
		key := strNode("statuses")
		key.HeadComment = "Status overrides, merged entry by entry with the built-in defaults"
		topMapping.Content = append(topMapping.Content, key, statusesSeq)
	}

	if c.TypesExclusive {
		key := strNode("types_exclusive")
		key.HeadComment = "true: types below is the complete type table, not overrides merged onto the built-in defaults"
		topMapping.Content = append(topMapping.Content, key, scalar("true", "!!bool"))
	}

	if len(typesSeq.Content) > 0 {
		key := strNode("types")
		if c.TypesExclusive {
			key.HeadComment = "The complete type table (types_exclusive: true)"
		} else {
			key.HeadComment = "Type overrides, merged entry by entry with the built-in defaults"
		}
		topMapping.Content = append(topMapping.Content, key, typesSeq)
	}

	if len(prioritiesSeq.Content) > 0 {
		key := strNode("priorities")
		key.HeadComment = "Priority overrides, merged entry by entry with the built-in defaults"
		topMapping.Content = append(topMapping.Content, key, prioritiesSeq)
	}

	if c.tasksNode != nil {
		key := strNode("tasks")
		key.HeadComment = "Tasks run with lif run <name>"
		topMapping.Content = append(topMapping.Content, key, c.tasksNode)
	}

	if c.hooksNode != nil {
		key := strNode("hooks")
		key.HeadComment = "Hooks run tasks on item events; enable with lif hooks allow"
		topMapping.Content = append(topMapping.Content, key, c.hooksNode)
	}

	// Wrap in a document node
	return &yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{topMapping},
	}
}

// IsValidStatus returns true if the status is a known status name, built-in
// or configured.
func (c *Config) IsValidStatus(status string) bool {
	for _, s := range c.StatusList() {
		if s.Name == status {
			return true
		}
	}
	return false
}

// RequiredFieldsFor returns the front matter keys that must be set when an item
// enters status. Returns nil when no policy applies.
func (c *Config) RequiredFieldsFor(status string) []string {
	if c == nil {
		return nil
	}
	return c.Items.RequireFieldsOn[status]
}

// GetCommitField returns the configured commit field name, or DefaultCommitField.
func (c *Config) GetCommitField() string {
	if c == nil || c.Items.CommitField == "" {
		return DefaultCommitField
	}
	return c.Items.CommitField
}

// mergeDefaults merges a config's override entries into a copy of a
// built-in default table, matched by name. An override naming an entry the
// defaults already have is applied onto that entry via apply; an override
// naming an unknown entry is appended as a new one (apply runs against a
// zero-value T, so it must itself set every field the new entry needs,
// starting with the name). This is the single place the three list merges
// (statuses, types, priorities) share, so a merge rule such as "only an
// explicit field touches the default" is written once, not duplicated per
// type - see StatusList's Archive handling for why that duplication was the
// actual defect the previous copy-pasted version hid.
func mergeDefaults[T, O any](defaults []T, overrides []O, key func(T) string, name func(O) string, apply func(*T, O)) []T {
	out := make([]T, len(defaults))
	copy(out, defaults)
	for _, override := range overrides {
		merged := false
		for i := range out {
			if key(out[i]) != name(override) {
				continue
			}
			apply(&out[i], override)
			merged = true
			break
		}
		if !merged {
			var fresh T
			apply(&fresh, override)
			out = append(out, fresh)
		}
	}
	return out
}

// StatusList returns the built-in statuses with any configured overrides
// applied. A config entry overrides its named status field by field; a name
// the defaults do not carry is appended, so custom statuses are possible
// without this being their dedicated feature.
func (c *Config) StatusList() []StatusConfig {
	if c == nil {
		out := make([]StatusConfig, len(DefaultStatuses))
		copy(out, DefaultStatuses)
		return out
	}
	return mergeDefaults(DefaultStatuses, c.Statuses,
		func(t StatusConfig) string { return t.Name },
		func(o StatusOverride) string { return o.Name },
		func(t *StatusConfig, o StatusOverride) {
			t.Name = o.Name
			if o.colorSet || o.Color != "" {
				t.Color = o.Color
			}
			if o.descriptionSet || o.Description != "" {
				t.Description = o.Description
			}
			if o.shortSet || o.Short != "" {
				t.Short = o.Short
			}
			// Archive is a pointer: only an explicit archive: key (true or
			// false) touches it, so a colour-only override cannot flip a
			// status like "completed" back to non-archiving by accident.
			if o.Archive != nil {
				t.Archive = *o.Archive
			}
		},
	)
}

// StatusNames returns the names of the merged status list (see StatusList).
func (c *Config) StatusNames() []string {
	list := c.StatusList()
	names := make([]string, len(list))
	for i, s := range list {
		names[i] = s.Name
	}
	return names
}

// GetStatus returns the StatusConfig for a status name from the merged list
// (see StatusList), or nil if unknown.
func (c *Config) GetStatus(name string) *StatusConfig {
	list := c.StatusList()
	for i := range list {
		if list[i].Name == name {
			return &list[i]
		}
	}
	return nil
}

// GetDefaultStatus returns the default status name for new items.
func (c *Config) GetDefaultStatus() string {
	if c == nil || c.Items.DefaultStatus == "" {
		return "todo"
	}
	return c.Items.DefaultStatus
}

// GetDefaultType returns the default type name for new items.
func (c *Config) GetDefaultType() string {
	if c == nil {
		return ""
	}
	return c.Items.DefaultType
}

// GetDefaultPriority returns lif.default_priority, or "" when unset.
// Only `lif create -i` reads it.
func (c *Config) GetDefaultPriority() string {
	if c == nil {
		return ""
	}
	return c.Items.DefaultPriority
}

// GetDefaultTags returns lif.default_tags in file order, or nil when unset.
// Only `lif create -i` reads it.
func (c *Config) GetDefaultTags() []string {
	if c == nil || len(c.Items.DefaultTags) == 0 {
		return nil
	}
	return append([]string(nil), c.Items.DefaultTags...)
}

// IsArchiveStatus returns true if the given status is marked for archiving.
func (c *Config) IsArchiveStatus(name string) bool {
	if s := c.GetStatus(name); s != nil {
		return s.Archive
	}
	return false
}

// GetType returns the TypeConfig for a type name from the merged list (see
// TypeList), or nil if unknown.
func (c *Config) GetType(name string) *TypeConfig {
	list := c.TypeList()
	for i := range list {
		if list[i].Name == name {
			return &list[i]
		}
	}
	return nil
}

// RankOf returns the hierarchy rank of a type name. An unknown type sits at
// LeafRank; a known type's rank is TypeList()'s merged value, which is
// itself LeafRank when the type carries no explicit rank (see TypeList) -
// so an explicit "rank: 0" on a known type is returned as 0, not coerced.
func (c *Config) RankOf(typeName string) int {
	if t := c.GetType(typeName); t != nil {
		return t.Rank
	}
	return LeafRank
}

// ShortOf returns the single-character code for a type: the configured one,
// otherwise the upper-cased first letter, and "?" for an unknown type.
func (c *Config) ShortOf(typeName string) string {
	t := c.GetType(typeName)
	if t == nil {
		return "?"
	}
	if t.Short != "" {
		return t.Short
	}
	if t.Name == "" {
		return "?"
	}
	// First RUNE, not first byte: a type named "Änderung" would otherwise
	// yield half a rune and render as U+FFFD.
	r, _ := utf8.DecodeRuneInString(t.Name)
	return strings.ToUpper(string(r))
}

// ShortOfStatus returns the single-character code for a status: the
// configured one, otherwise the upper-cased first letter, and "?" for an
// unknown status. Mirrors ShortOf.
func (c *Config) ShortOfStatus(name string) string {
	s := c.GetStatus(name)
	if s == nil {
		return "?"
	}
	if s.Short != "" {
		return s.Short
	}
	if s.Name == "" {
		return "?"
	}
	r, _ := utf8.DecodeRuneInString(s.Name)
	return strings.ToUpper(string(r))
}

// IsRoadmapType reports whether a type appears as its own container in the
// aggregate views. Anything not explicitly switched off is visible.
func (c *Config) IsRoadmapType(typeName string) bool {
	if t := c.GetType(typeName); t != nil && t.Roadmap != nil {
		return *t.Roadmap
	}
	return true
}

// TypesAtRank returns the names of every type on the given rank, in the order
// the merged list carries them.
func (c *Config) TypesAtRank(rank int) []string {
	var names []string
	for _, t := range c.TypeList() {
		r := t.Rank
		if r == 0 {
			r = LeafRank
		}
		if r == rank {
			names = append(names, t.Name)
		}
	}
	return names
}

// ContainerTypeNames returns the names of every type on a container rank (1
// through MaxContainerRank), in rank order and, within a rank, in the order of
// the merged list. It is the one loop over the container ranks: the roadmap
// commands and the ready filter all read it (beans-v2ox, beans-g5dn).
func (c *Config) ContainerTypeNames() []string {
	var names []string
	for rank := 1; rank <= MaxContainerRank; rank++ {
		names = append(names, c.TypesAtRank(rank)...)
	}
	return names
}

// TypeNames returns the names of the merged type list (see TypeList).
func (c *Config) TypeNames() []string {
	list := c.TypeList()
	names := make([]string, len(list))
	for i, t := range list {
		names[i] = t.Name
	}
	return names
}

// IsValidType returns true if the type is a known type name, built-in or
// configured.
func (c *Config) IsValidType(typeName string) bool {
	for _, t := range c.TypeList() {
		if t.Name == typeName {
			return true
		}
	}
	return false
}

// TypeList returns the built-in types with any configured overrides applied.
// A config entry overrides its named type field by field; a name the
// defaults do not carry is appended.
func (c *Config) TypeList() []TypeConfig {
	if c == nil {
		out := make([]TypeConfig, len(DefaultTypes))
		copy(out, DefaultTypes)
		return out
	}
	defaults := DefaultTypes
	if c.TypesExclusive {
		// An exclusive config is its own complete type table: nothing from
		// DefaultTypes should be merged in underneath it.
		defaults = nil
	}
	return mergeDefaults(defaults, c.Types,
		func(t TypeConfig) string { return t.Name },
		func(o TypeOverride) string { return o.Name },
		func(t *TypeConfig, o TypeOverride) {
			t.Name = o.Name
			if o.colorSet || o.Color != "" {
				t.Color = o.Color
			}
			if o.descriptionSet || o.Description != "" {
				t.Description = o.Description
			}
			if o.Rank != nil {
				t.Rank = *o.Rank
			}
			// Emphasis is a pointer: only an explicit emphasis: key (true or
			// false) touches it, so a colour-only override cannot flip a
			// type like "milestone" back to non-emphasised by accident.
			if o.Emphasis != nil {
				t.Emphasis = *o.Emphasis
			}
			// Roadmap is a pointer for the same reason: an omitted key must
			// stay distinct from an explicit "roadmap: false", or a
			// colour-only override would silently hide the type. Deep-copy
			// it, like every other pointer field here - handing out o.Roadmap
			// itself would alias a caller's TypeOverride, letting a mutation
			// through the merged TypeList() reach back into c.Types.
			if o.Roadmap != nil {
				t.Roadmap = boolPtr(*o.Roadmap)
			}
			if o.shortSet || o.Short != "" {
				t.Short = o.Short
			}
			// An appended type starts from a zero TypeConfig, so an entry
			// without rank: would sit at rank 0 and outrank every container.
			// Leaf rank is the safe default and matches the old behaviour for
			// unknown types - but only when rank was never named at all: an
			// explicit "rank: 0" is a real, distinct value (o.Rank != nil)
			// and must not be silently promoted to LeafRank.
			if o.Rank == nil && t.Rank == 0 {
				t.Rank = LeafRank
			}
		},
	)
}

// ItemColors holds resolved color information for rendering an item
type ItemColors struct {
	StatusColor   string
	TypeColor     string
	PriorityColor string
	IsArchive     bool
}

// GetItemColors returns the resolved colors for an item based on its status, type, and priority.
func (c *Config) GetItemColors(status, typeName, priority string) ItemColors {
	colors := ItemColors{
		StatusColor:   "gray",
		TypeColor:     "",
		PriorityColor: "",
		IsArchive:     false,
	}

	if statusCfg := c.GetStatus(status); statusCfg != nil {
		colors.StatusColor = statusCfg.Color
	}
	colors.IsArchive = c.IsArchiveStatus(status)

	if typeCfg := c.GetType(typeName); typeCfg != nil {
		colors.TypeColor = typeCfg.Color
	}

	if priorityCfg := c.GetPriority(priority); priorityCfg != nil {
		colors.PriorityColor = priorityCfg.Color
	}

	return colors
}

// GetPriority returns the PriorityConfig for a priority name from the merged
// list (see PriorityList), or nil if unknown.
func (c *Config) GetPriority(name string) *PriorityConfig {
	list := c.PriorityList()
	for i := range list {
		if list[i].Name == name {
			return &list[i]
		}
	}
	return nil
}

// PriorityNames returns the names of the merged priority list (see
// PriorityList), in order from highest to lowest.
func (c *Config) PriorityNames() []string {
	list := c.PriorityList()
	names := make([]string, len(list))
	for i, p := range list {
		names[i] = p.Name
	}
	return names
}

// IsValidPriority returns true if the priority is a known priority name,
// built-in or configured. Empty string is valid (means no priority set).
func (c *Config) IsValidPriority(priority string) bool {
	if priority == "" {
		return true
	}
	for _, p := range c.PriorityList() {
		if p.Name == priority {
			return true
		}
	}
	return false
}

// PriorityList returns the built-in priorities with any configured overrides
// applied. A config entry overrides its named priority field by field; a
// name the defaults do not carry is appended.
func (c *Config) PriorityList() []PriorityConfig {
	if c == nil {
		out := make([]PriorityConfig, len(DefaultPriorities))
		copy(out, DefaultPriorities)
		return out
	}
	return mergeDefaults(DefaultPriorities, c.Priorities,
		func(t PriorityConfig) string { return t.Name },
		func(o PriorityOverride) string { return o.Name },
		func(t *PriorityConfig, o PriorityOverride) {
			t.Name = o.Name
			if o.colorSet || o.Color != "" {
				t.Color = o.Color
			}
			if o.descriptionSet || o.Description != "" {
				t.Description = o.Description
			}
			if o.symbolSet || o.Symbol != "" {
				t.Symbol = o.Symbol
			}
		},
	)
}

// boolPtr returns a pointer to the given bool value.
func boolPtr(b bool) *bool { return &b }

// DefaultWorktreeBaseRef is the default base ref for new worktree branches.
const DefaultWorktreeBaseRef = "main"

// ResolveWorktreePath returns the absolute path to the directory where worktrees
// should be created. If worktree.path is configured, it is used (with ~ expansion).
// Otherwise the root is ~/.beans/worktrees/<projectName>/ when that directory
// exists (a root made by the former tool), else ~/.lif/worktrees/<projectName>/.
// When both exist the legacy root wins; there is no ambiguity error. legacy
// reports that the legacy root was chosen; it is never true for worktree.path.
// The resolver only resolves: it creates no directory.
// projectName is used only when computing the default path.
func (c *Config) ResolveWorktreePath(projectName string) (path string, legacy bool, err error) {
	if c.Worktree.Path != "" {
		path, err = expandHome(c.Worktree.Path)
		return path, false, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false, fmt.Errorf("resolve home directory: %w", err)
	}
	if projectName == "" {
		projectName = "default"
	}
	legacyRoot := filepath.Join(home, LegacyHomeDir, "worktrees", projectName)
	if info, statErr := os.Stat(legacyRoot); statErr == nil && info.IsDir() {
		return legacyRoot, true, nil
	}
	return filepath.Join(home, HomeDir, "worktrees", projectName), false, nil
}

// expandHome expands a leading ~ in a path to the user's home directory.
func expandHome(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[1:]), nil
	}
	if filepath.IsAbs(path) {
		return path, nil
	}
	// Relative paths resolve against cwd
	return filepath.Abs(path)
}

// GetWorktreeBaseRef returns the configured base ref for new worktree branches.
// Returns "main" if not set.
func (c *Config) GetWorktreeBaseRef() string {
	if c == nil || c.Worktree.BaseRef == "" {
		return DefaultWorktreeBaseRef
	}
	return c.Worktree.BaseRef
}

// GetWorktreeSetup returns the configured setup command for new worktrees.
func (c *Config) GetWorktreeSetup() string {
	if c == nil {
		return ""
	}
	return c.Worktree.Setup
}

// GetWorktreeRun returns the configured run command for worktrees.
func (c *Config) GetWorktreeRun() string {
	if c == nil {
		return ""
	}
	return c.Worktree.Run
}

// GetWorktreeFetchTimeout returns the configured fetch timeout as a time.Duration.
// Returns 10s by default. Returns 0 if explicitly set to 0 (disables fetch).
func (c *Config) GetWorktreeFetchTimeout() time.Duration {
	if c == nil || c.Worktree.FetchTimeout == nil {
		return 10 * time.Second
	}
	return time.Duration(*c.Worktree.FetchTimeout) * time.Second
}

// GetWorktreeIntegrate returns the configured integration mode.
// Returns "local" if not set or invalid.
func (c *Config) GetWorktreeIntegrate() IntegrateMode {
	if c == nil {
		return IntegrateModeLocal
	}
	switch c.Worktree.Integrate {
	case IntegrateModeLocal, IntegrateModePR:
		return c.Worktree.Integrate
	default:
		return IntegrateModeLocal
	}
}

// IsAgentEnabled returns whether agent functionality is enabled.
// Returns true if not explicitly set.
func (c *Config) IsAgentEnabled() bool {
	if c == nil || c.Agent.Enabled == nil {
		return true
	}
	return *c.Agent.Enabled
}

// GetDefaultMode returns the configured default permission mode for agent sessions.
// Returns "act" if not set or invalid. Also accepts "yolo" as a backwards-compatible alias.
func (c *Config) GetDefaultMode() PermissionMode {
	if c == nil {
		return PermissionModeAct
	}
	switch c.Agent.DefaultMode {
	case PermissionModeAct, PermissionModePlan:
		return c.Agent.DefaultMode
	case "yolo":
		return PermissionModeAct // backwards-compatible alias
	default:
		return PermissionModeAct
	}
}

// GetDefaultEffort returns the raw configured default effort level for agent sessions.
// Returns empty string if not set. Use IsValidEffortLevel to validate before use.
func (c *Config) GetDefaultEffort() string {
	if c == nil {
		return ""
	}
	return c.Agent.DefaultEffort
}

// IsValidEffortLevel returns true if the effort level is a valid value.
func IsValidEffortLevel(effort string) bool {
	switch effort {
	case "low", "medium", "high", "max":
		return true
	default:
		return false
	}
}

// IsValidPermissionMode returns true if the mode is a valid permission mode.
func IsValidPermissionMode(mode string) bool {
	switch PermissionMode(mode) {
	case PermissionModeAct, PermissionModePlan, "yolo":
		return true
	default:
		return false
	}
}

// GetProjectName returns the configured project name, or empty string if not set.
func (c *Config) GetProjectName() string {
	if c == nil {
		return ""
	}
	return c.Project.Name
}

// GetServerPort returns the configured server port, or the default if not set.
func (c *Config) GetServerPort() int {
	if c == nil || c.Server.Port == 0 {
		return DefaultServerPort
	}
	return c.Server.Port
}

// GetCORSOrigins returns the configured CORS origins, or the defaults if not set.
func (c *Config) GetCORSOrigins() []string {
	if c != nil && len(c.Server.CORSOrigins) > 0 {
		return c.Server.CORSOrigins
	}
	return []string{"http://localhost:*", "http://127.0.0.1:*"}
}

// GetTheme returns the configured theme name, or "mocha" when unset.
func (c *Config) GetTheme() string {
	if c == nil || c.Display.Theme == "" {
		return "mocha"
	}
	return c.Display.Theme
}

// GetMaxWidth returns the configured width cap: DefaultMaxWidth when unset,
// -1 when the cap is explicitly disabled.
func (c *Config) GetMaxWidth() int {
	if c == nil || c.Display.MaxWidth == 0 {
		return DefaultMaxWidth
	}
	return c.Display.MaxWidth
}
