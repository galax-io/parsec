//go:build canary

package binary_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
	"github.com/galax-io/parsec/gatling/simlog"
	"github.com/galax-io/parsec/gatling/text"
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

	var mine []corpus.Run

	for _, run := range allRuns(t) {
		if logFormat(t, run) == gatling.FormatBinary {
			mine = append(mine, run)
		}
	}

	if len(mine) == 0 {
		t.Skipf("%s names no run whose simulation.log is a binary one", corpus.RunsEnv)
	}

	return mine
}

// allRuns is every run the value names, of either format and any version.
//
// The range gate reads this rather than canaryRuns, and the difference matters:
// filtering to this codec's own runs first is what let a version list carrying
// no text run skip the text codec's gate entirely, so trimming the workflow's
// matrix stopped a codec being canaried while the build stayed green.
func allRuns(t *testing.T) []corpus.Run {
	t.Helper()

	spec := os.Getenv(corpus.RunsEnv)
	if spec == "" {
		t.Skipf("%s is not set: point it at version=dir pairs of fresh Gatling runs, separated by \";\"",
			corpus.RunsEnv)
	}

	runs, err := corpus.ParseRuns(spec)
	if err != nil {
		t.Fatal(err)
	}

	return runs
}

// logFormat asks the log which format it is, rather than inferring it from the
// version. A version that changed format without saying so is exactly what a
// canary exists to notice — and an unreadable log comes back as
// gatling.FormatUnknown, which is neither format and belongs to neither codec.
func logFormat(t *testing.T, run corpus.Run) gatling.Format {
	t.Helper()

	return format(t, filepath.Join(run.Dir, "simulation.log"))
}

// verified is the runs a codec both reads and vouches for: inside that codec's
// supported range, so the decoder's output is evidence rather than a guess.
//
// An above-range run decodes and is summarised, but takes no part in an
// equality comparison — FR-004. Its records are unverified by construction, so
// a difference between it and an in-range run says nothing about the decoder,
// and failing on one would red the canary for the very case the version list
// exists to try.
//
// The range is passed in rather than taken from this package, because the
// cross-format comparison holds a text run to gatling/text's range and a binary
// run to this one. Using a single codec's bounds for both silently drops every
// legitimate text run — they are all older than 3.13.1 — and turns that
// comparison into a permanent skip.
func verified(runs []corpus.Run, oldest, newest gatling.Version) []corpus.Run {
	var out []corpus.Run

	for _, run := range runs {
		if run.Version.Compare(oldest) >= 0 && run.Version.Compare(newest) <= 0 {
			out = append(out, run)
		}
	}

	return out
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

	if os.Getenv("PARSEC_CANARY_PARTIAL") != "" {
		t.Skipf("PARSEC_CANARY_PARTIAL is set: this run was asked for a chosen subset of versions, " +
			"so it is not the gate that holds the supported range to the canary")
	}

	oldest, newest := binary.SupportedVersions()
	ran := map[gatling.Version]bool{}

	// allRuns, not canaryRuns: a list carrying no run of this codec's format
	// must fail here, not filter itself down to nothing and skip.
	for _, run := range allRuns(t) {
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

	oldest, newest := binary.SupportedVersions()

	runs := verified(canaryRuns(t), oldest, newest)
	if len(runs) < 2 {
		t.Skip("cross-version equality needs at least two binary runs inside the supported range")
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

	// Read through simlog rather than through this package's own reader. The
	// cross-format comparison hands this function a text log as often as a
	// binary one, and simlog is the module's own dispatcher: it identifies the
	// format from the bytes and hands the log to the codec that reads it. Using
	// it here also means the comparison exercises the dispatcher a consumer
	// actually calls.
	rd, err := simlog.NewReader(openCorpus(t, dir))
	if err != nil {
		t.Fatalf("%s: %v", dir, err)
	}

	var out []string

	for {
		rec, err := rd.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}

		if rec.Kind == gatling.KindRun {
			continue
		}

		out = append(out, fmt.Sprintf("%s\t%q\t%q\t%s\t%s",
			rec.Kind, groupPath(rec.Groups), rec.Name, rec.Status, rec.Event))
	}

	slices.Sort(out)

	return out
}

// firstShapeDiff leads with the two counts and then names the first row they
// disagree on. The counts come first because they usually say what happened on
// their own: two runs of the same probe produce the same number of records, so a
// difference there is a different probe rather than a different reading of one.
func firstShapeDiff(a, b []string) string {
	head := fmt.Sprintf("  %d records against %d", len(a), len(b))

	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return fmt.Sprintf("%s; first disagreement:\n  %s\n  %s", head, a[i], b[i])
		}
	}

	return head + "; they agree as far as the shorter one goes"
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

	var binaryDir, textDir string

	// Matched on the format each log actually is. A `default:` arm would file an
	// unreadable log — gatling.FormatUnknown, which is what format() returns when
	// detection fails — under "text", and the comparison would then die inside
	// the text codec saying nothing about the detection that really failed.
	binOldest, binNewest := binary.SupportedVersions()
	txtOldest, txtNewest := text.SupportedVersions()

	for _, run := range allRuns(t) {
		one := []corpus.Run{run}

		switch logFormat(t, run) {
		case gatling.FormatBinary:
			if len(verified(one, binOldest, binNewest)) == 1 {
				binaryDir = run.Dir
			}
		case gatling.FormatText:
			if len(verified(one, txtOldest, txtNewest)) == 1 {
				textDir = run.Dir
			}
		case gatling.FormatUnknown:
		}
	}

	if binaryDir == "" || textDir == "" {
		t.Skip("cross-format equality needs one run of each format")
	}

	got := maskedShape(t, binaryDir)
	want := maskedShape(t, textDir)

	if !slices.Equal(got, want) {
		t.Errorf("a binary run and a text run of the same probe disagree:\n%s", firstShapeDiff(got, want))

		return
	}

	summarize(t, "a text run and a binary run of the same probe: identical as a multiset (%d lines)", len(got))
}

