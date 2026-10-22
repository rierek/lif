package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/spf13/cobra"
)

var (
	forceDelete bool
	deleteJSON  bool
)

// itemWithLinks holds an item, its incoming links and the attachment files
// that go with it, all collected before the first write so the prompt can
// state the whole consequence.
type itemWithLinks struct {
	item        *item.Item
	links       []itemcore.IncomingLink
	attachments []string
}

var deleteCmd = &cobra.Command{
	Use:     "delete <id> [id...]",
	Aliases: []string{"rm"},
	Short:   "Delete one or more items",
	Long: `Deletes one or more items after confirmation (use -f to skip confirmation).

If other items reference the target item(s) (as parent or via blocking), you will be
warned and those references will be removed after confirmation. Use -f to skip all warnings.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		resolver := newResolver()

		// Collect all items and their incoming links upfront (validate before deleting)
		var targets []itemWithLinks
		for _, id := range args {
			b, err := resolver.Item(ctx, id)
			if err != nil {
				return cmdError(deleteJSON, output.ErrNotFound, "failed to find item: %v", err)
			}
			if b == nil {
				return cmdError(deleteJSON, output.ErrNotFound, "item not found: %s", id)
			}
			atts, err := core.AttachmentNames(b.ID)
			if err != nil {
				return cmdError(deleteJSON, output.ErrFileError, "cannot read attachments of %s: %v", b.ID, err)
			}
			targets = append(targets, itemWithLinks{
				item:        b,
				links:       core.FindIncomingLinks(b.ID),
				attachments: atts,
			})
		}

		// Prompt for confirmation (JSON implies force)
		if !forceDelete && !deleteJSON {
			if !confirmDeleteMultiple(targets) {
				fmt.Println("Cancelled")
				return nil
			}
		}

		// Delete all items via GraphQL mutation
		var deleted []*item.Item
		var totalLinksRemoved int
		for _, target := range targets {
			_, err := resolver.DeleteItem(ctx, target.item.ID)
			if err != nil {
				return cmdError(deleteJSON, output.ErrFileError, "failed to delete item %s: %v", target.item.ID, err)
			}
			deleted = append(deleted, target.item)
			totalLinksRemoved += len(target.links)
		}

		// Output results
		if deleteJSON {
			if len(deleted) == 1 {
				return output.SuccessSingle(deleted[0])
			}
			// Bare array, like every other batch verb — the hand-built
			// envelope here was the one shape a consumer had to special-case.
			return output.SuccessMultiple(deleted)
		}

		if totalLinksRemoved > 0 {
			fmt.Printf("Removed %d reference(s)\n", totalLinksRemoved)
		}
		for _, b := range deleted {
			fmt.Printf("Deleted %s\n", b.Path)
		}
		return nil
	},
}

// confirmDeleteMultiple prompts the user to confirm deletion of one or more items.
func confirmDeleteMultiple(targets []itemWithLinks) bool {
	itemsWithLinks := 0
	totalLinks := 0
	for _, t := range targets {
		if len(t.links) > 0 {
			itemsWithLinks++
			totalLinks += len(t.links)
		}
	}

	// Single item: use simpler format
	if len(targets) == 1 {
		t := targets[0]
		if len(t.links) > 0 {
			fmt.Printf("Warning: %d item(s) link to '%s':\n", len(t.links), t.item.Title)
			for _, link := range t.links {
				fmt.Printf("  - %s (%s) via %s\n", link.FromItem.ID, link.FromItem.Title, link.LinkType)
			}
		}
		// The attachment directory goes with the item, so the files are
		// named before the question rather than counted after it: they
		// are the container's documents, and a consent given to "delete
		// this item" is not consent to delete those unseen.
		if len(t.attachments) > 0 {
			fmt.Printf("Attachments to be deleted with '%s':\n", t.item.Title)
			for _, name := range t.attachments {
				fmt.Printf("  - %s\n", name)
			}
		}
		if len(t.links) > 0 {
			fmt.Print("Delete anyway and remove references? [y/N] ")
		} else {
			fmt.Printf("Delete '%s' (%s)? [y/N] ", t.item.Title, t.item.Path)
		}
	} else {
		// Multiple items: show batch summary. Attachments are counted
		// here rather than listed -- a batch of twenty items would bury
		// the question under file names.
		fmt.Printf("About to delete %d item(s):\n", len(targets))
		for _, t := range targets {
			suffix := ""
			if len(t.attachments) > 0 {
				suffix = fmt.Sprintf(" + %d attachment(s)", len(t.attachments))
			}
			if len(t.links) > 0 {
				fmt.Printf("  - %s (%s) ← %d incoming link(s)%s\n", t.item.ID, t.item.Title, len(t.links), suffix)
			} else {
				fmt.Printf("  - %s (%s)%s\n", t.item.ID, t.item.Title, suffix)
			}
		}
		if itemsWithLinks > 0 {
			fmt.Printf("\nWarning: %d item(s) have incoming references (%d total) that will be removed.\n", itemsWithLinks, totalLinks)
		}
		fmt.Print("\nProceed with deletion? [y/N] ")
	}

	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

func RegisterDeleteCmd(root *cobra.Command) {
	deleteCmd.Flags().BoolVarP(&forceDelete, "force", "f", false, "Skip confirmation and warnings")
	deleteCmd.Flags().BoolVar(&deleteJSON, "json", false, "Output as JSON (implies --force)")
	deleteCmd.ValidArgsFunction = completionUnbounded
	root.AddCommand(deleteCmd)
}
