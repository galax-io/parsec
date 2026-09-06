// Package corpus reads the account a load-test run gave of itself.
//
// A Gatling run states its own numbers in whatever artefact its version
// happened to write. Up to 3.13.x that is a machine-readable stats.json; from
// 3.14.0 Gatling stopped writing it and the figures survive only in the
// generated index.html and in a console summary that exists at all only because
// it was redirected at run time. Three shapes, one subject: a tree of requests
// and groups, each carrying a total/ok/ko split.
//
// This package reads all three into that one shape, so a test compares against
// what the run said rather than against a file format. Which artefacts a
// recording carries is the tool's decision, recorded at capture time and never
// recoverable afterwards; [Accounts] reports what is actually there rather than
// assuming a version wrote what its predecessor did.
//
// This is test support. Nothing here decodes a simulation.log, nothing here
// computes a statistic, and nothing here is part of the module's public API.
package corpus

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// ErrNoFigures reports that an artefact exists and is well-formed for its
// version, but states no per-request figures this reader can extract.
//
// It is not a failure. Gatling 3.13.x writes an index.html whose statistics
// table is filled in by JavaScript at page load, so the figures are genuinely
// absent from the file; the JSON beside it carries them instead. A caller
// treats this as "this reader has nothing here" and moves to the next one.
//
// An artefact whose shape is not recognised at all is a different matter and
// comes back as an ordinary error, because a reader that silently found nothing
// in a report it was handed cannot be told from one that checked everything.
var ErrNoFigures = errors.New("the artefact states no per-request figures")

// NodeKind says what a row of a report describes.
type NodeKind int

// The three kinds of row a Gatling report states figures for.
const (
	// KindRoot is the run total, which Gatling labels "All Requests".
	KindRoot NodeKind = iota
	// KindGroup is one group traversal, counted per traversal and not per
	// request enclosed.
	KindGroup
	// KindRequest is one named request.
	KindRequest
)

// String names the kind for an error message.
func (k NodeKind) String() string {
	switch k {
	case KindRoot:
		return "root"
	case KindGroup:
		return "group"
	case KindRequest:
		return "request"
	default:
		return fmt.Sprintf("NodeKind(%d)", int(k))
	}
}

// Triple is the total/ok/ko split Gatling states every count in.
//
// These are counts of discrete events, so a comparison against them is exact:
// a decoder that loses or invents one is wrong rather than imprecise.
type Triple struct {
	Total int64
	OK    int64
	KO    int64
}

// Rates is the mean-requests-per-second figure, kept as the report printed it
// rather than as a float.
//
// The value is a printed double, and "equal" for one of those means equal at
// the precision it was printed with. Parsing it here would throw away the only
// evidence of what that precision was.
//
// OK and KO are empty when the artefact does not carry the split: the HTML
// report has a single events-per-second column and no per-outcome breakdown,
// where stats.json has all three. That absence is the report's, and a caller
// compares what is present rather than treating an empty figure as zero.
type Rates struct {
	Total json.Number
	OK    json.Number
	KO    json.Number
}

// Node is one row of a run's own report.
//
// ID and Parent are how the tree is rebuilt from an artefact that states it
// flat, and are that artefact's own spelling: the HTML report carries Gatling's
// internal row hashes, and a reader of a nested format synthesises them. They
// are not identity a caller should compare across two readings. [Report.Path]
// is: a node is addressed by the names of its ancestors, which is what both a
// report and a decoded log agree on.
type Node struct {
	ID       string
	Parent   string
	Name     string
	Kind     NodeKind
	Requests Triple
	Rate     Rates
}

// Report is one artefact's account of one run.
type Report struct {
	// Source names the artefact this was read from, so a failure says which
	// of a recording's several accounts disagreed.
	Source string
	// Nodes is every row, the root first.
	Nodes []Node
}

// Root returns the run total.
func (r Report) Root() (Node, bool) {
	for _, n := range r.Nodes {
		if n.Kind == KindRoot {
			return n, true
		}
	}

	return Node{}, false
}

