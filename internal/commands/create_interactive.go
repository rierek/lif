package commands

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/internal/ui"
	"github.com/rierek/lif/pkg/candidates"
	"github.com/rierek/lif/pkg/item"
	"github.com/rierek/lif/pkg/itemcore"
	"github.com/rierek/lif/pkg/itemgraph"
	"github.com/rierek/lif/pkg/itemgraph/model"
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

// cancelHintLayout appends the cancel hint to the footer of every step. huh
// binds Ctrl+C but lists it in no field's help line.
type cancelHintLayout struct {
	huh.Layout
	groups []*huh.Group
}

func (l cancelHintLayout) View(f *huh.Form) string {
	// huh sizes every group to the tallest one (the parent picker); give each
	// its own content height so a one-line title is not padded.
	for _, g := range l.groups {
		h := lipgloss.Height(g.Content())
		if foot := g.Footer(); foot != "" {
			h += lipgloss.Height(foot)
		}
		g.WithHeight(h)
	}
	hint := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ui.ActiveTheme().Hex("overlay1"))).
		Render("ctrl+c cancel")
	return l.Layout.View(f) + "\n" + hint
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
		WithLayout(cancelHintLayout{layout, groups}).
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
// leaves Priority empty; the parent step drops a parent that is no candidate
// for the chosen type.
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
	return runCreateForm(in, out, picker,
		huh.NewGroup(huh.NewInput().Title("Title").Value(&d.Title)),
		huh.NewGroup(huh.NewSelect[string]().Title("Type").Options(typeOpts...).Value(&d.Type)),
		huh.NewGroup(huh.NewSelect[string]().Title("Priority").Options(prioOpts...).Value(&d.Priority)),
		picker.group,
	)
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
		return nil, cmdError(createJSON, mutationErrorCode(err), "failed to create item: %v", err)
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

// runCreateInteractiveWith is runCreateInteractive's testable core. It
// repeats form, menu and write while the user picks "create next". A cancel
// after at least one written item ends the run with success; before that it
// returns errCreateAborted.
func runCreateInteractiveWith(cmd *cobra.Command, args []string, in createInput, out io.Writer) (err error) {
	d := newCreateDraft(args)
	carry := newCreateCarry()
	wrote := false
	defer func() {
		if wrote && err == errCreateAborted {
			err = nil
		}
	}()
	for {
		if err := runCreateFormWith(&d, in, out); err != nil {
			return err
		}
		next, err := runCreateMenuWith(&d, createJSON, in, out)
		if err != nil {
			return err
		}
		b, err := writeCreateDraft(cmd, d)
		if err != nil {
			return err
		}
		wrote = true
		if err := printCreated(b); err != nil {
			return err
		}
		if !next {
			return nil
		}
		if err := runCreateCarryStep(&carry, in, out); err != nil {
			return err
		}
		d = nextCreateDraft(d, carry)
	}
}
