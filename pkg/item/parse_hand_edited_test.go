package item

import (
	"bytes"
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
