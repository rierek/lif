package commands

import (
	"testing"

	"github.com/rierek/lif/pkg/item"
)

func TestApplyTags(t *testing.T) {
	tests := []struct {
		name     string
		initial  []string
		toAdd    []string
		wantTags []string
		wantErr  bool
	}{
		{
			name:     "add single tag",
			initial:  nil,
			toAdd:    []string{"bug"},
			wantTags: []string{"bug"},
		},
		{
			name:     "add multiple tags",
			initial:  nil,
			toAdd:    []string{"bug", "urgent"},
			wantTags: []string{"bug", "urgent"},
		},
		{
			name:     "add to existing tags",
			initial:  []string{"existing"},
			toAdd:    []string{"new"},
			wantTags: []string{"existing", "new"},
		},
		{
			name:     "empty tags list",
			initial:  []string{"existing"},
			toAdd:    []string{},
			wantTags: []string{"existing"},
		},
		{
			name:    "invalid tag with spaces",
			initial: nil,
			toAdd:   []string{"invalid tag"},
			wantErr: true,
		},
		{
			name:     "uppercase tag gets normalized",
			initial:  nil,
			toAdd:    []string{"InvalidTag"},
			wantTags: []string{"invalidtag"}, // normalized to lowercase
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &item.Item{Tags: tt.initial}
			err := applyTags(b, tt.toAdd)

			if tt.wantErr {
				if err == nil {
					t.Errorf("applyTags() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("applyTags() unexpected error: %v", err)
				return
			}

			if len(b.Tags) != len(tt.wantTags) {
				t.Errorf("applyTags() tags count = %d, want %d", len(b.Tags), len(tt.wantTags))
				return
			}

			for i, want := range tt.wantTags {
				if b.Tags[i] != want {
					t.Errorf("applyTags() tags[%d] = %q, want %q", i, b.Tags[i], want)
				}
			}
		})
	}
}

func TestFormatCycle(t *testing.T) {
	tests := []struct {
		path []string
		want string
	}{
		{[]string{"a", "b", "c", "a"}, "a → b → c → a"},
		{[]string{"x", "y"}, "x → y"},
		{[]string{"single"}, "single"},
		{[]string{}, ""},
	}

	for _, tt := range tests {
		got := formatCycle(tt.path)
		if got != tt.want {
			t.Errorf("formatCycle(%v) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestApplyBodyReplace(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		old     string
		new     string
		want    string
		wantErr string
	}{
		{
			name: "successful replacement",
			body: "- [ ] Task 1\n- [ ] Task 2",
			old:  "- [ ] Task 1",
			new:  "- [x] Task 1",
			want: "- [x] Task 1\n- [ ] Task 2",
		},
		{
			name: "delete text with empty new",
			body: "Hello world",
			old:  " world",
			new:  "",
			want: "Hello",
		},
		{
			name: "replace in middle of text",
			body: "Line 1\nLine 2\nLine 3",
			old:  "Line 2",
			new:  "Modified Line 2",
			want: "Line 1\nModified Line 2\nLine 3",
		},
		{
			name: "replace entire body",
			body: "Old content",
			old:  "Old content",
			new:  "New content",
			want: "New content",
		},
		{
			name:    "text not found",
			body:    "Hello world",
			old:     "foo",
			new:     "bar",
			wantErr: "text not found in body",
		},
		{
			name:    "multiple matches",
			body:    "foo foo foo",
			old:     "foo",
			new:     "bar",
			wantErr: "text found 3 times in body (must be unique)",
		},
		{
			name:    "empty old string",
			body:    "Hello",
			old:     "",
			new:     "world",
			wantErr: "old text cannot be empty",
		},
		{
			name: "partial match only once",
			body: "Task 1\nTask 2\nTask 3",
			old:  "Task 2",
			new:  "Completed Task 2",
			want: "Task 1\nCompleted Task 2\nTask 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyBodyReplace(tt.body, tt.old, tt.new)

			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("applyBodyReplace() expected error containing %q, got nil", tt.wantErr)
					return
				}
				if !contains(err.Error(), tt.wantErr) {
					t.Errorf("applyBodyReplace() error = %q, want error containing %q", err.Error(), tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("applyBodyReplace() unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("applyBodyReplace() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplyBodyAppend(t *testing.T) {
	tests := []struct {
		name string
		body string
		text string
		want string
	}{
		{
			name: "append to empty body",
			body: "",
			text: "New content",
			want: "New content",
		},
		{
			name: "append to existing body",
			body: "Existing content",
			text: "New content",
			want: "Existing content\n\nNew content",
		},
		{
			name: "append strips trailing newlines from body",
			body: "Existing\n\n\n",
			text: "New",
			want: "Existing\n\nNew",
		},
		{
			name: "append multiline content",
			body: "Header",
			text: "## Section\n\nParagraph",
			want: "Header\n\n## Section\n\nParagraph",
		},
		{
			name: "append to body with single trailing newline",
			body: "Content\n",
			text: "More",
			want: "Content\n\nMore",
		},
		{
			name: "append empty text is no-op",
			body: "Content",
			text: "",
			want: "Content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyBodyAppend(tt.body, tt.text)
			if got != tt.want {
				t.Errorf("applyBodyAppend() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveAppendContent(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "direct value",
			value: "some text",
			want:  "some text",
		},
		{
			name:  "direct multiline value",
			value: "line 1\nline 2",
			want:  "line 1\nline 2",
		},
		{
			name:  "empty value",
			value: "",
			want:  "",
		},
		// Note: stdin case ("-") is tested in integration tests
		// as it's difficult to mock in unit tests
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveAppendContent(tt.value)
			if err != nil {
				t.Errorf("resolveAppendContent() unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("resolveAppendContent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseSetPair(t *testing.T) {
	tests := []struct {
		name      string
		arg       string
		wantKey   string
		wantValue string
		wantErr   bool
	}{
		{"simple", "release=0-4-1", "release", "0-4-1", false},
		{"value contains equals", "url=https://example.com?a=b", "url", "https://example.com?a=b", false},
		{"empty value", "flag=", "flag", "", false},
		{"no equals sign", "release", "", "", true},
		{"empty key", "=value", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, value, err := parseSetPair(tt.arg, "--set")
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseSetPair(%q) error = %v, wantErr %v", tt.arg, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if key != tt.wantKey || value != tt.wantValue {
				t.Errorf("parseSetPair(%q) = (%q, %q), want (%q, %q)", tt.arg, key, value, tt.wantKey, tt.wantValue)
			}
		})
	}
}

func TestValidateExtraKeys(t *testing.T) {
	t.Run("unknown keys pass", func(t *testing.T) {
		if err := validateExtraKeys([]string{"release=0-4-1", "klasse=bugfix"}, []string{"other"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("reserved set key names its flag", func(t *testing.T) {
		err := validateExtraKeys([]string{"status=done"}, nil)
		if err == nil {
			t.Fatal("expected error for reserved key")
		}
		if !contains(err.Error(), "--status") {
			t.Errorf("expected error to name --status, got %q", err.Error())
		}
	})

	t.Run("reserved unset key names its flag", func(t *testing.T) {
		err := validateExtraKeys(nil, []string{"title"})
		if err == nil {
			t.Fatal("expected error for reserved key")
		}
		if !contains(err.Error(), "--title") {
			t.Errorf("expected error to name --title, got %q", err.Error())
		}
	})

	t.Run("managed field without native flag", func(t *testing.T) {
		err := validateExtraKeys([]string{"order=1"}, nil)
		if err == nil {
			t.Fatal("expected error for managed field")
		}
		if !contains(err.Error(), "managed field") {
			t.Errorf("expected error to mention managed field, got %q", err.Error())
		}
	})

	t.Run("set without equals sign is a usage error", func(t *testing.T) {
		err := validateExtraKeys([]string{"release"}, nil)
		if err == nil {
			t.Fatal("expected error for missing equals sign")
		}
	})
}

func TestApplyExtraOps(t *testing.T) {
	t.Run("sets keys", func(t *testing.T) {
		b := &item.Item{}
		if err := applyExtraOps(b, []string{"release=0-4-1", "klasse=bugfix"}, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if b.Extra["release"] != "0-4-1" || b.Extra["klasse"] != "bugfix" {
			t.Errorf("Extra = %#v, want release=0-4-1, klasse=bugfix", b.Extra)
		}
	})

	t.Run("unsets keys", func(t *testing.T) {
		b := &item.Item{Extra: map[string]any{"release": "0-4-1"}}
		if err := applyExtraOps(b, nil, []string{"release"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := b.Extra["release"]; ok {
			t.Errorf("expected release to be removed, Extra = %#v", b.Extra)
		}
	})

	t.Run("unset of absent key is a no-op", func(t *testing.T) {
		b := &item.Item{}
		if err := applyExtraOps(b, nil, []string{"nope"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(b.Extra) != 0 {
			t.Errorf("expected no Extra keys, got %#v", b.Extra)
		}
	})

	t.Run("same key set and unset: unset wins", func(t *testing.T) {
		b := &item.Item{}
		if err := applyExtraOps(b, []string{"release=0-4-1"}, []string{"release"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := b.Extra["release"]; ok {
			t.Errorf("expected release to be unset, Extra = %#v", b.Extra)
		}
	})
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexOfSubstring(s, substr) >= 0)
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
