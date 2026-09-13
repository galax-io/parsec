package text_test

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/text"
	"github.com/galax-io/parsec/model"
)

// modelPreamble is the two lines every synthetic log here starts with: one assertion
// payload, then the run header naming a covered version.
const modelPreamble = "ASSERTION\tAAEBAAICAAAAAAAAAPA/\n" +
	"RUN\tio.galaxio.parsec.corpus.CorpusSimulation\tcorpussimulation\t1788379354534\t \t3.11.5\n"

// modelItems reads a whole log into a slice. Test-only: a real consumer streams.
func modelItems(t *testing.T, log string) []model.Item {
	t.Helper()

	rd, err := text.NewRunReader(strings.NewReader(log))
	if err != nil {
		t.Fatalf("NewRunReader: %v", err)
	}

	var got []model.Item

	for {
		it, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return got
		}

		if err != nil {
			t.Fatalf("Next: %v", err)
		}

		// Groups is valid only until the next call, so a test that keeps modelItems
		// must copy it — the same rule a consumer follows.
		it.Sample.Groups = slices.Clone(it.Sample.Groups)
		it.Group.Groups = slices.Clone(it.Group.Groups)
		got = append(got, it)
	}
}

// The gate belongs to the decoder and is not re-implemented: a refused log never
// reaches the conversion, and the error is the decoder's own.
func TestRefusedLogNeverReachesTheConversion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		log  string
	}{
		{name: "below the range", log: "RUN\tc\tr\t1788379354534\t \t3.10.0\n"},
		{name: "not a release", log: "RUN\tc\tr\t1788379354534\t \t3.12.0-M1\n"},
		{name: "no header", log: "USER\ts\tSTART\t1788379356160\n"},
		{name: "empty", log: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := text.NewRunReader(strings.NewReader(tt.log)); err == nil {
				t.Fatal("NewRunReader accepted a log the decoder refuses")
			}
		})
	}
}

// A read that stopped on an unreadable line is not a shorter successful one.
func TestUnreadableLineEndsTheStream(t *testing.T) {
	t.Parallel()

	log := modelPreamble +
		"REQUEST\t\tGET /ok\t1788379356162\t1788379356173\tOK\t \n" +
		"NONSENSE\twhat\n"

	rd, err := text.NewRunReader(strings.NewReader(log))
	if err != nil {
		t.Fatalf("NewRunReader: %v", err)
	}

	if _, err := rd.Next(); err != nil {
		t.Fatalf("the first item failed: %v", err)
	}

	_, err = rd.Next()
	if err == nil || errors.Is(err, io.EOF) {
		t.Fatalf("the unreadable line ended the stream as %v, want a syntax error", err)
	}

	var se *gatling.SyntaxError
	if !errors.As(err, &se) {
		t.Errorf("error is %T, want the decoder's own *gatling.SyntaxError", err)
	}

	// Every later call returns the same error: there is no next item after it.
	if _, again := rd.Next(); !errors.Is(again, err) {
		t.Errorf("a later Next returned %v, want the same error", again)
	}
}
