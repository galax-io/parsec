package text_test

import (
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/galax-io/parsec/model"
)

// Every recorded time is preserved exactly: not rounded, and not re-based
// against the run's start.
func TestTimesArePreservedExactly(t *testing.T) {
	t.Parallel()

	const (
		runStart  = 1788379354534
		reqStart  = 1788379356162
		reqEnd    = 1788379356173
		userAt    = 1788379356165
		errorAt   = 1788379356199
		groupStrt = 1788379356160
	)

	log := modelPreamble +
		"USER\ts\tSTART\t" + strconv.Itoa(userAt) + "\n" +
		"REQUEST\t\tGET /ok\t" + strconv.Itoa(reqStart) + "\t" + strconv.Itoa(reqEnd) + "\tOK\t \n" +
		"GROUP\touter\t" + strconv.Itoa(groupStrt) + "\t" + strconv.Itoa(reqEnd) + "\t11\tOK\n" +
		"ERROR\tboom\t" + strconv.Itoa(errorAt) + "\n"

	got := modelItems(t, log)
	if len(got) != 4 {
		t.Fatalf("got %d items, want 4", len(got))
	}

	tests := []struct {
		name string
		got  time.Time
		want int64
	}{
		{name: "user event", got: got[0].User.At, want: userAt},
		{name: "sample start", got: got[1].Sample.Start, want: reqStart},
		{name: "group start", got: got[2].Group.Start, want: groupStrt},
		{name: "run error", got: got[3].Error.At, want: errorAt},
	}

	for _, tt := range tests {
		if gotMs := tt.got.UnixMilli(); gotMs != tt.want {
			t.Errorf("%s = %d ms, want %d — a time was rounded or re-based", tt.name, gotMs, tt.want)
		}
	}

	// Re-basing would make the first timestamp zero, or relative to the header.
	if got[1].Sample.Start.UnixMilli() == 0 ||
		got[1].Sample.Start.UnixMilli() == reqStart-runStart {
		t.Error("sample start was re-based against the run start")
	}
}

// UTC only fixes how an instant prints. The same log must read the same way
// wherever it is read, which a location-dependent time would not.
func TestTimesAreInUTC(t *testing.T) {
	t.Parallel()

	got := modelItems(t, modelPreamble+"REQUEST\t\tGET /ok\t1788379356162\t1788379356173\tOK\t \n")
	if len(got) != 1 {
		t.Fatalf("got %d items, want 1", len(got))
	}

	if loc := got[0].Sample.Start.Location(); loc != time.UTC {
		t.Errorf("sample start is in %v, want UTC", loc)
	}
}

func TestDurationIsEndMinusStart(t *testing.T) {
	t.Parallel()

	got := modelItems(t, modelPreamble+"REQUEST\t\tGET /slow\t1788379356162\t1788379357665\tOK\t \n")
	if len(got) != 1 {
		t.Fatalf("got %d items, want 1", len(got))
	}

	d, ok := got[0].Sample.Duration.Get()
	if !ok {
		t.Fatal("a well-formed request has no duration")
	}

	if want := 1503 * time.Millisecond; d != want {
		t.Errorf("Duration = %v, want %v", d, want)
	}
}

// Gatling's own reader branches on an end equal to the minimum signed 64-bit
// integer, treating it as an event that never completed. Nothing here assumes
// the end is at or after the start: an unusable end yields no duration rather
// than a negative or enormous one a consumer could divide by.
func TestUnusableEndYieldsNoDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		end  string
	}{
		{name: "sentinel", end: strconv.FormatInt(math.MinInt64, 10)},
		{name: "before the start", end: "1788379356000"},
		{name: "one millisecond before the start", end: "1788379356161"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			log := modelPreamble + "REQUEST\t\tGET /ok\t1788379356162\t" + tt.end + "\tOK\t \n"

			got := modelItems(t, log)
			if len(got) != 1 {
				t.Fatalf("got %d items, want 1", len(got))
			}

			if d, ok := got[0].Sample.Duration.Get(); ok {
				t.Errorf("Duration = %v (set), want unset for an end that is not after the start", d)
			}

			// The sample is still delivered: an event that never completed is
			// part of the run, and only its duration is missing.
			if got[0].Sample.Name != "GET /ok" {
				t.Error("the sample itself was dropped")
			}

			if got[0].Sample.Outcome != model.OutcomeSuccess {
				t.Errorf("Outcome = %v, want the recorded one", got[0].Sample.Outcome)
			}
		})
	}
}

// A start equal to the end is a zero duration, which is a measurement, not an
// absence.
func TestZeroDurationIsRecordedNotAbsent(t *testing.T) {
	t.Parallel()

	got := modelItems(t, modelPreamble+"REQUEST\t\tGET /ok\t1788379356162\t1788379356162\tOK\t \n")
	if len(got) != 1 {
		t.Fatalf("got %d items, want 1", len(got))
	}

	d, ok := got[0].Sample.Duration.Get()
	if !ok {
		t.Fatal("a zero duration reads as absent")
	}

	if d != 0 {
		t.Errorf("Duration = %v, want 0", d)
	}
}

func TestGroupCumulatedDurationIsMilliseconds(t *testing.T) {
	t.Parallel()

	got := modelItems(t, modelPreamble+"GROUP\touter\t1788379356162\t1788379356200\t1503\tOK\n")
	if len(got) != 1 {
		t.Fatalf("got %d items, want 1", len(got))
	}

	d, ok := got[0].Group.CumulatedDuration.Get()
	if !ok {
		t.Fatal("a group has no cumulated duration")
	}

	if want := 1503 * time.Millisecond; d != want {
		t.Errorf("CumulatedDuration = %v, want %v", d, want)
	}
}

// A time the log could not resolve — a negative value, which Gatling writes only
// as its never-completed sentinel — reaches the model as the zero time, with no
// duration measured from it, and a negative cumulated response time reaches it
// unset. Neither ends the read: one bad field in a ten-million-record log is not
// a reason to refuse the run, and this is what the binary codec already does for
// the same input.
func TestAnAbsentTimeIsTheZeroTimeNotADateInThePast(t *testing.T) {
	t.Parallel()

	log := modelPreamble +
		"REQUEST\t\tGET /ok\t-5\t1788379356173\tOK\t \n" +
		"GROUP\touter\t1788379356162\t1788379356200\t-5\tOK\n" +
		"USER\ts\tEND\t-1\n"

	got := modelItems(t, log)
	if len(got) != 3 {
		t.Fatalf("got %d items, want 3", len(got))
	}

	if !got[0].Sample.Start.IsZero() {
		t.Errorf("Start = %v; want the zero time", got[0].Sample.Start)
	}

	if d, ok := got[0].Sample.Duration.Get(); ok {
		t.Errorf("Duration = %v; nothing can be measured from an absent start", d)
	}

	if d, ok := got[1].Group.CumulatedDuration.Get(); ok {
		t.Errorf("CumulatedDuration = %v; want unset for a negative value", d)
	}

	if !got[2].User.At.IsZero() {
		t.Errorf("At = %v; want the zero time", got[2].User.At)
	}
}
