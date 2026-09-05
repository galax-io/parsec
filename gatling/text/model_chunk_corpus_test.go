//go:build integration

package text_test

import (
	"path/filepath"
	"testing"
)

// The corpus counterpart of TestModelChunkedFixtures: the same agreement over
// the two recorded runs, which are far longer than any fixture.
func TestModelChunkedCorpus(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()
			checkModelChunked(t, filepath.Join(dir, "simulation.log"))
		})
	}
}
