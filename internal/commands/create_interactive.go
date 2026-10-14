package commands

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/xRiErOS/lif/internal/output"
	"github.com/xRiErOS/lif/internal/ui"
	"github.com/xRiErOS/lif/pkg/candidates"
	"github.com/xRiErOS/lif/pkg/item"
	"github.com/xRiErOS/lif/pkg/itemcore"
	"github.com/xRiErOS/lif/pkg/itemgraph"
	"github.com/xRiErOS/lif/pkg/itemgraph/model"
)

// createInteractive backs `lif create -i`.
var createInteractive bool

// errCreateAborted is returned on cancel: non-zero exit, no stderr line,
// empty stdout (D23), like lif pick.
var errCreateAborted = output.Silent("lif create: cancelled, nothing written")

// createInput returns the reader for the next huh program of a run. One
// bubbletea program can read ahead and swallow key bytes meant for the next,
// so every wizard function takes this source, never a bare reader.
type createInput func() io.Reader

// createDraft holds every value the write uses that a form step or menu
// entry can change.
type createDraft struct {
	Title, Type, Priority, Parent, Status, Body string
	Tags, Blocking, BlockedBy                   []string
}

// requireCreateTerminal is the -i guard: it runs before anything reads
// stdin, so an agent or script never waits for input.
func requireCreateTerminal() error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("lif create -i needs a terminal: stdin is not a terminal")
	}
	return nil
}

// runCreateInteractive opens /dev/tty once, as runPick does, and runs the
// form on it so stdout carries only the result.
func runCreateInteractive(cmd *cobra.Command, args []string) error {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return cmdError(createJSON, output.ErrFileError, "lif create -i: opening controlling terminal: %v", err)
	}
	defer tty.Close()
	return runCreateInteractiveWith(cmd, args, func() io.Reader { return tty }, tty)
}

// newCreateDraft pre-fills the draft from the title arguments, the create
// flags and the config defaults, in the order flag, config default, empty.
// There is no parent default.
func newCreateDraft(args []string) createDraft {
	d := createDraft{
		Title:     strings.Join(args, " "),
		Type:      cmp.Or(createType, cfg.GetDefaultType()),
		Priority:  cmp.Or(createPriority, cfg.GetDefaultPriority()),
		Status:    cmp.Or(createStatus, cfg.GetDefaultStatus()),
		Parent:    createParent,
		Body:      createBody,
		Tags:      createTag,
		Blocking:  createBlocking,
		BlockedBy: createBlockedBy,
	}
	if len(d.Tags) == 0 {
		d.Tags = cfg.GetDefaultTags()
	}
	if full, ok := core.NormalizeID(d.Parent); ok {
		d.Parent = full
	}
	return d
}

// runCreateForm builds and runs one wizard form: the single place that sets
// theme, keymap and the input/output pair.
func runCreateForm(in createInput, out io.Writer, layout huh.Layout, groups ...*huh.Group) error {
	if layout == nil {
		layout = huh.LayoutDefault
	}
	f := huh.NewForm(groups...).
		WithInput(in()).
		WithOutput(out).
		WithLayout(layout).
		WithKeyMap(huh.NewDefaultKeyMap()).
		WithTheme(ui.FormTheme(ui.FlavourFormPalette(ui.ActiveTheme())))
	if err := f.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return errCreateAborted
		}
		return err
	}
	return nil
}

