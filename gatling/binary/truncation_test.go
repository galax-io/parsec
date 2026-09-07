package binary_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
)

// cutEnding is how a read of a possibly-cut log finished, reduced to the three
// endings a caller may distinguish: a clean end, a cut short, or a failure.
type cutEnding int

const (
	endedClean cutEnding = iota
	endedCut
	endedFailed
)

func (e cutEnding) String() string {
	switch e {
	case endedClean:
		return "a clean end"
	case endedCut:
		return "a cut short"
	case endedFailed:
		return "a failure"
	default:
		return "an ending this test does not know"
	}
}

func endingOf(err error) cutEnding {
	var truncErr *gatling.TruncationError

	switch {
	case errors.Is(err, io.EOF):
		return endedClean
	case errors.As(err, &truncErr):
		return endedCut
	default:
		return endedFailed
	}
}

// readCut reads a log that may stop anywhere and reports what came out of it and
// how it ended. A constructor failure is an ending too: a log cut inside its run
// record yields no reader and no records.
func readCut(log []byte) ([]gatling.Record, cutEnding, error) {
	rd, err := binary.NewReader(bytes.NewReader(log))
	if err != nil {
		return nil, endingOf(err), err
	}

	var recs []gatling.Record

	for {
		rec, err := rd.Next()
		if err != nil {
			return recs, endingOf(err), err
		}

		if rec.Groups != nil {
			rec.Groups = append(make([]string, 0, len(rec.Groups)), rec.Groups...)
		}

		recs = append(recs, rec)
	}
}

func corpusLog(t *testing.T, dir string) []byte {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(dir, "simulation.log")) //nolint:gosec // a corpus path from the test's own glob
	if err != nil {
		t.Fatal(err)
	}

	return raw
}

// Every offset in the last 200 bytes of every recording. A killed run leaves the
// log at one of these, and none of them may panic, lose a record it had already
// completed, or come out as a damaged file.
func TestCutAtEveryOffset(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			raw := corpusLog(t, dir)

			whole, ending, err := readCut(raw)
			if ending != endedClean {
				t.Fatalf("the intact recording ended in %v with %v, want a clean end", ending, err)
			}

			delivered := 0
			// The last offset at which a cut read cleanly: the file ended on a
			// record boundary there, so every cut after it and before the next
			// boundary is a record that began exactly there.
			boundary := -1

			for n := max(len(raw)-200, 0); n <= len(raw); n++ {
				recs, ending, err := readCut(raw[:n])

				truncErr := assertEnding(t, n, ending, err)
				assertPrefixOf(t, n, recs, whole)

				// Pins Offset on its own. Every other assertion here checks the
				// sum Offset+Dropped, which stays right even if the reader
				// stopped tracking where records begin — this one does not.
				switch {
				case truncErr == nil:
					boundary = n
				case boundary >= 0 && truncErr.Offset != int64(boundary):
					t.Fatalf("cut at %d: the incomplete record is said to begin at %d, but a cut at %d read cleanly, so it begins there",
						n, truncErr.Offset, boundary)
				}

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

// assertEnding holds a cut read to one of the two endings a shortened log may
// have, and hands back the truncation when there was one. A failure is neither:
// a log cut short is not a damaged one.
func assertEnding(t *testing.T, n int, ending cutEnding, err error) *gatling.TruncationError {
	t.Helper()

	if ending == endedClean {
		// The format has no end marker, so a cut landing exactly on a record
		// boundary is a shorter valid log. No reader can tell it from a
		// complete one.
		return nil
	}

	var truncErr *gatling.TruncationError
	if ending != endedCut || !errors.As(err, &truncErr) {
		t.Fatalf("cut at %d ended in %v with %v, want a truncation or a clean end", n, ending, err)
	}

	if truncErr.Format != gatling.FormatBinary || truncErr.Line != 0 {
		t.Fatalf("cut at %d: %+v, want a binary truncation with no line number", n, truncErr)
	}

	if truncErr.Dropped <= 0 {
		t.Fatalf("cut at %d: Dropped = %d, want a positive count", n, truncErr.Dropped)
	}

	// The position names where the incomplete record began, so the stream
	// stopped at Offset+Dropped — which, for a file that simply ran out, is its
	// length.
	if got := truncErr.Offset + truncErr.Dropped; got != int64(n) {
		t.Fatalf("cut at %d: Offset+Dropped = %d, want the length of the cut file", n, got)
	}

	// What was still to come is named, so a decoder that stopped saying it does
	// not go unnoticed: the message renders it.
	if truncErr.Expected == "" {
		t.Fatalf("cut at %d: the truncation says nothing about what was still to come: %v", n, truncErr)
	}

	return truncErr
}

// assertPrefixOf requires the records a cut log yields to be the records the
// whole log yields, up to where the bytes stopped. Truncation changes nothing
// about how a complete record decodes.
func assertPrefixOf(t *testing.T, n int, recs, whole []gatling.Record) {
	t.Helper()

	if len(recs) > len(whole) {
		t.Fatalf("cut at %d yields %d records, more than the whole log's %d", n, len(recs), len(whole))
	}

	for i := range recs {
		if !reflect.DeepEqual(recs[i], whole[i]) {
			t.Fatalf("cut at %d: record %d differs from the whole log's:\n got  %+v\n want %+v",
				n, i, recs[i], whole[i])
		}
	}
}

// A cut inside the run record leaves no header, no version verdict and no
// record to salvage — but it still says the log was cut short, because a sidecar
// that attached in a run's first milliseconds needs "come back with more bytes",
// not "these bytes are not a Gatling log".
func TestCutInsideRunRecord(t *testing.T) {
	t.Parallel()

	raw := corpusLog(t, corpusDirs(t)[0])

	for _, n := range []int{1, 2, 5, 9} {
		_, ending, err := readCut(raw[:n])
		if ending != endedCut {
			t.Fatalf("%d bytes of the run record ended in %v with %v, want a cut short", n, ending, err)
		}

		var truncErr *gatling.TruncationError
		if !errors.As(err, &truncErr) {
			t.Fatalf("%d bytes of the run record ended with %v, want a truncation", n, err)
		}

		if truncErr.Offset != 0 || truncErr.Dropped != int64(n) {
			t.Fatalf("%d bytes: %+v, want the record's start at 0 and %d dropped", n, truncErr, n)
		}
	}
}

// The cut is terminal, like every other ending: there is no next record after it.
func TestTruncationIsTerminal(t *testing.T) {
	t.Parallel()

	raw := corpusLog(t, corpusDirs(t)[0])

	rd, err := binary.NewReader(bytes.NewReader(raw[:len(raw)-3]))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	var first error

	for {
		if _, first = rd.Next(); first != nil {
			break
		}
	}

	if endingOf(first) != endedCut {
		t.Fatalf("a log three bytes short ended with %v, want a cut short", first)
	}

	if _, again := rd.Next(); !errors.Is(again, first) {
		t.Fatalf("Next after the cut = %v, want the same error again", again)
	}
}

// The model path carries the same value: a consumer of the canonical run does
// not have to drop to the wire records to learn its log was cut.
func TestTruncationReachesRunReader(t *testing.T) {
	t.Parallel()

	raw := corpusLog(t, corpusDirs(t)[0])

	rd, err := binary.NewRunReader(bytes.NewReader(raw[:len(raw)-3]))
	if err != nil {
		t.Fatalf("NewRunReader: %v", err)
	}

	items := 0

	for {
		_, err := rd.Next()
		if err != nil {
			if endingOf(err) != endedCut {
				t.Fatalf("RunReader.Next ended with %v, want a cut short", err)
			}

			if items == 0 {
				t.Fatal("no item was delivered before the cut, want the run's completed items")
			}

			return
		}

		items++
	}
}

// An intact recording is never reported as cut.
func TestIntactRecordingIsNotTruncated(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			if _, ending, err := readCut(corpusLog(t, dir)); ending != endedClean {
				t.Fatalf("read ended in %v with %v, want io.EOF", ending, err)
			}
		})
	}
}

