//go:build integration || canary

package binary_test

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
	"github.com/galax-io/parsec/model"
)

// handKey is a position spelled the way a consumer had to spell it before there
// was a Position: the path joined with a comma, and the name. Lossy in general
// and exact on this corpus.
type handKey struct{ path, name string }

// handTally is the fold a consumer wrote by hand over the wire records: counts
// per request and per group, and the run span bounded as Gatling's report bounds
// it — request and group starts and user STARTs on one side, request and group
// ends and any user event on the other. It is written without the primitives on
// purpose, so that the primitive fold has something independent to be held to.
type handTally struct {
	global   counts
	requests map[handKey]counts
	groups   map[handKey]counts
	start    int64
	end      int64
}

func (ta *handTally) open(at int64) {
	if at != gatling.AbsentTimestamp {
		ta.start = min(ta.start, at)
	}
}

func (ta *handTally) close(at int64) {
	if at != gatling.AbsentTimestamp {
		ta.end = max(ta.end, at)
	}
}

func foldByHand(t *testing.T, dir string) handTally {
	t.Helper()

	ta := handTally{
		requests: map[handKey]counts{}, groups: map[handKey]counts{},
		start: math.MaxInt64, end: math.MinInt64,
	}

	for _, rec := range records(t, openCorpus(t, dir)) {
		switch rec.Kind {
		case gatling.KindRequest:
			k := handKey{path: strings.Join(rec.Groups, ","), name: rec.Name}
			ta.requests[k] = countStatus(t, ta.requests[k], rec.Status)
			ta.global = countStatus(t, ta.global, rec.Status)
			ta.open(rec.Start)
			ta.close(rec.End)
		case gatling.KindGroup:
			k := handKey{path: strings.Join(rec.Groups, ",")}
			ta.groups[k] = countStatus(t, ta.groups[k], rec.Status)
			ta.open(rec.Start)
			ta.close(rec.End)
		case gatling.KindUser:
			if rec.Event == gatling.EventStart {
				ta.open(rec.Timestamp)
			}

			ta.close(rec.Timestamp)
		case gatling.KindError, gatling.KindAssertion, gatling.KindRun, gatling.KindUnknown:
			// Neither counted by a report nor a bound of the run.
		}
	}

	return ta
}

func countStatus(t *testing.T, c counts, s gatling.Status) counts {
	t.Helper()

	c.total++

	switch s {
	case gatling.StatusOK:
		c.ok++
	case gatling.StatusKO:
		c.ko++
	case gatling.StatusUnknown:
		t.Fatal("a record decoded with no outcome")
	}

	return c
}

// primitiveTally is the same fold through the primitives: keyed by
// model.Position, bounded by model.Bounds, with no definition of its own.
type primitiveTally struct {
	global   counts
	requests map[model.Position]counts
	groups   map[model.Position]counts
	bounds   model.Bounds
}

func foldPrimitives(t *testing.T, dir string) primitiveTally {
	t.Helper()

	rd, err := binary.NewRunReader(openCorpus(t, dir))
	if err != nil {
		t.Fatalf("NewRunReader: %v", err)
	}

	ta := primitiveTally{requests: map[model.Position]counts{}, groups: map[model.Position]counts{}}

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
			ta.requests[pos] = countOutcome(t, ta.requests[pos], it.Sample.Outcome)
			ta.global = countOutcome(t, ta.global, it.Sample.Outcome)
		case model.ItemGroup:
			pos := it.Group.Position()
			ta.groups[pos] = countOutcome(t, ta.groups[pos], it.Group.Outcome)
		case model.ItemUser, model.ItemError, model.ItemAssertion:
			// Bounded above; nothing a report counts.
		case model.ItemUnknown:
			t.Fatal("the conversion produced an item of unknown kind")
		}
	}
}

func countOutcome(t *testing.T, c counts, o model.Outcome) counts {
	t.Helper()

	c.total++

	switch o {
	case model.OutcomeSuccess:
		c.ok++
	case model.OutcomeFailure:
		c.ko++
	case model.OutcomeUnknown:
		t.Fatal("the conversion produced an item with no outcome")
	}

	return c
}

// durationSec is the run span in whole seconds, rounded up, as Gatling's report
// computes it.
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

func spell(pos model.Position) handKey {
	if pos.Kind() == model.PositionGroup {
		return handKey{path: strings.Join(pos.Groups(), ",")}
	}

	return handKey{path: strings.Join(pos.Groups(), ","), name: pos.Name()}
}

