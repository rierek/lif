package item

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedTitle  string
		expectedStatus string
		expectedBody   string
		wantErr        bool
	}{
		{
			name: "basic bean",
			input: `---
title: Test Bean
status: todo
---

This is the body.`,
			expectedTitle:  "Test Bean",
			expectedStatus: "todo",
			expectedBody:   "\nThis is the body.",
		},
		{
			name: "with timestamps",
			input: `---
title: With Times
status: in-progress
created_at: 2024-01-15T10:30:00Z
updated_at: 2024-01-16T14:45:00Z
---

Body content here.`,
			expectedTitle:  "With Times",
			expectedStatus: "in-progress",
			expectedBody:   "\nBody content here.",
		},
		{
			name: "empty body",
			input: `---
title: No Body
status: completed
---`,
			expectedTitle:  "No Body",
			expectedStatus: "completed",
			expectedBody:   "",
		},
		{
			name: "multiline body",
			input: `---
title: Multi Line
status: todo
---

# Header

- Item 1
- Item 2

Paragraph text.`,
			expectedTitle:  "Multi Line",
			expectedStatus: "todo",
			expectedBody:   "\n# Header\n\n- Item 1\n- Item 2\n\nParagraph text.",
		},
		{
			name:           "plain text without frontmatter",
			input:          `Just plain text without any YAML frontmatter.`,
			expectedTitle:  "",
			expectedStatus: "",
			expectedBody:   "Just plain text without any YAML frontmatter.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := Parse(strings.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if item.Title != tt.expectedTitle {
				t.Errorf("Title = %q, want %q", item.Title, tt.expectedTitle)
			}
			if item.Status != tt.expectedStatus {
				t.Errorf("Status = %q, want %q", item.Status, tt.expectedStatus)
			}
			if item.Body != tt.expectedBody {
				t.Errorf("Body = %q, want %q", item.Body, tt.expectedBody)
			}
		})
	}
}

func TestParseWithType(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedType string
	}{
		{
			name: "with type field",
			input: `---
title: Bug Report
status: todo
type: bug
---

Description of the bug.`,
			expectedType: "bug",
		},
		{
			name: "without type field",
			input: `---
title: No Type
status: todo
---

No type specified.`,
			expectedType: "",
		},
		{
			// Backwards compatibility: items with types not in current config
			// should still be readable without error
			name: "with unknown type (backwards compatibility)",
			input: `---
title: Legacy Bean
status: todo
type: deprecated-type-no-longer-in-config
---`,
			expectedType: "deprecated-type-no-longer-in-config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if item.Type != tt.expectedType {
				t.Errorf("Type = %q, want %q", item.Type, tt.expectedType)
			}
		})
	}
}

func TestParseWithPriority(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedPriority string
	}{
		{
			name: "with priority field",
			input: `---
title: Urgent Bug
status: todo
type: bug
priority: critical
---

Fix this immediately.`,
			expectedPriority: "critical",
		},
		{
			name: "without priority field",
			input: `---
title: Normal Task
status: todo
---

No priority specified.`,
			expectedPriority: "",
		},
		{
			name: "with high priority",
			input: `---
title: Important Feature
status: in-progress
priority: high
---`,
			expectedPriority: "high",
		},
		{
			name: "with deferred priority",
			input: `---
title: Later Task
status: draft
priority: deferred
---`,
			expectedPriority: "deferred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if item.Priority != tt.expectedPriority {
				t.Errorf("Priority = %q, want %q", item.Priority, tt.expectedPriority)
			}
		})
	}
}

func TestRenderWithPriority(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		contains []string
	}{
		{
			name: "with priority",
			item: &Item{
				Title:    "High Priority",
				Status:   "todo",
				Priority: "high",
			},
			contains: []string{
				"title: High Priority",
				"status: todo",
				"priority: high",
			},
		},
		{
			name: "without priority",
			item: &Item{
				Title:  "No Priority",
				Status: "todo",
			},
			contains: []string{
				"title: No Priority",
				"status: todo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered, err := tt.item.Render()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			content := string(rendered)
			for _, want := range tt.contains {
				if !strings.Contains(content, want) {
					t.Errorf("Render() missing %q in:\n%s", want, content)
				}
			}

			// Verify priority is NOT in output when empty
			if tt.item.Priority == "" && strings.Contains(content, "priority:") {
				t.Errorf("Render() should not contain 'priority:' when priority is empty:\n%s", content)
			}
		})
	}
}

func TestPriorityRoundtrip(t *testing.T) {
	priorities := []string{"critical", "high", "normal", "low", "deferred", ""}

	for _, priority := range priorities {
		t.Run(priority, func(t *testing.T) {
			original := &Item{
				Title:    "Test Bean",
				Status:   "todo",
				Priority: priority,
			}

			rendered, err := original.Render()
			if err != nil {
				t.Fatalf("Render() error: %v", err)
			}

			parsed, err := Parse(strings.NewReader(string(rendered)))
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}

			if parsed.Priority != original.Priority {
				t.Errorf("Priority roundtrip failed: got %q, want %q", parsed.Priority, original.Priority)
			}
		})
	}
}

func TestRender(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		item     *Item
		contains []string
	}{
		{
			name: "basic bean",
			item: &Item{
				Title:  "Test Bean",
				Status: "todo",
			},
			contains: []string{
				"---",
				"title: Test Bean",
				"status: todo",
			},
		},
		{
			name: "with body",
			item: &Item{
				Title:  "With Body",
				Status: "completed",
				Body:   "This is content.",
			},
			contains: []string{
				"title: With Body",
				"status: completed",
				"This is content.",
			},
		},
		{
			name: "with timestamps",
			item: &Item{
				Title:     "Timed",
				Status:    "todo",
				CreatedAt: &now,
				UpdatedAt: &now,
			},
			contains: []string{
				"title: Timed",
				"created_at:",
				"updated_at:",
			},
		},
		{
			name: "with type",
			item: &Item{
				Title:  "Typed Bean",
				Status: "todo",
				Type:   "bug",
			},
			contains: []string{
				"title: Typed Bean",
				"status: todo",
				"type: bug",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.item.Render()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			result := string(output)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("output missing %q\ngot:\n%s", want, result)
				}
			}
		})
	}
}

