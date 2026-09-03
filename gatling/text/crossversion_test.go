//go:build integration

package text_test

import (
	"path/filepath"
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
