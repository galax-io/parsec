//go:build integration

package text_test

import (
	"path/filepath"
	"testing"
)

func TestChunkedCorpus(t *testing.T) {
	t.Parallel()

	for _, dir := range corpusDirs(t) {
		t.Run(filepath.Base(dir), func(t *testing.T) {
			t.Parallel()
			checkChunked(t, filepath.Join(dir, "simulation.log"))
		})
	}
}
