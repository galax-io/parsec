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

// sampler records the highest heap seen while the input flows through it.
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

		var m runtime.MemStats

		runtime.ReadMemStats(&m)
		s.peak = max(s.peak, m.HeapAlloc)
	}

	return n, err
}

func decodeSized(t *testing.T, size int64) (records int64, peak uint64) {
	t.Helper()

	runtime.GC()

	src := &sampler{r: newSynthLog(size), every: size / samples}

	rd, err := text.NewReader(src)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	for {
		_, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return records, src.peak
		}

		if err != nil {
			t.Fatalf("Next after %d records: %v", records, err)
		}

		records++
	}
}

// TestPeakMemory proves the reader's memory does not grow with the log: a
// 256 MiB log and one ten times larger must both stay under 32 MiB of heap,
// and the larger one may peak at most twice as high as the smaller (SC-004).
// Growth proportional to the input would show as ten times; the factor of
// two absorbs garbage-collector timing, which moves HeapAlloc by a few MiB
// between runs and between machines while the live heap stays one line
// buffer and a bounded name table.
//
//nolint:paralleltest // measures peak heap and must run alone
func TestPeakMemory(t *testing.T) {
	const small = 256 * mib

	n1, peak1 := decodeSized(t, small)
	t.Logf("%d MiB: %d records, peak heap %.1f MiB", small/mib, n1, float64(peak1)/mib)

	if peak1 >= 32*mib {
		t.Fatalf("peak heap %.1f MiB for a %d MiB log, want under 32 MiB", float64(peak1)/mib, small/mib)
	}

	if testing.Short() {
		t.Skip("the ten-times-larger run is skipped under -short")
	}

	n2, peak2 := decodeSized(t, 10*small)
	t.Logf("%d MiB: %d records, peak heap %.1f MiB", 10*small/mib, n2, float64(peak2)/mib)

	if peak2 >= 32*mib || peak2 > 2*peak1 {
		t.Fatalf("peak heap grew from %.1f MiB to %.1f MiB when the log grew ten times", float64(peak1)/mib, float64(peak2)/mib)
	}
}