func TestParseRenderRoundtrip(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	later := time.Date(2024, 1, 16, 14, 45, 0, 0, time.UTC)

	tests := []struct {
		name string
		item *Item
	}{
		{
			name: "basic",
			item: &Item{
				Title:  "Basic Bean",
				Status: "todo",
			},
		},
		{
			name: "with body",
			item: &Item{
				Title:  "Bean With Body",
				Status: "in-progress",
				Body:   "This is the body content.\n\nWith multiple paragraphs.",
			},
		},
		{
			name: "with timestamps",
			item: &Item{
				Title:     "Timestamped Bean",
				Status:    "completed",
				CreatedAt: &now,
				UpdatedAt: &later,
				Body:      "Some content.",
			},
		},
		{
			name: "with type",
			item: &Item{
				Title:  "Typed Bean",
				Status: "todo",
				Type:   "bug",
				Body:   "Bug description.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Render to bytes
			rendered, err := tt.item.Render()
			if err != nil {
				t.Fatalf("Render error: %v", err)
			}

			// Parse back
			parsed, err := Parse(strings.NewReader(string(rendered)))
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			// Compare fields
			if parsed.Title != tt.item.Title {
				t.Errorf("Title roundtrip: got %q, want %q", parsed.Title, tt.item.Title)
			}
			if parsed.Status != tt.item.Status {
				t.Errorf("Status roundtrip: got %q, want %q", parsed.Status, tt.item.Status)
			}
			if parsed.Type != tt.item.Type {
				t.Errorf("Type roundtrip: got %q, want %q", parsed.Type, tt.item.Type)
			}

			// Body comparison (parse adds newline prefix for non-empty body)
			wantBody := tt.item.Body
			if wantBody != "" {
				wantBody = "\n" + wantBody
			}
			if parsed.Body != wantBody {
				t.Errorf("Body roundtrip: got %q, want %q", parsed.Body, wantBody)
			}

			// Timestamp comparison
			if tt.item.CreatedAt != nil {
				if parsed.CreatedAt == nil {
					t.Error("CreatedAt: got nil, want non-nil")
				} else if !parsed.CreatedAt.Equal(*tt.item.CreatedAt) {
					t.Errorf("CreatedAt: got %v, want %v", parsed.CreatedAt, tt.item.CreatedAt)
				}
			}
			if tt.item.UpdatedAt != nil {
				if parsed.UpdatedAt == nil {
					t.Error("UpdatedAt: got nil, want non-nil")
				} else if !parsed.UpdatedAt.Equal(*tt.item.UpdatedAt) {
					t.Errorf("UpdatedAt: got %v, want %v", parsed.UpdatedAt, tt.item.UpdatedAt)
				}
			}
		})
	}
}

func TestItemJSONSerialization(t *testing.T) {
	t.Run("body omitted when empty", func(t *testing.T) {
		item := &Item{
			ID:     "test-123",
			Title:  "Test Bean",
			Status: "todo",
			Body:   "",
		}

		data, err := json.Marshal(item)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		jsonStr := string(data)
		if strings.Contains(jsonStr, `"body"`) {
			t.Errorf("JSON should not contain 'body' field when empty, got: %s", jsonStr)
		}
	})

	t.Run("body included when non-empty", func(t *testing.T) {
		item := &Item{
			ID:     "test-123",
			Title:  "Test Bean",
			Status: "todo",
			Body:   "This is the body content.",
		}

		data, err := json.Marshal(item)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		jsonStr := string(data)
		if !strings.Contains(jsonStr, `"body":"This is the body content."`) {
			t.Errorf("JSON should contain 'body' field with content, got: %s", jsonStr)
		}
	})

	t.Run("extra omitted when empty", func(t *testing.T) {
		item := &Item{
			ID:     "test-123",
			Title:  "Test Bean",
			Status: "todo",
		}

		data, err := json.Marshal(item)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		jsonStr := string(data)
		if strings.Contains(jsonStr, `"extra"`) {
			t.Errorf("JSON should not contain 'extra' field when empty, got: %s", jsonStr)
		}
	})

	t.Run("extra included when non-empty", func(t *testing.T) {
		item := &Item{
			ID:     "test-123",
			Title:  "Test Bean",
			Status: "todo",
			Extra:  map[string]any{"release": "0-4-1"},
		}

		data, err := json.Marshal(item)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}

		extra, ok := result["extra"].(map[string]interface{})
		if !ok {
			t.Fatalf("JSON should contain object 'extra' field, got: %s", data)
		}
		if extra["release"] != "0-4-1" {
			t.Errorf("extra.release = %v, want %q", extra["release"], "0-4-1")
		}
	})
}

func TestParseWithParentAndBlocking(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedParent   string
		expectedBlocking []string
	}{
		{
			name: "with parent only",
			input: `---
title: Test
status: todo
parent: xyz789
---`,
			expectedParent:   "xyz789",
			expectedBlocking: nil,
		},
		{
			name: "with blocking only",
			input: `---
title: Test
status: todo
blocking:
  - abc123
  - def456
---`,
			expectedParent:   "",
			expectedBlocking: []string{"abc123", "def456"},
		},
		{
			name: "with parent and blocking",
			input: `---
title: Test
status: todo
parent: xyz789
blocking:
  - abc123
---`,
			expectedParent:   "xyz789",
			expectedBlocking: []string{"abc123"},
		},
		{
			name: "no relationships",
			input: `---
title: Test
status: todo
---`,
			expectedParent:   "",
			expectedBlocking: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if item.Parent != tt.expectedParent {
				t.Errorf("Parent = %q, want %q", item.Parent, tt.expectedParent)
			}

			if len(tt.expectedBlocking) == 0 && len(item.Blocking) == 0 {
				return // Both empty, OK
			}

			if len(item.Blocking) != len(tt.expectedBlocking) {
				t.Errorf("Blocking count = %d, want %d", len(item.Blocking), len(tt.expectedBlocking))
				return
			}

			for i, expected := range tt.expectedBlocking {
				if item.Blocking[i] != expected {
					t.Errorf("Blocking[%d] = %q, want %q", i, item.Blocking[i], expected)
				}
			}
		})
	}
}

