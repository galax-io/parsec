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

// The corpus is the evidence, and here it contradicts the issue that asked for
// this feature. Issue #5 proposed detecting a text log by a leading 'R', for
// the RUN line. Both recorded runs open with 'A': Gatling writes one ASSERTION
// record per declared assertion ahead of the header, so an R-keyed rule
// misclassifies the ordinary case rather than an edge one.
func TestDetectCorpusIsText(t *testing.T) {
	t.Parallel()

	logs, err := filepath.Glob(filepath.Join("..", "testdata", "corpus", "gatling", "*", "simulation.log"))
	if err != nil {
		t.Fatalf("globbing the corpus: %v", err)
	}

	if len(logs) == 0 {
		t.Fatal("no corpus log found: the recorded runs are committed, so this is a broken checkout")
	}

	for _, log := range logs {
		t.Run(filepath.Base(filepath.Dir(log)), func(t *testing.T) {
			t.Parallel()

			opening := head(t, log)

			if !bytes.HasPrefix(opening, []byte("ASSERTION\t")) {
				t.Fatalf("%s opens with %q; this test exists because it opens with ASSERTION, "+
					"which is what falsifies issue #5's first-byte-'R' rule", log, opening)
			}

			got, err := gatling.Detect(opening)
			if err != nil || got != gatling.FormatText {
				t.Fatalf("Detect(%q) = %v, %v; want %v, <nil>", opening, got, err, gatling.FormatText)
			}
		})
	}
}

// The sample is 64 bytes cut from a real Gatling 3.15.1 run. It is not a corpus
// entry and nothing is compared against it beyond this: a binary log is
// identified as binary.
func TestDetectBinarySampleIsBinary(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "testdata", "samples", "gatling", "binary", "3.15.1-head.bin")
	opening := head(t, path)

	if opening[0] != 0x00 {
		t.Fatalf("%s opens with %#x; the whole binary rule rests on the run record's kind byte being 0x00, "+
			"and if a real log disagrees the recording wins — correct Detect and the spec, not the sample", path, opening[0])
	}

	got, err := gatling.Detect(opening)
	if err != nil || got != gatling.FormatBinary {
		t.Fatalf("Detect(sample) = %v, %v; want %v, <nil>", got, err, gatling.FormatBinary)
	}
}

// Detection is a property of the bytes. The signature is what guarantees it —
// Detect is handed a slice and can see no name, no extension and no directory —
// and this asserts the guarantee holds end to end: the same bytes read out of a
// file and built in memory give the same answer.
func TestDetectIgnoresTheFileName(t *testing.T) {
	t.Parallel()

	corpus := filepath.Join("..", "testdata", "corpus", "gatling", "3.11.5", "simulation.log")
	sample := filepath.Join("..", "testdata", "samples", "gatling", "binary", "3.15.1-head.bin")

	tests := []struct {
		name string
		path string
		want gatling.Format
	}{
		// The text log carries a .log name and the binary sample a .bin one;
		// both are ignored, and the misleading pairing is the point.
		{name: "a .log file that is text", path: corpus, want: gatling.FormatText},
		{name: "a .bin file that is binary", path: sample, want: gatling.FormatBinary},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opening := head(t, tt.path)

			fromFile, err := gatling.Detect(opening)
			if err != nil {
				t.Fatalf("Detect(from %s) = _, %v", tt.path, err)
			}

			// The same bytes, carrying no provenance at all.
			fromMemory, err := gatling.Detect(bytes.Clone(opening))
			if err != nil {
				t.Fatalf("Detect(from memory) = _, %v", err)
			}

			if fromFile != tt.want || fromMemory != tt.want {
				t.Fatalf("Detect = %v from the file and %v from memory; want %v for both",
					fromFile, fromMemory, tt.want)
			}
		})
	}
}
