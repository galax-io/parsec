package binary_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
)

// corpusDirs is every recorded run this codec covers: the directories under
// testdata/corpus/gatling whose simulation.log is a binary one. It asks the
// recording rather than assuming from the version, so a mis-recorded entry shows
// up here instead of quietly leaving a version untested.
func corpusDirs(t *testing.T) []string {
	t.Helper()

	logs, err := filepath.Glob(repoPath("testdata", "corpus", "gatling", "*", "simulation.log"))
	if err != nil {
		t.Fatalf("globbing the corpus: %v", err)
	}

	var dirs []string

	for _, log := range logs {
		if format(t, log) == gatling.FormatBinary {
			dirs = append(dirs, filepath.Dir(log))
		}
	}

	if len(dirs) == 0 {
		t.Fatal("no binary corpus entry found: the recorded runs are committed, so this is a broken checkout")
	}

	return dirs
}

func repoPath(parts ...string) string {
	return filepath.Join(append([]string{"..", ".."}, parts...)...)
}

// format says which Gatling log format wrote a file, from its opening bytes.
func format(t *testing.T, path string) gatling.Format {
	t.Helper()

	f, err := os.Open(path) //nolint:gosec // a corpus path from the test's own glob
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}

	defer func() { _ = f.Close() }()

	buf := make([]byte, gatling.DetectSize)

	n, err := f.Read(buf)
	if n == 0 && err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	got, err := gatling.Detect(buf[:n])
	if err != nil {
		return gatling.FormatUnknown
	}

	return got
}

// openCorpus opens one recording's log and closes it with the test.
func openCorpus(t *testing.T, dir string) io.Reader {
	t.Helper()

	f, err := os.Open(filepath.Join(dir, "simulation.log")) //nolint:gosec // a corpus path from the test's own glob
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = f.Close() })

	return f
}

// canonical renders a decoded log as text, one line per record, so a golden file
// is reviewable by eye and a difference names the record it is in.
//
// Records are numbered by their position in the stream. A binary log has no
// lines, so gatling.Record.Line is zero throughout and cannot serve.
func canonical(r io.Reader) ([]byte, error) {
	rd, err := binary.NewReader(r)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer

	h := rd.Header()
	fmt.Fprintf(&b, "HEADER class=%q run=%q start=%d description=%q version=%s\n",
		h.SimulationClass, h.RunID, h.Start, h.Description, h.Version)

	for _, p := range rd.Assertions() {
		fmt.Fprintf(&b, "ASSERTION payload=%q\n", p)
	}

	for _, w := range rd.Warnings() {
		fmt.Fprintf(&b, "WARNING %s\n", w.String())
	}

	for n := 1; ; n++ {
		rec, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return b.Bytes(), nil
		}

		if err != nil {
			return nil, err
		}

		writeRecord(&b, n, rec)
	}
}

func writeRecord(b *bytes.Buffer, n int, rec gatling.Record) {
	switch rec.Kind {
	case gatling.KindUser:
		fmt.Fprintf(b, "%d USER scenario=%q event=%s timestamp=%d\n", n, rec.Scenario, rec.Event, rec.Timestamp)
	case gatling.KindRequest:
		fmt.Fprintf(b, "%d REQUEST groups=%q name=%q start=%d end=%d status=%s message=%q\n",
			n, rec.Groups, rec.Name, rec.Start, rec.End, rec.Status, rec.Message)
	case gatling.KindGroup:
		fmt.Fprintf(b, "%d GROUP groups=%q start=%d end=%d cumulated=%d status=%s\n",
			n, rec.Groups, rec.Start, rec.End, rec.CumulatedResponseTime, rec.Status)
	case gatling.KindError:
		fmt.Fprintf(b, "%d ERROR message=%q timestamp=%d\n", n, rec.Message, rec.Timestamp)
	case gatling.KindAssertion:
		fmt.Fprintf(b, "%d ASSERTION payload=%q\n", n, rec.Payload)
	case gatling.KindRun, gatling.KindUnknown:
		fmt.Fprintf(b, "%d %s unexpected in the event stream\n", n, rec.Kind)
	default:
		fmt.Fprintf(b, "%d %s\n", n, rec.Kind)
	}
}

// firstDiff reports the first differing line of two canonical streams.
func firstDiff(got, want []byte) string {
	g, w := bytes.Split(got, []byte("\n")), bytes.Split(want, []byte("\n"))
	for i := 0; i < len(g) && i < len(w); i++ {
		if !bytes.Equal(g[i], w[i]) {
			return fmt.Sprintf("line %d:\n got  %s\n want %s", i+1, g[i], w[i])
		}
	}

	return fmt.Sprintf("one stream has %d lines and the other %d", len(g), len(w))
}

// records reads a whole log into a slice, copying each record's group path
// because the reader reuses the slice behind it.
func records(t *testing.T, r io.Reader, opts ...gatling.Option) []gatling.Record {
	t.Helper()

	rd, err := binary.NewReader(r, opts...)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	var out []gatling.Record

	for {
		rec, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return out
		}

		if err != nil {
			t.Fatalf("Next: %v", err)
		}

		// Copied without changing whether it is nil. `append([]string(nil), ...)`
		// and slices.Clone both flatten an empty non-nil slice to nil, which
		// would hide from every test built on this helper whether the reader
		// hands out one or the other.
		if rec.Groups != nil {
			rec.Groups = append(make([]string, 0, len(rec.Groups)), rec.Groups...)
		}

		out = append(out, rec)
	}
}
