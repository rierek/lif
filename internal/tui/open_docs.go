package tui

// open_docs.go — palette entry "open: documentation": opens the published
// TUI reference in the OS default browser. Failure shows the URL in a toast.

import (
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
)

// docsURL is the only place that names the documentation site.
const docsURL = "https://rierek.github.io/lif/reference/tui/views/"

// openURL starts the OS opener; a package variable so tests never launch a
// browser.
var openURL = func(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }() // reap; opener exit status is not observed
	return nil
}

func (m model) openDocs() (tea.Model, tea.Cmd) {
	if err := openURL(docsURL); err != nil {
		return m.showToast(toastError, "cannot open browser: "+docsURL, "", nil, false)
	}
	return m.showToast(toastInfo, "opened "+docsURL, "", nil, false)
}
