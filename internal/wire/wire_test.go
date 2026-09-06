package wire_test

import (
	"testing"
	"time"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/internal/wire"
	"github.com/galax-io/parsec/model"
)

const start = int64(1788670094356)

// The mapping is shared by both codecs, so it is tested here rather than twice.
// A copy in each package could disagree about what a record means while both
// looked correct, and a report written against the model would then depend on
// which log its numbers came from.
//
// Requests carry the most: two timestamps, an outcome and a failure that is set
// if and only if the record failed.
func TestRequestsMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rec  gatling.Record
		want func(*testing.T, model.Item)
	}{
		{
			name: "a successful request",
			rec: gatling.Record{
				Kind: gatling.KindRequest, Groups: []string{"outer"}, Name: "GET /ok",
				Start: start, End: start + 9, Status: gatling.StatusOK,
			},
			want: func(t *testing.T, it model.Item) {
				t.Helper()

				if it.Kind != model.ItemSample {
					t.Fatalf("Kind = %v", it.Kind)
				}

				if it.Sample.Name != "GET /ok" || len(it.Sample.Groups) != 1 {
					t.Fatalf("Sample = %+v", it.Sample)
				}

				if it.Sample.Outcome != model.OutcomeSuccess {
					t.Errorf("Outcome = %v", it.Sample.Outcome)
				}

				if d, ok := it.Sample.Duration.Get(); !ok || d != 9*time.Millisecond {
					t.Errorf("Duration = %v, %v", d, ok)
				}

				if _, ok := it.Sample.Failure.Get(); ok {
					t.Error("a successful request carries a failure")
				}
			},
		},
		{
			name: "a failed request keeps its message and no failure type",
			rec: gatling.Record{
				Kind: gatling.KindRequest, Name: "GET /fail", Start: start, End: start + 2,
				Status: gatling.StatusKO, Message: "status.find.is(200), found 500",
			},
			want: func(t *testing.T, it model.Item) {
				t.Helper()

				f, ok := it.Sample.Failure.Get()
				if !ok {
					t.Fatal("a failed request carries no failure")
				}

				if f.Message != "status.find.is(200), found 500" {
					t.Errorf("Message = %q", f.Message)
				}

				if f.Type != "" {
					t.Errorf("Type = %q; Gatling writes free text and this module invents no taxonomy", f.Type)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var it model.Item
			if !wire.Item(&it, &tt.rec) {
				t.Fatal("Item returned false for a record that is an event of the run")
			}

			tt.want(t, it)
		})
	}
}

// The other four kinds. Each is a smaller mapping than a request's, and each has
// one thing that would be easy to get wrong: a group's two durations are
// different quantities, a user event has a direction, and an assertion written
// among the events is yielded rather than dropped.
func TestTheOtherKindsMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rec  gatling.Record
		want func(*testing.T, model.Item)
	}{
		{
			name: "a group carries both durations, which are different quantities",
			rec: gatling.Record{
				Kind: gatling.KindGroup, Groups: []string{"outer"}, Start: start, End: start + 1504,
				CumulatedResponseTime: 1503, Status: gatling.StatusKO,
			},
			want: func(t *testing.T, it model.Item) {
				t.Helper()

				if it.Kind != model.ItemGroup {
					t.Fatalf("Kind = %v", it.Kind)
				}

				wall, ok := it.Group.Duration.Get()
				if !ok || wall != 1504*time.Millisecond {
					t.Errorf("Duration = %v, %v; want the wall clock across the traversal", wall, ok)
				}

				cumulated, ok := it.Group.CumulatedDuration.Get()
				if !ok || cumulated != 1503*time.Millisecond {
					t.Errorf("CumulatedDuration = %v, %v; want the sum of the requests inside", cumulated, ok)
				}
			},
		},
		{
			name: "a user event",
			rec:  gatling.Record{Kind: gatling.KindUser, Scenario: "s", Event: gatling.EventStart, Timestamp: start},
			want: func(t *testing.T, it model.Item) {
				t.Helper()

				if it.Kind != model.ItemUser || it.User.Kind != model.UserStart || it.User.Scenario != "s" {
					t.Fatalf("User = %+v", it.User)
				}
			},
		},
		{
			name: "an error",
			rec:  gatling.Record{Kind: gatling.KindError, Message: "boom", Timestamp: start},
			want: func(t *testing.T, it model.Item) {
				t.Helper()

				if it.Kind != model.ItemError || it.Error.Message != "boom" {
					t.Fatalf("Error = %+v", it.Error)
				}
			},
		},
		{
			name: "an assertion written among the events is yielded, not dropped",
			rec:  gatling.Record{Kind: gatling.KindAssertion, Payload: "blob"},
			want: func(t *testing.T, it model.Item) {
				t.Helper()

				if it.Kind != model.ItemAssertion || it.Assertion != "blob" {
					t.Fatalf("Assertion = %q", it.Assertion)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var it model.Item
			if !wire.Item(&it, &tt.rec) {
				t.Fatal("Item returned false for a record that is an event of the run")
			}

			tt.want(t, it)
		})
	}
}

