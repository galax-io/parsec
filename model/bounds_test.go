package model_test

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/galax-io/parsec/model"
)

// at is a recorded instant, and the offsets below are milliseconds from it.
var at = time.UnixMilli(1_788_670_094_885).UTC()

func ms(offset int) time.Time { return at.Add(time.Duration(offset) * time.Millisecond) }

func sampleAt(start time.Time, duration model.Opt[time.Duration]) model.Item {
	return model.Item{Kind: model.ItemSample, Sample: model.Sample{Name: "r", Start: start, Duration: duration}}
}

func userAt(kind model.UserEventKind, when time.Time) model.Item {
	return model.Item{Kind: model.ItemUser, User: model.UserEvent{Scenario: "s", Kind: kind, At: when}}
}

// fold extends a fresh Bounds by every item, in the order given.
func fold(items []model.Item) model.Bounds {
	var b model.Bounds

	for i := range items {
		b.Extend(&items[i])
	}

	return b
}

// The span is what the tool's own report divides by: bounded by sample, group
// and virtual-user events, not by the run's recorded start and not by errors,
// with a virtual user able to set either end. Each row is one rule.
func TestBoundsFollowTheReportsRule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		items     []model.Item
		wantStart time.Time
		wantEnd   time.Time
	}{
		{
			name: "a sample sets both ends",
			items: []model.Item{
				sampleAt(ms(100), model.Some(50*time.Millisecond)),
			},
			wantStart: ms(100), wantEnd: ms(150),
		},
		{
			name: "a user START before the first request opens the run",
			items: []model.Item{
				sampleAt(ms(100), model.Some(50*time.Millisecond)),
				userAt(model.UserStart, ms(0)),
			},
			wantStart: ms(0), wantEnd: ms(150),
		},
		{
			name: "a user END after the last request closes the run",
			items: []model.Item{
				sampleAt(ms(100), model.Some(50*time.Millisecond)),
				userAt(model.UserEnd, ms(900)),
			},
			wantStart: ms(100), wantEnd: ms(900),
		},
		{
			name: "a user START alone opens and closes the run",
			items: []model.Item{
				userAt(model.UserStart, ms(10)),
			},
			wantStart: ms(10), wantEnd: ms(10),
		},
		{
			name: "a user END does not open the run",
			items: []model.Item{
				userAt(model.UserEnd, ms(0)),
				sampleAt(ms(100), model.Some(50*time.Millisecond)),
			},
			wantStart: ms(100), wantEnd: ms(150),
		},
		{
			name: "a sample with no recorded end contributes its start and nothing else",
			items: []model.Item{
				sampleAt(ms(100), model.Some(50*time.Millisecond)),
				sampleAt(ms(50), model.Opt[time.Duration]{}),
			},
			wantStart: ms(50), wantEnd: ms(150),
		},
		{
			name: "a group's end is its wall clock, and its cumulated time moves nothing",
			items: []model.Item{
				{Kind: model.ItemGroup, Group: model.GroupSample{
					Groups: []string{"g"}, Start: ms(100),
					Duration:          model.Some(200 * time.Millisecond),
					CumulatedDuration: model.Some(time.Hour),
				}},
			},
			wantStart: ms(100), wantEnd: ms(300),
		},
		{
			name: "an error and an assertion move nothing",
			items: []model.Item{
				sampleAt(ms(100), model.Some(50*time.Millisecond)),
				{Kind: model.ItemError, Error: model.RunError{Message: "boom", At: ms(-500)}},
				{Kind: model.ItemError, Error: model.RunError{Message: "boom", At: ms(5000)}},
				{Kind: model.ItemAssertion, Assertion: "payload"},
			},
			wantStart: ms(100), wantEnd: ms(150),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := fold(tt.items)

			if start, ok := b.Start(); !ok || !start.Equal(tt.wantStart) {
				t.Errorf("Start() = %v, %t; want %v", start, ok, tt.wantStart)
			}

			if end, ok := b.End(); !ok || !end.Equal(tt.wantEnd) {
				t.Errorf("End() = %v, %t; want %v", end, ok, tt.wantEnd)
			}
		})
	}
}

