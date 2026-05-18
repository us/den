// Package netpolicytest provides the per-suite executed-test floor mechanism
// for the netpolicy unit suites.
//
// testing.M.Run() returns only an exit code, never a test count, so a build
// that silently skips/excludes the whole suite would still exit 0. Each suite
// test calls Mark(t) as its first line; TestMain asserts the package floor via
// AssertFloor after m.Run(). This makes "the security suite did not run at all"
// a loud, non-zero failure rather than a green no-op.
package netpolicytest

import (
	"fmt"
	"os"
	"sync/atomic"
	"testing"
)

var marked int64

// Mark records that one netpolicy suite test executed. Call it as the first
// line of every suite test.
func Mark(t *testing.T) {
	t.Helper()
	atomic.AddInt64(&marked, 1)
}

// Count returns how many Mark calls have happened so far.
func Count() int64 { return atomic.LoadInt64(&marked) }

// AssertFloor is called from TestMain after m.Run(). It returns the code to
// pass to os.Exit: the original code if the executed-test count met floor,
// otherwise a non-zero code (preserving an already-failing code).
func AssertFloor(code, floor int) int {
	if c := Count(); c < int64(floor) {
		fmt.Fprintf(os.Stderr,
			"netpolicy suite executed-test floor NOT met: ran %d, want >= %d "+
				"(the security suite may have been skipped/excluded — failing loudly)\n",
			c, floor)
		if code == 0 {
			return 1
		}
	}
	return code
}
