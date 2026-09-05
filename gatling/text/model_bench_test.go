//go:build integration

package text_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/galax-io/parsec/gatling/text"
)

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
