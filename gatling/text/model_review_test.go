package text_test

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/galax-io/parsec/gatling/text"
	"github.com/galax-io/parsec/model"
)

// A millisecond count too large to be a duration is reported absent, never
// wrapped. time.Duration counts nanoseconds in an int64, so anything past
// MaxInt64/1e6 overflows — and wraps to a small plausible negative, not to
// obvious garbage, which is what makes it dangerous.
func TestOversizedDurationsAreAbsentNotNegative(t *testing.T) {
	t.Parallel()

	const overflowMs = math.MaxInt64/int64(time.Millisecond) + 1

	tests := []struct {
		name string
		line string
		read func(model.Item) model.Opt[time.Duration]
	}{
		{
			name: "request span just past the limit",
			line: "REQUEST\t\tGET /ok\t0\t" + strconv.FormatInt(overflowMs, 10) + "\tOK\t \n",
			read: func(it model.Item) model.Opt[time.Duration] { return it.Sample.Duration },
		},
		{
			name: "request span at the maximum timestamp",
			line: "REQUEST\t\tGET /ok\t0\t" + strconv.FormatInt(math.MaxInt64, 10) + "\tOK\t \n",
			read: func(it model.Item) model.Opt[time.Duration] { return it.Sample.Duration },
		},
		{
			name: "group cumulated response time just past the limit",
			line: "GROUP\touter\t0\t10\t" + strconv.FormatInt(overflowMs, 10) + "\tOK\n",
			read: func(it model.Item) model.Opt[time.Duration] { return it.Group.CumulatedDuration },
		},
		{
			name: "group cumulated response time at the maximum",
			line: "GROUP\touter\t0\t10\t" + strconv.FormatInt(math.MaxInt64, 10) + "\tOK\n",
			read: func(it model.Item) model.Opt[time.Duration] { return it.Group.CumulatedDuration },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := modelItems(t, modelPreamble+tt.line)
			if len(got) != 1 {
				t.Fatalf("got %d items, want 1", len(got))
			}

			d, ok := tt.read(got[0]).Get()
			if ok {
				t.Errorf("Duration = %v (set), want unset — an overflowed count is not a measurement", d)
			}

			if d < 0 {
				t.Errorf("Duration = %v, negative", d)
			}
		})
	}
}

// The largest count that still converts is a measurement, not an absence: the
// bound is checked, not approximated.
func TestLargestConvertibleDurationIsStillRecorded(t *testing.T) {
	t.Parallel()

	const maxMs = math.MaxInt64 / int64(time.Millisecond)

	got := modelItems(t, modelPreamble+"GROUP\touter\t0\t10\t"+strconv.FormatInt(maxMs, 10)+"\tOK\n")
	if len(got) != 1 {
		t.Fatalf("got %d items, want 1", len(got))
	}

	d, ok := got[0].Group.CumulatedDuration.Get()
	if !ok {
		t.Fatal("the largest convertible count reads as absent")
	}

	if want := time.Duration(maxMs) * time.Millisecond; d != want {
		t.Errorf("CumulatedDuration = %v, want %v", d, want)
	}
}

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

// The warning names the version once and says nothing about which package
// produced it: Warning.String() prepends the version, and the type is meant to
// read the same whatever tool a run came from.
func TestWarningNamesTheVersionOnce(t *testing.T) {
	t.Parallel()

	rd, err := text.NewRunReader(strings.NewReader(
		"RUN\tio.galaxio.parsec.corpus.CorpusSimulation\tr\t1788379354534\t \t3.99.0\n"))
	if err != nil {
		t.Fatalf("NewRunReader: %v", err)
	}

	w := rd.Run().Warnings[0]

	if strings.Contains(w.Reason, w.Version) {
		t.Errorf("Reason restates the version: %q", w.Reason)
	}

	if strings.Contains(w.Reason, "gatling") {
		t.Errorf("Reason names the source package in a tool-agnostic field: %q", w.Reason)
	}

	if n := strings.Count(w.String(), "3.99.0"); n != 1 {
		t.Errorf("String() names the version %d times, want once: %q", n, w.String())
	}

	oldest, newest := text.SupportedVersions()
	for _, want := range []string{oldest.String(), newest.String()} {
		if !strings.Contains(w.Reason, want) {
			t.Errorf("Reason does not name the verified bound %s: %q", want, w.Reason)
		}
	}
}
