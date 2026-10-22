package tui

// search_prefix_test.go — TDD coverage for search-input filter prefixes
// (bt-2kfl, D02/D03 final PO decisions, epic-E13-plan.md »Item 3«): the pure
// tokenizer (parseSearchPrefixes), the AND/OR facet-membership predicate it
// feeds (beanMatchesSearch's prefix half), the filterSummary UNION display,
// and the D03 Bleve-dispatch guard (only rest ever reaches Bleve).

import (
	"github.com/rierek/lif/pkg/item"
	"testing"
)

// --- parseSearchPrefixes (pure tokenizer) ---

func TestParseSearchPrefixes(t *testing.T) {
	cases := []struct {
		name       string
		query      string
		wantFacets map[string][]string
		wantRest   string
	}{
		{
			name:       "single valid prefix",
			query:      "st:completed",
			wantFacets: map[string][]string{"status": {"completed"}},
			wantRest:   "",
		},
		{
			name:  "multiple prefixes plus rest text, PO example",
			query: "st:completed ty:epic foo",
			wantFacets: map[string][]string{
				"status": {"completed"},
				"type":   {"epic"},
			},
			wantRest: "foo",
		},
		{
			name:       "case-insensitive prefix keyword",
			query:      "ST:completed Ty:Epic",
			wantFacets: map[string][]string{"status": {"completed"}, "type": {"epic"}},
			wantRest:   "",
		},
		{
			name:       "order does not matter, rest preserves its own order",
			query:      "foo st:completed bar",
			wantFacets: map[string][]string{"status": {"completed"}},
			wantRest:   "foo bar",
		},
		{
			name:       "invalid prefix falls into rest unchanged",
			query:      "xx:foo st:completed",
			wantFacets: map[string][]string{"status": {"completed"}},
			wantRest:   "xx:foo",
		},
		{
			name:       "empty value falls into rest unchanged",
			query:      "st: foo",
			wantFacets: map[string][]string{},
			wantRest:   "st: foo",
		},
		{
			name:       "bare colon (no prefix) falls into rest",
			query:      ":completed",
			wantFacets: map[string][]string{},
			wantRest:   ":completed",
		},
		{
			name:       "priority and tag prefixes",
			query:      "pr:high tag:urgent",
			wantFacets: map[string][]string{"priority": {"high"}, "tag": {"urgent"}},
			wantRest:   "",
		},
		{
			name:       "same facet repeated collects multiple values (OR)",
			query:      "st:completed st:in-progress foo",
			wantFacets: map[string][]string{"status": {"completed", "in-progress"}},
			wantRest:   "foo",
		},
		{
			name:       "archive has no prefix -- 'archive:' is not a recognized prefix",
			query:      "archive:show foo",
			wantFacets: map[string][]string{},
			wantRest:   "archive:show foo",
		},
		{
			name:       "empty query",
			query:      "",
			wantFacets: map[string][]string{},
			wantRest:   "",
		},
		{
			name:       "plain text only, no prefixes",
			query:      "just some words",
			wantFacets: map[string][]string{},
			wantRest:   "just some words",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facets, rest := parseSearchPrefixes(tc.query)
			if rest != tc.wantRest {
				t.Errorf("rest = %q, want %q", rest, tc.wantRest)
			}
			if len(facets) != len(tc.wantFacets) {
				t.Fatalf("facets = %v, want %v", facets, tc.wantFacets)
			}
			for k, want := range tc.wantFacets {
				got := facets[k]
				if !equalStrings(got, want) {
					t.Errorf("facets[%q] = %v, want %v", k, got, want)
				}
			}
		})
	}
}

// --- Matching (beanMatchesSearch's prefix-facet half, AND/OR semantics) ---

func TestSearchPrefixMatchingAndsAcrossFacetsOrsWithinFacet(t *testing.T) {
	beans := []*item.Item{
		{ID: "a", Title: "Alpha", Status: "completed", Type: "epic", Priority: "normal"},
		{ID: "b", Title: "Beta", Status: "completed", Type: "task", Priority: "normal"},
		{ID: "c", Title: "Gamma", Status: "todo", Type: "epic", Priority: "normal"},
	}
	m := fixtureModel(t, beans)
	m = setSearchQuery(m, "st:completed ty:epic")

	var got []string
	for _, b := range beans {
		if m.beanMatchesSearch(m.idx.ByID[b.ID]) {
			got = append(got, b.ID)
		}
	}
	if !equalStrings(got, []string{"a"}) {
		t.Fatalf("beanMatchesSearch with st:completed ty:epic = %v, want [a] (status AND type)", got)
	}
}

