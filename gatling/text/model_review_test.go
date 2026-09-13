package text_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling/text"
	"github.com/galax-io/parsec/model"
)

// An assertion written among the events rather than ahead of them is yielded as
// an item. The wire reader surfaces such a record, so dropping it would lose a
// payload one path preserves and the other does not.
func TestAssertionAfterTheHeaderBecomesAnItem(t *testing.T) {
	t.Parallel()

	const payload = "PAYLOAD-AFTER-HEADER"

	log := modelPreamble +
		"REQUEST\t\tGET /ok\t1788379356162\t1788379356173\tOK\t \n" +
		"ASSERTION\t" + payload + "\n" +
		"USER\ts\tEND\t1788379356200\n"

	got := modelItems(t, log)
	if len(got) != 3 {
		t.Fatalf("got %d items, want 3", len(got))
	}

	if got[1].Kind != model.ItemAssertion {
		t.Fatalf("item 1 Kind = %v, want %v", got[1].Kind, model.ItemAssertion)
	}

	if got[1].Assertion != payload {
		t.Errorf("Assertion = %q, want %q", got[1].Assertion, payload)
	}

	// The preamble's payload stays on the run; this one does not join it.
	rd, err := text.NewRunReader(strings.NewReader(log))
	if err != nil {
		t.Fatalf("NewRunReader: %v", err)
	}

	if want := []string{"AAEBAAICAAAAAAAAAPA/"}; !slices.Equal(rd.Run().Assertions, want) {
		t.Errorf("Run().Assertions = %v, want %v", rd.Run().Assertions, want)
	}
}

// The slices Run hands out are the caller's own, which is the rule the wrapped
// reader already keeps for the same two accessors.
func TestRunSlicesAreNotSharedBetweenCallers(t *testing.T) {
	t.Parallel()

	log := "ASSERTION\tFIRST\nASSERTION\tSECOND\n" +
		"RUN\tio.galaxio.parsec.corpus.CorpusSimulation\tr\t1788379354534\t \t3.99.0\n"

	rd, err := text.NewRunReader(strings.NewReader(log))
	if err != nil {
		t.Fatalf("NewRunReader: %v", err)
	}

	a, b := rd.Run(), rd.Run()

	if len(a.Assertions) != 2 || len(a.Warnings) != 1 {
		t.Fatalf("Run() = %d assertions, %d warnings; want 2 and 1", len(a.Assertions), len(a.Warnings))
	}

	a.Assertions[0] = "CLOBBERED"
	a.Warnings[0].Reason = "CLOBBERED"

	if b.Assertions[0] != "FIRST" {
		t.Errorf("mutating one caller's Assertions changed another's: %q", b.Assertions[0])
	}

	if got := rd.Run(); got.Assertions[0] != "FIRST" || got.Warnings[0].Reason == "CLOBBERED" {
		t.Error("mutating a returned Run changed the reader's own copy")
	}
}

// A run with nothing to warn about reports no warnings the same way it reports
// no assertions, so a caller testing either for nil and a caller testing for
// length agree.
func TestEmptyWarningsAndAssertionsAreBothNil(t *testing.T) {
	t.Parallel()

	rd, err := text.NewRunReader(strings.NewReader(
		"RUN\tio.galaxio.parsec.corpus.CorpusSimulation\tr\t1788379354534\t \t3.11.5\n"))
	if err != nil {
		t.Fatalf("NewRunReader: %v", err)
	}

	run := rd.Run()
	if run.Warnings != nil {
		t.Errorf("Warnings = %v, want nil for a covered version", run.Warnings)
	}

	if run.Assertions != nil {
		t.Errorf("Assertions = %v, want nil for a run that declared none", run.Assertions)
	}
}