// What has not been recorded cannot bound a run: a zero start, an event with no
// direction, an item of no kind. And nothing folded is nothing bounded.
func TestBoundsIgnoreWhatWasNeverRecorded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		items []model.Item
	}{
		{name: "nothing folded", items: nil},
		{name: "a sample whose start the source could not resolve", items: []model.Item{
			sampleAt(time.Time{}, model.Some(50*time.Millisecond)),
		}},
		{name: "a group whose start the source could not resolve", items: []model.Item{
			{Kind: model.ItemGroup, Group: model.GroupSample{Groups: []string{"g"}, Duration: model.Some(time.Second)}},
		}},
		{name: "a user event whose time the source could not resolve", items: []model.Item{
			userAt(model.UserStart, time.Time{}),
		}},
		{name: "a user event with no direction", items: []model.Item{
			userAt(model.UserEventUnknown, ms(10)),
		}},
		{name: "an item of no kind", items: []model.Item{
			{Kind: model.ItemUnknown, Sample: model.Sample{Start: ms(10), Duration: model.Some(time.Second)}},
		}},
		{name: "errors alone", items: []model.Item{
			{Kind: model.ItemError, Error: model.RunError{At: ms(10)}},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := fold(tt.items)

			if start, ok := b.Start(); ok || !start.IsZero() {
				t.Errorf("Start() = %v, %t; want absent", start, ok)
			}

			if end, ok := b.End(); ok || !end.IsZero() {
				t.Errorf("End() = %v, %t; want absent", end, ok)
			}
		})
	}
}

// A run whose samples all lack a recorded end still has a span: each of them was
// running at its own start, and the latest of those starts is the last instant
// the run is known to have been going. Reporting no end at all would have been a
// second kind of silence about the same items the fold counted.
func TestARunOfEndlessSamplesEndsAtTheLatestStart(t *testing.T) {
	t.Parallel()

	b := fold([]model.Item{
		sampleAt(ms(100), model.Opt[time.Duration]{}),
		sampleAt(ms(40), model.Opt[time.Duration]{}),
	})

	if start, ok := b.Start(); !ok || !start.Equal(ms(40)) {
		t.Errorf("Start() = %v, %t; want %v", start, ok, ms(40))
	}

	if end, ok := b.End(); !ok || !end.Equal(ms(100)) {
		t.Errorf("End() = %v, %t; want %v", end, ok, ms(100))
	}
}

// The fold is a minimum and a maximum, so the same items in any order give the
// same bounds. Concurrent virtual users interleave differently on every run,
// and file order is not evidence of anything.
func TestBoundsAreOrderIndependent(t *testing.T) {
	t.Parallel()

	items := []model.Item{
		userAt(model.UserStart, ms(0)),
		sampleAt(ms(100), model.Some(50*time.Millisecond)),
		{Kind: model.ItemGroup, Group: model.GroupSample{Groups: []string{"g"}, Start: ms(90), Duration: model.Some(300 * time.Millisecond)}},
		sampleAt(ms(20), model.Opt[time.Duration]{}),
		{Kind: model.ItemError, Error: model.RunError{At: ms(9000)}},
		userAt(model.UserEnd, ms(700)),
	}

	want := fold(items)

	r := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // a shuffle in a test, not a secret

	for range 25 {
		r.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })

		if got := fold(items); got != want {
			t.Fatalf("a different order gave different bounds: %+v, want %+v", got, want)
		}
	}
}

// Extending the bounds retains nothing and allocates nothing: it is called once
// per item of a multi-gigabyte log.
//
//nolint:paralleltest // AllocsPerRun measures the whole process and refuses to run in a parallel test
func TestBoundsExtendAllocatesNothing(t *testing.T) {
	items := []model.Item{
		userAt(model.UserStart, ms(0)),
		sampleAt(ms(100), model.Some(50*time.Millisecond)),
		{Kind: model.ItemGroup, Group: model.GroupSample{Groups: []string{"g"}, Start: ms(90), Duration: model.Some(300 * time.Millisecond)}},
		userAt(model.UserEnd, ms(700)),
	}

	var b model.Bounds

	if allocs := testing.AllocsPerRun(100, func() {
		for i := range items {
			b.Extend(&items[i])
		}
	}); allocs != 0 {
		t.Errorf("Extend allocated %.0f times per pass over four items; want none", allocs)
	}
}

