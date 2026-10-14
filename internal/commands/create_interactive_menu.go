package commands

import (
	"context"
	"io"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/xRiErOS/lif/pkg/candidates"
	"github.com/xRiErOS/lif/pkg/itemgraph"
)

// createMenuAction is what the action menu hands back to the run core.
type createMenuAction int

const createMenuCreate createMenuAction = iota

const (
	menuBody      = "body"
	menuCreate    = "create"
	menuTags      = "tags"
	menuBlockedBy = "blocked_by"
	menuBlocking  = "blocking"
	menuStatus    = "status"
)

// runCreateMenuWith shows the action menu until the user picks "create".
// Every other entry runs its step on *d and returns to the menu with the
// cursor back on "create". Ctrl+C anywhere returns errCreateAborted.
func runCreateMenuWith(d *createDraft, in createInput, out io.Writer) (createMenuAction, error) {
	for {
		choice := menuCreate
		// The "create next" slot of beans-tem3 goes directly after "create".
		err := runCreateForm(in, out, nil, huh.NewGroup(huh.NewSelect[string]().
			Title("Action").
			Options(
				huh.NewOption("go to body", menuBody),
				huh.NewOption("create", menuCreate),
				huh.NewOption("set tags", menuTags),
				huh.NewOption("set blocked_by", menuBlockedBy),
				huh.NewOption("set blocking", menuBlocking),
				huh.NewOption("set status", menuStatus),
			).Value(&choice)))
		if err != nil {
			return 0, err
		}
		switch choice {
		case menuCreate:
			return createMenuCreate, nil
		case menuBody:
			err = runCreateBodyStep(d, in, out)
		case menuTags:
			err = runCreateTagsStep(d, in, out)
		case menuBlockedBy:
			err = runCreateRelationStep(&d.BlockedBy, "Blocked by", in, out)
		case menuBlocking:
			err = runCreateRelationStep(&d.Blocking, "Blocking", in, out)
		case menuStatus:
			err = runCreateStatusStep(d, in, out)
		}
		if err != nil {
			return 0, err
		}
	}
}

// runCreateBodyStep edits the body inline; the huh editor key opens $EDITOR.
func runCreateBodyStep(d *createDraft, in createInput, out io.Writer) error {
	body := d.Body
	if err := runCreateForm(in, out, nil, huh.NewGroup(huh.NewText().Title("Body").Value(&body))); err != nil {
		return err
	}
	d.Body = body
	return nil
}

// runCreateTagsStep edits the tag set as a comma separated list. The draft
// changes only when every tag passes the check CreateItem runs.
func runCreateTagsStep(d *createDraft, in createInput, out io.Writer) error {
	text := strings.Join(d.Tags, ", ")
	field := huh.NewInput().Title("Tags").Description("comma separated").Value(&text).
		Validate(func(s string) error { return itemgraph.ValidateTags(splitTags(s)) })
	if err := runCreateForm(in, out, nil, huh.NewGroup(field)); err != nil {
		return err
	}
	d.Tags = splitTags(text)
	return nil
}

func splitTags(s string) []string {
	var tags []string
	for _, t := range strings.Split(s, ",") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

// runCreateRelationStep edits one relation list over the candidates a new
// item may relate to.
func runCreateRelationStep(ids *[]string, title string, in createInput, out io.Writer) error {
	resolver := &itemgraph.CoreResolver{Core: core}
	cands, err := candidates.BlockingCandidates(context.Background(), resolver, cfg, "")
	if err != nil {
		return err
	}
	opts := make([]huh.Option[string], 0, len(cands))
	for _, c := range cands {
		opts = append(opts, huh.NewOption(c.ID+"  "+c.Title, c.ID).Selected(slices.Contains(*ids, c.ID)))
	}
	var sel []string
	field := huh.NewMultiSelect[string]().Title(title).Filterable(true).Options(opts...).Value(&sel)
	if err := runCreateForm(in, out, nil, huh.NewGroup(field)); err != nil {
		return err
	}
	*ids = sel
	return nil
}

// runCreateStatusStep picks the draft status among the configured statuses.
func runCreateStatusStep(d *createDraft, in createInput, out io.Writer) error {
	status := d.Status
	if status == "" {
		status = cfg.GetDefaultStatus()
	}
	var opts []huh.Option[string]
	for _, s := range candidates.StatusCandidates(cfg) {
		opts = append(opts, huh.NewOption(s.Name, s.Name))
	}
	field := huh.NewSelect[string]().Title("Status").Options(opts...).Value(&status)
	if err := runCreateForm(in, out, nil, huh.NewGroup(field)); err != nil {
		return err
	}
	d.Status = status
	return nil
}
