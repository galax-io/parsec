//go:build integration

package text_test

import (
	"errors"
	"io"
	"runtime"
	"testing"

	"github.com/galax-io/parsec/gatling/text"
)

func convertSized(t *testing.T, size int64) (items int64, peak uint64) {
	t.Helper()

	runtime.GC()

	src := &sampler{r: newSynthLog(size), every: size / samples}

	rd, err := text.NewRunReader(src)
	if err != nil {
		t.Fatalf("NewRunReader: %v", err)
	}

	for {
		_, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return items, src.peak
		}

		if err != nil {
			t.Fatalf("Next after %d items: %v", items, err)
		}

		items++
	}
}

// TestModelPeakMemory is TestPeakMemory's counterpart through the canonical
// types. The conversion must not turn a streaming read into a buffered one: a
// 256 MiB log and one ten times larger both stay under 32 MiB of heap, and the
// larger may peak at most twice as high (SC-004).
//
// A run value holding what grows with the run — every sample, or every user
// event — would show here as growth proportional to the input.
//
//nolint:paralleltest // measures peak heap and must run alone
func TestModelPeakMemory(t *testing.T) {
	const small = 256 * mib

	n1, peak1 := convertSized(t, small)
	t.Logf("%d MiB: %d items, peak heap %.1f MiB", small/mib, n1, float64(peak1)/mib)

	if peak1 >= 32*mib {
		t.Fatalf("peak heap %.1f MiB for a %d MiB log, want under 32 MiB", float64(peak1)/mib, small/mib)
	}

	if testing.Short() {
		t.Skip("the ten-times-larger run is skipped under -short")
	}

	n2, peak2 := convertSized(t, 10*small)
	t.Logf("%d MiB: %d items, peak heap %.1f MiB", 10*small/mib, n2, float64(peak2)/mib)

	if peak2 >= 32*mib || peak2 > 2*peak1 {
		t.Fatalf("peak heap grew from %.1f MiB to %.1f MiB when the log grew ten times", float64(peak1)/mib, float64(peak2)/mib)
	}
}
