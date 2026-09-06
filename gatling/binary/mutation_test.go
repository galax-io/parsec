package binary_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
)

// One byte changed anywhere in a real recording. Every outcome must be one a
// caller can act on: the log decodes to something, or it fails with an error
// naming a byte. What must never happen is a panic, and what this format makes
// tempting is the third thing — a decoder that carries on with a desynchronised
// stream and returns records that look fine and are not.
//
// The check for that is the golden stream: any mutation that still decodes
// cleanly must decode to something *different* from the original. A mutation
// that changed a byte and changed nothing about the output would mean the
// decoder is ignoring bytes the writer wrote.
func TestOneCorruptedByteEitherDecodesDifferentlyOrFailsNamingIt(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile(filepath.Join(corpusDirs(t)[0], "simulation.log"))
	if err != nil {
		t.Fatal(err)
	}

	original := canonicalOf(t, raw)

	var (
		failed, changed, ignored int
		ignoredAt                []int
	)

	// A stride rather than every byte: the file is four thousand bytes and the
	// interesting positions — kind bytes, lengths, coders, cache indices — recur
	// throughout. Two strides that share no factor cover more than one.
	for i := 1; i < len(raw); i += 3 {
		mutated := bytes.Clone(raw)
		mutated[i] ^= 0xff

		out, err := decodeCanonical(mutated)
		switch {
		case err != nil:
			var se *gatling.SyntaxError
			if !errors.As(err, &se) && !errors.As(err, new(*gatling.VersionError)) {
				t.Fatalf("byte %d flipped: failed with %T: %v", i, err, err)
			}

			if se != nil && (se.Offset < 0 || se.Offset > int64(len(mutated))) {
				t.Fatalf("byte %d flipped: error names byte %d, outside the file", i, se.Offset)
			}

			failed++
		case out != original:
			changed++
		default:
			ignored++

			if len(ignoredAt) < 8 {
				ignoredAt = append(ignoredAt, i)
			}
		}
	}

	if ignored != 0 {
		t.Fatalf("%d mutations changed a byte and changed nothing about the decoded stream "+
			"(at %v): those bytes are being ignored", ignored, ignoredAt)
	}

	t.Logf("%d mutations refused naming a byte, %d decoded to a different stream", failed, changed)
}

func canonicalOf(t *testing.T, raw []byte) string {
	t.Helper()

	got, err := decodeCanonical(raw)
	if err != nil {
		t.Fatalf("the unmutated recording failed to decode: %v", err)
	}

	return got
}

// decodeCanonical renders a possibly-broken log, returning the failure instead
// of the rendering when there is one.
func decodeCanonical(raw []byte) (string, error) {
	rd, err := binary.NewReader(bytes.NewReader(raw))
	if err != nil {
		return "", err
	}

	var b bytes.Buffer

	h := rd.Header()
	fmt.Fprintf(&b, "%q %s %d %q\n", h.SimulationClass, h.Version, h.Start, h.Description)

	// The assertion payloads are part of what a mutation can change, and they
	// are a third of the file. Leaving them out of the rendering would let every
	// mutation inside one look like a byte the decoder ignores.
	for _, p := range rd.Assertions() {
		fmt.Fprintf(&b, "ASSERTION %q\n", p)
	}

	for n := 1; ; n++ {
		rec, err := rd.Next()
		if errors.Is(err, io.EOF) {
			return b.String(), nil
		}

		if err != nil {
			return "", err
		}

		writeRecord(&b, n, rec)
	}
}
