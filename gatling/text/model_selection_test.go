//go:build integration

package text_test

import (
	"errors"
	"io"
	"math"
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

// successesOf is the selection every statistic starts from, written once so
// both sides of the comparison below use the same one.
func successesOf(items []model.Item) []time.Duration {
	var out []time.Duration

	for _, it := range items {
		if it.Kind != model.ItemSample || it.Sample.Outcome != model.OutcomeSuccess {
			continue
		}

		d, ok := it.Sample.Duration.Get()
		if !ok {
			// An absent duration is not a zero one; give it a value no
			// measurement can take so the two cannot collide in the multiset.
			out = append(out, time.Duration(math.MinInt64))

			continue
		}

		out = append(out, d)
	}

	return out
}

func sameMultiset(a, b []time.Duration) bool {
	if len(a) != len(b) {
		return false
	}

	count := map[time.Duration]int{}
	for _, d := range a {
		count[d]++
	}

	for _, d := range b {
		count[d]--
	}

	for _, n := range count {
		if n != 0 {
			return false
		}
	}

	return true
}

// The same guarantee as TestSuccessSelectionIsUnchangedByFailures, on a real
// recording: what succeeded is the same whether or not the run's failures are
// in the input.
//
// The two selections must come from two different inputs. Selecting twice from
// one filtered slice would compare a set against itself and could not fail.
func TestCorpusSuccessSelectionIsUnchangedByFailures(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			all := readItems(t, filepath.Join(dir, "simulation.log"))

			// A second input: the same run with every failed sample removed
			// before selection, so the two selections walk different slices.
			var withoutFailures []model.Item

			ok, ko := 0, 0

			for _, it := range all {
				if it.Kind != model.ItemSample {
					withoutFailures = append(withoutFailures, it)

					continue
				}

				switch it.Sample.Outcome {
				case model.OutcomeSuccess:
					ok++

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

			withFailures := successesOf(all)
			withoutAny := successesOf(withoutFailures)

			if len(withFailures) != ok {
				t.Errorf("selected %d successes from the whole run, counted %d", len(withFailures), ok)
			}

			if !sameMultiset(withFailures, withoutAny) {
				t.Errorf("the selected successes differ once the run's %d failures are removed", ko)
			}

			// The failures really were in the first input, so the comparison
			// above was not between two identical slices.
			if len(all) == len(withoutFailures) {
				t.Fatal("no failure was removed; the two inputs are the same slice")
			}

			t.Logf("%s: %d successes unchanged by the presence of %d failures", filepath.Base(dir), ok, ko)
		})
	}
}
