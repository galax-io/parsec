//go:build integration || canary

package binary_test

import (
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/galax-io/parsec/internal/corpus"
)

// The run's own report states a figure for every request and every group, not
// only for the run as a whole. Until there was something that could read those
// rows, the three numbers above were the whole of what a binary recording was
// held to — so a decoder that renamed every request, or moved one between
// groups, or flipped an outcome and compensated with another, passed.
//
// Each account is compared on its own. A recording carries several, they were
// written by the same run, and a disagreement between two of them is worth
// seeing rather than averaging away.
func TestDecodedPerRequestFiguresMatchTheRunReport(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			accounts, err := corpus.Accounts(dir)
			if err != nil {
				t.Fatalf("reading what the run said about itself: %v", err)
			}

			if len(accounts) == 0 {
				t.Fatal("the recording carries no account of its own numbers, so it proves nothing")
			}

			ta := foldByHand(t, dir)
			trees := 0

			for source, rep := range accounts {
				if len(rep.Nodes) > 1 {
					trees++
				}

				for _, node := range rep.Nodes {
					compareNode(t, source, rep, node, ta)
				}
			}

			if trees == 0 {
				t.Errorf("%s: no account states per-request rows, so only the run total was checked",
					filepath.Base(dir))
			}
		})
	}
}

// compareNode holds one row of a report to the fold of the decoded records.
//
// The counts are compared exactly: they count discrete events, and a decoder
// that loses or invents one is wrong rather than imprecise. The rate is a
// printed double and is compared at the precision the report printed it with,
// which is what "exact" means for one of those; a report that does not state it
// — the generated HTML has one events-per-second column and no per-outcome
// split — is not compared against a figure it never gave.
func compareNode(t *testing.T, source string, rep corpus.Report, node corpus.Node, ta handTally) {
	t.Helper()

	var (
		key  handKey
		got  counts
		ok   bool
		what string
	)

	switch node.Kind {
	case corpus.KindRoot:
		got, ok, what = ta.global, true, "the run total"
	case corpus.KindGroup:
		key = handKey{path: strings.Join(append(rep.Path(node), node.Name), ",")}
		got, ok = ta.groups[key]
		what = "group " + strconv.Quote(key.path)
	case corpus.KindRequest:
		key = handKey{path: strings.Join(rep.Path(node), ","), name: node.Name}
		got, ok = ta.requests[key]
		what = "request " + strconv.Quote(node.Name) + " under " + strconv.Quote(key.path)
	}

	if !ok {
		t.Errorf("%s: %s is in the report and not in the decoded log", source, what)

		return
	}

	if want := (counts{int(node.Requests.Total), int(node.Requests.OK), int(node.Requests.KO)}); got != want {
		t.Errorf("%s: %s decoded %d (%d ok, %d ko); the report says %d (%d ok, %d ko)",
			source, what, got.total, got.ok, got.ko, want.total, want.ok, want.ko)
	}

	compareRate(t, source, what, got, node.Rate, handDuration(ta))
}

// compareRate checks the mean-requests-per-second figures the report states.
func compareRate(t *testing.T, source, what string, got counts, rate corpus.Rates, dur float64) {
	t.Helper()

	for _, r := range []struct {
		name   string
		ours   int
		theirs string
	}{
		{"total", got.total, rate.Total.String()},
		{"ok", got.ok, rate.OK.String()},
		{"ko", got.ko, rate.KO.String()},
	} {
		if r.theirs == "" {
			continue
		}

		if ours := float64(r.ours) / dur; !sameAtPrecision(ours, r.theirs) {
			t.Errorf("%s: %s mean %s requests/sec = %v; the report says %s",
				source, what, r.name, ours, r.theirs)
		}
	}
}

// handDuration is the run span in whole seconds, rounded up, as the report
// computes it.
func handDuration(ta handTally) float64 {
	return math.Ceil(float64(ta.end-ta.start) / 1000)
}

// Every row the report names must be in the decoded log, and every request and
// group the log records must be in the report. compareNode checks one
// direction; this checks the other, because a decoder that invented a request
// would satisfy the first and fail nobody.
func TestEveryDecodedRequestAndGroupIsInTheReport(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			accounts, err := corpus.Accounts(dir)
			if err != nil {
				t.Fatalf("reading what the run said about itself: %v", err)
			}

			named := map[handKey]bool{}

			for _, rep := range accounts {
				for _, node := range rep.Nodes {
					switch node.Kind {
					case corpus.KindGroup:
						named[handKey{path: strings.Join(append(rep.Path(node), node.Name), ",")}] = true
					case corpus.KindRequest:
						named[handKey{path: strings.Join(rep.Path(node), ","), name: node.Name}] = true
					case corpus.KindRoot:
					}
				}
			}

			ta := foldByHand(t, dir)

			for k := range ta.requests {
				if !named[k] {
					t.Errorf("the log records request %q under %q; no account of the run names it",
						k.name, k.path)
				}
			}

			for k := range ta.groups {
				if !named[k] {
					t.Errorf("the log records group %q; no account of the run names it", k.path)
				}
			}
		})
	}
}
