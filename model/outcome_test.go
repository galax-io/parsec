package model_test

import (
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/galax-io/parsec/model"
)

// A failure carries what the source recorded; a success carries none at all.
// The rule runs both ways, and it is what makes the presence of a Failure a
// usable test — the only numerator OpenNFR can express is one of presence.
func TestFailureIsSetExactlyWhenTheOutcomeIsFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sample  model.Sample
		wantErr bool
	}{
		{
			name:    "a success carries no failure",
			sample:  model.Sample{Name: "GET /ok", Outcome: model.OutcomeSuccess},
			wantErr: false,
		},
		{
			name: "a failure carries one",
			sample: model.Sample{
				Name:    "GET /fail",
				Outcome: model.OutcomeFailure,
				Failure: model.Some(model.Failure{Message: "status.find.is(200), but actually found 500"}),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.sample.Failure.IsSet(); got != tt.wantErr {
				t.Errorf("Failure.IsSet() = %t, want %t", got, tt.wantErr)
			}

			if (tt.sample.Outcome == model.OutcomeFailure) != tt.sample.Failure.IsSet() {
				t.Error("the outcome and the presence of a failure disagree")
			}
		})
	}
}

// The outcome is recorded on the sample and never inferred from whether some
// other field is set: a failure whose message the source left blank is still a
// failure, and a success is not made one by carrying text.
func TestOutcomeIsNotInferredFromAnotherField(t *testing.T) {
	t.Parallel()

	blank := model.Sample{
		Name:    "GET /fail",
		Outcome: model.OutcomeFailure,
		Failure: model.Some(model.Failure{}),
	}

	if blank.Outcome != model.OutcomeFailure {
		t.Error("a failure with a blank message stopped being a failure")
	}

	if !blank.Failure.IsSet() {
		t.Error("a blank failure reads as absent — presence, not content, is the test")
	}
}

// A group's outcome is its own. The model must be able to express a group that
// failed while every operation inside it succeeded, because Gatling records
// exactly that.
func TestGroupOutcomeIsIndependentOfItsSamples(t *testing.T) {
	t.Parallel()

	enclosed := []model.Sample{
		{Name: "GET /ok", Outcome: model.OutcomeSuccess},
		{Name: "GET /ok2", Outcome: model.OutcomeSuccess},
	}
	group := model.GroupSample{Groups: []string{"outer"}, Outcome: model.OutcomeFailure}

	for _, s := range enclosed {
		if s.Outcome != model.OutcomeSuccess {
			t.Fatalf("%s is not a success", s.Name)
		}
	}

	if group.Outcome != model.OutcomeFailure {
		t.Error("the group's own outcome was overridden by what it encloses")
	}
}

// successes is the selection every statistic starts from.
func successes(items []model.Item) []model.Sample {
	var out []model.Sample

	for _, it := range items {
		if it.Kind == model.ItemSample && it.Sample.Outcome == model.OutcomeSuccess {
			out = append(out, it.Sample)
		}
	}

	return out
}

// Selecting what succeeded returns the same thing however many failures the run
// contains. This is the correctness failure the ecosystem has already made, and
// the model is where it is either possible or not.
func TestSuccessSelectionIsUnchangedByFailures(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // generating test data, not keys

	ok := make([]model.Item, 0, 32)
	for i := range 32 {
		ok = append(ok, model.Item{Kind: model.ItemSample, Sample: model.Sample{
			Name:     "GET /ok",
			Outcome:  model.OutcomeSuccess,
			Duration: model.Some(time.Duration(i+1) * time.Millisecond),
		}})
	}

	want := successes(ok)
	if len(want) != 32 {
		t.Fatalf("selected %d successes from 32", len(want))
	}

	for _, failures := range []int{0, 1, 17, 1000} {
		mixed := slicesCloneItems(ok)

		for range failures {
			mixed = append(mixed, model.Item{Kind: model.ItemSample, Sample: model.Sample{
				Name:     "GET /fail",
				Outcome:  model.OutcomeFailure,
				Duration: model.Some(time.Duration(rng.IntN(10_000)) * time.Millisecond),
				Failure:  model.Some(model.Failure{Message: "boom"}),
			}})
		}

		rng.Shuffle(len(mixed), func(i, j int) { mixed[i], mixed[j] = mixed[j], mixed[i] })

		got := successes(mixed)
		if len(got) != len(want) {
			t.Fatalf("with %d failures added, selected %d successes, want %d", failures, len(got), len(want))
		}

		if !sameDurations(got, want) {
			t.Errorf("with %d failures added, the selected successes differ as a multiset", failures)
		}
	}
}

// An all-failure run selects nothing, and an all-success run selects everything.
func TestSuccessSelectionAtTheExtremes(t *testing.T) {
	t.Parallel()

	allKO := []model.Item{
		{Kind: model.ItemSample, Sample: model.Sample{Outcome: model.OutcomeFailure, Failure: model.Some(model.Failure{})}},
		{Kind: model.ItemSample, Sample: model.Sample{Outcome: model.OutcomeFailure, Failure: model.Some(model.Failure{})}},
	}
	if got := successes(allKO); len(got) != 0 {
		t.Errorf("selected %d successes from a run with none", len(got))
	}

	allOK := []model.Item{
		{Kind: model.ItemSample, Sample: model.Sample{Outcome: model.OutcomeSuccess}},
		{Kind: model.ItemSample, Sample: model.Sample{Outcome: model.OutcomeSuccess}},
	}
	if got := successes(allOK); len(got) != 2 {
		t.Errorf("selected %d successes from a run of two", len(got))
	}
}

func slicesCloneItems(in []model.Item) []model.Item {
	out := make([]model.Item, len(in))
	copy(out, in)

	return out
}

// absentDuration stands in for a sample the source recorded no duration for. It
// is a value no measurement can take, so an absent duration and a recorded zero
// cannot collide in the multiset — which is the one distinction Opt exists to
// keep, and the one a bare `d, _ := …Get()` would throw away.
const absentDuration = time.Duration(math.MinInt64)

func durationOf(s model.Sample) time.Duration {
	d, ok := s.Duration.Get()
	if !ok {
		return absentDuration
	}

	return d
}

// sameDurations compares two selections as multisets of duration, which is what
// a percentile is computed over.
func sameDurations(a, b []model.Sample) bool {
	if len(a) != len(b) {
		return false
	}

	count := map[time.Duration]int{}

	for _, s := range a {
		count[durationOf(s)]++
	}

	for _, s := range b {
		count[durationOf(s)]--
	}

	for _, n := range count {
		if n != 0 {
			return false
		}
	}

	return true
}
