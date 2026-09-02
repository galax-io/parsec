//go:build integration

package text_test

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/text"
)

var update = flag.Bool("update", false, "rewrite records.golden from the decoder's output")

// corpusDirs lists every recorded run. It fails, rather than skips, when there
// is none: an empty integration run is a failure by constitution.
func corpusDirs(t *testing.T) []string {
	t.Helper()

	logs, err := filepath.Glob(filepath.Join("..", "..", "testdata", "corpus", "gatling", "*", "simulation.log"))
	if err != nil {
		t.Fatal(err)
	}

	if len(logs) == 0 {
		t.Fatal("no recorded run under testdata/corpus/gatling/<version>/ — the corpus is missing, not optional")
	}

	dirs := make([]string, 0, len(logs))
	for _, log := range logs {
		dirs = append(dirs, filepath.Dir(log))
	}

	return dirs
}

// canonical decodes a whole log into its canonical text form: one line per
// item, every string %q-quoted so a lone space and an empty string differ.
func canonical(r io.Reader) ([]byte, error) {
	rd, err := text.NewReader(r)
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

	for {
		rec, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return b.Bytes(), nil
		}

		if err != nil {
			return nil, err
		}

		writeRecord(&b, rec)
	}
}

func writeRecord(b *bytes.Buffer, rec gatling.Record) {
	switch rec.Kind {
	case gatling.KindUser:
		fmt.Fprintf(b, "%d USER scenario=%q event=%s timestamp=%d\n", rec.Line, rec.Scenario, rec.Event, rec.Timestamp)
	case gatling.KindRequest:
		fmt.Fprintf(b, "%d REQUEST groups=%q name=%q start=%d end=%d status=%s message=%q\n",
			rec.Line, rec.Groups, rec.Name, rec.Start, rec.End, rec.Status, rec.Message)
	case gatling.KindGroup:
		fmt.Fprintf(b, "%d GROUP groups=%q start=%d end=%d cumulated=%d status=%s\n",
			rec.Line, rec.Groups, rec.Start, rec.End, rec.CumulatedResponseTime, rec.Status)
	case gatling.KindError:
		fmt.Fprintf(b, "%d ERROR message=%q timestamp=%d\n", rec.Line, rec.Message, rec.Timestamp)
	case gatling.KindAssertion:
		fmt.Fprintf(b, "%d ASSERTION payload=%q\n", rec.Line, rec.Payload)
	case gatling.KindRun, gatling.KindUnknown:
		fmt.Fprintf(b, "%d %s unexpected in the event stream\n", rec.Line, rec.Kind)
	default:
		fmt.Fprintf(b, "%d %s\n", rec.Line, rec.Kind)
	}
}

func TestGolden(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			f, err := os.Open(filepath.Join(dir, "simulation.log")) //nolint:gosec // a corpus path from the test's own glob
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = f.Close() }()

			got, err := canonical(f)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			golden := filepath.Join(dir, "records.golden")
			if *update {
				if err := os.WriteFile(golden, got, 0o600); err != nil {
					t.Fatal(err)
				}

				t.Logf("rewrote %s — review it line by line against simulation.log before committing", golden)

				return
			}

			want, err := os.ReadFile(golden) //nolint:gosec // a corpus path from the test's own glob
			if err != nil {
				t.Fatalf("%v — generate it with -update, then review it", err)
			}

			if !bytes.Equal(got, want) {
				t.Fatalf("decoded stream differs from %s:\n%s", golden, firstDiff(got, want))
			}
		})
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

	return fmt.Sprintf("got %d lines, want %d", len(g), len(w))
}
