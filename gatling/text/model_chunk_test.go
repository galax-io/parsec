package text_test

import (
	"bytes"
	"errors"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"testing/iotest"

	"github.com/galax-io/parsec/gatling/text"
	"github.com/galax-io/parsec/model"
)

// modelOutcome is everything a conversion produces, so two reads of the same
// bytes can be compared whole.
type modelOutcome struct {
	run     model.Run
	items   []model.Item
	err     string // the final error's text, "" for a clean io.EOF
	openErr string // NewRunReader's error text, "" when it succeeded
}

func readModelOutcome(r io.Reader) modelOutcome {
	rd, err := text.NewRunReader(r)
	if err != nil {
		return modelOutcome{openErr: err.Error()}
	}

	o := modelOutcome{run: rd.Run()}

	for {
		it, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return o
		}

		if err != nil {
			o.err = err.Error()

			return o
		}

		it.Sample.Groups = slices.Clone(it.Sample.Groups)
		it.Group.Groups = slices.Clone(it.Group.Groups)
		o.items = append(o.items, it)
	}
}

// checkModelChunked reads the same bytes whole, one byte at a time, and in
// random-sized pieces, and requires all three to agree. A conversion that
// buffered, or that depended on where a Read happened to land, would differ.
func checkModelChunked(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path) //nolint:gosec // a corpus path from the test's own glob
	if err != nil {
		t.Fatal(err)
	}

	want := readModelOutcome(bytes.NewReader(data))

	t.Run("one byte at a time", func(t *testing.T) {
		t.Parallel()

		if got := readModelOutcome(iotest.OneByteReader(bytes.NewReader(data))); !reflect.DeepEqual(got, want) {
			t.Error("a one-byte-at-a-time read differs from a whole-file read")
		}
	})

	t.Run("random chunks", func(t *testing.T) {
		t.Parallel()

		for seed := range uint64(16) {
			src := &randomChunkReader{data: data, rng: rand.New(rand.NewPCG(seed, 0))} //nolint:gosec // test input sizing, not security

			if got := readModelOutcome(src); !reflect.DeepEqual(got, want) {
				t.Fatalf("seed %d: a chunked read differs from a whole-file read", seed)
			}
		}
	})
}

func TestModelChunkedFixtures(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(filepath.Join("testdata", "fixtures", "*.fixture.log"))
	if err != nil {
		t.Fatal(err)
	}

	if len(paths) == 0 {
		t.Fatal("no fixtures found")
	}

	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			t.Parallel()
			checkModelChunked(t, p)
		})
	}
}

// A log that fails must fail identically however it arrives: the same error, at
// the same point, with the same items before it.
func TestModelChunkedFailuresAgree(t *testing.T) {
	t.Parallel()

	log := modelPreamble +
		"REQUEST\t\tGET /ok\t1788379356162\t1788379356173\tOK\t \n" +
		"NONSENSE\twhat\n"

	want := readModelOutcome(bytes.NewReader([]byte(log)))
	if want.err == "" {
		t.Fatal("the damaged log read cleanly")
	}

	got := readModelOutcome(iotest.OneByteReader(bytes.NewReader([]byte(log))))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("a chunked read of a damaged log gave %q after %d items, want %q after %d",
			got.err, len(got.items), want.err, len(want.items))
	}
}