// The format carries no end marker, so a cut landing exactly on a record
// boundary produces a shorter valid log: it decodes cleanly, reports nothing,
// and cannot be told from a complete run by any reader. Only something that can
// see the writer knows the difference, which is why this module never guesses at
// it. The test insists such cuts exist and are silent, so that a later change
// cannot start inventing a signal the artefact does not carry.
func TestACutOnARecordBoundaryIsASilentShorterLog(t *testing.T) {
	t.Parallel()

	raw := corpusLog(t, corpusDirs(t)[0])
	whole, _, _ := readCut(raw)

	boundaries := 0

	for n := max(len(raw)-200, 0); n < len(raw); n++ {
		recs, ending, _ := readCut(raw[:n])
		if ending != endedClean {
			continue
		}

		boundaries++

		if len(recs) >= len(whole) {
			t.Fatalf("a log cut at byte %d of %d read cleanly and yielded all %d records: nothing was cut",
				n, len(raw), len(recs))
		}
	}

	if boundaries == 0 {
		t.Fatal("no cut landed on a record boundary, so nothing here tested the case")
	}
}

// stalledAfter hands over a prefix and then makes no progress, which the
// io.Reader contract permits and says must not be taken for an end.
type stalledAfter struct {
	data []byte
}

func (s *stalledAfter) Read(p []byte) (int, error) {
	if len(s.data) == 0 {
		return 0, nil
	}

	n := copy(p, s.data)
	s.data = s.data[n:]

	return n, nil
}

// The whole public path ends rather than hangs. A follower whose source stalls
// mid-record used to leave the decoder spinning with no error and nothing to
// cancel, which is the one failure a caller cannot recover from.
func TestAStalledSourceEndsTheReadRatherThanHanging(t *testing.T) {
	t.Parallel()

	raw := corpusLog(t, corpusDirs(t)[0])

	rd, err := binary.NewReader(&stalledAfter{data: raw[:len(raw)-3]})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	for {
		_, err := rd.Next()
		if err == nil {
			continue
		}

		if !errors.Is(err, io.ErrNoProgress) {
			t.Fatalf("a stalled source ended the read with %v; want io.ErrNoProgress", err)
		}

		return
	}
}

// And the constructor too: a run record that stalls part-way through leaves
// NewReader with the same choice, and it must make the same one.
func TestAStalledSourceEndsTheConstructorRatherThanHanging(t *testing.T) {
	t.Parallel()

	raw := corpusLog(t, corpusDirs(t)[0])

	if _, err := binary.NewReader(&stalledAfter{data: raw[:4]}); !errors.Is(err, io.ErrNoProgress) {
		t.Fatalf("NewReader over a stalled source = %v; want io.ErrNoProgress", err)
	}
}
