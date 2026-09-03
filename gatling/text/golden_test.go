//go:build integration

package text_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite records.golden from the decoder's output")

func TestGolden(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			f, err := os.Open(filepath.Join(dir, "simulation.log")) //nolint:gosec // a corpus path from the test's own glob
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = f.Close() }()

			got, err := canonical(f)
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
