package tasks

import "strings"

// Printable returns s without the control characters of C0 and C1 and DEL,
// except newline and tab. Text from a .lif.yml (a task desc, a gate message)
// may come from a cloned repository: an escape sequence in it must not reach
// the terminal.
func Printable(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			return -1
		}
		return r
	}, s)
}
