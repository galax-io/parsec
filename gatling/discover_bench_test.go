package gatling_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/galax-io/parsec/gatling"
)

// benchRoot builds a results root holding n runs, with modification times a
// second apart so the ordering has real work to do.
func benchRoot(tb testing.TB, n int) string {
	tb.Helper()

	root := tb.TempDir()
	base := time.Date(2026, time.September, 6, 4, 47, 0, 0, time.UTC)

	for i := range n {
		dir := filepath.Join(root, fmt.Sprintf("corpussimulation-2026090604%012d", i))
		if err := os.Mkdir(dir, 0o750); err != nil {
			tb.Fatalf("mkdir: %v", err)
		}

		if err := os.WriteFile(filepath.Join(dir, "simulation.log"), nil, 0o600); err != nil {
			tb.Fatalf("write: %v", err)
		}

		mod := base.Add(time.Duration(i) * time.Second)
		if err := os.Chtimes(filepath.Join(dir, "simulation.log"), mod, mod); err != nil {
			tb.Fatalf("chtimes: %v", err)
		}
	}

	return root
}

// BenchmarkFindRun holds the bound this feature states instead of a throughput
// figure: it decodes nothing and opens no simulation.log, so the only thing
// worth measuring is that the work stays proportional to the size of the results
// root and never to what is inside a run.
//
// One directory read, one stat per entry to test for a log, and a second stat
// for each entry that turns out to be a run. Allocations track the entry count.
func BenchmarkFindRun(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("runs=%d", n), func(b *testing.B) {
			root := benchRoot(b, n)

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				if _, err := gatling.FindRun(root); err != nil {
					b.Fatalf("FindRun: %v", err)
				}
			}
		})
	}
}

// BenchmarkFindRunLastRun is the same root with a pointer to read, so the cost
// of the file — a stat, a bounded read and a split — is visible beside the scan
// rather than hidden inside it.
func BenchmarkFindRunLastRun(b *testing.B) {
	root := benchRoot(b, 1000)

	pointer := fmt.Sprintf("corpussimulation-2026090604%012d", 42)
	if err := os.WriteFile(filepath.Join(root, "lastRun.txt"), []byte(pointer+"\n"), 0o600); err != nil {
		b.Fatalf("write lastRun.txt: %v", err)
	}

	// Guard the benchmark against measuring the wrong thing: if the name ever
	// stopped matching a run, this would quietly time the plain scan again.
	if loc, err := gatling.FindRun(root); err != nil || loc.Found != gatling.FoundByLastRun {
		b.Fatalf("setup: FindRun = %+v, %v; want a run found by lastRun.txt", loc, err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if _, err := gatling.FindRun(root); err != nil {
			b.Fatalf("FindRun: %v", err)
		}
	}
}

// BenchmarkFindRunNamedPath is the path a caller already knows: two stats and no
// listing at all, which is the floor the other two are measured against.
func BenchmarkFindRunNamedPath(b *testing.B) {
	root := benchRoot(b, 10)
	run := filepath.Join(root, "corpussimulation-2026090604000000000005")

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if _, err := gatling.FindRun(run); err != nil {
			b.Fatalf("FindRun: %v", err)
		}
	}
}
