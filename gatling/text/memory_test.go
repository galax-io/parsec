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

// sampler records the highest heap seen while the input flows through it,
// looking every 4 MiB of input.
type sampler struct {
	r     io.Reader
	since int64
	peak  uint64
}

func (s *sampler) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	s.since += int64(n)

	if s.since >= 4*mib {
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

	src := &sampler{r: newSynthLog(size)}

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
// and the larger one must not peak meaningfully above the smaller (SC-004).
// The tolerance between the two covers garbage-collector timing, not growth:
// the live heap is about one line buffer, and HeapAlloc oscillates above it
// between collections.
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

	if peak2 >= 32*mib || peak2 > peak1+4*mib {
		t.Fatalf("peak heap grew from %.1f MiB to %.1f MiB when the log grew ten times", float64(peak1)/mib, float64(peak2)/mib)
	}
}
