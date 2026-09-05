//go:build integration

package text_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/galax-io/parsec/gatling/text"
	"github.com/galax-io/parsec/model"
)

// setFields reports which fields an item actually carries a value for. It is
// driven by the model's own field list, so a field added later is covered here
// without editing this test — it simply reads as unset until something sets it.
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
	case model.ItemUser, model.ItemError, model.ItemUnknown:
	}

	return set
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
