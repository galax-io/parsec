package binary_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
	"github.com/galax-io/parsec/gatling/text"
	"github.com/galax-io/parsec/model"
)

// runReader is what both codecs offer a consumer. Naming it here is the point of
// the test below: everything after this line is written once and runs over both.
type runReader interface {
	Run() model.Run
	Next() (model.Item, error)
}

// A report written against the canonical model must not be able to tell which
// log format it was reading. This renders a text run and a binary run through
// exactly the same code — one function, no branch on format, no type switch —
// and requires both to produce a report of the same shape.
//
// It is the milestone's central claim, and the reason the record-to-model
// conversion lives in internal/wire rather than twice.
func TestOneReportRendersBothFormats(t *testing.T) {
	t.Parallel()

	binaryRun := openReader(t, newestBinary(t), func(r io.Reader) (runReader, error) {
		return binary.NewRunReader(r)
	})

	textRun := openReader(t, newestText(t), func(r io.Reader) (runReader, error) {
		return text.NewRunReader(r)
	})

	for _, run := range []struct {
		name string
		rd   runReader
	}{{"binary", binaryRun}, {"text", textRun}} {
		report := render(t, run.rd)

		if report.samples == 0 || report.users == 0 || report.groups == 0 {
			t.Errorf("%s: the report has %d samples, %d user events and %d groups; "+
				"a run missing a whole kind is not the same report", run.name, report.samples, report.users, report.groups)
		}

		if report.tool != "gatling" {
			t.Errorf("%s: Run.Tool is %q", run.name, report.tool)
		}

		if report.start.IsZero() {
			t.Errorf("%s: Run.Start is zero", run.name)
		}
	}

	// The two formats must declare the same capabilities, or a report would have
	// to know which log it came from to know which fields it may trust. This is
	// asserted rather than assumed — R10 asked the question and the corpus is
	// what answers it.
	if got, want := binary.Capabilities(), text.Capabilities(); got != want {
		t.Errorf("the binary codec declares %v and the text codec %v; a difference is allowed, "+
			"but it must be a recorded finding rather than a surprise", got, want)
	}
}

type report struct {
	samples, users, groups, errs int
	tool                         string
	start                        time.Time
}

// render is the consumer: one pass over model items, no knowledge of Gatling and
// none of either log format.
func render(t *testing.T, rd runReader) report {
	t.Helper()

	out := report{tool: rd.Run().Tool, start: rd.Run().Start}

	for {
		it, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return out
		}

		if err != nil {
			t.Fatalf("Next: %v", err)
		}

		switch it.Kind {
		case model.ItemSample:
			out.samples++
		case model.ItemUser:
			out.users++
		case model.ItemGroup:
			out.groups++
		case model.ItemError:
			out.errs++
		case model.ItemAssertion, model.ItemUnknown:
		}
	}
}

func openReader(t *testing.T, dir string, open func(io.Reader) (runReader, error)) runReader {
	t.Helper()

	rd, err := open(openCorpus(t, dir))
	if err != nil {
		t.Fatalf("%s: %v", dir, err)
	}

	return rd
}

func newestBinary(t *testing.T) string {
	t.Helper()

	dirs := corpusDirs(t)

	return dirs[len(dirs)-1]
}

// newestText is the text corpus entry the other codec reads, found the same way
// the binary ones are: by asking the recording what wrote it.
func newestText(t *testing.T) string {
	t.Helper()

	entries, err := os.ReadDir(repoPath("testdata", "corpus", "gatling"))
	if err != nil {
		t.Fatal(err)
	}

	var newest string

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		dir := repoPath("testdata", "corpus", "gatling", e.Name())
		if _, err := os.Stat(filepath.Join(dir, "simulation.log")); err != nil {
			continue
		}

		if format(t, filepath.Join(dir, "simulation.log")) == gatling.FormatText {
			newest = dir
		}
	}

	if newest == "" {
		t.Fatal("no text corpus entry found: the text recordings are committed too")
	}

	return newest
}

// A sample of the model a consumer sees, printed so a reader of the test output
// can check the two formats really do look alike rather than trusting a count.
func ExampleNewRunReader() {
	f, err := os.Open(filepath.Join("..", "..", "testdata", "corpus", "gatling", "3.15.1", "simulation.log"))
	if err != nil {
		return
	}

	defer func() { _ = f.Close() }()

	rd, err := binary.NewRunReader(f)
	if err != nil {
		return
	}

	run := rd.Run()
	fmt.Println(run.Tool, run.ToolVersion, run.Name)

	// Output:
	// gatling 3.15.1 io.galaxio.parsec.corpus.CorpusSimulation
}
