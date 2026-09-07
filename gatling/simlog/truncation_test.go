package simlog_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/simlog"
)

// A log cut short reaches a caller through this package with the same ending the
// codec gives it: identification adds a format and nothing else.
func TestACutLogIsCutShortThroughSimlogToo(t *testing.T) {
	t.Parallel()

	for _, log := range corpusLogs(t) {
		t.Run(filepath.Base(filepath.Dir(log)), func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(log) //nolint:gosec // a corpus path from the test's own glob
			if err != nil {
				t.Fatal(err)
			}

			rd, err := simlog.NewReader(bytes.NewReader(raw[:len(raw)-3]))
			if err != nil {
				t.Fatalf("NewReader: %v", err)
			}

			delivered := 0

			for {
				_, err := rd.Next()
				if err == nil {
					delivered++

					continue
				}

				var truncErr *gatling.TruncationError
				if !errors.As(err, &truncErr) {
					t.Fatalf("a log three bytes short ended with %v, want a truncation", err)
				}

				if delivered == 0 {
					t.Fatal("no record was delivered before the cut")
				}

				return
			}
		})
	}
}

// Before the format is known there is no codec to attribute a cut to, so a
// stream too short to identify is refused as one whose format is not yet
// settled. It carries the same operational meaning — come back with more bytes —
// which is why NewReader's documentation names both endings for a follower.
func TestAStreamTooShortToIdentifyIsRefusedAsShort(t *testing.T) {
	t.Parallel()

	for _, head := range []string{"R", "RU", "\x00\x00\x00"} {
		_, err := simlog.NewReader(bytes.NewReader([]byte(head)))

		var formatErr *gatling.FormatError
		if !errors.As(err, &formatErr) || !formatErr.Short {
			t.Fatalf("%q opened with %v, want a *gatling.FormatError marked Short", head, err)
		}

		if errors.Is(err, io.EOF) {
			t.Fatalf("%q was reported as the end of a log: %v", head, err)
		}
	}
}
