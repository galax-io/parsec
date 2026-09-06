package corpus

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// rootID is what every reader calls the run total.
//
// The artefacts disagree: the HTML report labels that row ROOT, while
// stats.json gives it the same hashed spelling it gives a real group —
// group_missing-name--1146707516, because the run total is modelled as a group
// with no name. Every other row carries Gatling's own identity unchanged, and
// the two artefacts agree on those exactly, so normalising this one row is what
// lets a JSON reading and an HTML reading of the same run be compared at all.
const rootID = "ROOT"

// statsFigures is the figure block shared by global_stats.json and by every
// node of stats.json.
//
// The counts and rates are maps keyed "total", "ok" and "ko" — Gatling's own
// shape — and every value is kept as json.Number so a printed double survives
// as the text it was printed as.
type statsFigures struct {
	NumberOfRequests              map[string]json.Number `json:"numberOfRequests"`
	MeanNumberOfRequestsPerSecond map[string]json.Number `json:"meanNumberOfRequestsPerSecond"`
}

// statsNode is one node of stats.json.
type statsNode struct {
	Type          string               `json:"type"`
	Name          string               `json:"name"`
	PathFormatted string               `json:"pathFormatted"`
	Stats         statsFigures         `json:"stats"`
	Contents      map[string]statsNode `json:"contents"`
}

// FromStatsJSON reads the per-request tree out of a Gatling stats.json.
//
// It is written up to Gatling 3.13.x, at the run directory's root through
// 3.12.x and under js/ from 3.13.0. From 3.14.0 Gatling writes no such file and
// [FromReportHTML] is the reader for that line.
func FromStatsJSON(path string) (Report, error) {
	var doc statsNode
	if err := decodeJSON(path, &doc); err != nil {
		return Report{}, err
	}

	if doc.Stats.NumberOfRequests == nil {
		return Report{}, fmt.Errorf("%w: %s carries no numberOfRequests", ErrNoFigures, path)
	}

	rep := Report{Source: path}

	if err := appendStatsNode(&rep, doc, "", true); err != nil {
		return Report{}, fmt.Errorf("%s: %w", path, err)
	}

	if err := rep.validate(); err != nil {
		return Report{}, err
	}

	if err := rep.requireTree(); err != nil {
		return Report{}, err
	}

	return rep, nil
}

// FromGlobalStatsJSON reads global_stats.json, which states the run total and
// nothing beneath it.
//
// It is a second, independent account of the same figures stats.json states at
// its root — written by the same versions, in a separate file — and is read as
// its own source so that a disagreement between the two is visible rather than
// averaged away.
func FromGlobalStatsJSON(path string) (Report, error) {
	var doc statsFigures
	if err := decodeJSON(path, &doc); err != nil {
		return Report{}, err
	}

	if doc.NumberOfRequests == nil {
		return Report{}, fmt.Errorf("%w: %s carries no numberOfRequests", ErrNoFigures, path)
	}

	requests, err := triple(doc.NumberOfRequests)
	if err != nil {
		return Report{}, fmt.Errorf("%s: %w", path, err)
	}

	return Report{
		Source: path,
		Nodes: []Node{{
			ID:       rootID,
			Name:     "All Requests",
			Kind:     KindRoot,
			Requests: requests,
			Rate:     rates(doc.MeanNumberOfRequestsPerSecond),
		}},
	}, nil
}

// appendStatsNode flattens one node and its children into the report.
//
// Children are visited in sorted key order. The JSON object they live in has no
// order of its own, and a reader that passed Go's map iteration through to its
// output would produce a different slice on every run — which a test comparing
// two readings could not use.
func appendStatsNode(rep *Report, node statsNode, parent string, isRoot bool) error {
	kind, err := statsKind(node, isRoot)
	if err != nil {
		return err
	}

	id := node.PathFormatted
	if isRoot {
		id = rootID
	}

	requests, err := triple(node.Stats.NumberOfRequests)
	if err != nil {
		return fmt.Errorf("row %q: %w", node.Name, err)
	}

	rep.Nodes = append(rep.Nodes, Node{
		ID:       id,
		Parent:   parent,
		Name:     node.Name,
		Kind:     kind,
		Requests: requests,
		Rate:     rates(node.Stats.MeanNumberOfRequestsPerSecond),
	})

	keys := make([]string, 0, len(node.Contents))
	for k := range node.Contents {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		if err := appendStatsNode(rep, node.Contents[k], id, false); err != nil {
			return err
		}
	}

	return nil
}

// statsKind maps Gatling's own node type onto a kind. The run total is written
// as a GROUP with no name, so the caller says which node is the root rather
// than the file.
func statsKind(node statsNode, isRoot bool) (NodeKind, error) {
	if isRoot {
		return KindRoot, nil
	}

	switch node.Type {
	case "GROUP":
		return KindGroup, nil
	case "REQUEST":
		return KindRequest, nil
	default:
		return 0, fmt.Errorf("row %q has type %q, which is neither GROUP nor REQUEST", node.Name, node.Type)
	}
}

// triple reads Gatling's total/ok/ko count map. A missing or non-integer figure
// fails: these are counts, and a report that cannot state one is not something
// to compare against.
func triple(m map[string]json.Number) (Triple, error) {
	var out Triple

	for name, into := range map[string]*int64{"total": &out.Total, "ok": &out.OK, "ko": &out.KO} {
		raw, ok := m[name]
		if !ok {
			return Triple{}, fmt.Errorf("the count map has no %q", name)
		}

		n, err := raw.Int64()
		if err != nil {
			return Triple{}, fmt.Errorf("the %q count is %q, which is not an integer", name, raw)
		}

		*into = n
	}

	return out, nil
}

// rates reads the mean-requests-per-second map, leaving every figure as the
// text it was printed as. An absent split comes back empty rather than zero.
func rates(m map[string]json.Number) Rates {
	return Rates{Total: m["total"], OK: m["ok"], KO: m["ko"]}
}

// decodeJSON reads one JSON artefact with numbers left as text.
func decodeJSON(path string, into any) error {
	f, err := os.Open(path) //nolint:gosec // a path inside a recorded run directory, chosen by the caller
	if err != nil {
		return fmt.Errorf("opening the report: %w", err)
	}

	defer func() { _ = f.Close() }()

	dec := json.NewDecoder(f)
	dec.UseNumber()

	if err := dec.Decode(into); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	return nil
}