// runCreateFormWith runs the steps title, type, priority, parent from the
// values in *d and writes the results back. An untouched "(none)" priority
// leaves Priority empty; a parent that is no candidate for the chosen type
// is dropped.
func runCreateFormWith(d *createDraft, in createInput, out io.Writer) error {
	resolver := &itemgraph.CoreResolver{Core: core}

	typeOpts := make([]huh.Option[string], 0)
	for _, tc := range candidates.TypeCandidates(cfg) {
		typeOpts = append(typeOpts, huh.NewOption(tc.Name, tc.Name))
	}
	prioOpts := []huh.Option[string]{huh.NewOption("(none)", "")}
	for _, pc := range candidates.PriorityCandidates(cfg) {
		prioOpts = append(prioOpts, huh.NewOption(pc.Name, pc.Name))
	}

	picker := newParentPicker(context.Background(), resolver, cfg, &d.Type, &d.Parent)
	err := runCreateForm(in, out, picker.Layout(),
		huh.NewGroup(huh.NewInput().Title("Title").Value(&d.Title)),
		huh.NewGroup(huh.NewSelect[string]().Title("Type").Options(typeOpts...).Value(&d.Type)),
		huh.NewGroup(huh.NewSelect[string]().Title("Priority").Options(prioOpts...).Value(&d.Priority)),
		picker.Group(),
	)
	if err != nil {
		return err
	}

	if d.Parent != "" {
		cands, err := candidates.ParentCandidates(context.Background(), resolver, cfg, nil, []string{d.Type})
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(cands, func(it *item.Item) bool { return it.ID == d.Parent }) {
			d.Parent = ""
		}
	}
	return nil
}

// writeCreateDraft is the sole CreateItem call site for plain create and -i.
// It prints nothing.
func writeCreateDraft(cmd *cobra.Command, d createDraft) (*item.Item, error) {
	title := d.Title
	if title == "" {
		title = "Untitled"
	}
	input := model.CreateItemInput{Title: title, Status: &d.Status}
	if d.Status == "" {
		defaultStatus := cfg.GetDefaultStatus()
		input.Status = &defaultStatus
	}
	if d.Type != "" {
		input.Type = &d.Type
	} else {
		defaultType := cfg.GetDefaultType()
		input.Type = &defaultType
	}
	if d.Priority != "" {
		input.Priority = &d.Priority
	}
	if d.Body != "" {
		input.Body = &d.Body
	}
	if len(d.Tags) > 0 {
		input.Tags = d.Tags
	}
	if d.Parent != "" {
		input.Parent = &d.Parent
	}
	if len(d.Blocking) > 0 {
		input.Blocking = d.Blocking
	}
	if len(d.BlockedBy) > 0 {
		input.BlockedBy = d.BlockedBy
	}
	if createPrefix != "" {
		input.Prefix = &createPrefix
	}

	setMap, err := extraSetMap(createSet)
	if err != nil {
		return nil, cmdError(createJSON, output.ErrValidation, "%s", err)
	}

	resolver := &itemgraph.CoreResolver{Core: core}
	b, err := resolver.CreateItem(context.Background(), input, itemcore.WithExtraOps(setMap, nil))
	if err != nil {
		return nil, cmdError(createJSON, output.ErrFileError, "failed to create item: %v", err)
	}

	// An explicit order and --unset aren't part of the generated
	// CreateBeanInput type or WithExtraOps (which only sets), so
	// they're applied and persisted as a second write.
	if len(createUnset) > 0 || cmd.Flags().Changed("order") {
		// Capture the freshly-created item's own etag before mutating it,
		// and pass it as ifMatch below instead of nil: nil bypasses
		// optimistic concurrency control -- silently ignored under a
		// normal config, and under require_if_match:true it makes the
		// write fail outright, leaving the item on disk without the
		// order/unset that were just requested. The item's own current
		// etag satisfies require_if_match and still catches a genuine
		// concurrent external write between the two writes.
		etag := b.ETag()
		for _, key := range createUnset {
			delete(b.Extra, key)
		}
		if cmd.Flags().Changed("order") {
			b.Order = createOrder
		}
		if err := core.Update(b, &etag); err != nil {
			return nil, mutationError(createJSON, err)
		}
	}
	return b, nil
}

// printCreated prints the result: the item JSON or the plain Created line.
func printCreated(b *item.Item) error {
	if createJSON {
		return output.SuccessSingle(b)
	}
	fmt.Println(ui.Success.Render("Created ") + ui.ID.Render(b.ID) + " " + ui.Muted.Render(b.Path))
	return nil
}

// runCreateInteractiveWith is runCreateInteractive's testable core.
func runCreateInteractiveWith(cmd *cobra.Command, args []string, in createInput, out io.Writer) error {
	d := newCreateDraft(args)
	if err := runCreateFormWith(&d, in, out); err != nil {
		return err
	}
	b, err := writeCreateDraft(cmd, d)
	if err != nil {
		return err
	}
	return printCreated(b)
}
