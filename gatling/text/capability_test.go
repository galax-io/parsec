package text_test

import (
	"slices"
	"testing"

	"github.com/galax-io/parsec/gatling/text"
	"github.com/galax-io/parsec/model"
)

// What a Gatling text simulation.log records, and what it never does. The
// second list is the honest half: every entry is a measurement a consumer must
// be told is missing rather than shown as a zero.
func TestCapabilitiesNameWhatTheFormatDoesNotRecord(t *testing.T) {
	t.Parallel()

	c := text.Capabilities()

	provided := []model.Field{
		model.FieldSampleDuration,
		model.FieldGroupDuration,
		model.FieldGroupCumulatedDuration,
		model.FieldGroupOutcome,
	}

	absent := []model.Field{
		// A REQUEST record names no scenario: the log carries one on a USER
		// record, so a request cannot be attributed to one.
		model.FieldSampleScenario,
		model.FieldSampleResponseCode,
		model.FieldSampleBytesSent,
		model.FieldSampleBytesReceived,
		// Gatling writes free text, not a classification.
		model.FieldSampleFailureType,
		// Neither 3.11.5 nor 3.12.0 records which virtual user made a request.
		model.FieldSampleUserIdentity,
		model.FieldConnectTiming,
		model.FieldDNSTiming,
		model.FieldTLSTiming,
		// The assertion payload is carried through unread.
		model.FieldRequirements,
		model.FieldIntervalSeries,
	}

	for _, f := range provided {
		if !c.Provides(f) {
			t.Errorf("%v reads as absent, but the format records it", f)
		}
	}

	got := c.Absent()

	for _, f := range absent {
		if c.Provides(f) {
			t.Errorf("%v reads as provided, but the format records nothing of the kind", f)
		}

		if !slices.Contains(got, f) {
			t.Errorf("Absent() does not name %v", f)
		}
	}

	// The two lists together are every known field: a field added to the model
	// and left out of both would slip through unexamined.
	if want := len(provided) + len(absent); want != int(model.FieldIntervalSeries) {
		t.Errorf("this test accounts for %d fields, the model has %d — a new field is unexamined",
			want, int(model.FieldIntervalSeries))
	}

	if len(got) != len(absent) {
		t.Errorf("Absent() names %d fields, want %d", len(got), len(absent))
	}
}

// A consumer asks what is missing before it renders anything, rather than
// discovering that a whole column is empty and having to guess why.
func TestCapabilitiesAreReadableBeforeOpeningALog(t *testing.T) {
	t.Parallel()

	if len(text.Capabilities().Absent()) == 0 {
		t.Fatal("the source claims to record everything")
	}
}
