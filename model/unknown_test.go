package model_test

import (
	"testing"

	"github.com/galax-io/parsec/model"
)

// A value outside the set an enum names renders as the type and the number. It
// used to render as "unknown", which is what the zero value renders as — and the
// zero value means something: [model.Outcome]'s own documentation says it marks
// a sample that lost its outcome on the way rather than succeeding quietly. A
// value the module cannot name is a different fact from a value the source lost,
// and printing them the same way threw that distinction away.
//
// No adapter produces either. A value out of range can only arrive by a consumer
// casting an integer, and for that the number is the one useful thing to print.
func TestOutOfRangeValuesNameTheTypeAndTheNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Outcome", got: model.Outcome(200).String(), want: "Outcome(200)"},
		{name: "UserEventKind", got: model.UserEventKind(200).String(), want: "UserEventKind(200)"},
		{name: "ItemKind", got: model.ItemKind(200).String(), want: "ItemKind(200)"},
		{name: "PositionKind", got: model.PositionKind(200).String(), want: "PositionKind(200)"},
		{name: "Field", got: model.Field(60000).String(), want: "Field(60000)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.got != tt.want {
				t.Errorf("String() on an out-of-range %s = %q, want %q", tt.name, tt.got, tt.want)
			}
		})
	}
}

// The zero value is not at issue and does not move: every enum here names it
// "unknown" and documents that it does.
func TestZeroValuesStillReadAsUnknown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  string
	}{
		{name: "Outcome", got: model.OutcomeUnknown.String()},
		{name: "UserEventKind", got: model.UserEventUnknown.String()},
		{name: "ItemKind", got: model.ItemUnknown.String()},
		{name: "PositionKind", got: model.PositionUnknown.String()},
		{name: "Field", got: model.FieldUnknown.String()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.got != "unknown" {
				t.Errorf("%s zero value = %q, want %q", tt.name, tt.got, "unknown")
			}
		})
	}
}

// An out-of-range field is not provided, and is not reported as a missing
// measurement either: it names nothing, so there is nothing to be missing.
func TestOutOfRangeFieldIsNeitherProvidedNorAbsent(t *testing.T) {
	t.Parallel()

	beyond := model.Field(60000)

	c := model.NewCapabilities(beyond, model.FieldSampleDuration)
	if c.Provides(beyond) {
		t.Error("an out-of-range field reads as provided")
	}

	if !c.Provides(model.FieldSampleDuration) {
		t.Error("declaring an out-of-range field alongside a real one dropped the real one")
	}

	for _, f := range c.Absent() {
		if f == beyond {
			t.Error("an out-of-range field is reported absent")
		}
	}
}
