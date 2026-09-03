//go:build integration

package text_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

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
