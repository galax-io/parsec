//go:build integration

package text_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/galax-io/parsec/gatling/text"
)

func drainAll(b *testing.B, data []byte) int {
	b.Helper()

	rd, err := text.NewReader(bytes.NewReader(data))
	if err != nil {
		b.Fatal(err)
	}

	n := 0

	for {
		_, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return n
		}

		if err != nil {
			b.Fatal(err)
		}

		n++
	}
}

func largestCorpusLog(b *testing.B) []byte {
	b.Helper()

	logs, err := filepath.Glob(filepath.Join("..", "..", "testdata", "corpus", "gatling", "*", "simulation.log"))
	if err != nil || len(logs) == 0 {
		b.Fatal("no corpus log found")
	}

	var largest []byte

	for _, p := range logs {
		data, err := os.ReadFile(p) //nolint:gosec // a corpus path from the test's own glob
		if err != nil {
			b.Fatal(err)
		}

		if len(data) > len(largest) {
			largest = data
		}
	}

	return largest
}

// BenchmarkReader measures decoding throughput. The corpus file is tiny, so
// its number is dominated by NewReader's one-off buffer; the synthetic log
// gives the steady-state figure that is the regression baseline.
func BenchmarkReader(b *testing.B) {
	inputs := map[string][]byte{"corpus": largestCorpusLog(b)}

	synth, err := io.ReadAll(newSynthLog(64 << 20))
	if err != nil {
		b.Fatal(err)
	}

	inputs["synthetic-64MiB"] = synth

	for name, data := range inputs {
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()

			records := 0

			for b.Loop() {
				records = drainAll(b, data)
			}

			b.ReportMetric(float64(records), "records/op")
		})
	}
}

func drainModel(b *testing.B, data []byte) int {
	b.Helper()

	rd, err := text.NewRunReader(bytes.NewReader(data))
	if err != nil {
		b.Fatal(err)
	}

	n := 0

	for {
		_, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return n
		}

		if err != nil {
			b.Fatal(err)
		}

		n++
	}
}

// BenchmarkRunReader is BenchmarkReader's counterpart through the canonical
// types. Its B/op is the number that matters: the conversion is a mapping of
// values already in hand, so it should allocate no more per item than the
// decoder does per record, and a rise here is the regression to explain.
func BenchmarkRunReader(b *testing.B) {
	inputs := map[string][]byte{"corpus": largestCorpusLog(b)}

	synth, err := io.ReadAll(newSynthLog(64 << 20))
	if err != nil {
		b.Fatal(err)
	}

	inputs["synthetic-64MiB"] = synth

	for name, data := range inputs {
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()

			items := 0

			for b.Loop() {
				items = drainModel(b, data)
			}

			b.ReportMetric(float64(items), "items/op")
		})
	}
}