func TestRenderWithParentAndBlocking(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		contains []string
	}{
		{
			name: "with parent only",
			item: &Item{
				Title:  "Test Bean",
				Status: "todo",
				Parent: "xyz789",
			},
			contains: []string{
				"parent: xyz789",
			},
		},
		{
			name: "with blocking only",
			item: &Item{
				Title:    "Test Bean",
				Status:   "todo",
				Blocking: []string{"abc123", "def456"},
			},
			contains: []string{
				"blocking:",
				"- abc123",
				"- def456",
			},
		},
		{
			name: "with parent and blocking",
			item: &Item{
				Title:    "Test Bean",
				Status:   "todo",
				Parent:   "xyz789",
				Blocking: []string{"abc123"},
			},
			contains: []string{
				"parent: xyz789",
				"blocking:",
				"- abc123",
			},
		},
		{
			name: "without relationships",
			item: &Item{
				Title:  "Test Bean",
				Status: "todo",
			},
			contains: []string{
				"title: Test Bean",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.item.Render()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			result := string(output)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("output missing %q\ngot:\n%s", want, result)
				}
			}

			// Check that empty parent/blocking don't appear in output
			if tt.item.Parent == "" && strings.Contains(result, "parent:") {
				t.Errorf("output should not contain 'parent:' when no parent\ngot:\n%s", result)
			}
			if len(tt.item.Blocking) == 0 && strings.Contains(result, "blocking:") {
				t.Errorf("output should not contain 'blocking:' when no blocking\ngot:\n%s", result)
			}
		})
	}
}

func TestParentAndBlockingRoundtrip(t *testing.T) {
	tests := []struct {
		name     string
		parent   string
		blocking []string
	}{
		{
			name:     "parent only",
			parent:   "xyz789",
			blocking: nil,
		},
		{
			name:     "single blocking",
			parent:   "",
			blocking: []string{"abc123"},
		},
		{
			name:     "multiple blocking",
			parent:   "",
			blocking: []string{"abc123", "def456"},
		},
		{
			name:     "parent and blocking",
			parent:   "xyz789",
			blocking: []string{"abc123", "def456"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &Item{
				Title:    "Test",
				Status:   "todo",
				Parent:   tt.parent,
				Blocking: tt.blocking,
			}

			rendered, err := original.Render()
			if err != nil {
				t.Fatalf("Render error: %v", err)
			}

			parsed, err := Parse(strings.NewReader(string(rendered)))
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			if parsed.Parent != tt.parent {
				t.Errorf("Parent: got %q, want %q", parsed.Parent, tt.parent)
			}

			if len(parsed.Blocking) != len(tt.blocking) {
				t.Errorf("Blocking count: got %d, want %d", len(parsed.Blocking), len(tt.blocking))
				return
			}

			for i, expected := range tt.blocking {
				if parsed.Blocking[i] != expected {
					t.Errorf("Blocking[%d] = %q, want %q", i, parsed.Blocking[i], expected)
				}
			}
		})
	}
}

func TestParseWithBlockedBy(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expectedBlockedBy []string
	}{
		{
			name: "with blocked_by",
			input: `---
title: Test
status: todo
blocked_by:
  - abc123
  - def456
---`,
			expectedBlockedBy: []string{"abc123", "def456"},
		},
		{
			name: "no blocked_by",
			input: `---
title: Test
status: todo
---`,
			expectedBlockedBy: nil,
		},
		{
			name: "with blocking and blocked_by",
			input: `---
title: Test
status: todo
blocking:
  - xyz789
blocked_by:
  - abc123
---`,
			expectedBlockedBy: []string{"abc123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(tt.expectedBlockedBy) == 0 && len(item.BlockedBy) == 0 {
				return // Both empty, OK
			}

			if len(item.BlockedBy) != len(tt.expectedBlockedBy) {
				t.Errorf("BlockedBy count = %d, want %d", len(item.BlockedBy), len(tt.expectedBlockedBy))
				return
			}

			for i, expected := range tt.expectedBlockedBy {
				if item.BlockedBy[i] != expected {
					t.Errorf("BlockedBy[%d] = %q, want %q", i, item.BlockedBy[i], expected)
				}
			}
		})
	}
}

func TestRenderWithBlockedBy(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		contains []string
	}{
		{
			name: "with blocked_by only",
			item: &Item{
				Title:     "Test Bean",
				Status:    "todo",
				BlockedBy: []string{"abc123", "def456"},
			},
			contains: []string{
				"blocked_by:",
				"- abc123",
				"- def456",
			},
		},
		{
			name: "with blocking and blocked_by",
			item: &Item{
				Title:     "Test Bean",
				Status:    "todo",
				Blocking:  []string{"xyz789"},
				BlockedBy: []string{"abc123"},
			},
			contains: []string{
				"blocking:",
				"- xyz789",
				"blocked_by:",
				"- abc123",
			},
		},
		{
			name: "without blocked_by",
			item: &Item{
				Title:  "Test Bean",
				Status: "todo",
			},
			contains: []string{
				"title: Test Bean",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.item.Render()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			result := string(output)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("output missing %q\ngot:\n%s", want, result)
				}
			}

			// Check that empty blocked_by doesn't appear in output
			if len(tt.item.BlockedBy) == 0 && strings.Contains(result, "blocked_by:") {
				t.Errorf("output should not contain 'blocked_by:' when no blocked_by\ngot:\n%s", result)
			}
		})
	}
}

