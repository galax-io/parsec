package binary_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
)

// A cache index of exactly math.MinInt32 negates to itself inside an int32 and
// stays negative, so a bounds check that only rejects large values let it reach
// an index. Four bytes took the caller's process down.
//
// Single-byte mutation cannot reach this value and the fuzzer needs a minute of
// luck, so it is pinned here explicitly.
func TestTheMostNegativeCacheIndexIsRefused(t *testing.T) {
	t.Parallel()

	log := (&builder{}).runRecord("3.15.1", []string{"s"}, nil).
		u8(1).i32(1).i32(math.MinInt32).bytes()

	rd, err := binary.NewReader(bytes.NewReader(log))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	if _, err := rd.Next(); !errors.As(err, new(*gatling.SyntaxError)) {
		t.Fatalf("Next = _, %v; want a *gatling.SyntaxError", err)
	}
}

// A group path is handed out with no spare capacity, so a caller appending to it
// cannot write into the reader's own array. gatling/text seals it for the same
// reason and has its own test; without this the two codecs differ in a way that
// silently reattributes samples to the wrong group.
func TestAppendingToAGroupPathCannotReachTheReader(t *testing.T) {
	t.Parallel()

	log := (&builder{}).runRecord("3.15.1", []string{"s"}, nil).
		u8(1).i32(2).newString("outer").newString("inner").newString("GET /a").
		i32(1).i32(2).u8(1).newString("").
		u8(1).i32(2).ref(1).ref(2).ref(3).i32(3).i32(4).u8(1).ref(4).
		bytes()

	rd, err := binary.NewReader(bytes.NewReader(log))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	first, err := rd.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}

	mine := append(first.Groups, "mine") //nolint:gocritic // appending is the point

	if _, err := rd.Next(); err != nil {
		t.Fatalf("Next: %v", err)
	}

	if got := fmt.Sprint(mine); got != "[outer inner mine]" {
		t.Fatalf("the caller's slice reads %s; the reader wrote through it", got)
	}
}

// Groups is empty and non-nil for every record without groups, whatever was read
// before it. model.Run holds the same nil/empty parity rule for Warnings, and a
// consumer marshalling the two differently for the same run is the cost.
func TestGroupsAreEmptyNotNilWhateverCameBefore(t *testing.T) {
	t.Parallel()

	log := (&builder{}).runRecord("3.15.1", []string{"s"}, nil).
		request("GET /a", true).
		u8(1).i32(1).newString("outer").ref(1).i32(1).i32(2).u8(1).ref(2).
		request("GET /b", true).
		bytes()

	for i, rec := range records(t, bytes.NewReader(log)) {
		if len(rec.Groups) == 0 && rec.Groups == nil {
			t.Fatalf("record %d has a nil Groups; every record hands out an empty, non-nil slice", i)
		}
	}
}

// Run hands out the caller's own slices, as gatling/text does. Two consumers of
// one reader must not be able to corrupt each other.
func TestRunIsTheCallersOwn(t *testing.T) {
	t.Parallel()

	rd, err := binary.NewRunReader(bytes.NewReader(minimal("3.16.0")))
	if err != nil {
		t.Fatalf("NewRunReader: %v", err)
	}

	first := rd.Run()
	if len(first.Warnings) != 1 {
		t.Fatalf("%d warnings; want the one an above-range version raises", len(first.Warnings))
	}

	first.Warnings[0].Reason = "CLOBBERED"

	if rd.Run().Warnings[0].Reason == "CLOBBERED" {
		t.Fatal("Run hands out the reader's own slice: one caller can rewrite another's run")
	}
}

// An error that merely wraps io.EOF is a broken stream, not the end of the log.
// Reading it as a clean end would report a partial run as complete.
func TestWrappedEOFIsNotACleanEnd(t *testing.T) {
	t.Parallel()

	broken := errors.New("transport closed")

	rd, err := binary.NewReader(&failAfter{
		b:   minimal("3.15.1"),
		err: fmt.Errorf("%w: %w", broken, io.EOF),
	})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	_, err = rd.Next()
	if errors.Is(err, io.EOF) && !errors.Is(err, broken) {
		t.Fatal("a wrapped io.EOF was read as the clean end of the log")
	}

	if !errors.Is(err, broken) {
		t.Fatalf("Next = %v; want the source's own failure", err)
	}
}

// A source that fails mid-record is not a truncated log. Telling an operator
// their simulation.log is short sends them to re-record a run that was fine.
func TestASourceFailureIsNotReportedAsTruncation(t *testing.T) {
	t.Parallel()

	broken := errors.New("connection reset by peer")

	log := (&builder{}).runRecord("3.15.1", []string{"s"}, nil).
		u8(1).i32(0).newString("GET /a").bytes()

	rd, err := binary.NewReader(&failAfter{b: log[:len(log)-2], err: broken})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	if _, err := rd.Next(); !errors.Is(err, broken) {
		t.Fatalf("Next = %v; the cause must survive so a caller can tell a broken "+
			"stream from a short file", err)
	}
}

// A run start that would make a timestamp wrap is refused once, at the header,
// rather than producing plausible instants in the distant past for every record.
func TestARunStartThatCannotCarryAnOffsetIsRefused(t *testing.T) {
	t.Parallel()

	for _, start := range []int64{-1, math.MaxInt64, math.MinInt64} {
		w := &builder{}
		w.u8(0).str("3.15.1").str("io.example.Sim").i64(start).str("").i32(0).i32(0)

		if _, err := binary.NewReader(bytes.NewReader(w.bytes())); !errors.As(err, new(*gatling.SyntaxError)) {
			t.Errorf("a run start of %d was accepted: %v", start, err)
		}
	}
}

// An offset that cannot be resolved is reported absent, not refused: FR-009 asks
// for absence, and one such field must not end a ten-million-record read.
func TestAnUnresolvableOffsetIsAbsentNotFatal(t *testing.T) {
	t.Parallel()

	log := (&builder{}).runRecord("3.15.1", []string{"s"}, nil).
		u8(2).i32(0).u8(1).i32(-5).
		u8(2).i32(0).u8(0).i32(10).
		bytes()

	recs := records(t, bytes.NewReader(log))
	if len(recs) != 2 {
		t.Fatalf("%d records; a single unresolvable offset must not end the read", len(recs))
	}

	if recs[0].Timestamp != math.MinInt64 {
		t.Errorf("Timestamp = %d; want the absent sentinel", recs[0].Timestamp)
	}
}

// An unpaired UTF-16 surrogate is refused rather than replaced. A name that
// decodes to a different name regroups a report and never says so.
func TestAnUnpairedSurrogateIsRefused(t *testing.T) {
	t.Parallel()

	w := &builder{}
	w.u8(0).str("3.15.1").str("io.example.Sim").i64(runStart)
	// A lone high surrogate, little-endian, as the run description.
	w.i32(2)
	w.b = append(w.b, 0x3d, 0xd8, 0x01)

	_, err := binary.NewReader(bytes.NewReader(w.bytes()))

	var se *gatling.SyntaxError
	if !errors.As(err, &se) {
		t.Fatalf("NewReader = _, %v; want a *gatling.SyntaxError", err)
	}
}

// failAfter serves its bytes and then fails, rather than ending.
type failAfter struct {
	b   []byte
	err error
}

func (f *failAfter) Read(p []byte) (int, error) {
	if len(f.b) == 0 {
		return 0, f.err
	}

	n := copy(p, f.b)
	f.b = f.b[n:]

	return n, nil
}
