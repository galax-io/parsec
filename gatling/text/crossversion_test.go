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
