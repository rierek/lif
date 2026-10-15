package item

import (
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"basic spaces", "Hello World", "hello-world"},
		{"underscores", "hello_world", "hello-world"},
		{"special chars", "Hello! World?", "hello-world"},
		{"multiple spaces", "hello   world", "hello-world"},
		{"multiple dashes", "hello---world", "hello-world"},
		{"leading trailing dashes", "--hello--", "hello"},
		{"empty string", "", ""},
		{"numbers", "Test 123", "test-123"},
		{"mixed special chars", "Hello, World! How's it going?", "hello-world-hows-it-going"},
		{"only special chars", "!@#$%^&*()", ""},
		{"unicode letters", "Café Résumé", "café-résumé"},
		{"already lowercase", "already-slugified", "already-slugified"},
		{"all caps", "ALL CAPS", "all-caps"},
		{"spaces and underscores mixed", "hello world_test", "hello-world-test"},
		{
			"truncation at 50 chars",
			"this is a very long title that should be truncated to fifty characters",
			"this-is-a-very-long-title-that-should-be-truncated",
		},
		{
			"truncation removes trailing dash",
			"this is a very long title that should be truncated-at dash",
			"this-is-a-very-long-title-that-should-be-truncated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slugify(tt.input)
			if got != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseFilename(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		expectedID   string
		expectedSlug string
	}{
		// New format with double-dash
		{"new format basic", "abc--my-slug.md", "abc", "my-slug"},
		{"new format with prefix", "beans-z5r9--add-unit-tests.md", "beans-z5r9", "add-unit-tests"},
		{"new format long slug", "xyz--this-is-a-longer-slug.md", "xyz", "this-is-a-longer-slug"},

		// Dot format
		{"dot format basic", "abc.my-slug.md", "abc", "my-slug"},
		{"dot format with prefix", "beans-z5r9.add-unit-tests.md", "beans-z5r9", "add-unit-tests"},

		// Legacy format with single dash
		{"legacy format basic", "abc-my-slug.md", "abc", "my-slug"},
		{"legacy format multi-part slug", "abc-my-multi-part-slug.md", "abc", "my-multi-part-slug"},

		// ID only
		{"id only with md", "abc.md", "abc", ""},
		{"id only with prefix", "beans-z5r9.md", "beans", "z5r9"}, // legacy format interpretation
		{"id only no extension", "abc", "abc", ""},

		// Edge cases
		{"empty string", "", "", ""},
		{"just md extension", ".md", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotSlug := ParseFilename(tt.filename)
			if gotID != tt.expectedID || gotSlug != tt.expectedSlug {
				t.Errorf("ParseFilename(%q) = (%q, %q), want (%q, %q)",
					tt.filename, gotID, gotSlug, tt.expectedID, tt.expectedSlug)
			}
		})
	}
}

func TestBuildFilename(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		slug     string
		expected string
	}{
		{"with slug", "abc", "my-slug", "abc--my-slug.md"},
		{"empty slug", "abc", "", "abc.md"},
		{"prefixed id empty slug", "beans-z5r9", "", "beans-z5r9--.md"},
		{"with prefix id", "beans-z5r9", "add-tests", "beans-z5r9--add-tests.md"},
		{"long slug", "xyz", "this-is-a-longer-slug", "xyz--this-is-a-longer-slug.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildFilename(tt.id, tt.slug)
			if got != tt.expected {
				t.Errorf("BuildFilename(%q, %q) = %q, want %q",
					tt.id, tt.slug, got, tt.expected)
			}
		})
	}
}

