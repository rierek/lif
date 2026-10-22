// Package data is the single touchpoint between the TUI and the lif store.
// Every read, write and watch of item data goes through Store, which uses the
// same itemcore and CoreResolver as the CLI, in-process.
package data

import (
	"strings"

	"github.com/rierek/lif/pkg/item"
)

// IsArchived reports whether it currently lives under the store's archive/
// directory: the core stamps every archived item's Path with a leading
// "archive/" segment and changes nothing else.
func IsArchived(it *item.Item) bool {
	return strings.HasPrefix(it.Path, "archive/")
}
