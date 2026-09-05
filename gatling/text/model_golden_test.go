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
	"time"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/text"
	"github.com/galax-io/parsec/model"
)

// modelTally is decodeTally's counterpart, folded from the canonical types
// instead of the wire records. It produces the same tally so the very same
// report-checking machinery judges both, and a disagreement can only be the
// conversion's.
func modelTally(t *testing.T, path string) tally {
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

	ta := tally{
		requests:    map[statsKey]triple{},
		groups:      map[statsKey]triple{},
		users:       map[gatling.Event]int{},
		injectStart: math.MaxInt64,
		injectEnd:   math.MinInt64,
	}

	for {
		it, err := rd.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			t.Fatalf("Next: %v", err)
		}

		switch it.Kind {
		case model.ItemSample:
			k := statsKey{path: strings.Join(it.Sample.Groups, ","), name: it.Sample.Name}
			c := ta.requests[k]
			c.add(status(it.Sample.Outcome))
			ta.requests[k] = c
			ta.global.add(status(it.Sample.Outcome))
			ta.injectStart = min(ta.injectStart, it.Sample.Start.UnixMilli())
			ta.injectEnd = max(ta.injectEnd, endAfter(it.Sample.Start, it.Sample.Duration))
		case model.ItemGroup:
			k := statsKey{path: strings.Join(it.Group.Groups, ",")}
			c := ta.groups[k]
			c.add(status(it.Group.Outcome))
			ta.groups[k] = c
			ta.injectStart = min(ta.injectStart, it.Group.Start.UnixMilli())
			ta.injectEnd = max(ta.injectEnd, endAfter(it.Group.Start, it.Group.Duration))
		case model.ItemUser:
			if it.User.Kind == model.UserStart {
				ta.users[gatling.EventStart]++
				ta.injectStart = min(ta.injectStart, it.User.At.UnixMilli())
			} else {
				ta.users[gatling.EventEnd]++
			}

			ta.injectEnd = max(ta.injectEnd, it.User.At.UnixMilli())
		case model.ItemError:
			ta.errors++
		case model.ItemUnknown:
			t.Fatal("the conversion produced an item of unknown kind")
		}
	}

	return ta
}

// status maps an outcome back to the wire status the shared triple counts in.
func status(o model.Outcome) gatling.Status {
	if o == model.OutcomeSuccess {
		return gatling.StatusOK
	}

	return gatling.StatusKO
}

// endAfter reconstructs an end timestamp from a start and a wall-clock
// duration. An event whose end was unusable carries no duration and cannot
// extend the run span, which is what the wire path does with such a record too.
func endAfter(start time.Time, d model.Opt[time.Duration]) int64 {
	v, ok := d.Get()
	if !ok {
		return math.MinInt64
	}

	return start.Add(v).UnixMilli()
}

// TestModelAgainstReport is TestReport's counterpart through the canonical
// types: every count the kept report files carry, matched exactly by the model.
func TestModelAgainstReport(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			ta := modelTally(t, filepath.Join(dir, "simulation.log"))
			// The span every rate divides by comes from the model alone: it is
			// bounded by sample, group and user timestamps, and the model
			// carries all three.
			dur := ta.durationSec()

			var global reportStats
			loadJSON(t, filepath.Join(dir, "global_stats.json"), &global)
			checkCounts(t, "global_stats.json", ta.global, global.NumberOfRequests)
			checkRates(t, "global_stats.json", ta.global, dur, global.MeanNumberOfRequestsPerSecond)

			var root reportNode
			loadJSON(t, filepath.Join(dir, "stats.json"), &root)
			checkCounts(t, "stats.json root", ta.global, root.Stats.NumberOfRequests)
			checkRates(t, "stats.json root", ta.global, dur, root.Stats.MeanNumberOfRequestsPerSecond)

			seen := 0
			walk(t, root, nil, ta, dur, &seen)

			if want := len(ta.requests) + len(ta.groups); seen != want {
				t.Errorf("stats.json describes %d requests and groups, the model has %d", seen, want)
			}
		})
	}
}

// The conversion preserves counts exactly: one item per event record, and the
// same split by outcome. A disagreement here is the conversion's alone, because
// both sides read the same log with the same decoder.
func TestModelMatchesTheWireRecords(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			log := filepath.Join(dir, "simulation.log")
			wire := decodeTally(t, log)
			mdl := modelTally(t, log)

			if wire.global != mdl.global {
				t.Errorf("run totals: model %+v, wire %+v", mdl.global, wire.global)
			}

			if len(wire.requests) != len(mdl.requests) {
				t.Errorf("model has %d request positions, wire has %d", len(mdl.requests), len(wire.requests))
			}

			for k, want := range wire.requests {
				if got := mdl.requests[k]; got != want {
					t.Errorf("request %v: model %+v, wire %+v", k, got, want)
				}
			}

			for k, want := range wire.groups {
				if got := mdl.groups[k]; got != want {
					t.Errorf("group %v: model %+v, wire %+v", k, got, want)
				}
			}

			for _, e := range []gatling.Event{gatling.EventStart, gatling.EventEnd} {
				if wire.users[e] != mdl.users[e] {
					t.Errorf("user %v: model %d, wire %d", e, mdl.users[e], wire.users[e])
				}
			}

			if wire.errors != mdl.errors {
				t.Errorf("run errors: model %d, wire %d", mdl.errors, wire.errors)
			}

			t.Logf("%s through the model: %d requests (%d ok, %d ko), %d groups, %d user starts, %d user ends, %d errors",
				filepath.Base(dir), mdl.global.total, mdl.global.ok, mdl.global.ko, len(mdl.groups),
				mdl.users[gatling.EventStart], mdl.users[gatling.EventEnd], mdl.errors)
		})
	}
}