// Path returns the names of a node's ancestors, outermost first, with the root
// excluded. A request's path is the group path it was recorded under; a group's
// path does not include its own name.
//
// It returns nil when the node is not part of this report.
func (r Report) Path(n Node) []string {
	byID := make(map[string]Node, len(r.Nodes))
	for _, m := range r.Nodes {
		byID[m.ID] = m
	}

	if _, ok := byID[n.ID]; !ok {
		return nil
	}

	var reversed []string

	for cur := n; cur.Parent != ""; {
		parent, ok := byID[cur.Parent]
		if !ok || parent.Kind == KindRoot {
			break
		}

		reversed = append(reversed, parent.Name)
		cur = parent
	}

	slices.Reverse(reversed)

	return reversed
}

// validate holds a freshly read report to the shape every reader must produce,
// so a reader that half-understood an artefact fails here rather than being
// compared against and quietly agreeing about less than it should.
func (r Report) validate() error {
	byID := make(map[string]Node, len(r.Nodes))
	roots := 0

	for _, n := range r.Nodes {
		if _, dup := byID[n.ID]; dup {
			return fmt.Errorf("%s: two rows share the id %q", r.Source, n.ID)
		}

		byID[n.ID] = n

		if n.Parent == "" {
			roots++

			if n.Kind != KindRoot {
				return fmt.Errorf("%s: row %q has no parent but is a %s, not the run total",
					r.Source, n.ID, n.Kind)
			}
		}
	}

	if roots != 1 {
		return fmt.Errorf("%s: %d rows have no parent; a report states exactly one run total", r.Source, roots)
	}

	return r.validateReachable(byID)
}

// requireTree rejects a report that states a run total and nothing beneath it.
//
// It is separate from [Report.validate] because root-only is a legitimate shape
// for one artefact and a symptom for the others: the console summary carries a
// Global Information block and nothing else, by design, while a stats.json or a
// generated report that yielded one row was not understood. Folding the two
// together would either lose the console as a source or let a half-read report
// pass as agreement.
func (r Report) requireTree() error {
	if len(r.Nodes) < 2 {
		return fmt.Errorf("%w: %s states a run total and nothing beneath it", ErrNoFigures, r.Source)
	}

	return nil
}

// validateReachable checks that every row hangs off the root through parents
// that exist. A row whose parent is missing, or a cycle, would make Path return
// a truncated or unbounded answer.
func (r Report) validateReachable(byID map[string]Node) error {
	for _, n := range r.Nodes {
		seen := map[string]bool{n.ID: true}

		for cur := n; cur.Parent != ""; {
			parent, ok := byID[cur.Parent]
			if !ok {
				return fmt.Errorf("%s: row %q names parent %q, which is not in the report",
					r.Source, cur.ID, cur.Parent)
			}

			if seen[parent.ID] {
				return fmt.Errorf("%s: row %q is its own ancestor", r.Source, n.ID)
			}

			seen[parent.ID] = true
			cur = parent
		}
	}

	return nil
}

// Accounts reads every account a recording gives of its own numbers, keyed by
// the artefact each came from.
//
// A reader whose artefact is absent, or whose artefact states no figures, is
// left out. A reader whose artefact is present but unreadable fails the call:
// the point of holding a decoder to a report is lost if an unreadable report
// reads as agreement.
//
// An empty result is not an error here — a caller decides what to do with a
// recording that proves nothing, and the answer differs between a test over the
// committed corpus and a canary over a run that finished a minute ago.
func Accounts(dir string) (map[string]Report, error) {
	readers := []struct {
		source string
		path   string
		read   func(string) (Report, error)
	}{
		{"stats.json", filepath.Join(dir, "stats.json"), FromStatsJSON},
		{"js/stats.json", filepath.Join(dir, "js", "stats.json"), FromStatsJSON},
		{"global_stats.json", filepath.Join(dir, "global_stats.json"), FromGlobalStatsJSON},
		{"js/global_stats.json", filepath.Join(dir, "js", "global_stats.json"), FromGlobalStatsJSON},
		{"index.html", filepath.Join(dir, "index.html"), FromReportHTML},
		{"console.txt", filepath.Join(dir, "console.txt"), FromConsole},
	}

	out := map[string]Report{}

	for _, rd := range readers {
		if _, err := os.Stat(rd.path); errors.Is(err, os.ErrNotExist) {
			continue
		}

		rep, err := rd.read(rd.path)

		switch {
		case errors.Is(err, ErrNoFigures):
			continue
		case err != nil:
			return nil, err
		}

		rep.Source = rd.source
		out[rd.source] = rep
	}

	return out, nil
}
