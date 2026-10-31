package commands

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"github.com/rierek/lif/internal/output"
	"github.com/rierek/lif/pkg/item"
)

// errInteractiveAborted is returned on cancel in list -i and roadmap -i: non-zero
// exit, no stderr line, like errCreateAborted.
var errInteractiveAborted = output.Silent("lif: cancelled")

// requirePickTerminal is the -i guard of list and roadmap: it runs before
// anything reads stdin, so an agent or script never waits for input. The
// result goes to stdout, so stdout must be a terminal too.
func requirePickTerminal(cmdName string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("lif %s -i needs a terminal: stdin is not a terminal", cmdName)
	}
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("lif %s -i needs a terminal: stdout is not a terminal", cmdName)
	}
	return nil
}

// openPickTTY opens /dev/tty once, as runCreateInteractive does, so the form
// draws there and stdout carries only the result. Call the returned close
// function when done.
func openPickTTY(cmdName string) (createInput, io.Writer, func(), error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("lif %s -i: opening controlling terminal: %w", cmdName, err)
	}
	return func() io.Reader { return tty }, tty, func() { tty.Close() }, nil
}

// pickLabel is the select label of an item: id, status, title.
func pickLabel(b *item.Item) string {
	return fmt.Sprintf("%s  %s  %s", b.ID, b.Status, b.Title)
}

// selectItem asks the user to choose one of items. items must not be empty.
func selectItem(title string, items []*item.Item, in createInput, out io.Writer) (*item.Item, error) {
	opts := make([]huh.Option[int], len(items))
	for i, b := range items {
		opts[i] = huh.NewOption(pickLabel(b), i)
	}
	var idx int
	err := runForm(errInteractiveAborted, in, out, nil, huh.NewGroup(
		huh.NewSelect[int]().Title(title).Options(opts...).Value(&idx)))
	if err != nil {
		return nil, err
	}
	return items[idx], nil
}

// askQuery runs a one-field form for the search query, pre-filled with *q.
func askQuery(q *string, in createInput, out io.Writer) error {
	return runForm(errInteractiveAborted, in, out, nil, huh.NewGroup(
		huh.NewInput().Title("Search").Value(q)))
}

// errPickJSON rejects -i with --json: the form needs a terminal, JSON is for
// scripts.
func errPickJSON(cmdName string) error {
	return errors.New("lif " + cmdName + ": --interactive cannot be combined with --json")
}
