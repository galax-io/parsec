package text_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/text"
)

// TestReaderGroupsSurviveAppend covers the one way a caller can be surprised by
// the reused Groups slice. The record's doc says it is valid until the next
// Next, so a caller that keeps it copies it — but append reads as a copy and
// would silently not be one if the slice carried spare capacity: it would write
// into the reader's own array, and the next record would rewrite what the
// caller believed it owned.
func TestReaderGroupsSurviveAppend(t *testing.T) {
	t.Parallel()

	log := headerLine +
		"GROUP\touter,inner\t1788379356180\t1788379357700\t1520\tOK\n" +
		"GROUP\tzzz,yyy\t1788379357800\t1788379357900\t100\tOK\n"

	r, err := text.NewReader(strings.NewReader(log))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	first, err := r.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}

	mine := append(first.Groups, "mine") //nolint:gocritic // appending is the point: it must not alias
	want := strings.Join(mine, "/")

	if _, err := r.Next(); err != nil {
		t.Fatalf("Next: %v", err)
	}

	if got := strings.Join(mine, "/"); got != want {
		t.Fatalf("the next record rewrote the caller's slice: %q became %q", want, got)
	}
}

// TestReaderAssertionPreambleIsBounded holds the preamble to the same rule as
// the rest of the reader. Assertions are held whole because a simulation
// declares a handful of them — a property of the simulation, which a damaged
// file is not bound by.
func TestReaderAssertionPreambleIsBounded(t *testing.T) {
	t.Parallel()

	// A preamble that never reaches a header, streamed rather than built, so
	// the test does not itself hold what the reader must refuse to hold.
	line := "ASSERTION\t" + strings.Repeat("p", 64<<10) + "\n"

	readers := make([]io.Reader, 512)
	for i := range readers {
		readers[i] = strings.NewReader(line)
	}

	_, err := text.NewReader(io.MultiReader(readers...))

	var syntaxErr *gatling.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("got %v, want a *gatling.SyntaxError once the preamble passed its bound", err)
	}

	if !strings.Contains(syntaxErr.Error(), "run header") {
		t.Fatalf("the error does not say what was missing: %v", syntaxErr)
	}
}

// TestReaderSurplusAssertionNamesTheTrueCount pins the diagnostic. The surplus
// is found before the header names the version and ruled on afterwards, so the
// count has to travel with the line number rather than be assumed.
func TestReaderSurplusAssertionNamesTheTrueCount(t *testing.T) {
	t.Parallel()

	_, err := text.NewReader(strings.NewReader("ASSERTION\tpayload\ta\tb\n" + headerLine))

	var syntaxErr *gatling.SyntaxError
	if !errors.As(err, &syntaxErr) || syntaxErr.Line != 1 {
		t.Fatalf("got %v, want a *gatling.SyntaxError at line 1", err)
	}

	if !strings.Contains(syntaxErr.Found, "4 fields") {
		t.Fatalf("error found %q, want it to name the 4 fields the line has", syntaxErr.Found)
	}
}

// TestReaderErrorQuotesBoundedly keeps the reader's memory bounded on the one
// path where it reports that the input is not: a line runs to MaxLineLen, and
// quoting escapes, so quoting one whole would put megabytes in the error.
func TestReaderErrorQuotesBoundedly(t *testing.T) {
	t.Parallel()

	r, err := text.NewReader(strings.NewReader(headerLine + strings.Repeat("x", 128<<10) + "\n"))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	_, err = r.Next()

	var syntaxErr *gatling.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("got %v, want *gatling.SyntaxError", err)
	}

	if len(syntaxErr.Found) > 256 {
		t.Fatalf("the error carries %d bytes of the line; it must carry a head of it", len(syntaxErr.Found))
	}

	if !strings.Contains(syntaxErr.Found, "131072 bytes") {
		t.Fatalf("a shortened value must say how long it was: %q", syntaxErr.Found)
	}
}

// aboveRangeHeader names a release no recording covers, so the reader decodes
// leniently and hands back a warning.
const aboveRangeHeader = "RUN\tio.galaxio.parsec.corpus.CorpusSimulation\tcorpussimulation\t1788379354534\t \t3.13.0\n"

// TestReaderErrorRecordAboveRange holds the error record to the same forward
// compatibility as every other kind. Its message may span separators (FR-008b),
// so the field count cannot say where the timestamp is; taking simply the last
// field would make a newer version's appended field end the whole read, which
// is the opposite of what the version gate promised when it warned instead of
// refusing (FR-008a).
func TestReaderErrorRecordAboveRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		line    string
		message string
	}{
		{"a field appended after the timestamp", "ERROR\tboom\t1788379357701\tnewfield\n", "boom"},
		{"a message holding a separator", "ERROR\tfailed\tafter 42\t1788379357701\n", "failed\tafter 42"},
		{"both at once", "ERROR\tcode\t500\t1788379357701\tnewfield\n", "code\t500"},
		{"neither", "ERROR\tplain\t1788379357701\n", "plain"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r, err := text.NewReader(strings.NewReader(aboveRangeHeader + tc.line))
			if err != nil {
				t.Fatalf("NewReader: %v", err)
			}

			if len(r.Warnings()) != 1 {
				t.Fatalf("Warnings() = %v, want the one raised for a version above the range", r.Warnings())
			}

			rec, err := r.Next()
			if err != nil {
				t.Fatalf("Next: %v", err)
			}

			if rec.Kind != gatling.KindError || rec.Message != tc.message || rec.Timestamp != 1788379357701 {
				t.Fatalf("got %+v, want an ERROR at 1788379357701 with message %q", rec, tc.message)
			}
		})
	}
}

// TestReaderErrorRecordAboveRangeWithoutATimestamp is the floor of the search:
// when no field reads as a timestamp the record is damaged whatever the
// version, and the error must name the field the format puts it in rather than
// whichever one the walk stopped at.
func TestReaderErrorRecordAboveRangeWithoutATimestamp(t *testing.T) {
	t.Parallel()

	r, err := text.NewReader(strings.NewReader(aboveRangeHeader + "ERROR\tboom\tnope\n"))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	_, err = r.Next()

	var syntaxErr *gatling.SyntaxError
	if !errors.As(err, &syntaxErr) || syntaxErr.Line != 2 {
		t.Fatalf("got %v, want a *gatling.SyntaxError at line 2", err)
	}

	if !strings.Contains(syntaxErr.Found, "nope") {
		t.Fatalf("error found %q, want it to name the last field", syntaxErr.Found)
	}
}

// TestReaderErrorRecordInsideRangeIsExact is the other half: inside the covered
// range the format is known, so a field behind the timestamp is damage and ends
// the read rather than being ignored.
func TestReaderErrorRecordInsideRangeIsExact(t *testing.T) {
	t.Parallel()

	r, err := text.NewReader(strings.NewReader(headerLine + "ERROR\tboom\t1788379357701\tnewfield\n"))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	_, err = r.Next()

	var syntaxErr *gatling.SyntaxError
	if !errors.As(err, &syntaxErr) || syntaxErr.Line != 2 {
		t.Fatalf("got %v, want a *gatling.SyntaxError at line 2", err)
	}
}