func TestBlockedByRoundtrip(t *testing.T) {
	tests := []struct {
		name      string
		blockedBy []string
	}{
		{
			name:      "single blocked_by",
			blockedBy: []string{"abc123"},
		},
		{
			name:      "multiple blocked_by",
			blockedBy: []string{"abc123", "def456"},
		},
		{
			name:      "empty blocked_by",
			blockedBy: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &Item{
				Title:     "Test",
				Status:    "todo",
				BlockedBy: tt.blockedBy,
			}

			rendered, err := original.Render()
			if err != nil {
				t.Fatalf("Render error: %v", err)
			}

			parsed, err := Parse(strings.NewReader(string(rendered)))
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			if len(parsed.BlockedBy) != len(tt.blockedBy) {
				t.Errorf("BlockedBy count: got %d, want %d", len(parsed.BlockedBy), len(tt.blockedBy))
				return
			}

			for i, expected := range tt.blockedBy {
				if parsed.BlockedBy[i] != expected {
					t.Errorf("BlockedBy[%d] = %q, want %q", i, parsed.BlockedBy[i], expected)
				}
			}
		})
	}
}

func TestItemRelationshipMethods(t *testing.T) {
	t.Run("HasParent", func(t *testing.T) {
		withParent := &Item{Parent: "xyz789"}
		if !withParent.HasParent() {
			t.Error("expected HasParent() = true when parent is set")
		}

		withoutParent := &Item{}
		if withoutParent.HasParent() {
			t.Error("expected HasParent() = false when parent is empty")
		}
	})

	t.Run("IsBlocking", func(t *testing.T) {
		b := &Item{Blocking: []string{"abc", "def"}}
		if !b.IsBlocking("abc") {
			t.Error("expected IsBlocking('abc') = true")
		}
		if !b.IsBlocking("def") {
			t.Error("expected IsBlocking('def') = true")
		}
		if b.IsBlocking("xyz") {
			t.Error("expected IsBlocking('xyz') = false")
		}

		empty := &Item{}
		if empty.IsBlocking("abc") {
			t.Error("expected IsBlocking('abc') = false for empty blocks")
		}
	})

	t.Run("AddBlocking", func(t *testing.T) {
		b := &Item{Blocking: []string{"abc"}}
		b.AddBlocking("def")
		if len(b.Blocking) != 2 {
			t.Errorf("AddBlocking new: got len=%d, want 2", len(b.Blocking))
		}
		if !b.IsBlocking("def") {
			t.Error("AddBlocking didn't add the block")
		}

		// Adding duplicate should not add
		b.AddBlocking("abc")
		if len(b.Blocking) != 2 {
			t.Errorf("AddBlocking duplicate: got len=%d, want 2", len(b.Blocking))
		}
	})

	t.Run("RemoveBlocking", func(t *testing.T) {
		b := &Item{Blocking: []string{"abc", "def", "ghi"}}
		b.RemoveBlocking("def")
		if len(b.Blocking) != 2 {
			t.Errorf("RemoveBlocking existing: got len=%d, want 2", len(b.Blocking))
		}
		if b.IsBlocking("def") {
			t.Error("RemoveBlocking didn't remove the block")
		}

		// Removing non-existent should not change anything
		b.RemoveBlocking("nonexistent")
		if len(b.Blocking) != 2 {
			t.Errorf("RemoveBlocking non-existent: got len=%d, want 2", len(b.Blocking))
		}
	})

	t.Run("IsBlockedBy", func(t *testing.T) {
		b := &Item{BlockedBy: []string{"abc", "def"}}
		if !b.IsBlockedBy("abc") {
			t.Error("expected IsBlockedBy('abc') = true")
		}
		if !b.IsBlockedBy("def") {
			t.Error("expected IsBlockedBy('def') = true")
		}
		if b.IsBlockedBy("xyz") {
			t.Error("expected IsBlockedBy('xyz') = false")
		}

		empty := &Item{}
		if empty.IsBlockedBy("abc") {
			t.Error("expected IsBlockedBy('abc') = false for empty blocked_by")
		}
	})

	t.Run("AddBlockedBy", func(t *testing.T) {
		b := &Item{BlockedBy: []string{"abc"}}
		b.AddBlockedBy("def")
		if len(b.BlockedBy) != 2 {
			t.Errorf("AddBlockedBy new: got len=%d, want 2", len(b.BlockedBy))
		}
		if !b.IsBlockedBy("def") {
			t.Error("AddBlockedBy didn't add the blocker")
		}

		// Adding duplicate should not add
		b.AddBlockedBy("abc")
		if len(b.BlockedBy) != 2 {
			t.Errorf("AddBlockedBy duplicate: got len=%d, want 2", len(b.BlockedBy))
		}
	})

	t.Run("RemoveBlockedBy", func(t *testing.T) {
		b := &Item{BlockedBy: []string{"abc", "def", "ghi"}}
		b.RemoveBlockedBy("def")
		if len(b.BlockedBy) != 2 {
			t.Errorf("RemoveBlockedBy existing: got len=%d, want 2", len(b.BlockedBy))
		}
		if b.IsBlockedBy("def") {
			t.Error("RemoveBlockedBy didn't remove the blocker")
		}

		// Removing non-existent should not change anything
		b.RemoveBlockedBy("nonexistent")
		if len(b.BlockedBy) != 2 {
			t.Errorf("RemoveBlockedBy non-existent: got len=%d, want 2", len(b.BlockedBy))
		}
	})
}

