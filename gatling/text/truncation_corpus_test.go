//go:build integration

package text_test

import (
	"os"
	"path/filepath"
	"testing"
)

// The same sweep as TestCutAtEveryOffset, over the recorded runs rather than the
// fixtures: a real 3.11.5 or 3.12.0 log carries record shapes no hand-written
// fixture has — a group path spanning records, a long ERROR message, a multi-KB
// tail — and a truncation defect reachable only through one of those would never
// be cut otherwise. The corpus accessor this needs sits behind the integration
// tag, which is where every other corpus test in this package lives.
func TestCutAtEveryOffsetOverTheCorpus(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(filepath.Join(dir, "simulation.log")) //nolint:gosec // a corpus path from the test's own glob
			if err != nil {
				t.Fatal(err)
			}

			whole, ending, err := readCut(t, raw)
			if ending != endedClean {
				t.Fatalf("the intact recording ended %v with %v, want a clean end", ending, err)
			}

			if len(whole) == 0 {
				t.Fatal("the intact recording yielded no records, so the sweep compares nothing")
			}

			delivered := 0

			for n := max(len(raw)-200, 0); n <= len(raw); n++ {
				recs, ending, err := readCut(t, raw[:n])

				assertEnding(t, raw, n, ending, err)
				assertPrefixOf(t, n, recs, whole)

				if len(recs) < delivered {
					t.Fatalf("cut at %d yields %d records after a shorter prefix yielded %d", n, len(recs), delivered)
				}

				delivered = len(recs)
			}

			if delivered != len(whole) {
				t.Fatalf("the last cut yields %d records, the whole recording %d", delivered, len(whole))
			}
		})
	}
}
