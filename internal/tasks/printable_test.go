package tasks

import "testing"

func TestPrintableStripsEscapeSequencesButKeepsNewlineAndTab(t *testing.T) {
	in := "\x1b[31mRED\x1b]0;TITLE\a\x1b[2J ok\tx\ny\u009b31m\x7f"
	if got, want := Printable(in), "[31mRED]0;TITLE[2J ok\tx\ny31m"; got != want {
		t.Errorf("Printable = %q, want %q", got, want)
	}
	if got := Printable("plain ünïcode ✓"); got != "plain ünïcode ✓" {
		t.Errorf("Printable changed plain text: %q", got)
	}
}
