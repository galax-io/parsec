//go:build integration || canary

// Shared by the end-to-end suite over the recorded corpus and by the canary
// over runs a fresh Gatling produced moments ago: both hold the decoder to a
// run's own report and to the other runs of the same simulation.

package text_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/text"
)

// corpusDirs lists every recorded run. It fails, rather than skips, when there
// is none: an empty integration run is a failure by constitution.
func corpusDirs(t *testing.T) []string {
	t.Helper()

	logs, err := filepath.Glob(filepath.Join("..", "..", "testdata", "corpus", "gatling", "*", "simulation.log"))
	if err != nil {
		t.Fatal(err)
	}

	if len(logs) == 0 {
		t.Fatal("no recorded run under testdata/corpus/gatling/<version>/ — the corpus is missing, not optional")
	}

	dirs := make([]string, 0, len(logs))
	for _, log := range logs {
		dirs = append(dirs, filepath.Dir(log))
	}

	return dirs
}

// canonical decodes a whole log into its canonical text form: one line per
// item, every string %q-quoted so a lone space and an empty string differ.
func canonical(r io.Reader) ([]byte, error) {
	rd, err := text.NewReader(r)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer

	h := rd.Header()
	fmt.Fprintf(&b, "HEADER class=%q run=%q start=%d description=%q version=%s\n",
		h.SimulationClass, h.RunID, h.Start, h.Description, h.Version)

	for _, p := range rd.Assertions() {
		fmt.Fprintf(&b, "ASSERTION payload=%q\n", p)
	}

	for _, w := range rd.Warnings() {
		fmt.Fprintf(&b, "WARNING %s\n", w.String())
	}

	for {
		rec, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return b.Bytes(), nil
		}

		if err != nil {
			return nil, err
		}

		writeRecord(&b, rec)
	}
}

func writeRecord(b *bytes.Buffer, rec gatling.Record) {
	switch rec.Kind {
	case gatling.KindUser:
		fmt.Fprintf(b, "%d USER scenario=%q event=%s timestamp=%d\n", rec.Line, rec.Scenario, rec.Event, rec.Timestamp)
	case gatling.KindRequest:
		fmt.Fprintf(b, "%d REQUEST groups=%q name=%q start=%d end=%d status=%s message=%q\n",
			rec.Line, rec.Groups, rec.Name, rec.Start, rec.End, rec.Status, rec.Message)
	case gatling.KindGroup:
		fmt.Fprintf(b, "%d GROUP groups=%q start=%d end=%d cumulated=%d status=%s\n",
			rec.Line, rec.Groups, rec.Start, rec.End, rec.CumulatedResponseTime, rec.Status)
	case gatling.KindError:
		fmt.Fprintf(b, "%d ERROR message=%q timestamp=%d\n", rec.Line, rec.Message, rec.Timestamp)
	case gatling.KindAssertion:
		fmt.Fprintf(b, "%d ASSERTION payload=%q\n", rec.Line, rec.Payload)
	case gatling.KindRun, gatling.KindUnknown:
		fmt.Fprintf(b, "%d %s unexpected in the event stream\n", rec.Line, rec.Kind)
	default:
		fmt.Fprintf(b, "%d %s\n", rec.Line, rec.Kind)
	}
}

// firstDiff reports the first differing line of two canonical streams.
func firstDiff(got, want []byte) string {
	g, w := bytes.Split(got, []byte("\n")), bytes.Split(want, []byte("\n"))
	for i := 0; i < len(g) && i < len(w); i++ {
		if !bytes.Equal(g[i], w[i]) {
			return fmt.Sprintf("line %d:\n got  %s\n want %s", i+1, g[i], w[i])
		}
	}

	return fmt.Sprintf("got %d lines, want %d", len(g), len(w))
}

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

// Two recordings of the same simulation legitimately differ in three ways:
// every timing value (timestamps and cumulated response times), the run's
// identity (id and Gatling version), and file order — concurrent virtual users
// interleave differently on every run, so order is not evidence of anything.
// Everything else must agree exactly, as a multiset.
//
// The version gate's warning goes with identity. It is a statement about which
// version wrote the log, not about what the log holds, and only a run above the
// supported range carries one — so comparing it would make an above-range run
// unequal to an in-range one by construction, which is precisely the case the
// canary's version list exists to try.
var (
	timing   = regexp.MustCompile(`\b(start|end|timestamp|cumulated)=-?\d+`)
	identity = regexp.MustCompile(`\brun="[^"]*"|\bversion=\S+`)
	lineNo   = regexp.MustCompile(`(?m)^\d+ `)
)

func maskedSorted(t *testing.T, dir string) []string {
	t.Helper()

	f, err := os.Open(filepath.Join(dir, "simulation.log")) //nolint:gosec // a corpus path from the test's own glob
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	stream, err := canonical(f)
	if err != nil {
		t.Fatalf("%s: %v", dir, err)
	}

	s := string(stream)
	s = timing.ReplaceAllString(s, "$1=…")
	s = identity.ReplaceAllString(s, "…")
	s = lineNo.ReplaceAllString(s, "")

	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	lines = slices.DeleteFunc(lines, func(l string) bool { return strings.HasPrefix(l, "WARNING ") })
	slices.Sort(lines)

	return lines
}
