package text_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/text"
)

// cutEnding is how a read of a possibly-cut log finished, reduced to the three
// endings a caller may distinguish: a clean end, a cut short, or a failure.
type cutEnding int

const (
	endedClean cutEnding = iota
	endedCut
	endedFailed
)

// readCut reads a log that may stop anywhere and reports what came out of it and
// how it ended. A constructor failure is an ending too: a log cut before its run
// header yields no reader and no records.
func readCut(t *testing.T, log []byte) ([]gatling.Record, cutEnding, error) {
	t.Helper()

	rd, err := text.NewReader(bytes.NewReader(log))
	if err != nil {
		return nil, endingOf(err), err
	}

	var recs []gatling.Record

	for {
		rec, err := rd.Next()
		if err != nil {
			return recs, endingOf(err), err
		}

		rec.Groups = append([]string(nil), rec.Groups...)
		recs = append(recs, rec)
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

func fixtureBytes(t *testing.T, name string) []byte {
	t.Helper()

	b, err := os.ReadFile(filepath.Join("testdata", "fixtures", name+".fixture.log")) //nolint:gosec // a fixture path built from a name this test supplies
	if err != nil {
		t.Fatal(err)
	}

	return b
}

// A log whose final line was never terminated is the ordinary ending of a killed
// run: Gatling writes through a buffer, so an aborted test stops mid-line. Every
// record before it is delivered, and the read says how much was lost.
func TestTruncatedLastLine(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"truncated-last-line", "unterminated-last-line"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			raw := fixtureBytes(t, name)

			recs, ending, err := readCut(t, raw)
			if ending != endedCut {
				t.Fatalf("read ended %v with %v, want a truncation", ending, err)
			}

			var truncErr *gatling.TruncationError
			if !errors.As(err, &truncErr) {
				t.Fatalf("got %v, want *gatling.TruncationError", err)
			}

			if len(recs) != 3 {
				t.Fatalf("%d records were delivered before the cut, want 3", len(recs))
			}

			if truncErr.Format != gatling.FormatText || truncErr.Line != 6 || truncErr.Offset != 0 {
				t.Fatalf("got %+v, want a text truncation on line 6 with no byte offset", truncErr)
			}

			// The dropped count is the unterminated tail: everything after the
			// last newline the file carries.
			wantDropped := int64(len(raw) - bytes.LastIndexByte(raw, '\n') - 1)
			if truncErr.Dropped != wantDropped {
				t.Fatalf("Dropped = %d, want %d", truncErr.Dropped, wantDropped)
			}

			if !strings.Contains(truncErr.Error(), "cut short") {
				t.Fatalf("the message does not say the log was cut short: %v", truncErr)
			}
		})
	}
}

// The cut is terminal, like every other ending: there is no next record after it.
func TestTruncationIsTerminal(t *testing.T) {
	t.Parallel()

	rd, err := text.NewReader(bytes.NewReader(fixtureBytes(t, "truncated-last-line")))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	var first error

	for {
		if _, first = rd.Next(); first != nil {
			break
		}
	}

	if _, again := rd.Next(); !errors.Is(again, first) {
		t.Fatalf("Next after the cut = %v, want the same error again", again)
	}
}

// Cutting a valid log at every offset in its tail: nothing panics, the records
// that arrive are always a prefix of the whole log's records, and the ending is
// never a syntax error — a cut log is not a damaged one.
func TestCutAtEveryOffset(t *testing.T) {
	t.Parallel()

	raw := fixtureBytes(t, "version-3.11.5")

	whole, ending, err := readCut(t, raw)
	if ending != endedClean {
		t.Fatalf("the intact fixture ended %v with %v, want a clean end", ending, err)
	}

	from := max(len(raw)-200, 0)
	delivered := 0

	for n := from; n <= len(raw); n++ {
		recs, ending, err := readCut(t, raw[:n])

		assertEnding(t, raw, n, ending, err)
		assertPrefixOf(t, n, recs, whole)

		if len(recs) < delivered {
			t.Fatalf("cut at %d yields %d records after %d bytes yielded %d: a longer prefix lost a record",
				n, len(recs), n-1, delivered)
		}

		delivered = len(recs)
	}

	if delivered != len(whole) {
		t.Fatalf("the last cut yields %d records, the whole log %d", delivered, len(whole))
	}
}