// Two folds written independently — one by hand over the wire records, one
// keyed and bounded through the primitives — agree on every recording: the same
// rows, the same counts under each, the same span; and both agree with every
// account the run kept of its own numbers.
func TestPrimitiveFoldMatchesTheHandRolledOne(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			hand := foldByHand(t, dir)
			prim := foldPrimitives(t, dir)

			if prim.global != hand.global {
				t.Errorf("run totals: primitives %+v, hand %+v", prim.global, hand.global)
			}

			for source, want := range runAccounts(t, dir) {
				if prim.global != want {
					t.Errorf("primitives folded %+v; %s says %+v", prim.global, source, want)
				}
			}

			if len(prim.requests) != len(hand.requests) || len(prim.groups) != len(hand.groups) {
				t.Errorf("primitives found %d request and %d group positions, the hand-rolled fold %d and %d",
					len(prim.requests), len(prim.groups), len(hand.requests), len(hand.groups))
			}

			for pos, got := range prim.requests {
				if want, ok := hand.requests[spell(pos)]; !ok || got != want {
					t.Errorf("request %s: primitives %+v, hand %+v (found by hand: %t)", pos, got, want, ok)
				}
			}

			for pos, got := range prim.groups {
				if want, ok := hand.groups[spell(pos)]; !ok || got != want {
					t.Errorf("group %s: primitives %+v, hand %+v (found by hand: %t)", pos, got, want, ok)
				}
			}

			start, _ := prim.bounds.Start()
			end, _ := prim.bounds.End()

			if start.UnixMilli() != hand.start || end.UnixMilli() != hand.end {
				t.Errorf("bounds: primitives %d..%d, hand %d..%d", start.UnixMilli(), end.UnixMilli(), hand.start, hand.end)
			}
		})
	}
}

// The span the primitives fold is the one Gatling divided by. From 3.14.0 the
// console summary prints the mean throughput; before that, global_stats.json
// carries it as meanNumberOfRequestsPerSecond. Each figure comes out exactly at
// the precision the run printed it with.
func TestBoundsReproduceTheConsoleThroughput(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			prim := foldPrimitives(t, dir)
			dur := prim.durationSec(t)
			source, want := reportedThroughput(t, dir)

			ours := map[string]float64{
				"total": float64(prim.global.total) / dur,
				"ok":    float64(prim.global.ok) / dur,
				"ko":    float64(prim.global.ko) / dur,
			}

			for name, theirs := range want {
				if !sameAtPrecision(ours[name], theirs) {
					t.Errorf("%s: mean %s requests/sec = %v over %.0f s, %s says %s", filepath.Base(dir), name, ours[name], dur, source, theirs)
				}
			}
		})
	}
}

// throughputLine is the 3.14.0+ console shape:
//
//	> mean throughput (rps)   |      25.5 |        21 |       4.5
var throughputLine = regexp.MustCompile(`mean throughput \(rps\)\s*\|\s*([\d.,]+)\s*\|\s*([\d.,]+)\s*\|\s*([\d.,]+)`)

// reportedThroughput returns the run's own mean requests per second, split
// total/OK/KO, as strings so the comparison keeps the precision printed.
func reportedThroughput(t *testing.T, dir string) (source string, rates map[string]string) {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(dir, "console.txt")) //nolint:gosec // a corpus path
	if err != nil {
		t.Fatalf("%s: %v", dir, err)
	}

	if m := throughputLine.FindStringSubmatch(string(raw)); m != nil {
		return "console.txt", map[string]string{"total": m[1], "ok": m[2], "ko": m[3]}
	}

	f, err := os.Open(filepath.Join(dir, "js", "global_stats.json")) //nolint:gosec // a corpus path
	if err != nil {
		t.Fatalf("%s: neither console.txt nor js/global_stats.json carries a throughput: %v", dir, err)
	}
	defer func() { _ = f.Close() }()

	var doc struct {
		Mean map[string]json.Number `json:"meanNumberOfRequestsPerSecond"`
	}

	dec := json.NewDecoder(f)
	dec.UseNumber()

	if err := dec.Decode(&doc); err != nil {
		t.Fatalf("%s/js/global_stats.json: %v", dir, err)
	}

	return "global_stats.json", map[string]string{
		"total": doc.Mean["total"].String(), "ok": doc.Mean["ok"].String(), "ko": doc.Mean["ko"].String(),
	}
}

// sameAtPrecision compares ours to the printed figure at the precision it was
// printed with, which is what "exact" means for a printed double.
func sameAtPrecision(ours float64, theirs string) bool {
	theirs = strings.ReplaceAll(theirs, ",", "")

	decimals := 0
	if i := strings.IndexByte(theirs, '.'); i >= 0 {
		decimals = len(theirs) - i - 1
	}

	return strconv.FormatFloat(ours, 'f', decimals, 64) == theirs
}