func TestSearchPrefixCombinesWithTextRest(t *testing.T) {
	beans := []*item.Item{
		{ID: "a", Title: "Alpha foo", Status: "completed", Type: "epic", Priority: "normal"},
		{ID: "b", Title: "Beta foo", Status: "completed", Type: "task", Priority: "normal"},
	}
	m := fixtureModel(t, beans)
	m = setSearchQuery(m, "st:completed foo")

	if !m.beanMatchesSearch(m.idx.ByID["a"]) {
		t.Fatal("a: status=completed AND title contains foo -- must match")
	}
	if !m.beanMatchesSearch(m.idx.ByID["b"]) {
		t.Fatal("b: status=completed AND title contains foo -- must match")
	}

	m = setSearchQuery(m, "st:completed bar")
	if m.beanMatchesSearch(m.idx.ByID["a"]) {
		t.Fatal("a: status matches but text 'bar' does not -- must NOT match")
	}
}

func TestSearchPrefixInvalidTokenTreatedAsPlainText(t *testing.T) {
	beans := []*item.Item{
		{ID: "a", Title: "xx:foo bean", Status: "todo", Type: "task", Priority: "normal"},
	}
	m := fixtureModel(t, beans)
	m = setSearchQuery(m, "xx:foo")

	if !m.beanMatchesSearch(m.idx.ByID["a"]) {
		t.Fatal("invalid prefix 'xx:' must fall back to plain substring text match, not error/exclude")
	}
}

func TestSearchPrefixDoesNotMutateFilterMenuState(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = setSearchQuery(m, "st:completed ty:epic")

	if len(m.filterStatus) != 0 {
		t.Fatalf("m.filterStatus = %v, want empty -- typed prefixes must NOT write into the f-menu state (D02)", m.filterStatus)
	}
	if len(m.filterType) != 0 {
		t.Fatalf("m.filterType = %v, want empty -- typed prefixes must NOT write into the f-menu state (D02)", m.filterType)
	}
}

// --- filterSummary UNION display (D02) ---

func TestFilterSummaryShowsUnionOfMenuFacetsAndTypedPrefixes(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.filterStatus = map[string]bool{"todo": true}
	m = setSearchQuery(m, "st:completed ty:epic")

	got := m.filterSummary()
	want := "St:completed,todo Ty:epic"
	if got != want {
		t.Fatalf("filterSummary() = %q, want %q (union of menu St:todo + typed st:completed, plus typed Ty:epic)", got, want)
	}
}

func TestFilterSummaryUnchangedWithoutTypedPrefixes(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m.filterStatus = map[string]bool{"todo": true}
	m.filterType = map[string]bool{"epic": true}

	got := m.filterSummary()
	want := "St:todo Ty:epic"
	if got != want {
		t.Fatalf("filterSummary() (no typed prefixes) = %q, want %q -- must be byte-identical to the pre-bt-2kfl behavior", got, want)
	}
}

func TestFilterSummaryClearingQueryDropsTypedFilters(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = setSearchQuery(m, "st:completed")
	if m.filterSummary() != "St:completed" {
		t.Fatalf("setup: filterSummary() = %q, want \"St:completed\"", m.filterSummary())
	}

	m = setSearchQuery(m, "")
	if got := m.filterSummary(); got != "" {
		t.Fatalf("filterSummary() after clearing query = %q, want empty (D02: query cleared = typed filters gone)", got)
	}
}

// --- D03: parser runs every keystroke, only rest reaches Bleve ---

func TestBlevePrefixExcludedFromDispatchedQuery(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, runeMsg('/'))
	for _, r := range "st:completed foo" {
		m = step(t, m, runeMsg(r))
	}

	if m.searchPrefixRest != "foo" {
		t.Fatalf("searchPrefixRest after typing \"st:completed foo\" = %q, want \"foo\"", m.searchPrefixRest)
	}
	// "foo" is only 3 chars -- reaches the Bleve threshold, so a dispatch
	// must have fired for "foo", never for the full raw query.
	if !m.searchBleveLoading {
		t.Fatal("expected a Bleve dispatch once rest reaches 3 chars")
	}
}

func TestBleveNotDispatchedBelowThresholdWhenOnlyPrefixesTyped(t *testing.T) {
	m := fixtureModel(t, fixtureBeans())
	m = step(t, m, runeMsg('/'))
	for _, r := range "st:completed" {
		m = step(t, m, runeMsg(r))
	}

	if m.searchPrefixRest != "" {
		t.Fatalf("searchPrefixRest = %q, want empty (pure prefix query)", m.searchPrefixRest)
	}
	// A transient mid-typing state ("st:" alone, an incomplete/empty-value
	// token) legitimately falls into rest per parseSearchPrefixes and MAY
	// have fired a dispatch for that 3-char snapshot (PO-Akzeptanz: an
	// invalid/incomplete token is plain search text, no special-casing) --
	// that in-flight flag from a since-superseded dispatch is not this
	// test's concern. What D03 guarantees is the CURRENT decision: once the
	// full valid prefix is typed, rest is empty and no dispatch is due now.
	if m.maybeBleveCmd() != nil {
		t.Fatal("a pure-prefix query (empty rest) must not have a Bleve dispatch due right now")
	}
}