// A run header is not an event, and neither is a record that was never decoded.
// Both must be refused rather than turned into an item of some default kind.
func TestNonEventsAreRefused(t *testing.T) {
	t.Parallel()

	for _, kind := range []gatling.Kind{gatling.KindRun, gatling.KindUnknown} {
		rec := gatling.Record{Kind: kind}

		var it model.Item
		if wire.Item(&it, &rec) {
			t.Errorf("Item accepted a %s record", kind)
		}
	}
}

// An end before the start yields no duration rather than a negative one: a
// consumer dividing by it is the failure this prevents. Nothing assumes an end
// is at or after a start, because the wire records document a sentinel for an
// event that never completed.
func TestAnEndBeforeTheStartYieldsNoDuration(t *testing.T) {
	t.Parallel()

	rec := gatling.Record{
		Kind: gatling.KindRequest, Name: "never finished",
		Start: start, End: int64(-1) << 63, Status: gatling.StatusKO,
	}

	var it model.Item
	if !wire.Item(&it, &rec) {
		t.Fatal("Item refused a request record")
	}

	if d, ok := it.Sample.Duration.Get(); ok {
		t.Fatalf("Duration = %v; an event that never completed has none", d)
	}
}

// The conversion clears what it does not set. Reusing an Item across calls must
// not leave a previous record's fields behind, which is the one bug a
// pointer-in, pointer-out signature invites.
func TestItemIsClearedBetweenRecords(t *testing.T) {
	t.Parallel()

	var it model.Item

	sample := gatling.Record{Kind: gatling.KindRequest, Name: "GET /ok", Start: start, End: start, Status: gatling.StatusOK}
	if !wire.Item(&it, &sample) {
		t.Fatal("Item refused a request record")
	}

	user := gatling.Record{Kind: gatling.KindUser, Scenario: "s", Event: gatling.EventEnd, Timestamp: start}
	if !wire.Item(&it, &user) {
		t.Fatal("Item refused a user record")
	}

	if it.Sample.Name != "" {
		t.Fatalf("the previous record's sample survived into a user item: %+v", it.Sample)
	}
}

// Millis is exported because the run header needs the same conversion as the
// records, and a second copy of one line is a second place for the two to
// disagree.
func TestMillisIsExactAndUTC(t *testing.T) {
	t.Parallel()

	got := wire.Millis(start)

	if got.UnixMilli() != start {
		t.Fatalf("Millis(%d).UnixMilli() = %d", start, got.UnixMilli())
	}

	if got.Location() != time.UTC {
		t.Fatalf("Millis returned %v; UTC is what makes a run read the same on every machine", got.Location())
	}
}

// An instant the log could not resolve reaches the model as the zero time, not
// as a date 292 million years in the past. Both codecs write
// gatling.AbsentTimestamp for one and refuse a run that starts before the epoch,
// so the negative half of the number line is absence and nothing else — and zero
// milliseconds is 1970, a recorded instant.
func TestAnAbsentTimestampIsTheZeroTime(t *testing.T) {
	t.Parallel()

	for _, ms := range []int64{gatling.AbsentTimestamp, -1} {
		if got := wire.Millis(ms); !got.IsZero() {
			t.Errorf("Millis(%d) = %v; want the zero time", ms, got)
		}
	}

	if got := wire.Millis(0); got.IsZero() || got.UnixMilli() != 0 {
		t.Errorf("Millis(0) = %v; zero milliseconds is 1970, a recorded instant, not an absence", got)
	}
}

// A record whose start the log could not resolve is still delivered — it is an
// event of the run — with a zero start and no duration, because nothing can be
// measured from an absent start. The same holds for a group.
func TestAnAbsentStartYieldsAZeroStartAndNoDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rec  gatling.Record
	}{
		{
			name: "a request with an absent start",
			rec: gatling.Record{
				Kind: gatling.KindRequest, Name: "r", Start: gatling.AbsentTimestamp, End: start,
				Status: gatling.StatusOK,
			},
		},
		{
			name: "a request with an absent start and an absent end",
			rec: gatling.Record{
				Kind: gatling.KindRequest, Name: "r", Start: gatling.AbsentTimestamp, End: gatling.AbsentTimestamp,
				Status: gatling.StatusOK,
			},
		},
		{
			name: "a group with an absent start",
			rec: gatling.Record{
				Kind: gatling.KindGroup, Groups: []string{"g"}, Start: gatling.AbsentTimestamp, End: start,
				CumulatedResponseTime: 5, Status: gatling.StatusOK,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var it model.Item
			if !wire.Item(&it, &tt.rec) {
				t.Fatal("Item refused the record")
			}

			at, d := it.Sample.Start, it.Sample.Duration
			if it.Kind == model.ItemGroup {
				at, d = it.Group.Start, it.Group.Duration
			}

			if !at.IsZero() {
				t.Errorf("Start = %v; want the zero time for a start the log could not resolve", at)
			}

			if v, ok := d.Get(); ok {
				t.Errorf("Duration = %v; nothing can be measured from an absent start", v)
			}
		})
	}
}
