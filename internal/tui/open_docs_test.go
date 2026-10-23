package tui

import (
	"errors"
	"strings"
	"testing"
)

func stubOpener(t *testing.T, err error) *[]string {
	var got []string
	old := openURL
	openURL = func(u string) error { got = append(got, u); return err }
	t.Cleanup(func() { openURL = old })
	return &got
}

func TestOpenDocsPaletteEntryListed(t *testing.T) {
	for _, it := range paletteActions(model{}) {
		if it.label == "open: documentation" {
			return
		}
	}
	t.Fatal("palette lacks open: documentation")
}

func TestOpenDocsSuccessToastsAndKeepsView(t *testing.T) {
	got := stubOpener(t, nil)
	m := model{view: viewReady}
	nm, _ := m.dispatchPalette(paletteItem{kind: paletteKindAction, actionID: "open_docs"})
	mm := nm.(model)
	if len(*got) != 1 || (*got)[0] != docsURL {
		t.Fatalf("opener calls = %v", *got)
	}
	if mm.toast == nil || mm.toast.kind != toastInfo || !strings.Contains(mm.toast.title, docsURL) {
		t.Fatalf("toast = %+v", mm.toast)
	}
	if mm.view != viewReady {
		t.Fatalf("view changed to %v", mm.view)
	}
}

func TestOpenDocsFailureToastsURL(t *testing.T) {
	stubOpener(t, errors.New("no opener"))
	nm, _ := model{}.dispatchPalette(paletteItem{kind: paletteKindAction, actionID: "open_docs"})
	mm := nm.(model)
	if mm.toast == nil || mm.toast.kind != toastError || !strings.Contains(mm.toast.title, docsURL) {
		t.Fatalf("toast = %+v", mm.toast)
	}
}
