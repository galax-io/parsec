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

// chunked hands the same bytes over in pieces of a fixed size, so a decoder that
// assumed a read returns everything it asked for fails here.
type chunked struct {
	b    []byte
	size int
}

func (c *chunked) Read(p []byte) (int, error) {
	if len(c.b) == 0 {
		return 0, io.EOF
	}

	n := min(min(len(p), c.size), len(c.b))
	copy(p, c.b[:n])
	c.b = c.b[n:]

	return n, nil
}

// A log split at arbitrary byte boundaries must decode to exactly what the whole
// file decodes to. One byte at a time is the worst case and the one a network
// stream actually produces.
func TestChunkedReadsMatchWholeFile(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(filepath.Join(dir, "simulation.log")) //nolint:gosec // a corpus path
			if err != nil {
				t.Fatal(err)
			}

			want := records(t, bytes.NewReader(raw))

			for _, size := range []int{1, 2, 3, 7, 64, 4096} {
				got := records(t, &chunked{b: raw, size: size})

				if len(got) != len(want) {
					t.Fatalf("%d-byte chunks yield %d records; the whole file yields %d",
						size, len(got), len(want))
				}

				for i := range got {
					if !reflect.DeepEqual(got[i], want[i]) {
						t.Fatalf("%d-byte chunks differ at record %d:\n got  %+v\n want %+v",
							size, i, got[i], want[i])
					}
				}
			}
		})
	}
}

// A log that fails must fail identically however it arrives: the same offset,
// the same message. A decoder whose error position depended on how the bytes
// were delivered would send a caller to the wrong byte of their file.
func TestATruncatedLogFailsAtTheSameOffsetHoweverItArrives(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join(corpusDirs(t)[0], "simulation.log"))
	if err != nil {
		t.Fatal(err)
	}

	cut := raw[:len(raw)-7]

	whole := drain(t, bytes.NewReader(cut))

	for _, size := range []int{1, 3, 64} {
		got := drain(t, &chunked{b: cut, size: size})

		if got.records != whole.records {
			t.Fatalf("%d-byte chunks delivered %d records before failing; the whole file delivered %d",
				size, got.records, whole.records)
		}

		if got.err.Error() != whole.err.Error() {
			t.Fatalf("%d-byte chunks failed with %v; the whole file failed with %v", size, got.err, whole.err)
		}
	}
}

type outcome struct {
	records int
	err     error
}

// drain reads a log to its end or its failure, and says which.
func drain(t *testing.T, r io.Reader) outcome {
	t.Helper()

	rd, err := binary.NewReader(r)
	if err != nil {
		return outcome{err: err}
	}

	var out outcome

	for {
		_, err := rd.Next()
		if err != nil {
			out.err = err

			if errors.Is(err, io.EOF) {
				t.Fatal("the log ended cleanly; this test needs one that does not")
			}

			return out
		}

		out.records++
	}
}

// Once a read has failed there is no next record, and every later call must say
// the same thing. A reader that returned io.EOF after a failure would let a
// caller mistake a broken log for a complete one.
func TestAFailedReadStaysFailed(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join(corpusDirs(t)[0], "simulation.log"))
	if err != nil {
		t.Fatal(err)
	}

	rd, err := binary.NewReader(bytes.NewReader(raw[:len(raw)-7]))
	if err != nil {
		t.Fatal(err)
	}

	var first error

	for first == nil {
		_, first = rd.Next()
	}

	if errors.Is(first, io.EOF) {
		t.Fatal("a truncated log ended cleanly")
	}

	for range 3 {
		_, again := rd.Next()
		if again != first { //nolint:errorlint // identity is the point: the same error, not an equal one
			t.Fatalf("a later Next returned %v; want the same error, %v", again, first)
		}
	}
}

// A clean end is io.EOF and nothing else, and it too must repeat.
func TestACleanEndStaysClean(t *testing.T) {
	t.Parallel()

	rd, err := binary.NewReader(bytes.NewReader(minimal("3.15.1")))
	if err != nil {
		t.Fatal(err)
	}

	for range 3 {
		rec, err := rd.Next()
		if !errors.Is(err, io.EOF) {
			t.Fatalf("Next = %+v, %v; want io.EOF", rec, err)
		}
	}
}

// A record kind the recorded versions never write ends the read, naming the byte
// it was at. Continuing past one is not possible: the format gives no length for
// a record it does not describe, so there is no way to find the next.
func TestAnUnknownRecordKindEndsTheRead(t *testing.T) {
	t.Parallel()

	log := (&builder{}).runRecord("3.15.1", []string{"scenario"}, nil).u8(0x07).bytes()

	rd, err := binary.NewReader(bytes.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}

	_, err = rd.Next()

	var se *gatling.SyntaxError
	if !errors.As(err, &se) {
		t.Fatalf("Next = _, %v; want a *gatling.SyntaxError", err)
	}

	if se.Format != gatling.FormatBinary {
		t.Errorf("the error names format %v", se.Format)
	}
}

// A second run record means the stream desynchronised: the run record occurs
// once and carries the start every later record is resolved against.
func TestASecondRunRecordIsRefused(t *testing.T) {
	t.Parallel()

	log := (&builder{}).
		runRecord("3.15.1", []string{"scenario"}, nil).
		runRecord("3.15.1", []string{"scenario"}, nil).
		bytes()

	rd, err := binary.NewReader(bytes.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := rd.Next(); !errors.As(err, new(*gatling.SyntaxError)) {
		t.Fatalf("Next = _, %v; want a *gatling.SyntaxError", err)
	}
}
