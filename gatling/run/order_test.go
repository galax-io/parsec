package run

import (
	"math/rand/v2"
	"testing"
	"time"
)

// names renders candidates for a failure message.
func names(runs []runDir) []string {
	out := make([]string, len(runs))
	for i, r := range runs {
		out[i] = r.name
	}

	return out
}

// permutations lists every ordering of runs — six for the triple below.
func permutations(runs []runDir) [][]runDir {
	if len(runs) <= 1 {
		return [][]runDir{append([]runDir(nil), runs...)}
	}

	var out [][]runDir

	for i := range runs {
		rest := make([]runDir, 0, len(runs)-1)
		rest = append(rest, runs[:i]...)
		rest = append(rest, runs[i+1:]...)

		for _, p := range permutations(rest) {
			out = append(out, append([]runDir{runs[i]}, p...))
		}
	}

	return out
}

// randomRoot builds two to eight candidates mixing stamped and renamed
// directories across two modification times, so equal times — the case a
// clone, an rsync or a cache restore produces — are the common case.
func randomRoot(rng *rand.Rand) []runDir {
	pool := []string{
		"sima-20260909000000000", "sima-20260101000000000", "simb-20260909000000000",
		"simb-20200101000000000", "simc-20990101000000000", "simAA", "archived-run",
		"another-run", "simz", "sim-2026090900000000", // sixteen digits: not a stamp
	}
	times := []time.Time{
		time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.September, 9, 0, 0, 0, 0, time.UTC),
	}

	n := 2 + rng.IntN(7)
	rng.Shuffle(len(pool), func(a, b int) { pool[a], pool[b] = pool[b], pool[a] })

	runs := make([]runDir, 0, n)
	for _, name := range pool[:n] {
		runs = append(runs, runDir{name: name, mod: times[rng.IntN(len(times))]})
	}

	return runs
}

// newest is a maximum, and a maximum is only well defined over a total order.
// With the pairwise tie-break that preceded compareRuns, the mixed triple below
// had two answers depending on the order the candidates were scanned in, and
// os.ReadDir's name order picked the wrong one. Every permutation of it, and
// a thousand shuffles of random roots, must agree with themselves.
func TestNewestIsIndependentOfCandidateOrder(t *testing.T) {
	t.Parallel()

	same := time.Date(2026, time.September, 10, 0, 0, 0, 0, time.UTC)
	mixed := []runDir{
		{name: "simA-20990101000000000", mod: same},
		{name: "simB-20200101000000000", mod: same},
		{name: "simAA", mod: same},
	}

	for _, p := range permutations(mixed) {
		if got := newest(p); got != "simA-20990101000000000" {
			t.Fatalf("newest(%v) = %s; want the run with the latest stamp", names(p), got)
		}
	}

	rng := rand.New(rand.NewPCG(2026, 9)) //nolint:gosec // a seeded generator for a repeatable shuffle; nothing here is secret

	for i := range 1000 {
		runs := randomRoot(rng)
		want := newest(runs)

		rng.Shuffle(len(runs), func(a, b int) { runs[a], runs[b] = runs[b], runs[a] })

		if got := newest(runs); got != want {
			t.Fatalf("root %d: newest = %s in one order and %s in another, for %v", i, want, got, names(runs))
		}
	}
}
