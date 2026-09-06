//go:build integration

package binary_test

import (
	"errors"
	"io"
	"runtime"
	"testing"

	"github.com/galax-io/parsec/gatling/binary"
)

const mib = 1 << 20

// samples is how many times a run looks at the heap, whatever its size. A peak
// is a max over the draws taken, so more draws find a higher one: sampling at a
// fixed byte interval would give a ten-times-larger log ten times the chances to
// catch a collector trough, and the two peaks would not be comparable — which is
// the comparison the test exists to make.
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

	rd, err := binary.NewReader(src)
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

// The reader's memory must not grow with the log. A 256 MiB log and one ten
// times larger both stay under 32 MiB of heap, and the larger may peak at most
// twice as high as the smaller. Growth proportional to the input would show as
// ten times; the factor of two absorbs collector timing, which moves HeapAlloc
// by a few MiB between runs while the live heap stays a fixed read buffer, a
// reused group path and a table of four distinct strings.
//
// This is the format's own wrinkle: the string table is held for the whole read,
// so "bounded" has to mean bounded by the *simulation* and not by the *run*. The
// synthetic log introduces its names once and refers back for the rest, which is
// exactly the shape a soak run has.
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
		t.Fatalf("peak heap grew from %.1f MiB to %.1f MiB when the log grew ten times",
			float64(peak1)/mib, float64(peak2)/mib)
	}
}

// Memory must follow the number of distinct names, not the number of records.
// That is the format's own wrinkle: the string table is held for the whole read,
// so a decoder could be bounded in every other way and still grow with a run
// that keeps inventing names.
//
// The synthetic log introduces four names and then refers back for millions of
// records, which is the shape a soak run has. Ten times the records must cost
// the same table.
//
//nolint:paralleltest // measures peak heap and must run alone
func TestMemoryFollowsNamesNotRecords(t *testing.T) {
	n1, peak1 := decodeSized(t, 16*mib)
	n2, peak2 := decodeSized(t, 160*mib)

	if n2 < 9*n1 {
		t.Fatalf("the larger log yielded %d records and the smaller %d; the comparison needs ten times", n2, n1)
	}

	t.Logf("%d records: %.1f MiB; %d records: %.1f MiB — the same four names throughout",
		n1, float64(peak1)/mib, n2, float64(peak2)/mib)

	if peak2 > 2*peak1 {
		t.Fatalf("ten times the records cost %.1f MiB against %.1f MiB; the table is growing with the run",
			float64(peak2)/mib, float64(peak1)/mib)
	}
}