func TestNewID(t *testing.T) {
	t.Run("length without prefix", func(t *testing.T) {
		id, err := NewID("", 4)
		if err != nil {
			t.Fatalf("NewID(\"\", 4) unexpected error: %v", err)
		}
		if len(id) != 4 {
			t.Errorf("NewID(\"\", 4) length = %d, want 4", len(id))
		}
	})

	t.Run("length with prefix", func(t *testing.T) {
		id, err := NewID("beans-", 4)
		if err != nil {
			t.Fatalf("NewID(\"beans-\", 4) unexpected error: %v", err)
		}
		if len(id) != 10 { // "beans-" (6) + 4
			t.Errorf("NewID(\"beans-\", 4) length = %d, want 10", len(id))
		}
	})

	t.Run("prefix preserved", func(t *testing.T) {
		prefix := "myapp-"
		id, err := NewID(prefix, 4)
		if err != nil {
			t.Fatalf("NewID(%q, 4) unexpected error: %v", prefix, err)
		}
		if !strings.HasPrefix(id, prefix) {
			t.Errorf("NewID(%q, 4) = %q, should start with prefix", prefix, id)
		}
	})

	t.Run("uses valid alphabet", func(t *testing.T) {
		id, err := NewID("", 100) // generate long ID to test alphabet
		if err != nil {
			t.Fatalf("NewID(\"\", 100) unexpected error: %v", err)
		}
		for _, r := range id {
			if !strings.ContainsRune(idAlphabet, r) {
				t.Errorf("NewID contains invalid character %q, should only use %q", r, idAlphabet)
			}
		}
	})

	t.Run("generates unique IDs", func(t *testing.T) {
		seen := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id, err := NewID("", 8)
			if err != nil {
				t.Fatalf("NewID(\"\", 8) unexpected error: %v", err)
			}
			if seen[id] {
				t.Errorf("NewID generated duplicate: %q", id)
			}
			seen[id] = true
		}
	})

	t.Run("never contains blocked words", func(t *testing.T) {
		for i := 0; i < 10000; i++ {
			id, err := NewID("", 4)
			if err != nil {
				t.Fatalf("NewID(\"\", 4) unexpected error: %v", err)
			}
			if containsBlockedWord(id) {
				t.Errorf("NewID generated ID containing blocked word: %q", id)
			}
		}
	})
}

func TestContainsBlockedWord(t *testing.T) {
	tests := []struct {
		id      string
		blocked bool
	}{
		{"cock", true},
		{"nazi", true},
		{"dick", true},
		{"fuck", true},
		{"ab3d", false},
		{"z7x2", false},
		{"safe", false},
		{"naz1", true}, // contains "naz" substring
		{"sass", true}, // contains "ass"
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := containsBlockedWord(tt.id)
			if got != tt.blocked {
				t.Errorf("containsBlockedWord(%q) = %v, want %v", tt.id, got, tt.blocked)
			}
		})
	}
}

func TestParseFilenameAndBuildFilenameRoundtrip(t *testing.T) {
	tests := []struct {
		name string
		id   string
		slug string
	}{
		{"basic", "abc", "my-slug"},
		{"with prefix", "beans-z5r9", "add-tests"},
		{"no slug", "xyz", ""},
		{"prefixed no slug", "beans-z5r9", ""},
		{"empty prefix with slug", "z5r9", "add-tests"},
		{"hyphenated prefix with slug", "SYNC-TASK-z5r9", "add-tests"},
		{"hyphenated prefix no slug", "SYNC-TASK-z5r9", ""},
		{"dotted id no slug", "a.b", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := BuildFilename(tt.id, tt.slug)
			gotID, gotSlug := ParseFilename(filename)
			if gotID != tt.id || gotSlug != tt.slug {
				t.Errorf("Roundtrip failed: BuildFilename(%q, %q) = %q, ParseFilename = (%q, %q)",
					tt.id, tt.slug, filename, gotID, gotSlug)
			}
		})
	}
}

func TestIDSuffix(t *testing.T) {
	tests := []struct {
		name, id, prefix, want string
	}{
		{"strips prefix", "bew_BeWiki-Python-Download-ljs5", "bew_BeWiki-Python-Download-", "ljs5"},
		{"no prefix match returns id", "abc-ljs5", "xyz-", "abc-ljs5"},
		{"empty prefix returns id", "abc-ljs5", "", "abc-ljs5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IDSuffix(tt.id, tt.prefix); got != tt.want {
				t.Errorf("IDSuffix(%q,%q) = %q, want %q", tt.id, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestRebrandID(t *testing.T) {
	tests := []struct {
		name, id, oldP, newP, want string
	}{
		{"long to short", "bew_BeWiki-Python-Download-ljs5", "bew_BeWiki-Python-Download-", "bew-", "bew-ljs5"},
		{"idempotent when already short", "bew-ljs5", "bew-", "bew-", "bew-ljs5"},
		{"id without old prefix keeps suffix", "ljs5", "bew_", "bew-", "bew-ljs5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RebrandID(tt.id, tt.oldP, tt.newP); got != tt.want {
				t.Errorf("RebrandID(%q,%q,%q) = %q, want %q", tt.id, tt.oldP, tt.newP, got, tt.want)
			}
		})
	}
}
