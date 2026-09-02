package text_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/text"
)

// readGuarded drains a log and turns a panic into a test failure. The recover
// lives here, in test code, never in the reader.
func readGuarded(t *testing.T, what string, data []byte) {
	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%s: the reader panicked: %v\ninput prefix: %q", what, r, head(data, 200))
		}
	}()

	rd, err := text.NewReader(bytes.NewReader(data))
	if err != nil {
		assertTyped(t, what, err)

		return
	}

	for {
		_, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return
		}

		if err != nil {
			assertTyped(t, what, err)

			return
		}
	}
}

// assertTyped insists that whatever ends a read is one of the two errors the
// contract names, never something unexpected escaping from inside.
func assertTyped(t *testing.T, what string, err error) {
	t.Helper()

	var (
		se *gatling.SyntaxError
		ve *gatling.VersionError
	)

	if !errors.As(err, &se) && !errors.As(err, &ve) {
		t.Fatalf("%s: read ended with %T (%v), want *gatling.SyntaxError or *gatling.VersionError", what, err, err)
	}
}

func head(b []byte, n int) []byte {
	if len(b) > n {
		return b[:n]
	}

	return b
}

func loadFixtures(t *testing.T) map[string][]byte {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join("testdata", "fixtures", "*.fixture.log"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no fixtures found: %v", err)
	}

	sources := make(map[string][]byte, len(paths))

	for _, p := range paths {
		data, err := os.ReadFile(p) //nolint:gosec // a fixture path from the test's own glob
		if err != nil {
			t.Fatal(err)
		}

		sources[filepath.Base(p)] = data
	}

	return sources
}

// mutate applies one random, seeded corruption and describes it.
func mutate(rng *rand.Rand, src []byte) ([]byte, string) {
	data := append([]byte(nil), src...)
	if len(data) == 0 {
		return []byte{byte(rng.IntN(256))}, "one random byte in an empty input" //nolint:gosec // bounded by IntN; the narrowing is the point
	}

	at := rng.IntN(len(data))

	switch rng.IntN(6) {
	case 0:
		data[at] ^= byte(1 + rng.IntN(255)) //nolint:gosec // bounded by IntN; the narrowing is the point

		return data, fmt.Sprintf("flip byte %d", at)
	case 1:
		data = append(data[:at], append([]byte{'\t'}, data[at:]...)...)

		return data, fmt.Sprintf("insert tab at %d", at)
	case 2:
		data = append(data[:at], data[at+1:]...)

		return data, fmt.Sprintf("delete byte %d", at)
	case 3:
		return data[:at], fmt.Sprintf("truncate at %d", at)
	case 4:
		lines := bytes.SplitAfter(data, []byte("\n"))
		i := rng.IntN(len(lines))
		lines = append(lines[:i+1], append([][]byte{lines[i]}, lines[i+1:]...)...)

		return bytes.Join(lines, nil), fmt.Sprintf("duplicate line %d", i+1)
	default:
		data = append(data[:at], append([]byte{'\r'}, data[at:]...)...)

		return data, fmt.Sprintf("insert CR at %d", at)
	}
}

// TestMutations applies 10,000 seeded corruptions to the fixtures and requires
// that every one ends in a typed error or a clean end — never a panic (SC-007).
func TestMutations(t *testing.T) {
	t.Parallel()

	const rounds = 10_000

	sources := loadFixtures(t)

	// Sorted, not in map order: the seed below exists so that a failure
	// reproduces, and it cannot if the index it draws lands on a different
	// fixture each run.
	names := slices.Sorted(maps.Keys(sources))

	rng := rand.New(rand.NewPCG(2026, 9)) //nolint:gosec // seeded so a failure reproduces; not a security use

	for i := range rounds {
		name := names[rng.IntN(len(names))]
		data, how := mutate(rng, sources[name])
		readGuarded(t, fmt.Sprintf("round %d, %s, %s", i, name, how), data)
	}
}

// FuzzReader is seeded from every fixture for local exploration with
// go test -fuzz=FuzzReader; under a plain go test it runs the seeds only.
func FuzzReader(f *testing.F) {
	paths, err := filepath.Glob(filepath.Join("testdata", "fixtures", "*.fixture.log"))
	if err != nil || len(paths) == 0 {
		f.Fatalf("no fixtures found: %v", err)
	}

	for _, p := range paths {
		data, err := os.ReadFile(p) //nolint:gosec // a fixture path from the test's own glob
		if err != nil {
			f.Fatal(err)
		}

		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		readGuarded(t, "fuzz input", data)
	})
}
