package commands

import (
	"io"
	"slices"

	"github.com/charmbracelet/huh"
)

// createCarry is the carry-over choice of one `create next` run. It lives in
// process memory only: the first checklist of a run starts all ticked, every
// later one with the entries confirmed last time.
type createCarry struct{ Type, Priority, Parent, Tags bool }

func newCreateCarry() createCarry { return createCarry{true, true, true, true} }

const (
	carryType     = "type"
	carryPriority = "priority"
	carryParent   = "parent"
	carryTags     = "tags"
)

// runCreateCarryStep shows the carry-over checklist and stores the confirmed
// entries in *c. Ctrl+C returns errCreateAborted.
func runCreateCarryStep(c *createCarry, in createInput, out io.Writer) error {
	opts := []huh.Option[string]{
		huh.NewOption(carryType, carryType).Selected(c.Type),
		huh.NewOption(carryPriority, carryPriority).Selected(c.Priority),
		huh.NewOption(carryParent, carryParent).Selected(c.Parent),
		huh.NewOption(carryTags, carryTags).Selected(c.Tags),
	}
	var sel []string
	field := huh.NewMultiSelect[string]().
		Title("Carry over to the next item").
		Filterable(true).Options(opts...).Value(&sel)
	if err := runCreateForm(in, out, nil, huh.NewGroup(field)); err != nil {
		return err
	}
	*c = createCarry{
		Type:     slices.Contains(sel, carryType),
		Priority: slices.Contains(sel, carryPriority),
		Parent:   slices.Contains(sel, carryParent),
		Tags:     slices.Contains(sel, carryTags),
	}
	return nil
}

// nextCreateDraft builds the draft of the form after `create next`: the
// first form's pre-fill without the title argument, the ticked fields
// overlaid by the item just written; title and body stay empty (D06).
func nextCreateDraft(prev createDraft, c createCarry) createDraft {
	d := newCreateDraft(nil)
	d.Title, d.Body = "", ""
	if c.Type {
		d.Type = prev.Type
	}
	if c.Priority {
		d.Priority = prev.Priority
	}
	if c.Parent {
		d.Parent = prev.Parent
	}
	if c.Tags {
		d.Tags = prev.Tags
	}
	return d
}
