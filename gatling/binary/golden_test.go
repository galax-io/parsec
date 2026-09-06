//go:build integration

package binary_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite records.golden from the decoder's output")

// The recorded record stream, byte for byte. It is the only check that pins
// every field of every record at once: a tolerance test folds records into
// counts and cannot see a name decoded wrongly, a timestamp off by the run
// start, or a group path in the wrong order.
func TestGolden(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			got, err := canonical(openCorpus(t, dir))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			golden := filepath.Join(dir, "records.golden")
			if *update {
				if err := os.WriteFile(golden, got, 0o600); err != nil {
					t.Fatal(err)
				}

				t.Logf("rewrote %s — review it line by line against simulation.log before committing", golden)

				return
			}

			want, err := os.ReadFile(golden) //nolint:gosec // a corpus path from the test's own glob
			if err != nil {
				t.Fatalf("%v — generate it with -update, then review it", err)
			}

			if !bytes.Equal(got, want) {
				t.Fatalf("decoded stream differs from %s:\n%s", golden, firstDiff(got, want))
			}
		})
	}
}