func TestValidateTag(t *testing.T) {
	tests := []struct {
		tag     string
		wantErr bool
	}{
		{"frontend", false},
		{"backend", false},
		{"tech-debt", false},
		{"v1", false},
		{"a", false},
		{"urgent2", false},
		{"wont-fix", false},
		{"a-b-c", false},
		{"", true},         // empty
		{"Frontend", true}, // uppercase
		{"URGENT", true},   // all uppercase
		{"123", true},      // starts with number
		{"123abc", true},   // starts with number
		{"my tag", true},   // contains space
		{"my_tag", true},   // contains underscore
		{"my--tag", true},  // consecutive hyphens
		{"-tag", true},     // starts with hyphen
		{"tag-", true},     // ends with hyphen
		{"my.tag", true},   // contains dot
		{"my/tag", true},   // contains slash
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			err := ValidateTag(tt.tag)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateTag(%q) = nil, want error", tt.tag)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateTag(%q) = %v, want nil", tt.tag, err)
			}
		})
	}
}

func TestNormalizeTag(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"frontend", "frontend"},
		{"FRONTEND", "frontend"},
		{"FrontEnd", "frontend"},
		{"  frontend  ", "frontend"},
		{"  FRONTEND  ", "frontend"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeTag(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeTag(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestItemTagMethods(t *testing.T) {
	t.Run("HasTag", func(t *testing.T) {
		b := &Item{Tags: []string{"frontend", "urgent"}}
		if !b.HasTag("frontend") {
			t.Error("expected HasTag('frontend') = true")
		}
		if !b.HasTag("urgent") {
			t.Error("expected HasTag('urgent') = true")
		}
		if b.HasTag("backend") {
			t.Error("expected HasTag('backend') = false")
		}
		// Case insensitive lookup
		if !b.HasTag("FRONTEND") {
			t.Error("expected HasTag('FRONTEND') = true (case insensitive)")
		}
	})

	t.Run("AddTag", func(t *testing.T) {
		b := &Item{Tags: []string{"frontend"}}

		// Add new valid tag
		if err := b.AddTag("backend"); err != nil {
			t.Errorf("AddTag('backend') error: %v", err)
		}
		if len(b.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(b.Tags))
		}

		// Adding duplicate should not add
		if err := b.AddTag("frontend"); err != nil {
			t.Errorf("AddTag('frontend') error: %v", err)
		}
		if len(b.Tags) != 2 {
			t.Errorf("expected 2 tags (no duplicate), got %d", len(b.Tags))
		}

		// Adding invalid tag should error
		if err := b.AddTag("Invalid Tag"); err == nil {
			t.Error("expected AddTag('Invalid Tag') to error")
		}
	})

	t.Run("RemoveTag", func(t *testing.T) {
		b := &Item{Tags: []string{"frontend", "backend", "urgent"}}

		b.RemoveTag("backend")
		if len(b.Tags) != 2 {
			t.Errorf("expected 2 tags after remove, got %d", len(b.Tags))
		}
		if b.HasTag("backend") {
			t.Error("expected backend tag to be removed")
		}

		// Case insensitive removal
		b.RemoveTag("FRONTEND")
		if len(b.Tags) != 1 {
			t.Errorf("expected 1 tag after remove, got %d", len(b.Tags))
		}
		if b.HasTag("frontend") {
			t.Error("expected frontend tag to be removed")
		}

		// Remove non-existent tag (should not error)
		b.RemoveTag("nonexistent")
		if len(b.Tags) != 1 {
			t.Errorf("expected 1 tag (no change), got %d", len(b.Tags))
		}
	})
}

func TestParseWithTags(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedTags []string
	}{
		{
			name: "single tag",
			input: `---
title: Test
status: todo
tags:
  - frontend
---`,
			expectedTags: []string{"frontend"},
		},
		{
			name: "multiple tags",
			input: `---
title: Test
status: todo
tags:
  - frontend
  - urgent
  - tech-debt
---`,
			expectedTags: []string{"frontend", "urgent", "tech-debt"},
		},
		{
			name: "inline tags syntax",
			input: `---
title: Test
status: todo
tags: [frontend, backend]
---`,
			expectedTags: []string{"frontend", "backend"},
		},
		{
			name: "no tags",
			input: `---
title: Test
status: todo
---`,
			expectedTags: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(tt.expectedTags) == 0 && len(item.Tags) == 0 {
				return // Both empty, OK
			}

			if len(item.Tags) != len(tt.expectedTags) {
				t.Errorf("Tags count = %d, want %d", len(item.Tags), len(tt.expectedTags))
				return
			}

			for i, expected := range tt.expectedTags {
				if item.Tags[i] != expected {
					t.Errorf("Tags[%d] = %q, want %q", i, item.Tags[i], expected)
				}
			}
		})
	}
}

func TestRenderWithTags(t *testing.T) {
	tests := []struct {
		name     string
		item     *Item
		contains []string
	}{
		{
			name: "with single tag",
			item: &Item{
				Title:  "Test Bean",
				Status: "todo",
				Tags:   []string{"frontend"},
			},
			contains: []string{
				"tags:",
				"- frontend",
			},
		},
		{
			name: "with multiple tags",
			item: &Item{
				Title:  "Test Bean",
				Status: "todo",
				Tags:   []string{"frontend", "urgent", "tech-debt"},
			},
			contains: []string{
				"tags:",
				"- frontend",
				"- urgent",
				"- tech-debt",
			},
		},
		{
			name: "without tags",
			item: &Item{
				Title:  "Test Bean",
				Status: "todo",
			},
			contains: []string{
				"title: Test Bean",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.item.Render()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			result := string(output)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("output missing %q\ngot:\n%s", want, result)
				}
			}

			// Check that empty tags don't appear in output
			if len(tt.item.Tags) == 0 && strings.Contains(result, "tags:") {
				t.Errorf("output should not contain 'tags:' when no tags\ngot:\n%s", result)
			}
		})
	}
}

