package text_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/text"
)

const (
	headerLine = "RUN\tio.galaxio.parsec.corpus.CorpusSimulation\tcorpussimulation\t1788379354534\t \t3.11.5\n"
	eventLines = "USER\tCorpus recording\tSTART\t1788379356165\n" +
		"REQUEST\t\tGET /ok\t1788379356162\t1788379356173\tOK\t \n" +
		"GROUP\touter\t1788379356180\t1788379357700\t1520\tKO\n" +
		"ERROR\tunresolvable url: Failed to build request \t1788379357701\n" +
		"USER\tCorpus recording\tEND\t1788379357702\n"
)

var payloads = []string{"AAEBAAICAAAAAAAAAPA/", "AAEBAAMCAAAAAAAAAPA/", "AAMAAQdHRVQgL29rAwABAgMAAAAAAABM7UA="}

func preamble() string {
	var b strings.Builder
	for _, p := range payloads {
		b.WriteString("ASSERTION\t" + p + "\n")
	}

	return b.String()
}

func readAll(t *testing.T, r *text.Reader) []gatling.Record {
	t.Helper()

	var recs []gatling.Record

	for {
		rec, err := r.Next()
		if errors.Is(err, io.EOF) {
			return recs
		}

		if err != nil {
			t.Fatalf("Next: %v", err)
		}

		rec.Groups = append([]string(nil), rec.Groups...) // the slice is only valid until the next call
		recs = append(recs, rec)
	}
}

func checkHeader(t *testing.T, hdr gatling.Header) {
	t.Helper()

	want := gatling.Header{
		SimulationClass: "io.galaxio.parsec.corpus.CorpusSimulation",
		RunID:           "corpussimulation",
		Start:           1788379354534,
		Version:         gatling.Version{Major: 3, Minor: 11, Patch: 5},
	}

	if hdr != want {
		t.Fatalf("Header() = %+v, want %+v", hdr, want)
	}
}

func checkAssertions(t *testing.T, got []string) {
	t.Helper()

	if len(got) != len(payloads) {
		t.Fatalf("Assertions() = %v, want %v", got, payloads)
	}

	for i := range payloads {
		if got[i] != payloads[i] {
			t.Fatalf("Assertions()[%d] = %q, want %q", i, got[i], payloads[i])
		}
	}
}

func TestReaderPreamble(t *testing.T) {
	t.Parallel()

	r, err := text.NewReader(strings.NewReader(preamble() + headerLine + eventLines))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	checkHeader(t, r.Header())
	checkAssertions(t, r.Assertions())

	if w := r.Warnings(); len(w) != 0 {
		t.Fatalf("Warnings() = %v for a covered version", w)
	}

	recs := readAll(t, r)

	wantKinds := []gatling.Kind{gatling.KindUser, gatling.KindRequest, gatling.KindGroup, gatling.KindError, gatling.KindUser}
	if len(recs) != len(wantKinds) {
		t.Fatalf("got %d records, want %d", len(recs), len(wantKinds))
	}

	for i, rec := range recs {
		// three assertions and the header occupy lines 1–4; events start at line 5.
		if rec.Kind != wantKinds[i] || rec.Line != 5+i {
			t.Fatalf("record %d = kind %v at line %d, want %v at line %d", i, rec.Kind, rec.Line, wantKinds[i], 5+i)
		}
	}

	if _, err := r.Next(); !errors.Is(err, io.EOF) {
		t.Fatalf("Next after the end = %v, want io.EOF again", err)
	}
}

func TestReaderHeaderOnLineOne(t *testing.T) {
	t.Parallel()

	r, err := text.NewReader(strings.NewReader(headerLine + eventLines))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	if len(r.Assertions()) != 0 {
		t.Fatalf("Assertions() = %v, want none", r.Assertions())
	}

	recs := readAll(t, r)
	if len(recs) != 5 || recs[0].Line != 2 {
		t.Fatalf("got %d records, first at line %d; want 5 starting at line 2", len(recs), recs[0].Line)
	}
}

func TestReaderAssertionAfterHeader(t *testing.T) {
	t.Parallel()

	r, err := text.NewReader(strings.NewReader(headerLine + "ASSERTION\tlate\n" + eventLines))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	recs := readAll(t, r)
	if len(recs) != 6 || recs[0].Kind != gatling.KindAssertion || recs[0].Payload != "late" || recs[0].Line != 2 {
		t.Fatalf("got %+v, want a KindAssertion record with payload \"late\" at line 2 first", recs)
	}
}

func TestReaderEmptyEventStream(t *testing.T) {
	t.Parallel()

	r, err := text.NewReader(strings.NewReader(headerLine))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	if recs := readAll(t, r); len(recs) != 0 {
		t.Fatalf("got %d records from a header-only log, want none", len(recs))
	}
}

// failingReader serves its data, then fails with a caller-supplied error, as a
// network stream or a vanished file would.
type failingReader struct {
	data []byte
	err  error
}

func (f *failingReader) Read(p []byte) (int, error) {
	if len(f.data) == 0 {
		return 0, f.err
	}

	n := copy(p, f.data)
	f.data = f.data[n:]

	return n, nil
}

func TestReaderUnderlyingError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("disk vanished")

	t.Run("during the events", func(t *testing.T) {
		t.Parallel()

		r, err := text.NewReader(&failingReader{data: []byte(headerLine + eventLines), err: sentinel})
		if err != nil {
			t.Fatalf("NewReader: %v", err)
		}

		_, err = drain(r)
		if !errors.Is(err, sentinel) {
			t.Fatalf("got %v, want the stream's own error wrapped", err)
		}

		if !strings.Contains(err.Error(), "reading line 7") {
			t.Fatalf("error %q does not say which line was being read", err)
		}

		if _, again := r.Next(); !errors.Is(again, sentinel) {
			t.Fatalf("Next after the failure = %v, want the same error again", again)
		}
	})

	t.Run("before the header", func(t *testing.T) {
		t.Parallel()

		_, err := text.NewReader(&failingReader{data: []byte(preamble()), err: sentinel})
		if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "reading line 4") {
			t.Fatalf("got %v, want the stream's own error wrapped with line 4", err)
		}
	})
}
