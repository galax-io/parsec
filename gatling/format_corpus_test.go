package gatling_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/galax-io/parsec/gatling"
)

// head reads the leading bytes of a file the way a caller would, and fails the
// test rather than skipping: these files are committed, so a missing one is a
// broken repository and not an absent optional fixture.
func head(t *testing.T, path string) []byte {
	t.Helper()

	f, err := os.Open(path) //nolint:gosec // a path this test spells out itself
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}

	defer f.Close() //nolint:errcheck // read-only

	buf := make([]byte, gatling.DetectSize)

	n, err := f.Read(buf)
	if n == 0 && err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	return buf[:n]
}

// binaryFrom is the first Gatling release that writes a binary simulation.log.
// A corpus entry is expected to be one format or the other by its version, so
// this test asserts what Detect says against what the recording is, rather than
// against what Detect itself thinks it is.
var binaryFrom = gatling.Version{Major: 3, Minor: 13}

// The corpus is the evidence, and here it contradicts the issue that asked for
// this feature. Issue #5 proposed detecting a text log by a leading 'R', for
// the RUN line. Every text run opens with 'A': Gatling writes one ASSERTION
// record per declared assertion ahead of the header, so an R-keyed rule
// misclassifies the ordinary case rather than an edge one.
//
// The corpus now spans both formats, so this also pins the boundary: every
// entry below 3.13.0 is text and every entry from it is binary, and Detect
// agrees with the recording in both directions.
func TestDetectCorpus(t *testing.T) {
	t.Parallel()

	logs, err := filepath.Glob(filepath.Join("..", "testdata", "corpus", "gatling", "*", "simulation.log"))
	if err != nil {
		t.Fatalf("globbing the corpus: %v", err)
	}

	if len(logs) == 0 {
		t.Fatal("no corpus log found: the recorded runs are committed, so this is a broken checkout")
	}

	var texts, binaries int

	for _, log := range logs {
		name := filepath.Base(filepath.Dir(log))

		version, err := gatling.ParseVersion(name)
		if err != nil {
			t.Fatalf("corpus entry %q is not a version directory: %v", name, err)
		}

		want := gatling.FormatText
		if version.Compare(binaryFrom) >= 0 {
			want = gatling.FormatBinary
		}

		if want == gatling.FormatText {
			texts++
		} else {
			binaries++
		}

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			opening := head(t, log)
			assertOpensAs(t, log, opening, want)

			got, err := gatling.Detect(opening)
			if err != nil || got != want {
				t.Fatalf("Detect(%q) = %v, %v; want %v, <nil>", opening, got, err, want)
			}
		})
	}

	// A corpus of one format would let a one-sided rule pass this test.
	if texts == 0 || binaries == 0 {
		t.Fatalf("the corpus holds %d text and %d binary entries; detection is only "+
			"tested in both directions when it holds some of each", texts, binaries)
	}
}

// assertOpensAs checks the bytes themselves, so the test says why it exists
// rather than only that Detect agrees with itself.
func assertOpensAs(t *testing.T, log string, opening []byte, want gatling.Format) {
	t.Helper()

	switch want {
	case gatling.FormatText:
		if !bytes.HasPrefix(opening, []byte("ASSERTION\t")) {
			t.Fatalf("%s opens with %q; this test exists because it opens with ASSERTION, "+
				"which is what falsifies issue #5's first-byte-'R' rule", log, opening)
		}
	case gatling.FormatBinary:
		if opening[0] != 0x00 {
			t.Fatalf("%s opens with %#x; the whole binary rule rests on the run record's kind byte "+
				"being 0x00, and if a real log disagrees the recording wins — correct Detect and the "+
				"spec, not the corpus", log, opening[0])
		}
	case gatling.FormatUnknown:
		t.Fatalf("%s: no format expected for it, which is a bug in this test", log)
	}
}
