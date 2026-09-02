//go:build integration

package text_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Two recordings of the same simulation legitimately differ in three ways:
// every timing value (timestamps and cumulated response times), the run's
// identity (id and Gatling version), and file order — concurrent virtual users
// interleave differently on every run, so order is not evidence of anything.
// Everything else must agree exactly, as a multiset.
//
// The version gate's warning goes with identity. It is a statement about which
// version wrote the log, not about what the log holds, and only a run above the
// supported range carries one (FR-025) — so comparing it would make such a run
// unequal to an in-range one by construction.
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

func TestCrossVersion(t *testing.T) {
	t.Parallel()

	dirs := corpusDirs(t)
	if len(dirs) < 2 {
		t.Fatalf("cross-version equality needs two recordings, found %d", len(dirs))
	}

	base := maskedSorted(t, dirs[0])

	for _, dir := range dirs[1:] {
		other := maskedSorted(t, dir)

		if len(base) != len(other) {
			t.Errorf("%s has %d lines, %s has %d", filepath.Base(dirs[0]), len(base), filepath.Base(dir), len(other))
		}

		for i := 0; i < len(base) && i < len(other); i++ {
			if base[i] != other[i] {
				t.Fatalf("the recordings differ once timing, identity and order are set aside:\n %s: %s\n %s: %s",
					filepath.Base(dirs[0]), base[i], filepath.Base(dir), other[i])
			}
		}

		t.Logf("%s and %s: %d lines, identical as a multiset", filepath.Base(dirs[0]), filepath.Base(dir), len(base))
	}
}

// TestCrossVersionMaskCoversTheWarning holds the mask itself to the rule it
// states. A run above the supported range decodes and carries a warning, and an
// in-range run carries none, so a mask that left the warning in place would
// report the two as different logs when the records are the same.
func TestCrossVersionMaskCoversTheWarning(t *testing.T) {
	t.Parallel()

	const events = "USER\tCorpus recording\tSTART\t1788379356165\n" +
		"REQUEST\t\tGET /ok\t1788379356162\t1788379356173\tOK\t \n" +
		"GROUP\touter,inner\t1788379356180\t1788379357700\t1520\tKO\n" +
		"USER\tCorpus recording\tEND\t1788379357702\n"

	header := func(version string) string {
		return "RUN\tio.galaxio.parsec.corpus.CorpusSimulation\tcorpussimulation\t1788379354534\t \t" + version + "\n"
	}

	write := func(version string) string {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "simulation.log"), []byte(header(version)+events), 0o600); err != nil {
			t.Fatal(err)
		}

		return dir
	}

	inRange := maskedSorted(t, write("3.11.5"))
	aboveRange := maskedSorted(t, write("3.13.0"))

	if !slices.Equal(inRange, aboveRange) {
		t.Fatalf("the same records compare unequal across the range bound:\n in range: %v\n above:    %v", inRange, aboveRange)
	}
}