// A fold that met an item it could not place in time reports no bounds at all.
//
// The item still belongs to the run, and the model carries no end without a
// start to measure from, so the end the source recorded for it is unreachable
// here. Reporting the span of everything else would be a span too short and a
// rate too high, with nothing anywhere to say so.
func TestBoundsRefuseASpanTheyCannotVouchFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		items []model.Item
	}{
		{
			name: "a sample whose start is absent takes its recorded end with it",
			items: []model.Item{
				sampleAt(ms(0), model.Some(10*time.Millisecond)),
				sampleAt(time.Time{}, model.Opt[time.Duration]{}),
			},
		},
		{
			name: "a group whose start is absent",
			items: []model.Item{
				sampleAt(ms(0), model.Some(10*time.Millisecond)),
				{Kind: model.ItemGroup, Group: model.GroupSample{Groups: []string{"g"}, Duration: model.Some(time.Second)}},
			},
		},
		{
			name: "a user event the source could not place",
			items: []model.Item{
				sampleAt(ms(0), model.Some(10*time.Millisecond)),
				userAt(model.UserStart, time.Time{}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := fold(tt.items)

			if start, ok := b.Start(); ok {
				t.Errorf("Start() = %v, set; a fold that could not place an item has no span to report", start)
			}

			if end, ok := b.End(); ok {
				t.Errorf("End() = %v, set; a fold that could not place an item has no span to report", end)
			}
		})
	}
}

// A virtual-user END earlier than every sample start used to leave the two
// bounds crossed, and both were then reported as absent. Now each end-less
// sample carries the end to its own start, so the span covers every item the
// fold counted and there is nothing to cross.
func TestAnEarlyUserEndDoesNotShortenTheSpan(t *testing.T) {
	t.Parallel()

	// A run cut short: every request carries the never-completed sentinel, and
	// the only event that closes the run is earlier than any of them.
	b := fold([]model.Item{
		userAt(model.UserEnd, ms(1000)),
		sampleAt(ms(5000), model.Opt[time.Duration]{}),
		sampleAt(ms(9000), model.Opt[time.Duration]{}),
	})

	start, ok := b.Start()
	if !ok || !start.Equal(ms(5000)) {
		t.Errorf("Start() = %v, %t; want %v — the earliest start is still known", start, ok, ms(5000))
	}

	if end, ok := b.End(); !ok || !end.Equal(ms(9000)) {
		t.Errorf("End() = %v, %t; want %v — the latest instant the run is known to have been going",
			end, ok, ms(9000))
	}
}

// Every source this package documents promises a non-negative Duration, and the
// Gatling path keeps that promise. Bounds is exported, so an adapter that broke
// it must not be able to drag the end behind the start: a negative duration
// gives no end past the item's own start, which is where an item with no
// recorded end leaves it too.
func TestBoundsIgnoreANegativeDuration(t *testing.T) {
	t.Parallel()

	b := fold([]model.Item{sampleAt(ms(10000), model.Some(-5*time.Second))})

	start, ok := b.Start()
	if !ok || !start.Equal(ms(10000)) {
		t.Errorf("Start() = %v, %t; want %v", start, ok, ms(10000))
	}

	end, ok := b.End()
	if !ok || !end.Equal(ms(10000)) {
		t.Errorf("End() = %v, %t; want %v — never behind the start it was folded from",
			end, ok, ms(10000))
	}
}

// Bucketing by position is what Position exists for, so bounds kept per position
// must be readable out of the map that holds them. Read-only accessors on a
// pointer receiver would not compile here.
func TestBoundsAreReadableWhereTheyAreNotAddressable(t *testing.T) {
	t.Parallel()

	var b model.Bounds

	items := []model.Item{sampleAt(ms(100), model.Some(50*time.Millisecond))}
	for i := range items {
		b.Extend(&items[i])
	}

	byPosition := map[model.Position]model.Bounds{
		model.NewSamplePosition(nil, "r"): b,
	}

	start, ok := byPosition[model.NewSamplePosition(nil, "r")].Start()
	if !ok || !start.Equal(ms(100)) {
		t.Errorf("Start() read from a map = %v, %t; want %v", start, ok, ms(100))
	}
}

