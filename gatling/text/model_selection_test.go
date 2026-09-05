//go:build integration

package text_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/galax-io/parsec/gatling/text"
	"github.com/galax-io/parsec/model"
)

func readItems(t *testing.T, path string) []model.Item {
	t.Helper()

	f, err := os.Open(path) //nolint:gosec // a corpus path from the test's own glob
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	rd, err := text.NewRunReader(f)
	if err != nil {
		t.Fatalf("NewRunReader: %v", err)
	}

	var out []model.Item

	for {
		it, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return out
		}

		if err != nil {
			t.Fatalf("Next: %v", err)
		}

		it.Sample.Groups = slices.Clone(it.Sample.Groups)
		it.Group.Groups = slices.Clone(it.Group.Groups)
		out = append(out, it)
	}
}

// The same guarantee as TestSuccessSelectionIsUnchangedByFailures, on a real
// recording rather than a generated one: removing the failures a run actually
// contains does not change what succeeded.
func TestCorpusSuccessSelectionIsUnchangedByFailures(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			all := readItems(t, filepath.Join(dir, "simulation.log"))

			var withoutFailures []model.Item

			okDurations := map[time.Duration]int{}
			ok, ko := 0, 0

			for _, it := range all {
				if it.Kind != model.ItemSample {
					withoutFailures = append(withoutFailures, it)
					continue
				}

				switch it.Sample.Outcome {
				case model.OutcomeSuccess:
					ok++

					d, _ := it.Sample.Duration.Get()
					okDurations[d]++

					withoutFailures = append(withoutFailures, it)
				case model.OutcomeFailure:
					ko++
				case model.OutcomeUnknown:
					t.Fatalf("the conversion produced a sample with no outcome: %q", it.Sample.Name)
				}
			}

			if ok == 0 || ko == 0 {
				t.Fatalf("the recording has %d successes and %d failures; it must have both", ok, ko)
			}

			// Selecting from the run with its failures removed must give the
			// same multiset as selecting from the run with them present.
			stripped := 0

			for _, it := range withoutFailures {
				if it.Kind != model.ItemSample {
					continue
				}

				stripped++

				d, _ := it.Sample.Duration.Get()
				okDurations[d]--
			}

			if stripped != ok {
				t.Errorf("selected %d successes with failures removed, %d with them present", stripped, ok)
			}

			for d, n := range okDurations {
				if n != 0 {
					t.Errorf("duration %v appears %d more times in one selection than the other", d, n)
				}
			}

			t.Logf("%s: %d successes unchanged by the presence of %d failures", filepath.Base(dir), ok, ko)
		})
	}
}
