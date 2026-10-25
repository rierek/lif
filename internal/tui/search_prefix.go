package tui

// search_prefix.go — Such-Präfixe `st:`/`ty:`/`pr:`/`tag:` in der `/`-Suche
// (bt-2kfl, PO-Entscheidungen Grilling 2026-07-17 D02/D03 final,
// epic-E13-plan.md »Item 3«). Kept in its own file (Implementer-Wahl per
// plan step 1) so view_browse_repo.go's/box_filter_facets.go's diffs stay
// small -- both files are shared with parallel worktree work in the same
// wave (bt-d3ps/bt-nxuk, disjunct file families).
//
// D02 (separate additive layer): a typed prefix NEVER writes into
// m.filterStatus/m.filterType/m.filterPriority/m.filterTag (the f-menu's own
// state) -- it lives entirely in m.searchPrefixFacets/m.searchPrefixRest
// (types.go), and filterSummary (box_filter_facets.go) renders the UNION of
// both layers for display only.
// D03 (parse every keystroke): keySearchInput (update.go) calls
// applySearchPrefixes() after every m.searchQuery update; only the derived
// rest ever reaches maybeBleveCmd/dispatchBleveIfDue -- prefix tokens
// themselves never reach Bleve.

import (
	"github.com/rierek/lif/pkg/item"
	"strings"
)

// searchPrefixKeys maps a search-prefix token's keyword (case-insensitive,
// lower-cased before lookup) to filterSummary's facet-key namespace
// ("status"/"type"/"priority"/"tag") -- mirrors facetHead's abbreviation set
// (box_filter_facets.go) MINUS "archive" (bean bt-2kfl PO-Zitat: "das hat
// keinen Präfix" -- Show-archived stays a menu-only toggle, no `archive:`
// prefix keyword exists).
var searchPrefixKeys = map[string]string{
	"st":  "status",
	"ty":  "type",
	"pr":  "priority",
	"tag": "tag",
}

// parseSearchPrefixes tokenizes query on whitespace (strings.Fields) and
// splits each token into a `<prefix>:<value>` facet pair vs. plain text
// (epic-E13-plan.md »Item 3« step 1, PO-Akzeptanz "ungültiges Präfix/Wert ->
// kein Fehler, fällt in rest"). A token qualifies as a facet pair only when
// it has a NON-EMPTY prefix (before the first ':') that resolves via
// searchPrefixKeys (case-insensitive) AND a non-empty value (after the
// ':') -- everything else (no colon, empty prefix ":foo", empty value
// "st:", unrecognized prefix "xx:foo") falls into rest UNCHANGED, verbatim,
// in its original position (space-joined, PO-Akzeptanz "Reihenfolge egal"
// applies to prefix-vs-prefix ordering, not to rest's own token order).
//
// Facet values are lower-cased before storage: Status/Type/Priority's
// canonical enum values (data.StatusValues() et al.) are already lower-case
// (box_filter_facets.go buildFilterItems), so this keeps a typed "St:Epic"
// consistent with the menu's own "epic" both for matching (EqualFold-safe
// either way) AND for the filterSummary union display, which would
// otherwise show mismatched casing side-by-side for the same logical value
// (Implementer-Entscheidung, PO nannte keine Case-Regel für WERTE, nur für
// die Präfix-Keywords selbst).
//
// Same facet repeated (e.g. "st:completed st:in-progress") collects BOTH
// values under one key -- OR-within-facet semantics, consumed by
// beanMatchesSearchPrefixFacets below (mirrors beanMatchesFacets,
// box_filter_facets.go).
func parseSearchPrefixes(query string) (facets map[string][]string, rest string) {
	facets = map[string][]string{}
	var restTokens []string
	for _, tok := range strings.Fields(query) {
		idx := strings.Index(tok, ":")
		if idx <= 0 || idx == len(tok)-1 {
			restTokens = append(restTokens, tok)
			continue
		}
		key, ok := searchPrefixKeys[strings.ToLower(tok[:idx])]
		if !ok {
			restTokens = append(restTokens, tok)
			continue
		}
		value := strings.ToLower(tok[idx+1:])
		facets[key] = append(facets[key], value)
	}
	return facets, strings.Join(restTokens, " ")
}

// applySearchPrefixes re-derives m.searchPrefixFacets/m.searchPrefixRest
// from the CURRENT m.searchQuery (D03: called on every keystroke,
// keySearchInput, update.go) -- the single call site that keeps the two
// fields in sync with searchQuery; nothing else in this package writes them
// directly (test helper setSearchQuery, update_test.go, mirrors this exact
// call for tests that set m.searchQuery outside a real tea.Update
// round-trip).
func (m model) applySearchPrefixes() model {
	m.searchPrefixFacets, m.searchPrefixRest = parseSearchPrefixes(m.searchQuery)
	return m
}

// beanMatchesSearchPrefixFacets is beanMatchesSearch's (view_browse_repo.go)
// prefix-facet half: an empty m.searchPrefixFacets imposes no constraint
// (matches everything, mirrors beanMatchesFacets' own empty-map contract,
// box_filter_facets.go); a present facet key requires membership among its
// collected values (OR within the facet), and every present key must hold
// (AND across facets) -- exactly the "st:completed ty:epic foo" ->
// status=completed AND type=epic PO-Akzeptanz.
func (m model) beanMatchesSearchPrefixFacets(b *item.Item) bool {
	for facet, values := range m.searchPrefixFacets {
		if !searchPrefixFacetHit(b, facet, values) {
			return false
		}
	}
	return true
}

// searchPrefixFacetHit checks ONE facet's OR-membership against b, case-
// insensitively (strings.EqualFold) -- parseSearchPrefixes already lower-
// cases the stored values, but b.Status/b.Type/b.Priority/b.Tags are
// compared fold-safe regardless in case a caller ever stores mixed case.
func searchPrefixFacetHit(b *item.Item, facet string, values []string) bool {
	switch facet {
	case "status":
		return containsFold(values, b.Status)
	case "type":
		return containsFold(values, b.Type)
	case "priority":
		return containsFold(values, b.Priority)
	case "tag":
		for _, t := range b.Tags {
			if containsFold(values, t) {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func containsFold(values []string, s string) bool {
	for _, v := range values {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}
