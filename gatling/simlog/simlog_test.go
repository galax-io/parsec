package simlog_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
	"github.com/galax-io/parsec/gatling/simlog"
	"github.com/galax-io/parsec/gatling/text"
	"github.com/galax-io/parsec/model"
)

func repoPath(parts ...string) string {
	return filepath.Join(append([]string{"..", ".."}, parts...)...)
}

func corpusLogs(t *testing.T) []string {
	t.Helper()

	logs, err := filepath.Glob(repoPath("testdata", "corpus", "gatling", "*", "simulation.log"))
	if err != nil {
		t.Fatalf("globbing the corpus: %v", err)
	}

	if len(logs) == 0 {
		t.Fatal("no corpus log found: the recorded runs are committed, so this is a broken checkout")
	}

	return logs
}

// binaryLog is a complete recorded binary run. The 64-byte sample this package
// used to open lived only to prove that a binary log is identified as binary;
// now that a codec reads one, a recording says the same thing and more.
func binaryLog() string {
	return repoPath("testdata", "corpus", "gatling", "3.15.1", "simulation.log")
}

func open(t *testing.T, path string) *os.File {
	t.Helper()

	f, err := os.Open(path) //nolint:gosec // a corpus or sample path this test spells out
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}

	t.Cleanup(func() { _ = f.Close() })

	return f
}

// records drains a reader. Groups is backed by a slice the reader reuses, so it
// is cloned before the record is kept — the contract says so, and comparing
// aliased slices would compare the same memory twice.
func records(t *testing.T, rd interface {
	Next() (gatling.Record, error)
},
) []gatling.Record {
	t.Helper()

	var out []gatling.Record

	for {
		rec, err := rd.Next()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				t.Fatalf("Next() = _, %v", err)
			}

			return out
		}

		rec.Groups = slices.Clone(rec.Groups)
		out = append(out, rec)
	}
}

func items(t *testing.T, rd interface {
	Next() (model.Item, error)
},
) []model.Item {
	t.Helper()

	var out []model.Item

	for {
		it, err := rd.Next()
		if err != nil {
			if !errors.Is(err, io.EOF) {
				t.Fatalf("Next() = _, %v", err)
			}

			return out
		}

		it.Sample.Groups = slices.Clone(it.Sample.Groups)
		it.Group.Groups = slices.Clone(it.Group.Groups)
		out = append(out, it)
	}
}

// codecFor opens a corpus log through the codec that actually wrote it, so a
// dispatch comparison holds for every entry rather than only the text ones.
// The format is asked of the file, not assumed from the version directory.
func codecFor(t *testing.T, log string) (simlog.RecordReader, error) {
	t.Helper()

	if formatOf(t, log) == gatling.FormatBinary {
		return binary.NewReader(open(t, log))
	}

	return text.NewReader(open(t, log))
}

func runCodecFor(t *testing.T, log string) (simlog.RunReader, error) {
	t.Helper()

	if formatOf(t, log) == gatling.FormatBinary {
		return binary.NewRunReader(open(t, log))
	}

	return text.NewRunReader(open(t, log))
}

func formatOf(t *testing.T, log string) gatling.Format {
	t.Helper()

	f := open(t, log)

	buf := make([]byte, gatling.DetectSize)

	n, err := f.Read(buf)
	if n == 0 && err != nil {
		t.Fatalf("reading %s: %v", log, err)
	}

	got, err := gatling.Detect(buf[:n])
	if err != nil {
		t.Fatalf("detecting %s: %v", log, err)
	}

	return got
}

// Dispatch adds identification and forwarding, and nothing else. A log read
// through this package must decode to exactly what its own codec decodes it to,
// or the convenience has changed the meaning of the data.
func TestDispatchMatchesTheCodecRecords(t *testing.T) {
	t.Parallel()

	for _, log := range corpusLogs(t) {
		t.Run(filepath.Base(filepath.Dir(log)), func(t *testing.T) {
			t.Parallel()

			direct, err := codecFor(t, log)
			if err != nil {
				t.Fatalf("the codec for %s: %v", log, err)
			}

			dispatched, err := simlog.NewReader(open(t, log))
			if err != nil {
				t.Fatalf("simlog.NewReader: %v", err)
			}

			assertSamePreamble(t, dispatched, direct)
			assertSameRecords(t, records(t, dispatched), records(t, direct))
		})
	}
}

// The same, for the model-facing reader: a consumer building on model/ must not
// get a different answer for having let this package pick the codec.
func TestDispatchMatchesTheCodecItems(t *testing.T) {
	t.Parallel()

	for _, log := range corpusLogs(t) {
		t.Run(filepath.Base(filepath.Dir(log)), func(t *testing.T) {
			t.Parallel()

			direct, err := runCodecFor(t, log)
			if err != nil {
				t.Fatalf("the codec for %s: %v", log, err)
			}

			dispatched, err := simlog.NewRunReader(open(t, log))
			if err != nil {
				t.Fatalf("simlog.NewRunReader: %v", err)
			}

			if got, want := dispatched.Run(), direct.Run(); !reflect.DeepEqual(got, want) {
				t.Fatalf("Run() = %+v; the codec says %+v", got, want)
			}

			got, want := items(t, dispatched), items(t, direct)
			if len(want) == 0 {
				t.Fatal("the codec yielded no items, so comparing against it proved nothing")
			}

			if len(got) != len(want) {
				t.Fatalf("got %d items, the codec yields %d", len(got), len(want))
			}

			for i := range got {
				if !reflect.DeepEqual(got[i], want[i]) {
					t.Fatalf("item %d differs:\n dispatched %+v\n direct     %+v", i, got[i], want[i])
				}
			}
		})
	}
}

// assertSamePreamble compares everything a reader knows before its first record.
func assertSamePreamble(t *testing.T, dispatched, direct simlog.RecordReader) {
	t.Helper()

	if got, want := dispatched.Header(), direct.Header(); got != want {
		t.Fatalf("Header() = %+v; the codec says %+v", got, want)
	}

	if got, want := dispatched.Assertions(), direct.Assertions(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Assertions() differ: %d dispatched, %d direct", len(got), len(want))
	}

	if got, want := dispatched.Warnings(), direct.Warnings(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Warnings() = %+v; the codec says %+v", got, want)
	}
}

// assertSameRecords compares two record streams field for field.
func assertSameRecords(t *testing.T, got, want []gatling.Record) {
	t.Helper()

	// Without this, a regression that made both sides yield nothing would
	// satisfy every check below and report success on a comparison of two empty
	// slices — the two sides are the same codec, so they fail together.
	if len(want) == 0 {
		t.Fatal("the codec yielded no records, so comparing against it proved nothing")
	}

	if len(got) != len(want) {
		t.Fatalf("got %d records, the codec yields %d", len(got), len(want))
	}

	for i := range got {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Fatalf("record %d differs:\n dispatched %+v\n direct     %+v", i, got[i], want[i])
		}
	}
}