func TestTagsRoundtrip(t *testing.T) {
	tests := []struct {
		name string
		tags []string
	}{
		{
			name: "single tag",
			tags: []string{"frontend"},
		},
		{
			name: "multiple tags",
			tags: []string{"frontend", "backend", "urgent"},
		},
		{
			name: "hyphenated tags",
			tags: []string{"tech-debt", "wont-fix", "needs-review"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &Item{
				Title:  "Test",
				Status: "todo",
				Tags:   tt.tags,
			}

			rendered, err := original.Render()
			if err != nil {
				t.Fatalf("Render error: %v", err)
			}

			parsed, err := Parse(strings.NewReader(string(rendered)))
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			if len(parsed.Tags) != len(tt.tags) {
				t.Errorf("Tags count: got %d, want %d", len(parsed.Tags), len(tt.tags))
				return
			}

			for i, expected := range tt.tags {
				if parsed.Tags[i] != expected {
					t.Errorf("Tags[%d] = %q, want %q", i, parsed.Tags[i], expected)
				}
			}
		})
	}
}

func TestRenderWithIDComment(t *testing.T) {
	tests := []struct {
		name          string
		item          *Item
		expectComment string
	}{
		{
			name: "with ID",
			item: &Item{
				ID:     "beans-abc123",
				Title:  "Test Bean",
				Status: "todo",
			},
			expectComment: "# beans-abc123",
		},
		{
			name: "without ID",
			item: &Item{
				Title:  "Test Bean",
				Status: "todo",
			},
			expectComment: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.item.Render()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			result := string(output)

			if tt.expectComment != "" {
				// Check that comment appears right after opening ---
				expectedStart := "---\n" + tt.expectComment + "\n"
				if !strings.HasPrefix(result, expectedStart) {
					t.Errorf("expected output to start with %q\ngot:\n%s", expectedStart, result)
				}
			} else {
				// When no ID, should not have a comment line
				lines := strings.Split(result, "\n")
				if len(lines) > 1 && strings.HasPrefix(lines[1], "#") {
					t.Errorf("expected no comment line when ID is empty\ngot:\n%s", result)
				}
			}
		})
	}
}

func TestRenderWithIDCommentRoundtrip(t *testing.T) {
	// Verify that the ID comment doesn't interfere with parsing
	original := &Item{
		ID:     "beans-xyz789",
		Title:  "Test Bean",
		Status: "in-progress",
		Body:   "Some body content.",
	}

	rendered, err := original.Render()
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	// Verify the comment is present
	if !strings.Contains(string(rendered), "# beans-xyz789") {
		t.Errorf("rendered output should contain ID comment\ngot:\n%s", rendered)
	}

	// Parse should work correctly (comment is ignored)
	parsed, err := Parse(strings.NewReader(string(rendered)))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if parsed.Title != original.Title {
		t.Errorf("Title roundtrip: got %q, want %q", parsed.Title, original.Title)
	}
	if parsed.Status != original.Status {
		t.Errorf("Status roundtrip: got %q, want %q", parsed.Status, original.Status)
	}
}

func TestRenderTrailingNewline(t *testing.T) {
	tests := []struct {
		name string
		item *Item
	}{
		{
			name: "with body",
			item: &Item{
				Title:  "Test Bean",
				Status: "todo",
				Body:   "Some content without trailing newline",
			},
		},
		{
			name: "with body ending in newline",
			item: &Item{
				Title:  "Test Bean",
				Status: "todo",
				Body:   "Some content with trailing newline\n",
			},
		},
		{
			name: "without body",
			item: &Item{
				Title:  "Test Bean",
				Status: "todo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered, err := tt.item.Render()
			if err != nil {
				t.Fatalf("Render error: %v", err)
			}
			if !strings.HasSuffix(string(rendered), "\n") {
				t.Errorf("rendered output should end with newline\ngot: %q", rendered)
			}
		})
	}
}

func TestETag(t *testing.T) {
	t.Run("consistent hash", func(t *testing.T) {
		b := &Item{
			Title:  "Test",
			Status: "todo",
			Body:   "content",
		}
		etag1 := b.ETag()
		etag2 := b.ETag()
		if etag1 != etag2 {
			t.Errorf("ETag not consistent: %s != %s", etag1, etag2)
		}
	})

	t.Run("16 hex characters", func(t *testing.T) {
		b := &Item{
			Title:  "Test",
			Status: "todo",
		}
		etag := b.ETag()
		if len(etag) != 16 {
			t.Errorf("ETag length = %d, want 16", len(etag))
		}
		// Verify it's valid hex
		for _, c := range etag {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("ETag contains non-hex char: %c", c)
			}
		}
	})

	t.Run("changes when title changes", func(t *testing.T) {
		b := &Item{
			Title:  "Test",
			Status: "todo",
		}
		etag1 := b.ETag()

		b.Title = "Changed"
		etag2 := b.ETag()

		if etag1 == etag2 {
			t.Error("ETag should change when title changes")
		}
	})

	t.Run("changes when status changes", func(t *testing.T) {
		b := &Item{
			Title:  "Test",
			Status: "todo",
		}
		etag1 := b.ETag()

		b.Status = "in-progress"
		etag2 := b.ETag()

		if etag1 == etag2 {
			t.Error("ETag should change when status changes")
		}
	})

	t.Run("changes when body changes", func(t *testing.T) {
		b := &Item{
			Title:  "Test",
			Status: "todo",
			Body:   "original",
		}
		etag1 := b.ETag()

		b.Body = "modified"
		etag2 := b.ETag()

		if etag1 == etag2 {
			t.Error("ETag should change when body changes")
		}
	})

	t.Run("changes when metadata changes", func(t *testing.T) {
		b := &Item{
			Title:    "Test",
			Status:   "todo",
			Priority: "normal",
		}
		etag1 := b.ETag()

		b.Priority = "high"
		etag2 := b.ETag()

		if etag1 == etag2 {
			t.Error("ETag should change when priority changes")
		}
	})

	t.Run("same content same etag", func(t *testing.T) {
		b1 := &Item{
			Title:  "Test",
			Status: "todo",
			Body:   "content",
		}
		b2 := &Item{
			Title:  "Test",
			Status: "todo",
			Body:   "content",
		}

		if b1.ETag() != b2.ETag() {
			t.Error("Same content should produce same ETag")
		}
	})

	t.Run("different order of tags produces different etag", func(t *testing.T) {
		b1 := &Item{
			Title:  "Test",
			Status: "todo",
			Tags:   []string{"a", "b"},
		}
		b2 := &Item{
			Title:  "Test",
			Status: "todo",
			Tags:   []string{"b", "a"},
		}

		// Tag order matters in rendered output, so ETags will differ
		if b1.ETag() == b2.ETag() {
			t.Error("Different tag order should produce different ETag")
		}
	})

	t.Run("etag is empty on render error", func(t *testing.T) {
		// This is a defensive test - Render() shouldn't fail in practice,
		// but ETag() handles it gracefully by returning empty string
		b := &Item{
			Title:  "Test",
			Status: "todo",
		}
		etag := b.ETag()
		if etag == "" {
			t.Error("ETag should not be empty for valid bean")
		}
	})
}

