package itemgraph

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/rierek/lif/pkg/config"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/itemgraph/model"
)

const (
	// MaxTitleLength is the largest title, in characters, a mutation accepts.
	MaxTitleLength = 1000

	// MaxBodyBytes is the largest body a mutation accepts.
	MaxBodyBytes = 1 << 20
)

// prefixPattern matches an acceptable custom ID prefix. A prefix is prepended
// verbatim to the generated ID, and the ID becomes part of the item's filename,
// so anything carrying a path separator, a dot or whitespace would let a caller
// write outside the store. Single hyphens stay legal because real stores use
// one as the separator between prefix and ID ("beans-abc1"), as do prefixes
// mirrored from an external tracker ("SYNC-TASK-"); a doubled hyphen does not,
// because ParseFilename splits an ID from its slug on the first "--".
var prefixPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*(?:-[A-Za-z0-9]+)*-?$`)

// ValidatePrefix checks a caller-supplied item ID prefix.
func ValidatePrefix(prefix string) error {
	if !prefixPattern.MatchString(prefix) {
		return itemcore.Invalid("invalid prefix %q: must start with a letter and contain only letters, digits and single hyphens", prefix)
	}
	return nil
}

// ValidateTitle checks that a title is present and within the length limit.
func ValidateTitle(title string) error {
	if strings.TrimSpace(title) == "" {
		return itemcore.Invalid("invalid title: must not be empty")
	}
	if n := utf8.RuneCountInString(title); n > MaxTitleLength {
		return itemcore.Invalid("invalid title: %d characters exceeds the limit of %d", n, MaxTitleLength)
	}
	return nil
}

// ValidateBody checks that a body is within the size limit.
func ValidateBody(body string) error {
	if len(body) > MaxBodyBytes {
		return itemcore.Invalid("invalid body: %d bytes exceeds the limit of %d", len(body), MaxBodyBytes)
	}
	return nil
}

// ValidateTags checks every tag against the shared tag rules.
func ValidateTags(tags []string) error {
	for _, tag := range tags {
		if err := item.ValidateTag(item.NormalizeTag(tag)); err != nil {
			return &itemcore.ValidationError{Err: err}
		}
	}
	return nil
}

// validateCreateInput checks every caller-supplied field of a create before
// anything is written.
func (r *CoreResolver) validateCreateInput(input model.CreateItemInput) error {
	if err := ValidateTitle(input.Title); err != nil {
		return err
	}
	if err := r.validateEnums(input.Status, input.Type, input.Priority); err != nil {
		return err
	}
	if input.Body != nil {
		if err := ValidateBody(*input.Body); err != nil {
			return err
		}
	}
	if err := ValidateTags(input.Tags); err != nil {
		return err
	}
	if input.Prefix != nil && *input.Prefix != "" {
		if err := ValidatePrefix(*input.Prefix); err != nil {
			return err
		}
	}
	return nil
}

// validateUpdateInput checks the fields an update actually supplies. Fields the
// input leaves alone are not validated: an item written by an older build may
// carry a value this one no longer accepts, and that must not make the rest of
// the item uneditable. The body is checked after the modifications are applied.
func (r *CoreResolver) validateUpdateInput(input model.UpdateItemInput) error {
	if input.Title != nil {
		if err := ValidateTitle(*input.Title); err != nil {
			return err
		}
	}
	if err := r.validateEnums(input.Status, input.Type, input.Priority); err != nil {
		return err
	}
	if err := ValidateTags(input.Tags); err != nil {
		return err
	}
	if err := ValidateTags(input.AddTags); err != nil {
		return err
	}
	return nil
}

// validationConfig returns the config the enum checks read. A Core built
// without one still has to validate, so fall back to the built-in tables.
func (r *CoreResolver) validationConfig() *config.Config {
	if cfg := r.Core.Config(); cfg != nil {
		return cfg
	}
	return config.Default()
}

// validateEnums checks status, type and priority against the configured
// tables. Empty values are left to the defaults applied further down.
func (r *CoreResolver) validateEnums(status, typeName, priority *string) error {
	cfg := r.validationConfig()
	if status != nil && *status != "" && !cfg.IsValidStatus(*status) {
		return itemcore.Invalid("invalid status %q: must be one of %s", *status, cfg.StatusNames())
	}
	if typeName != nil && *typeName != "" && !cfg.IsValidType(*typeName) {
		return itemcore.Invalid("invalid type %q: must be one of %s", *typeName, cfg.TypeNames())
	}
	if priority != nil && *priority != "" && !cfg.IsValidPriority(*priority) {
		return itemcore.Invalid("invalid priority %q: must be one of %s", *priority, cfg.PriorityNames())
	}
	return nil
}
