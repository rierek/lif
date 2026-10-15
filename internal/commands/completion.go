package commands

import (
	"sort"

	"github.com/rierek/lif/pkg/item"
	"github.com/spf13/cobra"
)

// completionDirective is the ShellCompDirective every ValidArgsFunction in
// this file returns alongside its candidates. ShellCompDirectiveKeepOrder
// tells the shell to preserve itemIDCandidates' actionable-first ordering
// instead of re-sorting candidates alphabetically (beans-sfle AC-01).
const completionDirective = cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveKeepOrder

// completionUnbounded is the ValidArgsFunction for verbs whose cobra.Args
// arity has no ceiling (cobra.MinimumNArgs(1): complete, delete, scrap,
// show, start, tag, each declaring "Use: <verb> <id> [id...]"). Every
// position, however many the user has already typed, still names an item, so
// candidates never stop (R-04 AC-04).
func completionUnbounded(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return itemIDCandidates(), completionDirective
}

// completionUnboundedFiltered is completionUnbounded's per-verb-narrowed
// sibling: same unbounded-arity offering, but restricted to items keep
// reports true for (beans-j5so). complete/start/scrap use it to exclude
// items that verb cannot validly act on again (e.g. an item already
// completed); show/delete/tag stay on plain completionUnbounded because
// every item, regardless of status, remains a valid target for them
// (beans-sfle AC-02).
func completionUnboundedFiltered(keep func(*item.Item) bool) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return itemIDCandidatesFiltered(keep), completionDirective
	}
}

// completionNoFileComp is the ValidArgsFunction for verbs whose first
// positional argument is free-form text with no item-ID or file-path
// meaning -- create's title, graphql's query string. Without a
// ValidArgsFunction at all, cobra answers with ShellCompDirectiveDefault,
// which makes the shell fall back to file-name completion in the cwd
// (beans-12cb). This returns no candidates and blocks that fallback,
// without offering item-ID candidates that would never be a correct
// answer for either verb's first position.
func completionNoFileComp(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// completionUpTo returns a ValidArgsFunction that offers item-ID candidates
// only while fewer than n positions are already filled, then stops (R-04
// AC-05). n is the number of positions that actually name an existing item,
// which is not always the verb's cobra.Args ceiling: rename declares
// cobra.MaximumNArgs(2) but n is 1 there, because its second position is a
// brand-new identifier the caller is choosing, never one looked up in the
// store (R-04 AC-04's rename exclusion).
func completionUpTo(n int) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) >= n {
			return nil, completionDirective
		}
		return itemIDCandidates(), completionDirective
	}
}

// completionStatusRank buckets an item's status into the three-tier
// actionable-first order beans-sfle AC-01 requires: todo/in-progress (0)
// before draft (1), and both before completed/scrapped (2). A status
// outside this vocabulary is not actionable either, so it ranks with draft
// rather than being silently dropped -- every item stays a candidate
// regardless of status (AC-02).
func completionStatusRank(status string) int {
	switch status {
	case "todo", "in-progress":
		return 0
	case "completed", "scrapped":
		return 2
	default:
		return 1
	}
}

// itemIDCandidates lists every item's ID as a shell-completion candidate,
// each annotated with type/title/status for the shell's description column,
// ordered actionable-first (beans-sfle AC-01). Sorting never removes a
// candidate: every item the store holds is still present afterward, just
// reordered, so prefix input keeps reaching any item-ID (AC-02). It walks
// only core.All(), the in-memory slice PersistentPreRunE already populated
// via Core.Load (pkg/itemcore) before any ValidArgsFunction runs -- it
// reads Item.ID/.Type/.Title/.Status and never an item's Body, never
// pkg/search, and triggers no further disk read (R-04 SC-01, SC-03).
func itemIDCandidates() []string {
	return itemIDCandidatesFiltered(nil)
}

// itemIDCandidatesFiltered is itemIDCandidates' predicate-narrowed sibling
// (beans-j5so): identical candidate shape and actionable-first ordering,
// but an item is included only when keep(item) is true. A nil keep keeps
// every item, making itemIDCandidates() == itemIDCandidatesFiltered(nil).
func itemIDCandidatesFiltered(keep func(*item.Item) bool) []string {
	if core == nil {
		return nil
	}
	all := core.All()
	ordered := make([]*item.Item, 0, len(all))
	for _, b := range all {
		if keep == nil || keep(b) {
			ordered = append(ordered, b)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		return completionStatusRank(ordered[i].Status) < completionStatusRank(ordered[j].Status)
	})
	candidates := make([]string, 0, len(ordered))
	for _, b := range ordered {
		candidates = append(candidates, b.ID+"\t"+b.Type+" "+b.Title+" ("+b.Status+")")
	}
	return candidates
}

// isArchivedStatus reports whether status is one of cfg's Archive-marked
// statuses (pkg/config StatusConfig.Archive via cfg.GetStatus), the single
// source completion's per-verb narrowing predicates read from -- never a
// second, hand-maintained list of terminal status names (beans-j5so). An
// unknown status is not archived: it is not actionable either, but that is
// completionStatusRank's concern, not this predicate's.
func isArchivedStatus(status string) bool {
	s := cfg.GetStatus(status)
	return s != nil && s.Archive
}
