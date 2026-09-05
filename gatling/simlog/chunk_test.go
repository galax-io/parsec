package simlog_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/galax-io/parsec/gatling/simlog"
)

// oneByteReader hands back a single byte per call. It is the shape that makes a
// swallowed head visible: identification cannot get its window in one read, so
// any assumption that it can shows up here rather than in production.
type oneByteReader struct{ r io.Reader }

func (o oneByteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	return o.r.Read(p[:1])
}

// chunkReader hands back at most n bytes per call.
type chunkReader struct {
	r io.Reader
	n int
}

func (c chunkReader) Read(p []byte) (int, error) {
	if len(p) > c.n {
		p = p[:c.n]
	}

	return c.r.Read(p)
}

// The bytes identification looked at must still reach the codec. This is a
// correctness requirement rather than a convenience: the binary codec arriving
// in v0.0.5 rebuilds a string cache from the first byte of the file, and would
// be silently wrong if one were missing.
func TestChunkedReadsMatchWholeFile(t *testing.T) {
	t.Parallel()

	for _, log := range corpusLogs(t) {
		t.Run(filepath.Base(filepath.Dir(log)), func(t *testing.T) {
			t.Parallel()

			whole, err := os.ReadFile(log) //nolint:gosec // a corpus path from the test's own glob
			if err != nil {
				t.Fatalf("reading %s: %v", log, err)
			}

			base, err := simlog.NewReader(bytes.NewReader(whole))
			if err != nil {
				t.Fatalf("whole-file NewReader: %v", err)
			}

			want := records(t, base)
			if len(want) == 0 {
				t.Fatal("the whole-file read yielded no records, so the comparison proves nothing")
			}

			readers := map[string]func() io.Reader{
				"one byte at a time": func() io.Reader { return oneByteReader{bytes.NewReader(whole)} },
				"three at a time":    func() io.Reader { return chunkReader{bytes.NewReader(whole), 3} },
				"nine at a time":     func() io.Reader { return chunkReader{bytes.NewReader(whole), 9} },
				"ten at a time":      func() io.Reader { return chunkReader{bytes.NewReader(whole), 10} },
				"eleven at a time":   func() io.Reader { return chunkReader{bytes.NewReader(whole), 11} },
			}

			for name, make := range readers {
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					rd, err := simlog.NewReader(make())
					if err != nil {
						t.Fatalf("chunked NewReader: %v", err)
					}

					got := records(t, rd)
					if len(got) != len(want) {
						t.Fatalf("got %d records, whole-file yields %d", len(got), len(want))
					}

					for i := range got {
						if !reflect.DeepEqual(got[i], want[i]) {
							t.Fatalf("record %d differs from the whole-file read:\n chunked %+v\n whole   %+v",
								i, got[i], want[i])
						}
					}
				})
			}
		})
	}
}

// The head is replayed, not consumed: the very first record of the log must
// still be there. A reader that swallowed its window would silently lose the
// run header and every assertion ahead of it.
func TestHeadReachesTheCodec(t *testing.T) {
	t.Parallel()

	for _, log := range corpusLogs(t) {
		t.Run(filepath.Base(filepath.Dir(log)), func(t *testing.T) {
			t.Parallel()

			whole, err := os.ReadFile(log) //nolint:gosec // a corpus path from the test's own glob
			if err != nil {
				t.Fatalf("reading %s: %v", log, err)
			}

			rd, err := simlog.NewReader(oneByteReader{bytes.NewReader(whole)})
			if err != nil {
				t.Fatalf("NewReader: %v", err)
			}

			// The corpus opens with ASSERTION records, which sit ahead of the
			// header and are exactly what a swallowed window would destroy.
			if got := rd.Assertions(); len(got) == 0 {
				t.Fatal("no assertions: the bytes identification examined did not reach the codec")
			}

			// Compared against the version the corpus directory is named for.
			// Version.String() renders the zero value as "0.0.0" and can never
			// be empty, so a != "" check would survive a header that did not
			// survive identification.
			want := filepath.Base(filepath.Dir(log))
			if got := rd.Header().Version.String(); got != want {
				t.Fatalf("Header().Version = %s; the recording is %s — the run header did not survive identification", got, want)
			}
		})
	}
}

// A log that fails must fail the same way however it was fed in.
func TestChunkedFailuresMatch(t *testing.T) {
	t.Parallel()

	damaged := []byte("RUN\tio.example.Sim\tsim\t1788379664977\t \t3.11.5\nUSER\tonly\ttwo\n")

	whole, wholeErr := drain(t, bytes.NewReader(damaged))
	chunked, chunkedErr := drain(t, oneByteReader{bytes.NewReader(damaged)})

	if wholeErr == nil || chunkedErr == nil {
		t.Fatalf("want both reads to fail; whole = %v, chunked = %v", wholeErr, chunkedErr)
	}

	if wholeErr.Error() != chunkedErr.Error() {
		t.Fatalf("failures differ:\n whole   %v\n chunked %v", wholeErr, chunkedErr)
	}

	if whole != chunked {
		t.Fatalf("got %d records before failing whole-file and %d chunked", whole, chunked)
	}
}

// drain counts the records a read yields before it ends, and returns the error
// that ended it.
func drain(t *testing.T, r io.Reader) (int, error) {
	t.Helper()

	rd, err := simlog.NewReader(r)
	if err != nil {
		return 0, err
	}

	n := 0

	for {
		if _, err := rd.Next(); err != nil {
			if errors.Is(err, io.EOF) {
				return n, nil
			}

			return n, err
		}

		n++
	}
}
