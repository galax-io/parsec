package gatling_test

import (
	"testing"

	"github.com/galax-io/parsec/gatling"
)

// Detect compares at most DetectSize bytes and allocates nothing on the paths a
// real log takes. The size of the log cannot reach it, which is the property
// the exported window exists to guarantee.
func BenchmarkDetect(b *testing.B) {
	inputs := map[string][]byte{
		"binary":     {0x00, 0x00, 0x00, 0x00, 0x06, '3', '.', '1', '5', '.'},
		"text run":   []byte("RUN\tio.example.Sim"),
		"text assrt": []byte("ASSERTION\tAAEBAAICAAAA"),
		"neither":    []byte("<html><head><title>"),
		"short":      []byte("ASSERT"),
	}

	for name, head := range inputs {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				_, _ = gatling.Detect(head)
			}
		})
	}
}

// The window is fixed, so a huge input costs the same as a small one. A
// regression here would mean detection had started reading the log.
func BenchmarkDetectIgnoresInputSize(b *testing.B) {
	small := []byte("ASSERTION\tAAEB")
	large := make([]byte, 1<<20)
	copy(large, "ASSERTION\t")

	b.Run("14 bytes", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_, _ = gatling.Detect(small)
		}
	})

	b.Run("1 MiB", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			_, _ = gatling.Detect(large)
		}
	})
}
