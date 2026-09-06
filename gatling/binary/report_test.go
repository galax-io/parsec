package binary_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// counts is what a run said about itself: how many requests it recorded, and how
// many of them failed.
type counts struct {
	total, ok, ko int
}

// The accounts Gatling gives of a run's own numbers, and where each lives.
//
// Up to 3.13.x it still writes js/global_stats.json, the same machine-readable
// file the text corpus relies on. From 3.14.0 it stops, and the run's numbers
// survive only in the generated report and in the console summary — which is why
// the corpus keeps both, and why extraction here is shaped two ways rather than
// one. Nothing is extracted at recording time: the artefacts are kept as Gatling
// wrote them so a later reader can check what the run actually said.
func runAccounts(t *testing.T, dir string) map[string]counts {
	t.Helper()

	out := map[string]counts{}

	if c, ok := statsJSON(t, dir); ok {
		out["global_stats.json"] = c
	}

	out["console.txt"] = consoleSummary(t, dir)

	return out
}

// statsJSON reads js/global_stats.json, which only the 3.13.x line still writes.
func statsJSON(t *testing.T, dir string) (counts, bool) {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(dir, "js", "global_stats.json")) //nolint:gosec // a corpus path
	if err != nil {
		return counts{}, false
	}

	var doc struct {
		NumberOfRequests struct {
			Total int `json:"total"`
			OK    int `json:"ok"`
			KO    int `json:"ko"`
		} `json:"numberOfRequests"`
	}

	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("%s/js/global_stats.json: %v", dir, err)
	}

	return counts{doc.NumberOfRequests.Total, doc.NumberOfRequests.OK, doc.NumberOfRequests.KO}, true
}

// Two shapes of the same line, because Gatling reworded the console summary at
// 3.14.0:
//
//	3.13.x   > request count       102 (OK=84     KO=18    )
//	3.14.0+  > request count   |   102 |    84 |    18
var (
	consoleOld = regexp.MustCompile(`request count\s+(\d+) \(OK=(\d+)\s+KO=(\d+)`)
	consoleNew = regexp.MustCompile(`request count\s*\|\s*([\d,]+)\s*\|\s*([\d,]+)\s*\|\s*([\d,]+)`)
)

func consoleSummary(t *testing.T, dir string) counts {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(dir, "console.txt")) //nolint:gosec // a corpus path
	if err != nil {
		t.Fatalf("%s: the console summary exists only because it was redirected at run time: %v", dir, err)
	}

	for _, re := range []*regexp.Regexp{consoleNew, consoleOld} {
		if m := re.FindStringSubmatch(string(raw)); m != nil {
			return counts{number(t, m[1]), number(t, m[2]), number(t, m[3])}
		}
	}

	t.Fatalf("%s/console.txt carries no Global Information request count in either shape", dir)

	return counts{}
}

func number(t *testing.T, s string) int {
	t.Helper()

	n, err := strconv.Atoi(strings.ReplaceAll(s, ",", ""))
	if err != nil {
		t.Fatalf("%q is not a count: %v", s, err)
	}

	return n
}