// assertEnding holds a cut read to one of the endings a shortened log may have.
func assertEnding(t *testing.T, raw []byte, n int, ending cutEnding, err error) {
	t.Helper()

	switch ending {
	case endedClean:
		// A cut landing exactly on a line boundary is a shorter valid log.
		return

	case endedCut:
		var truncErr *gatling.TruncationError
		if !errors.As(err, &truncErr) {
			t.Fatalf("cut at %d ended with %v, want a truncation", n, err)
		}

		if truncErr.Dropped <= 0 {
			t.Fatalf("cut at %d: Dropped = %d, want a positive count", n, truncErr.Dropped)
		}

		if truncErr.Line <= 0 {
			t.Fatalf("cut at %d: the truncation names no line: %+v", n, truncErr)
		}

	case endedFailed:
		// The only failure a prefix may produce is a preamble that ended on a
		// line boundary with no run header: there is no partial line to report,
		// and no header means no reader. A cut inside a line must never come out
		// as a failure — that is the confusion this feature removes.
		if n > 0 && raw[n-1] != '\n' {
			t.Fatalf("cut at %d, inside a line, ended with %v, want a truncation", n, err)
		}

		var syntaxErr *gatling.SyntaxError
		if !errors.As(err, &syntaxErr) || !strings.Contains(syntaxErr.Found, "end of input") {
			t.Fatalf("cut at %d ended with %v, want a truncation or a missing header", n, err)
		}
	}
}

// assertPrefixOf requires the records a cut log yields to be the records the
// whole log yields, up to where the bytes stopped.
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

// A log cut inside its preamble fails the constructor and says why: the sidecar
// that attached before the header was written needs "come back with more bytes",
// not "these bytes are not a Gatling log".
func TestCutInsidePreamble(t *testing.T) {
	t.Parallel()

	raw := fixtureBytes(t, "version-3.11.5")
	head := bytes.IndexByte(raw, '\n') + 1 // the ASSERTION line, then half of RUN

	_, err := text.NewReader(bytes.NewReader(raw[:head+10]))

	var truncErr *gatling.TruncationError
	if !errors.As(err, &truncErr) {
		t.Fatalf("NewReader on a cut preamble = %v, want *gatling.TruncationError", err)
	}

	if truncErr.Dropped != 10 {
		t.Fatalf("Dropped = %d, want the 10 bytes of the unfinished header line", truncErr.Dropped)
	}
}

// The model path carries the same value: a consumer of the canonical run does
// not have to drop to the wire records to learn its log was cut.
func TestTruncationReachesRunReader(t *testing.T) {
	t.Parallel()

	rd, err := text.NewRunReader(bytes.NewReader(fixtureBytes(t, "truncated-last-line")))
	if err != nil {
		t.Fatalf("NewRunReader: %v", err)
	}

	items := 0

	for {
		_, err := rd.Next()
		if err != nil {
			var truncErr *gatling.TruncationError
			if !errors.As(err, &truncErr) {
				t.Fatalf("RunReader.Next ended with %v, want *gatling.TruncationError", err)
			}

			if items == 0 {
				t.Fatal("no item was delivered before the cut, want the run's completed items")
			}

			return
		}

		items++
	}
}

// An intact log is never reported as cut.
func TestIntactLogIsNotTruncated(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"version-3.11.5", "version-3.12.0", "crlf"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, ending, err := readCut(t, fixtureBytes(t, name)); ending != endedClean {
				t.Fatalf("read ended %v with %v, want io.EOF", ending, err)
			}
		})
	}
}

// failAfter hands over data and then fails, which is what a broken source does
// and what a short file never does.
type failAfter struct {
	data []byte
	err  error
}

func (f *failAfter) Read(p []byte) (int, error) {
	if len(f.data) == 0 {
		return 0, f.err
	}

	n := copy(p, f.data)
	f.data = f.data[n:]

	return n, nil
}

