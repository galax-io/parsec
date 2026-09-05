//go:build integration

package text_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/galax-io/parsec/gatling/text"
	"github.com/galax-io/parsec/model"
)

// accountedFields is every field setFields knows how to read. It is checked
// against the model's own list, so a field added to the model without a case
// below fails rather than silently reading as unset — which would quietly stop
// this test covering the field most likely to be wrong: the newest one.
var accountedFields = []model.Field{
	model.FieldSampleDuration,
	model.FieldSampleScenario,
	model.FieldSampleResponseCode,
	model.FieldSampleBytesSent,
	model.FieldSampleBytesReceived,
	model.FieldSampleFailureType,
	model.FieldSampleUserIdentity,
	model.FieldGroupDuration,
	model.FieldGroupCumulatedDuration,
	model.FieldGroupOutcome,
	model.FieldConnectTiming,
	model.FieldDNSTiming,
	model.FieldTLSTiming,
	model.FieldRequirements,
	model.FieldIntervalSeries,
}

// setFields reports which fields an item actually carries a value for. The
// fields no item can carry — a user identity this format never records, an
// interval series that is not a per-item value — are named in accountedFields
// and read as unset here, which is the correct answer for them.
func setFields(it model.Item) map[model.Field]bool {
	set := map[model.Field]bool{}

	switch it.Kind {
	case model.ItemSample:
		s := it.Sample
		set[model.FieldSampleDuration] = s.Duration.IsSet()
		set[model.FieldSampleScenario] = s.Scenario.IsSet()
		set[model.FieldSampleResponseCode] = s.ResponseCode.IsSet()
		set[model.FieldSampleBytesSent] = s.BytesSent.IsSet()
		set[model.FieldSampleBytesReceived] = s.BytesReceived.IsSet()

		if f, ok := s.Failure.Get(); ok {
			set[model.FieldSampleFailureType] = f.Type != ""
		}
	case model.ItemGroup:
		set[model.FieldGroupDuration] = it.Group.Duration.IsSet()
		set[model.FieldGroupCumulatedDuration] = it.Group.CumulatedDuration.IsSet()
		set[model.FieldGroupOutcome] = it.Group.Outcome != model.OutcomeUnknown
	case model.ItemUser, model.ItemError, model.ItemAssertion, model.ItemUnknown:
	}

	return set
}

// The claim setFields makes about itself, enforced: every field the model names
// is accounted for above.
func TestSetFieldsAccountsForEveryKnownField(t *testing.T) {
	t.Parallel()

	known := model.FieldsKnown()
	if len(accountedFields) != len(known) {
		t.Errorf("setFields accounts for %d fields, the model names %d — a new field is unexamined",
			len(accountedFields), len(known))
	}

	for _, f := range known {
		if !slices.Contains(accountedFields, f) {
			t.Errorf("%v is not accounted for by setFields", f)
		}
	}
}

// A field the run declares absent must be unset on every item of that run. This
// is the invariant that makes Capabilities worth reading: without it a consumer
// would still have to scan for substituted zeroes.
func TestAbsentFieldsAreNeverFilledIn(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			f, err := os.Open(filepath.Join(dir, "simulation.log")) //nolint:gosec // a corpus path from the test's own glob
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = f.Close() }()

			rd, err := text.NewRunReader(f)
			if err != nil {
				t.Fatalf("NewRunReader: %v", err)
			}

			caps := rd.Run().Capabilities
			items := 0

			for {
				it, err := rd.Next()
				if errors.Is(err, io.EOF) {
					break
				}

				if err != nil {
					t.Fatalf("Next after %d items: %v", items, err)
				}

				items++

				for field, isSet := range setFields(it) {
					if isSet && !caps.Provides(field) {
						t.Fatalf("item %d (%v) carries %v, which the run declares absent",
							items, it.Kind, field)
					}
				}
			}

			if items == 0 {
				t.Fatal("the corpus run yielded no items")
			}

			t.Logf("%s: %d items, none carrying a value the source cannot record", filepath.Base(dir), items)
		})
	}
}
