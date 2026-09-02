package text_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/iotest"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/text"
)

// outcome is everything a read produces, so two reads can be compared whole.
type outcome struct {
	header     gatling.Header
	assertions []string
	warnings   []gatling.Warning
	records    []gatling.Record
	err        string // the final error's text, "" for a clean io.EOF
	openErr    string // NewReader's error text, "" when it succeeded
}

func readOutcome(r io.Reader) outcome {
	rd, err := text.NewReader(r)
	if err != nil {
		return outcome{openErr: err.Error()}
	}

	o := outcome{header: rd.Header(), assertions: rd.Assertions(), warnings: rd.Warnings()}

	for {
		rec, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return o
		}

		if err != nil {
			o.err = err.Error()

			return o
		}

		rec.Groups = append([]string(nil), rec.Groups...)
		o.records = append(o.records, rec)
	}
}

// randomChunkReader returns between 1 and 4096 bytes per Read, seeded so a
// failure reproduces.
type randomChunkReader struct {
	data []byte
	rng  *rand.Rand
}

func (r *randomChunkReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}

	n := min(len(p), len(r.data), 1+r.rng.IntN(4096))
	copy(p, r.data[:n])
	r.data = r.data[n:]

	return n, nil
}

// checkChunked asserts that a log decodes identically however the bytes arrive.
func checkChunked(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path) //nolint:gosec // a fixture or corpus path, both test-owned
	if err != nil {
		t.Fatal(err)
	}

	whole := readOutcome(bytes.NewReader(data))

	readers := map[string]io.Reader{
		"one byte at a time": iotest.OneByteReader(bytes.NewReader(data)),
		"half reads":         iotest.HalfReader(bytes.NewReader(data)),
		"random chunks":      &randomChunkReader{data: data, rng: rand.New(rand.NewPCG(7, 11))}, //nolint:gosec // seeded so a failure reproduces; not a security use
		"data-err reader":    iotest.DataErrReader(bytes.NewReader(data)),
	}

	for name, r := range readers {
		if got := readOutcome(r); !reflect.DeepEqual(got, whole) {
			t.Errorf("%s: %s differs from the whole-file read:\n got  %s\n want %s", filepath.Base(path), name, describe(got), describe(whole))
		}
	}
}

func describe(o outcome) string {
	return fmt.Sprintf("open=%q records=%d err=%q assertions=%d warnings=%d", o.openErr, len(o.records), o.err, len(o.assertions), len(o.warnings))
}

func TestChunkedFixtures(t *testing.T) {
	t.Parallel()

	fixtures, err := filepath.Glob(filepath.Join("testdata", "fixtures", "*.fixture.log"))
	if err != nil || len(fixtures) == 0 {
		t.Fatalf("no fixtures found: %v", err)
	}

	for _, f := range fixtures {
		t.Run(filepath.Base(f), func(t *testing.T) {
			t.Parallel()
			checkChunked(t, f)
		})
	}
}
