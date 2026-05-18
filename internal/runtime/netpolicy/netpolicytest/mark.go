// Package netpolicytest provides the per-suite executed-test floor mechanism
// for the netpolicy unit suites.
//
// testing.M.Run() returns only an exit code, never a test count, so a build
// that silently skips/excludes the whole suite would still exit 0. Each suite
// test calls Mark(t) as its first line; TestMain asserts the package floor via
// AssertFloor after m.Run(). This makes "the security suite did not run at all"
// a loud, non-zero failure rather than a green no-op.
//
// The floor is enforced ONLY on an unfiltered run (the threat is silent
// build-tag/constraint drift on a default `go test`). When an explicit
// `-run`/`-skip` filter is active the operator deliberately selected a subset
// (e.g. `make test-integration` runs `-run TestIntegration`, which legitimately
// matches zero pure-unit netpolicy tests) — enforcing the floor there would be
// a false failure, so AssertFloor becomes inert.
package netpolicytest

import (
	"flag"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
)

// runFilterActive reports whether an explicit test selection filter is in
// effect. Called after m.Run(), so the testing flags are parsed.
func runFilterActive() bool {
	for _, name := range []string{"test.run", "test.skip"} {
		if f := flag.Lookup(name); f != nil && f.Value.String() != "" {
			return true
		}
	}
	return false
}

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
	if runFilterActive() {
		return code // operator-selected subset; floor is meaningless here
	}
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
