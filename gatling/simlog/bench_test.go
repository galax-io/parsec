package simlog_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/galax-io/parsec/gatling/simlog"
	"github.com/galax-io/parsec/gatling/text"
)

// largestCorpusLog is what the constitution asks a decoder benchmark to run
// over: the biggest real artefact the project holds.
//
// It fails on an unreadable entry rather than skipping it. The sibling helper in
// gatling/text does the same, and a benchmark that quietly measured a smaller
// log because a corpus file had gone bad would report a number nobody could
// place.
func largestCorpusLog(b *testing.B) []byte {
	b.Helper()

	logs, err := filepath.Glob(repoPath("testdata", "corpus", "gatling", "*", "simulation.log"))
	if err != nil {
		b.Fatalf("globbing the corpus: %v", err)
	}

	if len(logs) == 0 {
		b.Fatal("no corpus log found: the recorded runs are committed, so this is a broken checkout")
	}

	var largest []byte

	for _, path := range logs {
		body, err := os.ReadFile(path) //nolint:gosec // a corpus path from the test's own glob
		if err != nil {
			b.Fatalf("reading %s: %v", path, err)
		}

		if len(body) > len(largest) {
			largest = body
		}
	}

	return largest
}

// Identification is a fixed cost paid once per opened log. The pair measures
// exactly that: whatever separates the two numbers is what dispatch costs, and
// it must not grow with the log.
func BenchmarkOpen(b *testing.B) {
	body := largestCorpusLog(b)

	b.Run("direct", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(body)))

		for b.Loop() {
			rd, err := text.NewReader(bytes.NewReader(body))
			if err != nil {
				b.Fatal(err)
			}

			drainBench(b, rd.Next)
		}
	})

	b.Run("dispatched", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(body)))

		for b.Loop() {
			rd, err := simlog.NewReader(bytes.NewReader(body))
			if err != nil {
				b.Fatal(err)
			}

			drainBench(b, rd.Next)
		}
	})
}

// BenchmarkIdentify isolates the constructor, so the cost of dispatch is not
// hidden inside the cost of decoding a whole log.
func BenchmarkIdentify(b *testing.B) {
	body := largestCorpusLog(b)

	b.Run("direct", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			if _, err := text.NewReader(bytes.NewReader(body)); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("dispatched", func(b *testing.B) {
		b.ReportAllocs()

		for b.Loop() {
			if _, err := simlog.NewReader(bytes.NewReader(body)); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func drainBench[T any](b *testing.B, next func() (T, error)) {
	b.Helper()

	for {
		if _, err := next(); err != nil {
			if !errors.Is(err, io.EOF) {
				b.Fatal(err)
			}

			return
		}
	}
}
