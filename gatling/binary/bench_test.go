//go:build integration

package binary_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/galax-io/parsec/gatling/binary"
)

// largest is the biggest recording this codec covers, read once and decoded from
// memory so the benchmark measures the decoder rather than the filesystem.
func largest(b *testing.B) []byte {
	b.Helper()

	logs, err := filepath.Glob(filepath.Join("..", "..", "testdata", "corpus", "gatling", "*", "simulation.log"))
	if err != nil {
		b.Fatal(err)
	}

	var biggest []byte

	for _, log := range logs {
		raw, err := os.ReadFile(log) //nolint:gosec // a corpus path from the benchmark's own glob
		if err != nil {
			b.Fatal(err)
		}

		if len(raw) > 0 && raw[0] == 0x00 && len(raw) > len(biggest) {
			biggest = raw
		}
	}

	if biggest == nil {
		b.Fatal("no binary recording found")
	}

	return biggest
}

// inputs is what both benchmarks run over. The corpus file is tiny, so its
// number is dominated by NewReader's one-off buffer; the synthetic log gives the
// steady-state figure that is the regression baseline, and it is the shape a
// soak run has — a handful of names, millions of records.
func inputs(b *testing.B) map[string][]byte {
	b.Helper()

	synth, err := io.ReadAll(newSynthLog(64 << 20))
	if err != nil {
		b.Fatal(err)
	}

	return map[string][]byte{"corpus": largest(b), "synthetic-64MiB": synth}
}

// Throughput and allocations per record. The figure to watch is allocs/op: once
// every name is cached, a record costs an index read rather than an allocation,
// so on the synthetic log it should be flat however long the log is. If it is
// not, something is copying that need not.
func BenchmarkDecode(b *testing.B) {
	for name, raw := range inputs(b) {
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(raw)))
			b.ReportAllocs()
			b.ResetTimer()

			var n int64

			for range b.N {
				rd, err := binary.NewReader(bytes.NewReader(raw))
				if err != nil {
					b.Fatal(err)
				}

				for {
					if _, err := rd.Next(); err != nil {
						if !errors.Is(err, io.EOF) {
							b.Fatal(err)
						}

						break
					}

					n++
				}
			}

			// Bytes per second understates this codec against the text one: the
			// binary format is several times denser, so the same megabyte is
			// many more records. Records per operation is what compares.
			b.ReportMetric(float64(n)/float64(b.N), "records/op")
		})
	}
}

// The model-facing path, which adds one conversion per record.
func BenchmarkDecodeToModel(b *testing.B) {
	for name, raw := range inputs(b) {
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(raw)))
			b.ReportAllocs()
			b.ResetTimer()

			var n int64

			for range b.N {
				rd, err := binary.NewRunReader(bytes.NewReader(raw))
				if err != nil {
					b.Fatal(err)
				}

				for {
					if _, err := rd.Next(); err != nil {
						if !errors.Is(err, io.EOF) {
							b.Fatal(err)
						}

						break
					}

					n++
				}
			}

			b.ReportMetric(float64(n)/float64(b.N), "items/op")
		})
	}
}
