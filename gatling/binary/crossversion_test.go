package binary_test

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"

	"github.com/galax-io/parsec/gatling"
)

// The same simulation recorded under three Gatling versions must decode to the
// same events. What may differ between the runs is what the runs themselves
// cannot repeat — when each happened, in what order the scheduler interleaved
// its users — and one thing Gatling itself changed: it reworded the check
// failure message at 3.14.0, from
//
//	status.find.is(200), but actually found 500
//
// to
//
//	status.find.is(200), found 500
//
// which is the entire thirteen-byte difference between the 3.13.1 log and the
// other two. Message text is therefore set aside along with timing and order.
//
// What is left is the shape of the run, and it must be identical: any difference
// there is the decoder reading one version differently from another, which is
// the thing a version-gated codec exists to catch.
func TestVersionsAgreeOnTheShapeOfTheRun(t *testing.T) {
	t.Parallel()

	dirs := corpusDirs(t)
	if len(dirs) < 2 {
		t.Skip("one recording cannot be compared with another")
	}

	shapes := map[string][]string{}

	for _, dir := range dirs {
		shapes[filepath.Base(dir)] = shape(t, dir)
	}

	first := filepath.Base(dirs[0])

	for _, dir := range dirs[1:] {
		name := filepath.Base(dir)

		a, b := shapes[first], shapes[name]
		if len(a) != len(b) {
			t.Fatalf("%s decodes to %d events and %s to %d", first, len(a), name, len(b))
		}

		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("%s and %s disagree at event %d:\n %s: %s\n %s: %s",
					first, name, i, first, a[i], name, b[i])
			}
		}
	}
}

// shape describes every event of a run without anything that legitimately
// differs between two runs of the same simulation: no timestamp, no duration, no
// message, and sorted, because the order six concurrent users finish in is not a
// property of the simulation.
func shape(t *testing.T, dir string) []string {
	t.Helper()

	var out []string

	for _, rec := range records(t, openCorpus(t, dir)) {
		switch rec.Kind {
		case gatling.KindRequest:
			out = append(out, fmt.Sprintf("REQUEST %q %q %s", rec.Groups, rec.Name, rec.Status))
		case gatling.KindGroup:
			out = append(out, fmt.Sprintf("GROUP %q %s", rec.Groups, rec.Status))
		case gatling.KindUser:
			out = append(out, fmt.Sprintf("USER %q %s", rec.Scenario, rec.Event))
		case gatling.KindError:
			out = append(out, "ERROR")
		case gatling.KindRun, gatling.KindAssertion, gatling.KindUnknown:
			t.Fatalf("%s in the event stream", rec.Kind)
		}
	}

	sort.Strings(out)

	return out
}
