//go:build canary

package binary_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
	"github.com/galax-io/parsec/internal/corpus"
)

// The recorded corpus proves this decoder against the past. These tests prove it
// against a Gatling that ran a minute ago — which for the binary codec nothing
// did until v0.0.7: the canary ran 3.11.5 and 3.12.0, both text, so the newer
// and riskier half of the module was held only to recordings.
//
// Every test here skips with a reason when PARSEC_CANARY_RUNS is unset, because
// a test that needs a real tool says so rather than faking it. The workflow
// counts passing tests and fails when none passed, so a skip can never be
// mistaken for a run.

// canaryRuns is the runs this codec is responsible for: the ones whose log this
// codec actually reads. A value naming runs of both formats is normal — the
// cross-format comparison needs one of each — and each codec takes its own.
func canaryRuns(t *testing.T) []corpus.Run {
	t.Helper()

	spec := os.Getenv(corpus.RunsEnv)
	if spec == "" {
		t.Skipf("%s is not set: point it at version=dir pairs of fresh Gatling runs, separated by \";\"",
			corpus.RunsEnv)
	}

	all, err := corpus.ParseRuns(spec)
	if err != nil {
		t.Fatal(err)
	}

	var mine []corpus.Run

	for _, run := range all {
		if isBinaryLog(t, filepath.Join(run.Dir, "simulation.log")) {
			mine = append(mine, run)
		}
	}

	if len(mine) == 0 {
		t.Skipf("%s names no run whose simulation.log is a binary one", corpus.RunsEnv)
	}

	return mine
}

// isBinaryLog asks the log which format it is, rather than inferring it from the
// version. A version that changed format without saying so is exactly what a
// canary exists to notice.
func isBinaryLog(t *testing.T, path string) bool {
	t.Helper()

	f, err := os.Open(path) //nolint:gosec // a run directory the workflow just produced
	if err != nil {
		t.Fatalf("Gatling left no log at %s: %v", path, err)
	}

	defer func() { _ = f.Close() }()

	return format(t, path) == gatling.FormatBinary
}

func summarize(t *testing.T, format string, args ...any) {
	t.Helper()

	line := fmt.Sprintf(format, args...)
	t.Log(line)
	corpus.Summarize(line)
}

// TestCanary decodes every fresh run and holds it to the account that run's own
// Gatling gave of itself — the same comparison the recorded corpus gets, against
// a report generated minutes ago rather than months.
func TestCanary(t *testing.T) {
	t.Parallel()

	for _, run := range canaryRuns(t) {
		t.Run(run.Version.String(), func(t *testing.T) {
			t.Parallel()

			log := filepath.Join(run.Dir, "simulation.log")

			f, err := os.Open(log) //nolint:gosec // a run directory the workflow just produced
			if err != nil {
				t.Fatalf("Gatling %s left no log: %v", run.Version, err)
			}

			defer func() { _ = f.Close() }()

			rd, err := binary.NewReader(f)
			if err != nil {
				t.Fatalf("Gatling %s: the decoder refused the log: %v", run.Version, err)
			}

			if got := rd.Header().Version; got != run.Version {
				t.Fatalf("Gatling %s wrote a log naming %s", run.Version, got)
			}

			// An unknown newer version decodes and warns; the warning is a
			// statement about which version wrote the log, not about what the
			// log holds, so it is reported as a candidate for widening the
			// range rather than failed on.
			for _, w := range rd.Warnings() {
				summarize(t, "Gatling %s decoded unverified: %s — record a corpus entry and widen SupportedVersions",
					run.Version, w)
			}

			accounts, err := corpus.Accounts(run.Dir)
			if err != nil {
				t.Fatalf("Gatling %s: reading what the run said about itself: %v", run.Version, err)
			}

			if len(accounts) == 0 {
				t.Fatalf("Gatling %s produced no account of its own numbers, so the run proves nothing",
					run.Version)
			}

			ta := foldByHand(t, run.Dir)
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
				t.Errorf("Gatling %s stated no per-request rows in any artefact; only its total was checked",
					run.Version)
			}

			summarize(t, "Gatling %s: %d requests (%d ok, %d ko), %d request names, %d groups — matched its own report",
				run.Version, ta.global.total, ta.global.ok, ta.global.ko, len(ta.requests), len(ta.groups))
		})
	}
}

// TestCanaryCoversSupportedRange fails when the supported range was widened
// without the canary running the new bound, so the gate and the canary cannot
// drift apart. Principle II ties a codec's range to its corpus coverage; this is
// the half of that rule a machine can enforce.
func TestCanaryCoversSupportedRange(t *testing.T) {
	t.Parallel()

	oldest, newest := binary.SupportedVersions()
	ran := map[gatling.Version]bool{}

	for _, run := range canaryRuns(t) {
		ran[run.Version] = true
	}

	for _, bound := range []gatling.Version{oldest, newest} {
		if !ran[bound] {
			t.Errorf("SupportedVersions covers %s but the canary did not run it: add it to the version list", bound)
		}
	}
}