// errBrokenSource is a source reporting a failure of its own that happens to
// wrap io.EOF, the way a truncated decompressor or a closed transport does.
var errBrokenSource = fmt.Errorf("decompressor gave up: %w", io.EOF)

// A source that breaks is not a log that ended, and it is certainly not a log
// cut short: that ending is a positive claim that the records delivered are what
// the run recorded. The binary codec held this rule; the scanner did not, so a
// broken transport came back either as a clean end or as a killed run.
func TestABrokenSourceIsNeitherAnEndingNorACut(t *testing.T) {
	t.Parallel()

	raw := fixtureBytes(t, "version-3.11.5")

	// Mid-line and on a line boundary: the scanner took a different branch for
	// each, and both branches were wrong.
	for _, name := range []string{"mid-line", "on a line boundary"} {
		cut := len(raw) - 8
		if name == "on a line boundary" {
			cut = bytes.LastIndexByte(raw, '\n') + 1
		}

		rd, err := text.NewReader(&failAfter{data: raw[:cut], err: errBrokenSource})
		if err != nil {
			t.Fatalf("%s: NewReader: %v", name, err)
		}

		var last error

		for {
			if _, last = rd.Next(); last != nil {
				break
			}
		}

		if errors.Is(last, io.EOF) {
			t.Fatalf("%s: a broken source was read as the clean end of the log: %v", name, last)
		}

		if errors.As(last, new(*gatling.TruncationError)) {
			t.Fatalf("%s: a broken source was read as a log cut short: %v", name, last)
		}

		if !strings.Contains(last.Error(), "decompressor gave up") {
			t.Fatalf("%s: the source's own failure is not named in the error: %v", name, last)
		}
	}
}

// The same during the preamble, where a broken source used to be blamed on the
// file: "expected a run header, found end of input" sends a caller to inspect a
// log that is fine.
func TestABrokenSourceInThePreambleIsNotAMissingHeader(t *testing.T) {
	t.Parallel()

	raw := fixtureBytes(t, "version-3.11.5")

	_, err := text.NewReader(&failAfter{data: raw[:bytes.IndexByte(raw, '\n')+1], err: errBrokenSource})

	if errors.As(err, new(*gatling.SyntaxError)) {
		t.Fatalf("a broken source is reported as a malformed preamble: %v", err)
	}

	if errors.Is(err, io.EOF) {
		t.Fatalf("a broken source matches io.EOF: %v", err)
	}
}

// The clean end is terminal, as the reader's documentation says and as the
// binary codec already did. A file still being appended to returns io.EOF and
// then more bytes; without the latch this reader delivered records after the end
// it had already declared, and the two codecs behind simlog.RecordReader
// disagreed about a contract both of them state.
func TestACleanEndIsTerminal(t *testing.T) {
	t.Parallel()

	raw := fixtureBytes(t, "version-3.11.5")
	cut := bytes.LastIndexByte(raw[:len(raw)-1], '\n') + 1

	rd, err := text.NewReader(&resumeAfterEOF{first: raw[:cut], rest: raw[cut:]})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	for {
		if _, err = rd.Next(); err != nil {
			break
		}
	}

	if !errors.Is(err, io.EOF) {
		t.Fatalf("the read ended with %v, want io.EOF", err)
	}

	if _, again := rd.Next(); !errors.Is(again, io.EOF) {
		t.Fatalf("Next after the clean end = %v, want io.EOF again", again)
	}
}

// resumeAfterEOF reports the end of its first half and then hands over the rest,
// which is exactly what a *os.File on a log still being written does.
type resumeAfterEOF struct {
	first, rest []byte
	ended       bool
}

func (r *resumeAfterEOF) Read(p []byte) (int, error) {
	if len(r.first) > 0 {
		n := copy(p, r.first)
		r.first = r.first[n:]

		return n, nil
	}

	if !r.ended {
		r.ended = true

		return 0, io.EOF
	}

	if len(r.rest) == 0 {
		return 0, io.EOF
	}

	n := copy(p, r.rest)
	r.rest = r.rest[n:]

	return n, nil
}