// groupPath renders a group path with the one difference the two log formats
// legitimately have in a recorded name collapsed: a comma inside a group name,
// which a text log cannot carry and writes as a space.
//
// The substitution is applied to the group names and to nothing else. Applying
// it to the finished line would mask a comma anywhere — including inside a
// request name, which both formats carry verbatim — so a decoder that dropped
// the comma from a request called "checkout,fast" would compare equal to one
// that kept it.
func groupPath(groups []string) string {
	out := make([]string, 0, len(groups))
	for _, g := range groups {
		out = append(out, strings.ReplaceAll(g, ",", " "))
	}

	return strings.Join(out, " / ")
}

// TestCanaryRefusesAVersionBelowTheRange pins what the gate does with a run
// older than this codec supports: it refuses, naming the version it found and
// the range it supports, and decodes nothing.
//
// It is driven from a synthesised log rather than from the canary's version
// list, because the list can never supply one. canaryRuns yields only binary
// logs, and every Gatling below 3.13.1 writes a text log; the single exception,
// 3.13.0, cannot produce a report at all, so its run fails before this test is
// reached. Reading the list here made the test pass having asserted nothing —
// while still counting toward the workflow's "no comparison test ran" guard.
func TestCanaryRefusesAVersionBelowTheRange(t *testing.T) {
	t.Parallel()

	oldest, _ := binary.SupportedVersions()

	// 3.13.0 is the one binary version below the range, and the reason it is not
	// in the corpus: it cannot read back the assertion records it writes.
	_, err := binary.NewReader(bytes.NewReader(minimal("3.13.0")))
	if err == nil {
		t.Fatal("a log naming 3.13.0 was decoded; it is below the supported range and must be refused")
	}

	for _, want := range []string{"3.13.0", oldest.String()} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal is %q; it must name %s", err, want)
		}
	}

	summarize(t, "a run below the supported range is refused naming both the version and the range: %v", err)
}
