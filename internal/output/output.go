package output

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/rierek/lif/pkg/item"
)

// Error codes for JSON responses
const (
	ErrNotFound      = "NOT_FOUND"
	ErrNoLifDir      = "NO_LIF_DIR"
	ErrInvalidStatus = "INVALID_STATUS"
	ErrFileError     = "FILE_ERROR"
	ErrValidation    = "VALIDATION_ERROR"
	ErrConflict      = "CONFLICT"
	ErrPolicy        = "POLICY_VIOLATION"
)

// Response is the standard JSON response envelope.
type Response struct {
	Success  bool         `json:"success"`
	Item     *item.Item   `json:"item,omitempty"`
	Items    []*item.Item `json:"items,omitempty"`
	Count    int          `json:"count,omitempty"`
	Message  string       `json:"message,omitempty"`
	Warnings []string     `json:"warnings,omitempty"`
	Error    string       `json:"error,omitempty"`
	Code     string       `json:"code,omitempty"`
	Path     string       `json:"path,omitempty"`
}

// JSON outputs a response as JSON to stdout.
func JSON(resp Response) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

// SuccessSingle outputs a single item directly (no wrapper).
// This allows intuitive jq usage: lif show --json <id> | jq '.title'
func SuccessSingle(b *item.Item) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(b)
}

// SuccessMultiple outputs an item array directly (no wrapper).
// This allows intuitive jq usage: lif list --json | jq '.[]'
func SuccessMultiple(items []*item.Item) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(items)
}

// SuccessMessage outputs a success response with just a message.
func SuccessMessage(message string) error {
	return JSON(Response{
		Success: true,
		Message: message,
	})
}

// SuccessInit outputs a success response for init command, naming the store
// directory init initialized.
func SuccessInit(path string) error {
	return JSON(Response{
		Success: true,
		Message: "Initialized " + filepath.Base(path) + " directory",
		Path:    path,
	})
}

// PartialFailure outputs a failed multi-item response naming the items that
// were already written before the failure struck. It is the one Response
// literal still built with the envelope: a bare array cannot carry the
// failure, so batch verbs keep this document for the case where a write
// loop over several IDs fails partway through.
//
// Like Error, it returns an *emittedError rather than the encoder's own
// result: JSON only fails if os.Stdout itself breaks, and surfacing that nil
// in the common case would leave reportExecutionError free to print a
// second, human-readable copy of the failure this function just wrote.
func PartialFailure(items []*item.Item, code string, err error) error {
	message := err.Error()
	_ = JSON(Response{
		Success: false,
		Items:   items,
		Count:   len(items),
		Error:   message,
		Code:    code,
	})
	return &emittedError{message: message}
}

// emittedError marks an error whose machine-readable document has already
// been written to stdout. The reporting path at the top of the CLI reads that
// mark to decide whether the failure still needs a human-readable line on
// stderr, so that a --json consumer gets exactly one artifact and an error
// raised before the output layer was reached is not swallowed.
type emittedError struct {
	message string
}

func (e *emittedError) Error() string { return e.message }

// Emitted reports whether err has already been written as a JSON document.
func Emitted(err error) bool {
	var e *emittedError
	return errors.As(err, &e)
}

// silentError marks an error that reached the top-level failure path but
// carries no message worth a stderr line -- e.g. an interactive picker the
// user backed out of. Distinct from emittedError: Emitted means a
// machine-readable document was already written; Silent means there is no
// document and none is needed either, because the outcome (a non-zero
// exit) already says everything the caller needs.
type silentError struct {
	message string
}

func (e *silentError) Error() string { return e.message }

// Silent wraps message as an error reportExecutionError will not print to
// stderr. The process still exits non-zero (R-11 AC3): only the redundant
// human-readable line is suppressed.
func Silent(message string) error {
	return &silentError{message: message}
}

// IsSilent reports whether err, or any error it wraps, was built by
// Silent.
func IsSilent(err error) bool {
	var e *silentError
	return errors.As(err, &e)
}

// Error outputs an error response and returns an error for command handling.
func Error(code string, message string) error {
	_ = JSON(Response{
		Success: false,
		Error:   message,
		Code:    code,
	})
	return &emittedError{message: message}
}

// ErrorFrom outputs an error response from an existing error.
func ErrorFrom(code string, err error) error {
	return Error(code, err.Error())
}
