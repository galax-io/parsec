package model_test

import (
	"testing"
	"time"

	"github.com/galax-io/parsec/model"
)

func TestOptZeroValueIsUnset(t *testing.T) {
	t.Parallel()

	var o model.Opt[int]

	if o.IsSet() {
		t.Error("the zero Opt reports itself set")
	}

	v, ok := o.Get()
	if ok {
		t.Error("Get on the zero Opt reports a value")
	}

	if v != 0 {
		t.Errorf("Get on the zero Opt returned %d, want the zero of its type", v)
	}
}

func TestOptSomeCarriesTheValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		set  model.Opt[int]
		want int
	}{
		{name: "positive", set: model.Some(42), want: 42},
		{name: "negative", set: model.Some(-1), want: -1},
		{name: "zero", set: model.Some(0), want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if !tt.set.IsSet() {
				t.Fatal("Some produced an unset Opt")
			}

			got, ok := tt.set.Get()
			if !ok {
				t.Fatal("Get on a set Opt reports no value")
			}

			if got != tt.want {
				t.Errorf("Get = %d, want %d", got, tt.want)
			}
		})
	}
}

// A recorded zero and an absent value are different facts. This is the whole
// reason Opt exists rather than a bare int: a source that measured zero bytes
// and a source that records no byte count must not read alike.
func TestOptRecordedZeroIsNotAbsent(t *testing.T) {
	t.Parallel()

	recorded := model.Some(0)

	var absent model.Opt[int]

	if !recorded.IsSet() {
		t.Error("a recorded zero reads as absent")
	}

	if absent.IsSet() {
		t.Error("an absent value reads as recorded")
	}

	if recorded == absent {
		t.Error("a recorded zero and an absent value compare equal")
	}
}

func TestOptOrReturnsFallbackOnlyWhenUnset(t *testing.T) {
	t.Parallel()

	if got := model.Some(7).Or(99); got != 7 {
		t.Errorf("Or on a set Opt = %d, want the value 7", got)
	}

	if got := model.Some(0).Or(99); got != 0 {
		t.Errorf("Or on a recorded zero = %d, want 0, not the fallback", got)
	}

	var absent model.Opt[int]

	if got := absent.Or(99); got != 99 {
		t.Errorf("Or on an unset Opt = %d, want the fallback 99", got)
	}
}

// Opt is used for durations on every sample, so it must work for a non-numeric
// type without any special casing.
func TestOptCarriesADuration(t *testing.T) {
	t.Parallel()

	d := model.Some(1500 * time.Millisecond)

	got, ok := d.Get()
	if !ok {
		t.Fatal("Get on a set duration reports no value")
	}

	if got != 1500*time.Millisecond {
		t.Errorf("Get = %v, want 1.5s", got)
	}

	var absent model.Opt[time.Duration]

	if _, ok := absent.Get(); ok {
		t.Error("an unset duration reports a value")
	}
}
