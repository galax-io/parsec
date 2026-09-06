//go:build canary

package text_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/text"
)

// canaryRun is one run a fresh Gatling produced moments ago: the version that
// was asked for, and the directory holding simulation.log and js/.
type canaryRun struct {
	version gatling.Version
	dir     string
}

// canaryRuns reads PARSEC_CANARY_RUNS, "version=dir" pairs separated by ";".
// Without it the canary skips with a reason: the constitution asks a test that
// needs a real tool to say so rather than fake it. In CI the job fails when
// no canary test passed, so a skip can never pass for a run.
func canaryRuns(t *testing.T) []canaryRun {
	t.Helper()

	spec := os.Getenv("PARSEC_CANARY_RUNS")
	if spec == "" {
		t.Skip("PARSEC_CANARY_RUNS is not set: point it at version=dir pairs of fresh Gatling runs, separated by \";\"")
	}

	var runs []canaryRun

	for pair := range strings.SplitSeq(spec, ";") {
		version, dir, ok := strings.Cut(pair, "=")
		if !ok {
			t.Fatalf("PARSEC_CANARY_RUNS entry %q is not version=dir", pair)
		}

		v, err := gatling.ParseVersion(version)
		if err != nil {
			t.Fatalf("PARSEC_CANARY_RUNS: %v", err)
		}

		// Only the runs this codec reads. From v0.0.7 the canary list carries
		// every supported version of both formats in one value, because the
		// cross-format comparison needs one of each; each codec takes its own
		// and leaves the rest to the other. The log is asked rather than the
		// version, so a release that changed format without saying so is
		// noticed here instead of being decoded by the wrong codec.
		log := filepath.Join(dir, "simulation.log")

		format, err := detectFormat(log)
		if err != nil {
			t.Fatalf("Gatling %s left no readable log at %s: %v", v, log, err)
		}

		if format != gatling.FormatText {
			continue
		}

		runs = append(runs, canaryRun{version: v, dir: dir})
	}

	if len(runs) == 0 {
		t.Skipf("PARSEC_CANARY_RUNS names no run whose simulation.log is a text one")
	}

	return runs
}

// summarize records a line in the job summary when there is one, and always
// in the test log, so the canary states which versions it tested.
func summarize(t *testing.T, format string, args ...any) {
	t.Helper()

	line := fmt.Sprintf(format, args...)
	t.Log(line)

	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		return
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600) //nolint:gosec // the path is the runner's own summary file
	if err != nil {
		t.Logf("cannot write the job summary: %v", err)

		return
	}

	defer func() { _ = f.Close() }()

	if _, err := fmt.Fprintf(f, "- %s\n", line); err != nil {
		t.Logf("cannot write the job summary: %v", err)
	}
}

// TestCanary decodes every fresh run and holds it to the report that run's
// own Gatling generated, exactly as TestReport holds the decoder to the
// recorded corpus. A failure names the version through the subtest and the
// check through the assertion.
func TestCanary(t *testing.T) {
	t.Parallel()

	for _, run := range canaryRuns(t) {
		t.Run(run.version.String(), func(t *testing.T) {
			t.Parallel()

			log := filepath.Join(run.dir, "simulation.log")

			f, err := os.Open(log) //nolint:gosec // a run directory the workflow just produced
			if err != nil {
				t.Fatalf("Gatling %s left no log: %v", run.version, err)
			}

			defer func() { _ = f.Close() }()

			rd, err := text.NewReader(f)
			if err != nil {
				t.Fatalf("Gatling %s: the decoder refused the log: %v", run.version, err)
			}

			if got := rd.Header().Version; got != run.version {
				t.Fatalf("Gatling %s wrote a log naming %s", run.version, got)
			}

			for _, w := range rd.Warnings() {
				summarize(t, "Gatling %s decoded unverified: %s — record a corpus entry and widen SupportedVersions", run.version, w)
			}

			ta := decodeTally(t, log)
			dur := ta.durationSec()

			var global reportStats
			loadJSON(t, filepath.Join(run.dir, "js", "global_stats.json"), &global)
			checkCounts(t, "global_stats.json", ta.global, global.NumberOfRequests)
			checkRates(t, "global_stats.json", ta.global, dur, global.MeanNumberOfRequestsPerSecond)

			var root reportNode
			loadJSON(t, filepath.Join(run.dir, "js", "stats.json"), &root)
			checkCounts(t, "stats.json root", ta.global, root.Stats.NumberOfRequests)
			checkRates(t, "stats.json root", ta.global, dur, root.Stats.MeanNumberOfRequestsPerSecond)

			seen := 0
			walk(t, root, nil, ta, dur, &seen)

			if want := len(ta.requests) + len(ta.groups); seen != want {
				t.Errorf("stats.json describes %d requests and groups, the log has %d", seen, want)
			}

			summarize(t, "Gatling %s: %d requests (%d ok, %d ko), %d groups, %d user starts, %d user ends, %d errors, span %.0fs — matched its own report",
				run.version, ta.global.total, ta.global.ok, ta.global.ko, len(ta.groups),
				ta.users[gatling.EventStart], ta.users[gatling.EventEnd], ta.errors, dur)
		})
	}
}

// TestCanaryCrossVersion holds every fresh run to every other: the same
// simulation must decode to the same multiset of records under each version
// once timing, identity and order are set aside.
func TestCanaryCrossVersion(t *testing.T) {
	t.Parallel()

	runs := canaryRuns(t)
	if len(runs) < 2 {
		t.Skip("cross-version equality needs at least two versions")
	}

	base := maskedSorted(t, runs[0].dir)

	for _, run := range runs[1:] {
		other := maskedSorted(t, run.dir)

		if len(base) != len(other) {
			t.Errorf("Gatling %s produced %d lines, %s produced %d", runs[0].version, len(base), run.version, len(other))
		}

		for i := 0; i < len(base) && i < len(other); i++ {
			if base[i] != other[i] {
				t.Fatalf("Gatling %s and %s disagree once timing, identity and order are set aside:\n %s\n %s",
					runs[0].version, run.version, base[i], other[i])
			}
		}

		summarize(t, "Gatling %s and %s: identical as a multiset (%d lines)", runs[0].version, run.version, len(base))
	}
}

// TestCanaryCoversSupportedRange fails when the supported range was widened
// without the canary running the new bound, so the two cannot drift apart.
func TestCanaryCoversSupportedRange(t *testing.T) {
	t.Parallel()

	oldest, newest := text.SupportedVersions()
	ran := map[gatling.Version]bool{}

	for _, run := range canaryRuns(t) {
		ran[run.version] = true
	}

	for _, bound := range []gatling.Version{oldest, newest} {
		if !ran[bound] {
			t.Errorf("SupportedVersions covers %s but the canary did not run it: add it to the version list", bound)
		}
	}
}
