package commands

import (
	"reflect"
	"strings"
	"testing"
)

// A parentless item under a non-empty parents list is a policy warning:
// exit 0 by default, exit 1 with --strict, and --fix leaves the store alone.
func TestCheckParentlessPolicyExitCodesAndFix(t *testing.T) {
	dir := newCheckStoreDir(t)
	out, code := runCheckOnStore(t, dir, "", true, "BEANS_CHECK_PARENTLESS=1")
	if code != 0 || !strings.Contains(out, "aaa1") || !strings.Contains(out, "epic") {
		t.Fatalf("default run: code=%d, want 0 and a warning naming aaa1 and epic:\n%s", code, out)
	}
	before := storeFiles(t, dir)

	out, code = runCheckOnStore(t, dir, "", false, "BEANS_CHECK_PARENTLESS=1", "BEANS_CHECK_STRICT=1")
	if code != 1 {
		t.Errorf("--strict: code=%d, want 1:\n%s", code, out)
	}
	runCheckOnStore(t, dir, "", false, "BEANS_CHECK_PARENTLESS=1", "BEANS_CHECK_FIX=1")
	if after := storeFiles(t, dir); !reflect.DeepEqual(before, after) {
		t.Errorf("--fix changed the store:\nbefore=%q\nafter=%q", before, after)
	}
}