// TestCanaryCrossVersion holds every fresh run to every other: the same
// simulation must decode to the same multiset of records under each version once
// what a run cannot repeat is set aside.
func TestCanaryCrossVersion(t *testing.T) {
	t.Parallel()

	runs := canaryRuns(t)
	if len(runs) < 2 {
		t.Skip("cross-version equality needs at least two binary runs")
	}

	base := maskedShape(t, runs[0].Dir)

	for _, run := range runs[1:] {
		other := maskedShape(t, run.Dir)

		if !slices.Equal(base, other) {
			t.Errorf("Gatling %s and %s disagree once timing, identity, order and message text are set aside:\n%s",
				runs[0].Version, run.Version, firstShapeDiff(base, other))

			continue
		}

		summarize(t, "Gatling %s and %s: identical as a multiset (%d lines)", runs[0].Version, run.Version, len(base))
	}
}

// maskedShape renders a run as the sorted multiset of what two runs of the same
// simulation must agree on.
//
// Three things legitimately differ and are dropped: every timing value, the
// run's identity, and file order, because concurrent virtual users interleave
// differently on every run. A fourth is dropped across versions specifically —
// the check failure message, which Gatling reworded at 3.14.0 from
// "status.find.is(200), but actually found 500" to "status.find.is(200), found
// 500". That is the whole difference between the 3.13.1 recording and the two
// after it, and it is Gatling's wording, not the decoder's reading.
func maskedShape(t *testing.T, dir string) []string {
	t.Helper()

	var out []string

	for _, rec := range records(t, openCorpus(t, dir)) {
		if rec.Kind == gatling.KindRun {
			continue
		}

		out = append(out, fmt.Sprintf("%s\t%q\t%q\t%s\t%s",
			rec.Kind, strings.Join(rec.Groups, " / "), rec.Name, rec.Status, rec.Event))
	}

	slices.Sort(out)

	return out
}

func firstShapeDiff(a, b []string) string {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return fmt.Sprintf("  %s\n  %s", a[i], b[i])
		}
	}

	return fmt.Sprintf("  %d lines against %d", len(a), len(b))
}

// TestCanaryCrossFormat holds a fresh binary run to a fresh text one.
//
// The two formats record one thing differently and it is not a defect: the probe
// declares a group called "inner, with comma", a text simulation.log separates a
// group path with commas so Gatling substitutes a space before writing, and the
// binary format length-prefixes each name so the comma survives. Both spellings
// are correct for their own format, and the comparison normalises that one
// difference rather than pretending it does not exist.
func TestCanaryCrossFormat(t *testing.T) {
	t.Parallel()

	spec := os.Getenv(corpus.RunsEnv)
	if spec == "" {
		t.Skipf("%s is not set", corpus.RunsEnv)
	}

	all, err := corpus.ParseRuns(spec)
	if err != nil {
		t.Fatal(err)
	}

	var binaryDir, textDir string

	for _, run := range all {
		switch {
		case isBinaryLog(t, filepath.Join(run.Dir, "simulation.log")):
			binaryDir = run.Dir
		default:
			textDir = run.Dir
		}
	}

	if binaryDir == "" || textDir == "" {
		t.Skip("cross-format equality needs one run of each format")
	}

	got := normaliseGroupSpelling(maskedShape(t, binaryDir))
	want := normaliseGroupSpelling(maskedShape(t, textDir))

	if !slices.Equal(got, want) {
		t.Errorf("a binary run and a text run of the same probe disagree:\n%s", firstShapeDiff(got, want))

		return
	}

	summarize(t, "a text run and a binary run of the same probe: identical as a multiset (%d lines)", len(got))
}

// normaliseGroupSpelling collapses the one difference the two log formats
// legitimately have in a recorded name: a comma inside a group name, which a
// text log cannot carry and writes as a space.
func normaliseGroupSpelling(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, strings.ReplaceAll(l, ",", " "))
	}

	slices.Sort(out)

	return out
}

// TestCanaryRefusesAVersionBelowTheRange states what the gate does with an old
// run, so that a maintainer listing one sees the refusal reported as the
// expected outcome and not as a failure of the codec.
//
// The refusal is the contract: below the range the decoder names the version it
// found and the range it supports, and decodes nothing.
func TestCanaryRefusesAVersionBelowTheRange(t *testing.T) {
	t.Parallel()

	oldest, _ := binary.SupportedVersions()

	for _, run := range canaryRuns(t) {
		if run.Version.Compare(oldest) >= 0 {
			continue
		}

		f, err := os.Open(filepath.Join(run.Dir, "simulation.log"))
		if err != nil {
			t.Fatalf("Gatling %s left no log: %v", run.Version, err)
		}

		_, err = binary.NewReader(f)
		_ = f.Close()

		if err == nil {
			t.Errorf("Gatling %s is below the supported range and was decoded anyway", run.Version)

			continue
		}

		if !strings.Contains(err.Error(), oldest.String()) {
			t.Errorf("the refusal of %s does not name the supported range: %v", run.Version, err)
		}

		summarize(t, "Gatling %s is below the supported range and was refused, as it must be: %v", run.Version, err)
	}
}
