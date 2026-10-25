package search

import (
	"regexp"
	"strings"
	"sync"

	"github.com/rierek/lif/pkg/item"
)

// LocalMatcher returns the substring rule shared by the CLI (Core.Search) and
// the TUI filter: an item matches when the query occurs, case-insensitively,
// anywhere in its title or ID. `*` stands for any run of characters, `?` for
// exactly one. Bleve covers whole-word, phrase, field and fuzzy queries; this
// rule is unioned with it so partial words ("team" in "Teams") and wildcards
// with capitals ("Bei*") find items too. An empty query matches nothing.
func LocalMatcher(query string) func(*item.Item) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return func(*item.Item) bool { return false }
	}
	if !strings.ContainsAny(q, "*?") {
		return func(b *item.Item) bool {
			return strings.Contains(strings.ToLower(b.Title), q) || strings.Contains(strings.ToLower(b.ID), q)
		}
	}
	re := globRegexp(q)
	return func(b *item.Item) bool {
		return re.MatchString(b.Title) || re.MatchString(b.ID)
	}
}

// globCache keeps the last compiled pattern: the TUI asks once per item per
// render with the same query.
var globCache struct {
	sync.Mutex
	q  string
	re *regexp.Regexp
}

func globRegexp(q string) *regexp.Regexp {
	globCache.Lock()
	defer globCache.Unlock()
	if globCache.re != nil && globCache.q == q {
		return globCache.re
	}
	var sb strings.Builder
	sb.WriteString("(?is)") // case-insensitive, `.` also matches newline
	for _, r := range q {
		switch r {
		case '*':
			sb.WriteString(".*")
		case '?':
			sb.WriteString(".")
		default:
			sb.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	globCache.q, globCache.re = q, regexp.MustCompile(sb.String())
	return globCache.re
}
