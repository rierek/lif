package item

import (
	"strings"
	"testing"
)

// 0g6b: the blank line after the front matter is a separator, not content.
func TestParseBodySeparator(t *testing.T) {
	cases := []struct {
		name, raw, want string
	}{
		{"lf", "---\ntitle: T\nstatus: todo\n---\n\nBody.\n", "Body."},
		{"crlf", "---\r\ntitle: T\r\nstatus: todo\r\n---\r\n\r\nBody.", "Body."},
		{"no separator", "---\ntitle: T\nstatus: todo\n---\nBody.\n", "Body."},
		{"empty", "---\ntitle: T\nstatus: todo\n---\n\n", ""},
		{"leading blank line kept", "---\ntitle: T\nstatus: todo\n---\n\n\nBody.\n", "\nBody."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := Parse(strings.NewReader(tc.raw))
			if err != nil {
				t.Fatal(err)
			}
			if b.Body != tc.want {
				t.Errorf("Body = %q, want %q", b.Body, tc.want)
			}
		})
	}
}