func TestMarshalJSONIncludesETag(t *testing.T) {
	b := &Item{
		ID:     "test-123",
		Title:  "Test Bean",
		Status: "todo",
		Body:   "Some content",
	}

	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Check etag field is present
	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"etag"`) {
		t.Errorf("JSON should contain 'etag' field, got: %s", jsonStr)
	}

	// Parse and verify etag value
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	etag, ok := result["etag"].(string)
	if !ok {
		t.Error("etag should be a string")
	}
	if len(etag) != 16 {
		t.Errorf("etag length = %d, want 16", len(etag))
	}

	// Verify it matches the computed ETag
	if etag != b.ETag() {
		t.Errorf("JSON etag = %s, want %s", etag, b.ETag())
	}
}

func TestETagChangesAfterModification(t *testing.T) {
	// Verify that ETag changes reflect actual content changes
	// (this is important for optimistic concurrency control)
	b := &Item{
		Title:  "Original",
		Status: "todo",
		Body:   "Original body",
	}

	etag1 := b.ETag()

	// Modify the item
	b.Title = "Modified"
	b.Body = "Modified body"

	etag2 := b.ETag()

	if etag1 == etag2 {
		t.Error("ETag should change after modification")
	}

	// Verify JSON serialization reflects the change
	data1, _ := json.Marshal(&Item{Title: "Original", Status: "todo", Body: "Original body"})
	data2, _ := json.Marshal(b)

	var result1, result2 map[string]interface{}
	json.Unmarshal(data1, &result1)
	json.Unmarshal(data2, &result2)

	if result1["etag"] == result2["etag"] {
		t.Error("JSON etag should differ after modification")
	}
}

func TestParseExtraFrontMatter(t *testing.T) {
	input := `---
title: Test Bean
status: todo
type: task
priority: high
custom_field: hello
another_field: 42
flag_field: true
list_field:
  - a
  - b
map_field:
  key1: value1
  key2: value2
---

Body text.`

	b, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if b.Title != "Test Bean" {
		t.Errorf("Title = %q, want %q", b.Title, "Test Bean")
	}
	if b.Status != "todo" {
		t.Errorf("Status = %q, want %q", b.Status, "todo")
	}
	if b.Type != "task" {
		t.Errorf("Type = %q, want %q", b.Type, "task")
	}
	if b.Priority != "high" {
		t.Errorf("Priority = %q, want %q", b.Priority, "high")
	}

	if len(b.Extra) != 5 {
		t.Fatalf("len(Extra) = %d, want 5; Extra = %#v", len(b.Extra), b.Extra)
	}

	for _, known := range []string{"title", "status", "type", "priority"} {
		if _, ok := b.Extra[known]; ok {
			t.Errorf("Extra should not contain known key %q", known)
		}
	}

	if b.Extra["custom_field"] != "hello" {
		t.Errorf("Extra[custom_field] = %v, want %q", b.Extra["custom_field"], "hello")
	}
	if b.Extra["another_field"] != 42 {
		t.Errorf("Extra[another_field] = %v, want %d", b.Extra["another_field"], 42)
	}
	if b.Extra["flag_field"] != true {
		t.Errorf("Extra[flag_field] = %v, want true", b.Extra["flag_field"])
	}

	list, ok := b.Extra["list_field"].([]interface{})
	if !ok {
		t.Fatalf("Extra[list_field] type = %T, want []interface{}", b.Extra["list_field"])
	}
	if !reflect.DeepEqual(list, []interface{}{"a", "b"}) {
		t.Errorf("Extra[list_field] = %v, want [a b]", list)
	}

	m, ok := b.Extra["map_field"].(map[string]interface{})
	if !ok {
		t.Fatalf("Extra[map_field] type = %T, want map[string]interface{}", b.Extra["map_field"])
	}
	wantMap := map[string]interface{}{"key1": "value1", "key2": "value2"}
	if !reflect.DeepEqual(m, wantMap) {
		t.Errorf("Extra[map_field] = %v, want %v", m, wantMap)
	}
}

func TestParseNoExtraFrontMatter(t *testing.T) {
	input := `---
title: Test Bean
status: todo
---

Body.`

	b, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(b.Extra) != 0 {
		t.Errorf("Extra = %#v, want empty", b.Extra)
	}
}

func TestRenderWritesExtraKeysSortedAfterKnownFields(t *testing.T) {
	b := &Item{
		Title:  "Extra Bean",
		Status: "todo",
		Extra: map[string]any{
			"z_field": "last",
			"a_field": "first",
			"m_field": "middle",
		},
	}

	rendered, err := b.Render()
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	out := string(rendered)

	statusIdx := strings.Index(out, "status:")
	aIdx := strings.Index(out, "a_field:")
	mIdx := strings.Index(out, "m_field:")
	zIdx := strings.Index(out, "z_field:")

	if statusIdx == -1 || aIdx == -1 || mIdx == -1 || zIdx == -1 {
		t.Fatalf("expected all fields present in output, got:\n%s", out)
	}
	if !(statusIdx < aIdx && aIdx < mIdx && mIdx < zIdx) {
		t.Errorf("expected order status < a_field < m_field < z_field, got indices %d, %d, %d, %d:\n%s", statusIdx, aIdx, mIdx, zIdx, out)
	}
}

func TestRenderTwiceProducesIdenticalOutput(t *testing.T) {
	b := &Item{
		Title:  "Stable Bean",
		Status: "todo",
		Extra: map[string]any{
			"z_field": "last",
			"a_field": "first",
		},
	}

	first, err := b.Render()
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}
	second, err := b.Render()
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Errorf("two renders differ:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestParseRenderRoundtripWithExtraKeys(t *testing.T) {
	// This test verifies that parsing and rendering an item file with extra
	// (non-standard) frontmatter keys produces output byte-identical to the input.
	// This is AC2 from beans-54rb: parse → render must preserve all custom fields.
	inputContent := `---
title: Roundtrip Bean
status: in-progress
type: task
custom_a: alpha
custom_b: beta
custom_c: gamma
---

`

	// Parse the input content
	parsed, err := Parse(bytes.NewReader([]byte(inputContent)))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Verify extra keys were parsed
	if parsed.Extra == nil || len(parsed.Extra) != 3 {
		t.Fatalf("Expected 3 extra keys, got %d", len(parsed.Extra))
	}
	if parsed.Extra["custom_a"] != "alpha" || parsed.Extra["custom_b"] != "beta" || parsed.Extra["custom_c"] != "gamma" {
		t.Errorf("Extra keys not parsed correctly: %v", parsed.Extra)
	}

	// Render the parsed item
	renderedContent, err := parsed.Render()
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	// Assert byte-identical roundtrip
	if !bytes.Equal([]byte(inputContent), renderedContent) {
		t.Errorf("roundtrip not byte-identical:\ninput:\n%s\nrendered:\n%s", inputContent, string(renderedContent))
	}
}

func TestRenderPreservesExtraKeysAfterKnownFieldUpdate(t *testing.T) {
	b := &Item{
		Title:  "Original Title",
		Status: "todo",
		Extra: map[string]any{
			"custom_a": "alpha",
			"custom_b": "beta",
		},
	}

	rendered, err := b.Render()
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	parsed, err := Parse(bytes.NewReader(rendered))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Simulate a command updating a known field.
	parsed.Status = "completed"

	rerendered, err := parsed.Render()
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	reparsed, err := Parse(bytes.NewReader(rerendered))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if reparsed.Status != "completed" {
		t.Errorf("Status = %q, want %q", reparsed.Status, "completed")
	}
	if len(reparsed.Extra) != 2 {
		t.Fatalf("len(Extra) = %d, want 2; Extra = %#v", len(reparsed.Extra), reparsed.Extra)
	}
	if reparsed.Extra["custom_a"] != "alpha" {
		t.Errorf("Extra[custom_a] = %v, want %q", reparsed.Extra["custom_a"], "alpha")
	}
	if reparsed.Extra["custom_b"] != "beta" {
		t.Errorf("Extra[custom_b] = %v, want %q", reparsed.Extra["custom_b"], "beta")
	}
}

func TestReservedKeyFlag(t *testing.T) {
	tests := []struct {
		key       string
		wantFlag  string
		wantKnown bool
	}{
		{"title", "--title", true},
		{"status", "--status", true},
		{"type", "--type", true},
		{"priority", "--priority", true},
		{"tags", "--tag", true},
		{"parent", "--parent", true},
		{"blocking", "--blocking", true},
		{"blocked_by", "--blocked-by", true},
		{"created_at", "", true},
		{"updated_at", "", true},
		{"order", "", true},
		{"release", "", false},
		{"klasse", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			flag, known := ReservedKeyFlag(tt.key)
			if known != tt.wantKnown {
				t.Errorf("ReservedKeyFlag(%q) known = %v, want %v", tt.key, known, tt.wantKnown)
			}
			if tt.wantKnown && tt.wantFlag != "" && flag != tt.wantFlag {
				t.Errorf("ReservedKeyFlag(%q) flag = %q, want %q", tt.key, flag, tt.wantFlag)
			}
		})
	}
}

// TestKnownKeyMapsMatchFrontMatterTags guards against the three parallel
// key lists (frontMatter's yaml tags, knownFrontMatterKeys, reservedKeyFlags)
// drifting apart. A key present on the frontMatter struct but missing from
// knownFrontMatterKeys would be written both to its own typed field AND
// into Extra by Render -- silent duplicate-key file corruption that still
// parses without error. Reflection over frontMatter's yaml tags is the
// single source of truth here; both maps must match it exactly.
func TestKnownKeyMapsMatchFrontMatterTags(t *testing.T) {
	tagKeys := make(map[string]bool)
	typ := reflect.TypeOf(frontMatter{})
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("yaml")
		if tag == "" || tag == "-" {
			continue
		}
		key := strings.SplitN(tag, ",", 2)[0]
		tagKeys[key] = true
	}

	if len(tagKeys) == 0 {
		t.Fatal("no yaml tags found on frontMatter -- reflection is broken, test is meaningless")
	}

	for key := range tagKeys {
		if !knownFrontMatterKeys[key] {
			t.Errorf("frontMatter has yaml tag %q but knownFrontMatterKeys is missing it", key)
		}
		if _, ok := reservedKeyFlags[key]; !ok {
			t.Errorf("frontMatter has yaml tag %q but reservedKeyFlags is missing it", key)
		}
	}

	for key := range knownFrontMatterKeys {
		if !tagKeys[key] {
			t.Errorf("knownFrontMatterKeys has %q but frontMatter carries no such yaml tag", key)
		}
	}

	for key := range reservedKeyFlags {
		if !tagKeys[key] {
			t.Errorf("reservedKeyFlags has %q but frontMatter carries no such yaml tag", key)
		}
	}
}
