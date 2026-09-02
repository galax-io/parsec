package text_test

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/text"
)

// drain reads until the first error and returns what came before it.
func drain(r *text.Reader) ([]gatling.Record, error) {
	var recs []gatling.Record

	for {
		rec, err := r.Next()
		if err != nil {
			return recs, err
		}

		rec.Groups = append([]string(nil), rec.Groups...)
		recs = append(recs, rec)
	}
}

func TestSyntaxErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		fixture     string
		wantLine    int
		wantFound   string
		wantRecords int
	}{
		{fixture: "unknown-kind", wantLine: 3, wantFound: "BOGUS"},
		{fixture: "user-three-fields", wantLine: 3, wantFound: "3 fields"},
		{fixture: "request-eight-fields", wantLine: 3, wantFound: "8 fields"},
		{fixture: "error-newline-in-message", wantLine: 3, wantFound: "2 fields"},
		{fixture: "truncated-last-line", wantLine: 6, wantFound: "end of input", wantRecords: 3},
		{fixture: "unterminated-last-line", wantLine: 6, wantFound: "end of input", wantRecords: 3},
		{fixture: "bad-timestamp", wantLine: 3, wantFound: "soon"},
		{fixture: "bad-status", wantLine: 3, wantFound: `"ok"`},
		{fixture: "bad-event", wantLine: 3, wantFound: "BEGIN"},
		{fixture: "second-run-header", wantLine: 3, wantFound: "RUN"},
	}

	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			t.Parallel()

			r, err := text.NewReader(openFixture(t, tt.fixture))
			if err != nil {
				t.Fatalf("NewReader: %v", err)
			}

			recs, err := drain(r)

			var se *gatling.SyntaxError
			if !errors.As(err, &se) {
				t.Fatalf("got %v, want *gatling.SyntaxError", err)
			}

			if se.Line != tt.wantLine || !strings.Contains(se.Found, tt.wantFound) || se.Expected == "" {
				t.Fatalf("got %+v, want line %d finding %q", se, tt.wantLine, tt.wantFound)
			}

			if len(recs) != tt.wantRecords {
				t.Fatalf("%d records were delivered before the failure, want %d", len(recs), tt.wantRecords)
			}

			// The failure is sticky: there is no next record after it.
			if _, again := r.Next(); !errors.Is(again, err) {
				t.Fatalf("Next after the failure = %v, want the same error again", again)
			}
		})
	}
}

func TestSyntaxErrorMessageWithSeparators(t *testing.T) {
	t.Parallel()

	r, err := text.NewReader(openFixture(t, "error-tab-in-message"))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	recs := readAll(t, r)
	if len(recs) != 4 || recs[0].Kind != gatling.KindError || recs[0].Message != "first\tsecond\tthird" || recs[0].Timestamp != 1788379357701 {
		t.Fatalf("got %+v, want the error message recovered whole", recs[0])
	}
}

func TestSyntaxCRLF(t *testing.T) {
	t.Parallel()

	r, err := text.NewReader(openFixture(t, "crlf"))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	if r.Header().Version != ver(3, 11, 5) {
		t.Fatalf("Header().Version = %v with CRLF endings", r.Header().Version)
	}

	recs := readAll(t, r)
	if len(recs) != 3 || recs[1].Kind != gatling.KindRequest || recs[1].Message != "" {
		t.Fatalf("got %+v, want 3 records with an empty request message", recs)
	}
}

func TestSyntaxFirstFailureWins(t *testing.T) {
	t.Parallel()

	var b strings.Builder

	b.WriteString("ASSERTION\tAAEBAAICAAAAAAAAAPA/\n")
	b.WriteString("RUN\tio.galaxio.parsec.corpus.CorpusSimulation\tcorpussimulation\t1788379354534\t \t3.11.5\n")

	for line := 3; line <= 1000; line++ {
		switch line {
		case 5, 900:
			b.WriteString("BOGUS\n")
		default:
			b.WriteString("REQUEST\t\tGET /ok\t1788379356162\t1788379356173\tOK\t \n")
		}
	}

	r, err := text.NewReader(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	recs, err := drain(r)

	var se *gatling.SyntaxError
	if !errors.As(err, &se) || se.Line != 5 {
		t.Fatalf("got %v, want a SyntaxError at line 5, the first damage", err)
	}

	if len(recs) != 2 || recs[len(recs)-1].Line != 4 {
		t.Fatalf("got %d records, last at line %d; want 2 ending at line 4", len(recs), recs[len(recs)-1].Line)
	}

	if _, err := r.Next(); !errors.As(err, &se) || se.Line != 5 {
		t.Fatalf("Next after the failure = %v, want line 5 again, never line 900", err)
	}

	if _, err := r.Next(); errors.Is(err, io.EOF) {
		t.Fatal("a failed read must never turn into a clean io.EOF")
	}
}
