//go:build integration

package text_test

import (
	"path/filepath"
	"testing"

	"github.com/galax-io/parsec/gatling"
)

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
