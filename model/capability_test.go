package model_test

import (
	"slices"
	"testing"

	"github.com/galax-io/parsec/model"
)

func TestCapabilitiesProvidesWhatItWasGiven(t *testing.T) {
	t.Parallel()

	c := model.NewCapabilities(model.FieldSampleDuration, model.FieldGroupOutcome)

	if !c.Provides(model.FieldSampleDuration) {
		t.Error("a field the source was given reads as absent")
	}

	if !c.Provides(model.FieldGroupOutcome) {
		t.Error("a field the source was given reads as absent")
	}

	if c.Provides(model.FieldSampleResponseCode) {
		t.Error("a field the source was not given reads as provided")
	}
}

// The set held is what the source provides, so a field this package gains later
// reads as absent for every adapter written before it. That is the conservative
// direction: the opposite storage would report a new field as present for a
// source that never recorded it.
func TestCapabilitiesUnknownFieldIsAbsent(t *testing.T) {
	t.Parallel()

	var empty model.Capabilities

	for _, f := range append([]model.Field{model.FieldUnknown}, model.FieldsKnown()...) {
		if empty.Provides(f) {
			t.Errorf("the zero Capabilities provides %v", f)
		}
	}
}

// FieldUnknown is the zero value and names nothing, so it is neither provided
// nor reported as a missing measurement.
func TestCapabilitiesNeverProvidesOrReportsUnknown(t *testing.T) {
	t.Parallel()

	c := model.NewCapabilities(model.FieldUnknown, model.FieldSampleDuration)

	if c.Provides(model.FieldUnknown) {
		t.Error("FieldUnknown reads as provided")
	}

	if slices.Contains(c.Absent(), model.FieldUnknown) {
		t.Error("Absent names FieldUnknown")
	}
}

func TestCapabilitiesAbsentNamesTheRest(t *testing.T) {
	t.Parallel()

	c := model.NewCapabilities(model.FieldSampleDuration, model.FieldGroupOutcome)
	absent := c.Absent()

	if slices.Contains(absent, model.FieldSampleDuration) {
		t.Error("Absent names a field the source provides")
	}

	for _, want := range []model.Field{
		model.FieldSampleScenario,
		model.FieldSampleResponseCode,
		model.FieldSampleUserIdentity,
		model.FieldIntervalSeries,
	} {
		if !slices.Contains(absent, want) {
			t.Errorf("Absent does not name %v", want)
		}
	}
}

// A report prints what a source cannot measure, so the order must not wobble
// between runs of the same program.
func TestCapabilitiesAbsentIsStablyOrdered(t *testing.T) {
	t.Parallel()

	c := model.NewCapabilities(model.FieldSampleDuration)
	first := c.Absent()

	if !slices.IsSorted(first) {
		t.Error("Absent is not sorted")
	}

	for range 8 {
		if got := c.Absent(); !slices.Equal(got, first) {
			t.Fatalf("Absent returned %v, then %v", first, got)
		}
	}
}

// The slice a caller gets is theirs: mutating it must not change what the
// capabilities report next time.
func TestCapabilitiesAbsentIsNotAliased(t *testing.T) {
	t.Parallel()

	c := model.NewCapabilities(model.FieldSampleDuration)

	got := c.Absent()
	if len(got) == 0 {
		t.Fatal("Absent named nothing")
	}

	got[0] = model.FieldUnknown

	if slices.Contains(c.Absent(), model.FieldUnknown) {
		t.Error("mutating the returned slice changed the capabilities")
	}
}

func TestFieldStringNamesEveryField(t *testing.T) {
	t.Parallel()

	known := model.FieldsKnown()
	seen := make(map[string]model.Field, len(known)+1)

	for _, f := range append([]model.Field{model.FieldUnknown}, known...) {
		s := f.String()
		if s == "" {
			t.Errorf("field %d has an empty name", int(f))
		}

		if prev, dup := seen[s]; dup {
			t.Errorf("fields %d and %d share the name %q", int(prev), int(f), s)
		}

		seen[s] = f
	}
}

// Every field this package names is reachable by the mechanism: declarable,
// queryable, and reportable as absent. A field added to the enum without being
// carried by the bounds would read as neither provided nor absent, which is the
// one state Capabilities must never produce.
func TestEveryKnownFieldIsReachable(t *testing.T) {
	t.Parallel()

	known := model.FieldsKnown()
	if len(known) == 0 {
		t.Fatal("FieldsKnown named nothing")
	}

	absent := model.NewCapabilities().Absent()
	if len(absent) != len(known) {
		t.Errorf("a source that provides nothing reports %d absent fields, but %d are known",
			len(absent), len(known))
	}

	for _, f := range known {
		if !slices.Contains(absent, f) {
			t.Errorf("%v is known but never reported absent", f)
		}

		if !model.NewCapabilities(f).Provides(f) {
			t.Errorf("%v is known but cannot be declared provided", f)
		}

		if f.String() == "unknown" {
			t.Errorf("field %d is known but has no name", int(f))
		}
	}
}
