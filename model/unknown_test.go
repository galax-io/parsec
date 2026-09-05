package model_test

import (
	"testing"

	"github.com/galax-io/parsec/model"
)

// A value outside the set every enum names reads as unknown rather than as a
// number or an empty string. No adapter produces one, but a report must not
// print garbage if a future one does.
func TestOutOfRangeValuesReadAsUnknown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  string
	}{
		{name: "Outcome", got: model.Outcome(200).String()},
		{name: "UserEventKind", got: model.UserEventKind(200).String()},
		{name: "ItemKind", got: model.ItemKind(200).String()},
		{name: "Field", got: model.Field(60000).String()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.got != "unknown" {
				t.Errorf("String() on an out-of-range %s = %q, want %q", tt.name, tt.got, "unknown")
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