// An item the fold counted whose end the source did not record is known to have
// been running at its own start instant, so it extends the end there.
//
// It used to extend only the start, which let End report an instant earlier than
// the start of an item the same fold had counted: a consumer divided a count
// that included the later sample by a span that ended before it began. That is
// the shape this type's own rationale calls dishonest — "a span too short and a
// rate too high, with nothing to say so" — one step removed from the case it was
// written for. Gatling's own arithmetic treats a request that never completed as
// occupying its start instant, and coverUser's virtual-user START already did.
func TestAnItemWithNoRecordedEndExtendsTheEnd(t *testing.T) {
	t.Parallel()

	b := fold([]model.Item{
		sampleAt(ms(10_000), model.Some(5*time.Second)),
		sampleAt(ms(20_000), model.Opt[time.Duration]{}),
	})

	start, ok := b.Start()
	if !ok || !start.Equal(ms(10_000)) {
		t.Errorf("Start() = %v, %v; want %v, true", start, ok, ms(10_000))
	}

	end, ok := b.End()
	if !ok || !end.Equal(ms(20_000)) {
		t.Errorf("End() = %v, %v; want %v, true", end, ok, ms(20_000))
	}
}

// The invariant the change buys, stated as a property rather than as one case:
// End, when it reports a value, is never earlier than the start of any item the
// fold counted. Every path that begins the bounds now also finishes them at or
// after that instant, and finish only ever raises.
func TestEndIsNeverBeforeACountedStart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		items []model.Item
	}{
		{
			name: "an end-less item last",
			items: []model.Item{
				sampleAt(ms(10_000), model.Some(5*time.Second)),
				sampleAt(ms(20_000), model.Opt[time.Duration]{}),
			},
		},
		{
			name: "an end-less item first",
			items: []model.Item{
				sampleAt(ms(20_000), model.Opt[time.Duration]{}),
				sampleAt(ms(10_000), model.Some(5*time.Second)),
			},
		},
		{
			name: "only end-less items",
			items: []model.Item{
				sampleAt(ms(10_000), model.Opt[time.Duration]{}),
				sampleAt(ms(30_000), model.Opt[time.Duration]{}),
			},
		},
		{
			name: "a virtual-user END earlier than every sample start",
			items: []model.Item{
				userAt(model.UserEnd, ms(1_000)),
				sampleAt(ms(10_000), model.Opt[time.Duration]{}),
			},
		},
		{
			name: "a group with no recorded duration",
			items: []model.Item{
				{Kind: model.ItemGroup, Group: model.GroupSample{Start: ms(40_000)}},
				sampleAt(ms(10_000), model.Some(time.Second)),
			},
		},
		{
			name: "a negative duration contributes no end past its start",
			items: []model.Item{
				sampleAt(ms(10_000), model.Some(-5*time.Second)),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := fold(tt.items)

			end, ok := b.End()
			if !ok {
				t.Fatalf("End() reported nothing for a fold that counted %d items", len(tt.items))
			}

			for i := range tt.items {
				var began time.Time

				switch tt.items[i].Kind {
				case model.ItemSample:
					began = tt.items[i].Sample.Start
				case model.ItemGroup:
					began = tt.items[i].Group.Start
				case model.ItemUser, model.ItemError, model.ItemAssertion, model.ItemUnknown:
					continue
				}

				if end.Before(began) {
					t.Errorf("End() = %v, before the start of a counted item at %v", end, began)
				}
			}
		})
	}
}

// What does not change: an item the source could not place in time still makes
// both report nothing, and an empty fold still reports nothing.
func TestAnUnplaceableItemStillReportsNothing(t *testing.T) {
	t.Parallel()

	b := fold([]model.Item{
		sampleAt(ms(10_000), model.Some(time.Second)),
		sampleAt(time.Time{}, model.Some(time.Second)),
	})

	if _, ok := b.Start(); ok {
		t.Error("Start() reported a value for a fold holding an item it could not place")
	}

	if _, ok := b.End(); ok {
		t.Error("End() reported a value for a fold holding an item it could not place")
	}

	var empty model.Bounds

	if _, ok := empty.Start(); ok {
		t.Error("the zero Bounds reported a start")
	}

	if _, ok := empty.End(); ok {
		t.Error("the zero Bounds reported an end")
	}
}
