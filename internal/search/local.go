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
	return fieldMatcher(query, func(b *item.Item) string { return b.Title }, func(b *item.Item) string { return b.ID })
}

// BodyMatcher applies LocalMatcher's rule (case-insensitive substring, `*`/`?`
// wildcards, empty query matches nothing) to the item body.
func BodyMatcher(query string) func(*item.Item) bool {
	return fieldMatcher(query, func(b *item.Item) string { return b.Body })
}

func fieldMatcher(query string, fields ...func(*item.Item) string) func(*item.Item) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return func(*item.Item) bool { return false }
	}
	if !strings.ContainsAny(q, "*?") {
		return func(b *item.Item) bool {
			for _, f := range fields {
				if strings.Contains(strings.ToLower(f(b)), q) {
					return true
				}
			}
			return false
		}
	}
	re := globRegexp(q)
	return func(b *item.Item) bool {
		for _, f := range fields {
			if re.MatchString(f(b)) {
				return true
			}
		}
		return false
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
