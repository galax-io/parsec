//go:build integration

package binary_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/galax-io/parsec/internal/corpus"
)

// A corpus entry is only evidence if it is complete. Every recorded run must
// carry the artefact, the decoded stream it is compared against, the note that
// says what was checked when it was made, and at least one account the tool gave
// of its own numbers.
//
// None of it can be added afterwards: an archived run cannot be re-run, and the
// console summary exists only if standard output was redirected while it
// happened. An entry missing a piece is not a smaller entry — it is one that
// cannot prove what the corpus exists to prove.
func TestEveryRecordingIsComplete(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()

			for _, name := range []string{"simulation.log", "records.golden", "RECORDING.md"} {
				if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
					t.Errorf("the recording has no %s: %v", name, err)
				}
			}

			accounts, err := corpus.Accounts(dir)
			if err != nil {
				t.Fatalf("reading what the run said about itself: %v", err)
			}

			if len(accounts) == 0 {
				t.Error("the recording carries no account of its own numbers, so it proves nothing")
			}
		})
	}
}

// The corpus is committed, so its size is everyone's. It stays small because the
// render-only assets a report needs — the vendored JavaScript, the stylesheets,
// the logos — are dropped: they carry nothing about a run, are byte-identical
// across runs of a version, and come to about a megabyte an entry.
func TestTheCorpusStaysSmall(t *testing.T) {
	t.Parallel()

	const ceiling = 5 << 20

	var total int64

	root := repoPath("testdata", "corpus", "gatling")

	err := filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		total += info.Size()

		return nil
	})
	if err != nil {
		t.Fatalf("walking the corpus: %v", err)
	}

	t.Logf("the recorded corpus is %.1f KiB", float64(total)/1024)

	if total > ceiling {
		t.Errorf("the corpus is %.1f MiB, past the %d MiB ceiling: a render-only asset has crept back in",
			float64(total)/(1<<20), ceiling>>20)
	}
}
