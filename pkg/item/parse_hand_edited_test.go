package item

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// beans-ih2r: editors on Windows put a UTF-8 byte order mark in front of the
// first --- line. Parse must read the item as if the mark was not there, and
// the next write must carry neither the mark nor a second front matter block.
func TestParseStripsUTF8BOM(t *testing.T) {
	raw := "\xEF\xBB\xBF---\n# bom-ab12\ntitle: Written in Notepad\nstatus: todo\n---\n\nBody text.\n"

	b, err := Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if b.Title != "Written in Notepad" || b.Status != "todo" {
		t.Errorf("Parse() title = %q, status = %q; want the values of the front matter behind the BOM", b.Title, b.Status)
	}
	if b.Body != "\nBody text." {
		t.Errorf("Parse() body = %q, want the text after the front matter only", b.Body)
	}

	// The etag names the bytes on disk: Core.Update hashes the file it reads
	// back, BOM included, so the two sides must agree.
	if got, want := b.ETag(), ETagOf([]byte(raw)); got != want {
		t.Errorf("ETag() = %s, want %s (hash of the raw file bytes, BOM included)", got, want)
	}

	b.ID = "bom-ab12"
	out, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if bytes.Contains(out, []byte("\xEF\xBB\xBF")) {
		t.Errorf("Render() wrote a BOM:\n%q", out)
	}
	if n := strings.Count(string(out), "---\n"); n != 2 {
		t.Errorf("Render() wrote %d front matter delimiters, want 2 (one block):\n%s", n, out)
	}
}

// beans-1wnd: a front matter number JSON cannot carry (.nan, .inf) used to
// load fine and then break every JSON output of the item; the GraphQL extra
// field even panicked. Parse refuses it at the door and names the key, so the
// loader's file path plus the key say exactly what to fix.
func TestParseRejectsNumbersJSONCannotCarry(t *testing.T) {
	tests := []struct {
		name  string
		front string
		key   string
	}{
		{"not a number", "ratio: .nan", "ratio"},
		{"positive infinity", "ratio: .inf", "ratio"},
		{"negative infinity", "ratio: -.inf", "ratio"},
		{"inside a map", "meta:\n  ratio: .NaN", "meta.ratio"},
		{"inside a list", "scores:\n  - 1\n  - .inf", "scores[1]"},
		{"inside a list of maps", "rows:\n  - a: 1\n  - a: .nan", "rows[1].a"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader("---\ntitle: T\nstatus: todo\n" + tc.front + "\n---\n"))
			if err == nil {
				t.Fatal("Parse() error = nil, want an error that names the key")
			}
			if want := `key "` + tc.key + `"`; !strings.Contains(err.Error(), want) {
				t.Errorf("Parse() error = %q, want it to contain %s", err, want)
			}
		})
	}
}

// The same value in TOML front matter, which the front matter reader also
// accepts, takes the same road.
func TestParseRejectsNumbersJSONCannotCarryInTOML(t *testing.T) {
	for name, input := range map[string]string{
		"top level":  "+++\ntitle = \"T\"\nratio = nan\n+++\n",
		"in a table": "+++\ntitle = \"T\"\n\n[meta]\nratio = inf\n+++\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(strings.NewReader(input)); err == nil || !strings.Contains(err.Error(), "ratio") {
				t.Errorf("Parse() error = %v, want an error that names ratio", err)
			}
		})
	}
}

// Finite numbers and text that only looks like the refused values stay valid.
func TestParseKeepsFiniteNumbersAndQuotedText(t *testing.T) {
	b, err := Parse(strings.NewReader("---\ntitle: T\nstatus: todo\nratio: 1.5\nzero: 0\nquoted: \".nan\"\nword: nan\n---\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	want := map[string]any{"ratio": 1.5, "zero": 0, "quoted": ".nan", "word": "nan"}
	if !reflect.DeepEqual(b.Extra, want) {
		t.Errorf("Extra = %#v, want %#v", b.Extra, want)
	}
}

// beans-533z: the ID of an item is the one in its file name, so a file renamed
// by hand answers to a new ID. Parse keeps the ID the file itself records in
// its "# <id>" comment, so lif check can tell the two apart.
func TestParseRecordsTheIDCommentLine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"as Render writes it", "---\n# ex-ab12\ntitle: T\n---\n", "ex-ab12"},
		{"CRLF line endings", "---\r\n# ex-ab12\r\ntitle: T\r\n---\r\n", "ex-ab12"},
		{"behind a BOM", "\xEF\xBB\xBF---\n# ex-ab12\ntitle: T\n---\n", "ex-ab12"},
		{"blank lines before the opening delimiter", "\n\n---\n# ex-ab12\ntitle: T\n---\n", "ex-ab12"},
		{"extra spaces after the hash", "---\n#   ex-ab12  \ntitle: T\n---\n", "ex-ab12"},
		{"no comment", "---\ntitle: T\n---\n", ""},
		{"a comment of several words is a note, not an ID", "---\n# remember to split this\ntitle: T\n---\n", ""},
		{"a comment below the first line is not the ID line", "---\ntitle: T\n# ex-ab12\n---\n", ""},
		{"no front matter", "# ex-ab12\nBody\n", ""},
		{"empty file", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := Parse(strings.NewReader(tc.input))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if b.RecordedID != tc.want {
				t.Errorf("RecordedID = %q, want %q", b.RecordedID, tc.want)
			}
		})
	}
}

// A write puts the item's own ID into the comment; the item must report that
// ID as recorded from then on, not the one it was loaded with.
func TestSetContentETagRefreshesTheRecordedID(t *testing.T) {
	b, err := Parse(strings.NewReader("---\n# ex-old1\ntitle: T\nstatus: todo\n---\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	b.ID = "ex-new1"
	content, err := b.Render()
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	b.SetContentETag(content)
	if b.RecordedID != "ex-new1" {
		t.Errorf("RecordedID after a write = %q, want %q", b.RecordedID, "ex-new1")
	}
}

// The recorded ID is for the checks, not for the data: neither JSON nor the
// written front matter carries it.
func TestRecordedIDIsNotPartOfTheItemData(t *testing.T) {
	b, err := Parse(strings.NewReader("---\n# ex-ab12\ntitle: T\nstatus: todo\n---\n"))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	b.ID = "ex-ab12"
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(data), "ecorded") {
		t.Errorf("JSON carries the recorded ID: %s", data)
	}
}
