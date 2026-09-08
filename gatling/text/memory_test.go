//go:build integration

package text_test

import (
	"errors"
	"io"
	"runtime"
	"testing"

	"github.com/galax-io/parsec/gatling/text"
)

const mib = 1 << 20

// samples is how many times a run looks at the heap, whatever its size. A peak
// is a max over the draws taken, so more draws find a higher one: sampling at a
// fixed byte interval would give a ten-times-larger log ten times the chances
// to catch a collector trough, and the two peaks would not be comparable —
// which is the comparison the test exists to make.
const samples = 64

// budget is the peak heap this package documents for a decode of any size.
const budget = 32 * mib

// drift is how far the live heap may move between a run and one ten times
// longer. SC-004 says the figure does not change; for a sampled measurement
// this is what "does not change" has to mean.
//
// Sampling after a collection leaves almost no spread to allow for: every pair
// measured for this change came back equal, 2.5 against 2.5 MiB idle and 2.3
// against 2.3 under sixteen spinning goroutines at GOMAXPROCS=2. The unswept
// figure this replaced is what a GitHub runner failed on, 7.4 against 15.9,
// for a live heap that had not moved at all.
const drift = 1 * mib

// sampler records the highest live heap seen while the input flows through it.
type sampler struct {
	r     io.Reader
	every int64
	since int64
	peak  uint64
}

func (s *sampler) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	s.since += int64(n)

	if s.since >= s.every {
		s.since = 0
		s.peak = max(s.peak, liveHeap())
	}

	return n, err
}

// liveHeap collects, then reports what is still held.
//
// The collection is the measurement. HeapAlloc on its own counts what has been
// allocated and not yet swept, so it reads the collector's backlog as if it
// were retention: against a fixed live set it moves 2.0x when the machine is
// idle and 8.4x under contention, while the same set sampled after a
// collection moves 1.03x and 1.05x. A ten-times-longer run gives the collector
// ten times as many chances to fall behind, which is why the unswept figure
// grows with the log while the live heap does not — the growth a bare peak
// comparison then reports as a leak.
//
// It costs about 280us against the 29us of the read alone, taken `samples`
// times a run: under 20ms, against a decode measured in seconds.
func liveHeap() uint64 {
	var m runtime.MemStats

	runtime.GC()
	runtime.ReadMemStats(&m)

	return m.HeapAlloc
}

func decodeSized(t *testing.T, size int64) (records int64, peak uint64) {
	t.Helper()

	runtime.GC()

	src := &sampler{r: newSynthLog(size), every: max(size/samples, 1)}

	rd, err := text.NewReader(src)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	for {
		_, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return records, max(src.peak, liveHeap())
		}

		if err != nil {
			t.Fatalf("Next after %d records: %v", records, err)
		}

		records++
	}
}

// TestPeakMemory proves the reader's memory does not grow with the log: a
// 256 MiB log and one ten times larger must both stay inside the budget, and
// the live heap of the larger is the live heap of the smaller, within drift
// (SC-004). Growth proportional to the input would show as ten times the
// figure and fails both bounds.
//
// What is held is one line buffer and a bounded name table, however long the
// log runs, and sampling after a collection is what makes the two figures
// comparable: this is the assertion a GitHub runner failed at 7.4 against
// 15.9 MiB, on unswept garbage, for a live heap that had not moved.
//
//nolint:paralleltest // measures peak heap and must run alone
func TestPeakMemory(t *testing.T) {
	const small = 256 * mib

	n1, peak1 := decodeSized(t, small)
	t.Logf("%d MiB: %d records, peak heap %.1f MiB", small/mib, n1, float64(peak1)/mib)

	if peak1 >= budget {
		t.Fatalf("peak heap %.1f MiB for a %d MiB log, want under %d MiB", float64(peak1)/mib, small/mib, budget/mib)
	}

	if testing.Short() {
		t.Skip("the ten-times-larger run is skipped under -short")
	}

	n2, peak2 := decodeSized(t, 10*small)
	t.Logf("%d MiB: %d records, peak heap %.1f MiB", 10*small/mib, n2, float64(peak2)/mib)

	if peak2 >= budget || peak2 > peak1+drift {
		t.Fatalf("peak heap grew from %.1f MiB to %.1f MiB when the log grew ten times", float64(peak1)/mib, float64(peak2)/mib)
	}
}
