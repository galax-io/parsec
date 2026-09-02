//go:build integration

package text_test

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/text"
)

// triple is the total/ok/ko split Gatling reports every count and rate in.
type triple struct{ total, ok, ko int64 }

func (c *triple) add(s gatling.Status) {
	c.total++

	if s == gatling.StatusOK {
		c.ok++
	} else {
		c.ko++
	}
}

// statsKey identifies a request by its group path and name, or a group by its
// path alone (empty name).
type statsKey struct{ path, name string }

// tally is everything the report can be checked against, derived from the
// decoded records alone.
type tally struct {
	global   triple
	requests map[statsKey]triple
	groups   map[statsKey]triple
	users    map[gatling.Event]int
	errors   int
	// injectStart and injectEnd bound the run span exactly as the report bounds it
	// (FR-021c): request and group starts and user START for the start; request and
	// group ends and any user event for the end. The header and errors do not count.
	injectStart, injectEnd int64
}

func decodeTally(t *testing.T, path string) tally {
	t.Helper()

	f, err := os.Open(path) //nolint:gosec // a corpus path from the test's own glob
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	rd, err := text.NewReader(f)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	ta := tally{
		requests:    map[statsKey]triple{},
		groups:      map[statsKey]triple{},
		users:       map[gatling.Event]int{},
		injectStart: math.MaxInt64,
		injectEnd:   math.MinInt64,
	}

	for {
		rec, err := rd.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			t.Fatalf("Next: %v", err)
		}

		switch rec.Kind {
		case gatling.KindRequest:
			k := statsKey{path: strings.Join(rec.Groups, ","), name: rec.Name}
			c := ta.requests[k]
			c.add(rec.Status)
			ta.requests[k] = c
			ta.global.add(rec.Status)
			ta.injectStart = min(ta.injectStart, rec.Start)
			ta.injectEnd = max(ta.injectEnd, rec.End)
		case gatling.KindGroup:
			k := statsKey{path: strings.Join(rec.Groups, ",")}
			c := ta.groups[k]
			c.add(rec.Status)
			ta.groups[k] = c
			ta.injectStart = min(ta.injectStart, rec.Start)
			ta.injectEnd = max(ta.injectEnd, rec.End)
		case gatling.KindUser:
			ta.users[rec.Event]++
			if rec.Event == gatling.EventStart {
				ta.injectStart = min(ta.injectStart, rec.Timestamp)
			}

			ta.injectEnd = max(ta.injectEnd, rec.Timestamp)
		case gatling.KindError:
			ta.errors++
		case gatling.KindRun, gatling.KindAssertion, gatling.KindUnknown:
		}
	}

	return ta
}

// durationSec is the run span in whole seconds, rounded up, as the report computes it.
func (ta tally) durationSec() float64 {
	return math.Ceil(float64(ta.injectEnd-ta.injectStart) / 1000)
}

// reportStats is the shape shared by global_stats.json and every node of stats.json.
type reportStats struct {
	NumberOfRequests              map[string]json.Number `json:"numberOfRequests"`
	MeanNumberOfRequestsPerSecond map[string]json.Number `json:"meanNumberOfRequestsPerSecond"`
}

type reportNode struct {
	Type     string                `json:"type"`
	Name     string                `json:"name"`
	Stats    reportStats           `json:"stats"`
	Contents map[string]reportNode `json:"contents"`
}

func loadJSON(t *testing.T, path string, into any) {
	t.Helper()

	f, err := os.Open(path) //nolint:gosec // a corpus path from the test's own glob
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	dec := json.NewDecoder(f)
	dec.UseNumber()

	if err := dec.Decode(into); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

// sameAtPrecision compares ours to the report's figure at the precision the
// report printed it with, which is what "exact" means for a printed double.
func sameAtPrecision(ours float64, theirs json.Number) bool {
	s := theirs.String()

	decimals := 0
	if i := strings.IndexByte(s, '.'); i >= 0 {
		decimals = len(s) - i - 1
	}

	return strconv.FormatFloat(ours, 'f', decimals, 64) == s
}

func checkCounts(t *testing.T, what string, got triple, want map[string]json.Number) {
	t.Helper()

	for name, val := range map[string]int64{"total": got.total, "ok": got.ok, "ko": got.ko} {
		theirs, err := want[name].Int64()
		if err != nil {
			t.Fatalf("%s: %s count %q is not an integer", what, name, want[name])
		}

		if val != theirs {
			t.Errorf("%s: %s = %d, report says %d", what, name, val, theirs)
		}
	}
}

func checkRates(t *testing.T, what string, got triple, dur float64, want map[string]json.Number) {
	t.Helper()

	for name, val := range map[string]int64{"total": got.total, "ok": got.ok, "ko": got.ko} {
		ours := float64(val) / dur
		if !sameAtPrecision(ours, want[name]) {
			t.Errorf("%s: mean %s requests/sec = %v, report says %s", what, name, ours, want[name])
		}
	}
}

func TestReport(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			ta := decodeTally(t, filepath.Join(dir, "simulation.log"))
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
				t.Errorf("stats.json describes %d requests and groups, the log has %d", seen, want)
			}

			t.Logf("%s: %d requests (%d ok, %d ko), %d groups, %d user starts, %d user ends, %d errors, span %.0fs",
				filepath.Base(dir), ta.global.total, ta.global.ok, ta.global.ko, len(ta.groups),
				ta.users[gatling.EventStart], ta.users[gatling.EventEnd], ta.errors, dur)
		})
	}
}

func walk(t *testing.T, node reportNode, path []string, ta tally, dur float64, seen *int) {
	t.Helper()

	for _, child := range node.Contents {
		var (
			k    statsKey
			got  triple
			ok   bool
			what string
		)

		switch child.Type {
		case "REQUEST":
			k = statsKey{path: strings.Join(path, ","), name: child.Name}
			got, ok = ta.requests[k]
			what = "request " + strconv.Quote(child.Name) + " under " + strconv.Quote(k.path)
		case "GROUP":
			k = statsKey{path: strings.Join(append(append([]string{}, path...), child.Name), ",")}
			got, ok = ta.groups[k]
			what = "group " + strconv.Quote(k.path)
		default:
			t.Fatalf("stats.json node %q has unknown type %q", child.Name, child.Type)
		}

		if !ok {
			t.Errorf("%s is in the report but not in the decoded log", what)

			continue
		}

		*seen++

		checkCounts(t, what, got, child.Stats.NumberOfRequests)
		checkRates(t, what, got, dur, child.Stats.MeanNumberOfRequestsPerSecond)

		if child.Type == "GROUP" {
			walk(t, child, append(append([]string{}, path...), child.Name), ta, dur, seen)
		}
	}
}
