//go:build integration

package text_test

import (
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling/text"
	"github.com/galax-io/parsec/model"
)

// primitiveTally is a third fold of a run, beside decodeTally over the wire
// records and modelTally over the model items: it buckets by model.Position
// and bounds the run with model.Bounds, and writes no definition of its own.
// decodeTally is what it is held to, because that one reads the log's own
// records through a different reader and derives the span from the recorded
// ends rather than from a start plus a duration — so a mistake in the rule the
// primitives share with the model path cannot cancel out on both sides.
type primitiveTally struct {
	global   triple
	requests map[model.Position]triple
	groups   map[model.Position]triple
	bounds   model.Bounds
}

func foldPrimitives(t *testing.T, path string) primitiveTally {
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

	ta := primitiveTally{requests: map[model.Position]triple{}, groups: map[model.Position]triple{}}

	for {
		it, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return ta
		}

		if err != nil {
			t.Fatalf("Next: %v", err)
		}

		ta.bounds.Extend(&it)

		switch it.Kind {
		case model.ItemSample:
			pos := it.Sample.Position()
			c := ta.requests[pos]
			c.add(okOutcome(t, it.Sample.Outcome, it.Sample.Name))
			ta.requests[pos] = c
			ta.global.add(okOutcome(t, it.Sample.Outcome, it.Sample.Name))
		case model.ItemGroup:
			pos := it.Group.Position()
			c := ta.groups[pos]
			c.add(okOutcome(t, it.Group.Outcome, pos.String()))
			ta.groups[pos] = c
		case model.ItemUser, model.ItemError, model.ItemAssertion:
			// Bounded above; nothing a report counts.
		case model.ItemUnknown:
			t.Fatal("the conversion produced an item of unknown kind")
		}
	}
}

// durationSec is the run span in whole seconds, rounded up, as Gatling's report
// computes it — taken from the primitives rather than from a hand-kept pair.
func (ta primitiveTally) durationSec(t *testing.T) float64 {
	t.Helper()

	start, ok := ta.bounds.Start()
	if !ok {
		t.Fatal("the fold found no start; a recorded run always has one")
	}

	end, ok := ta.bounds.End()
	if !ok {
		t.Fatal("the fold found no end; a recorded run always has one")
	}

	return math.Ceil(float64(end.Sub(start).Milliseconds()) / 1000)
}

// handKey is how the hand-rolled tallies spell a position: the path joined with
// a comma. The join is lossy in general — a comma inside a group name would
// merge two paths — and exact on this corpus, which is the point of a position
// that needs no spelling.
func handKey(pos model.Position) statsKey {
	if pos.Kind() == model.PositionGroup {
		return statsKey{path: strings.Join(pos.Groups(), ",")}
	}

	return statsKey{path: strings.Join(pos.Groups(), ","), name: pos.Name()}
}

// Two folds written independently — the one spec 002 wrote by hand, and one
// keyed and bounded through the primitives — agree on every corpus run: the
// same rows, the same counts under each, and the same span.
func TestPrimitiveFoldMatchesTheHandRolledOne(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			log := filepath.Join(dir, "simulation.log")
			hand := decodeTally(t, log)
			prim := foldPrimitives(t, log)

			if prim.global != hand.global {
				t.Errorf("run totals: primitives %+v, hand %+v", prim.global, hand.global)
			}

			if len(prim.requests) != len(hand.requests) || len(prim.groups) != len(hand.groups) {
				t.Errorf("primitives found %d request and %d group positions, the hand-rolled fold %d and %d",
					len(prim.requests), len(prim.groups), len(hand.requests), len(hand.groups))
			}

			for pos, got := range prim.requests {
				if want, ok := hand.requests[handKey(pos)]; !ok || got != want {
					t.Errorf("request %s: primitives %+v, hand %+v (found by hand: %t)", pos, got, want, ok)
				}
			}

			for pos, got := range prim.groups {
				if want, ok := hand.groups[handKey(pos)]; !ok || got != want {
					t.Errorf("group %s: primitives %+v, hand %+v (found by hand: %t)", pos, got, want, ok)
				}
			}

			start, _ := prim.bounds.Start()
			end, _ := prim.bounds.End()

			if start.UnixMilli() != hand.injectStart || end.UnixMilli() != hand.injectEnd {
				t.Errorf("bounds: primitives %d..%d, hand %d..%d",
					start.UnixMilli(), end.UnixMilli(), hand.injectStart, hand.injectEnd)
			}
		})
	}
}

// The span the primitives fold is the one Gatling's own report divided by: its
// mean requests per second come out exactly, at the precision it printed.
func TestBoundsReproduceTheReportRate(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			prim := foldPrimitives(t, filepath.Join(dir, "simulation.log"))
			dur := prim.durationSec(t)

			var global reportStats
			loadJSON(t, filepath.Join(dir, "global_stats.json"), &global)
			checkRates(t, "global_stats.json", prim.global, dur, global.MeanNumberOfRequestsPerSecond)

			var root reportNode
			loadJSON(t, filepath.Join(dir, "stats.json"), &root)
			checkRates(t, "stats.json root", prim.global, dur, root.Stats.MeanNumberOfRequestsPerSecond)
		})
	}
}
