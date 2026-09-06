package binary_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
)

// Every length in this format is read straight from the file, so a corrupt byte
// can ask for gigabytes. The cap has to be checked before the allocator is, and
// the input here holds four bytes while claiming a gigabyte.
func TestALengthPastTheEndIsRefusedWithoutReservingIt(t *testing.T) {
	t.Parallel()

	// A run record whose version string claims 0x3fffffff bytes.
	log := []byte{0x00, 0x3f, 0xff, 0xff, 0xff, 'x'}

	_, err := binary.NewReader(bytes.NewReader(log))

	var se *gatling.SyntaxError
	if !errors.As(err, &se) {
		t.Fatalf("NewReader = _, %v; want a *gatling.SyntaxError", err)
	}

	if se.Offset != 1 {
		t.Errorf("the error names byte %d; the length prefix is at 1", se.Offset)
	}
}

// A truncation must fail naming its offset rather than returning a short
// record — except where the cut lands exactly on a record boundary, which the
// format gives no way to detect.
//
// There is no end marker and no record count. A log cut between two records is
// byte-for-byte a shorter valid log, and this reader says so by reading it
// cleanly. That is a property of the format rather than a gap in the decoder,
// and it is asserted here rather than left for someone to discover: a caller who
// needs to know a log is complete must learn it from somewhere other than the
// log.
func TestTruncationFailsNamingTheOffsetOrLandsOnABoundary(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join(corpusDirs(t)[0], "simulation.log"))
	if err != nil {
		t.Fatal(err)
	}

	full := len(records(t, bytes.NewReader(raw)))

	var refused, boundaries int

	// Every byte would be slow and prove nothing more; a stride lands inside
	// lengths, string bodies, coder bytes, offsets and kind bytes alike.
	for cut := 1; cut < len(raw); cut += 7 {
		err := readAll(t, raw[:cut])
		if err == nil {
			// A clean read of a truncated log is only acceptable because it is
			// a shorter valid log. If it yielded everything, nothing was cut.
			if got := len(records(t, bytes.NewReader(raw[:cut]))); got >= full {
				t.Fatalf("a log cut at byte %d of %d yielded all %d records", cut, len(raw), got)
			}

			boundaries++

			continue
		}

		var se *gatling.SyntaxError
		if !errors.As(err, &se) {
			t.Fatalf("a log cut at byte %d failed with %T: %v", cut, err, err)
		}

		if se.Offset < 0 || se.Offset > int64(cut) {
			t.Fatalf("a log cut at byte %d failed naming byte %d, which is not in the file",
				cut, se.Offset)
		}

		refused++
	}

	if refused == 0 {
		t.Fatal("no truncation was refused, so nothing here tested a refusal")
	}

	t.Logf("%d truncations refused naming an offset, %d landed on a record boundary "+
		"and read as a shorter log", refused, boundaries)
}

// readAll reads a log to its end, returning nil for a clean one and the failure
// otherwise.
func readAll(t *testing.T, raw []byte) error {
	t.Helper()

	rd, err := binary.NewReader(bytes.NewReader(raw))
	if err != nil {
		return err
	}

	for {
		_, err := rd.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return err
		}
	}
}

// An assertion payload whose declared length runs past the end of the file is
// refused rather than skipped: a payload this module cannot carry whole is one a
// consumer must not be handed a truncated version of.
func TestAnAssertionPayloadPastTheEndIsRefused(t *testing.T) {
	t.Parallel()

	w := &builder{}
	w.u8(0).str("3.15.1").str("io.example.Sim").i64(runStart).str("").i32(0).i32(1).i32(1 << 20)

	_, err := binary.NewReader(bytes.NewReader(w.bytes()))
	if !errors.As(err, new(*gatling.SyntaxError)) {
		t.Fatalf("NewReader = _, %v; want a *gatling.SyntaxError", err)
	}
}

// A scenario index a user record names must exist. Accepting one that does not
// would mean either a panic or an invented scenario name, and the format gives
// no third option.
func TestAScenarioIndexOutsideTheRunIsRefused(t *testing.T) {
	t.Parallel()

	log := (&builder{}).
		runRecord("3.15.1", []string{"only one"}, nil).
		u8(2).i32(5).u8(1).i32(10).
		bytes()

	rd, err := binary.NewReader(bytes.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := rd.Next(); !errors.As(err, new(*gatling.SyntaxError)) {
		t.Fatalf("Next = _, %v; want a *gatling.SyntaxError", err)
	}
}

// A group depth read from the file sizes a loop, so it is capped like every
// other length: a corrupt count must not spin or allocate.
func TestAnAbsurdGroupDepthIsRefused(t *testing.T) {
	t.Parallel()

	log := (&builder{}).
		runRecord("3.15.1", []string{"scenario"}, nil).
		u8(1).i32(1 << 20).
		bytes()

	rd, err := binary.NewReader(bytes.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := rd.Next(); !errors.As(err, new(*gatling.SyntaxError)) {
		t.Fatalf("Next = _, %v; want a *gatling.SyntaxError", err)
	}
}

// A request whose group path refers to a cache entry that was never introduced
// must fail here rather than rename every record after it, which is the one
// corruption this format turns into silent, widespread wrongness.
func TestADanglingGroupReferenceEndsTheRead(t *testing.T) {
	t.Parallel()

	log := (&builder{}).
		runRecord("3.15.1", []string{"scenario"}, nil).
		u8(1).i32(1).ref(9).
		bytes()

	rd, err := binary.NewReader(bytes.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}

	_, err = rd.Next()

	var se *gatling.SyntaxError
	if !errors.As(err, &se) {
		t.Fatalf("Next = _, %v; want a *gatling.SyntaxError", err)
	}

	if !bytes.Contains([]byte(se.Found), []byte("cache entry")) {
		t.Errorf("the error says found %q; it must name the reference that could not be resolved", se.Found)
	}
}
