package commands

import "fmt"

// validateRoadmapDepth rejects a --depth the pruner cannot honour. Only an
// explicitly passed flag is checked: the zero value means the flag was never
// set, which is "no limit" and not a user error. changed comes from
// cobra's Flags().Changed, so `--depth 0` is still rejected.
func validateRoadmapDepth(depth int, changed bool) error {
	if changed && depth < 1 {
		return fmt.Errorf("--depth must be >= 1, got %d", depth)
	}
	return nil
}
