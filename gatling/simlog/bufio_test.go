package simlog_test

import (
	"bufio"
	"testing"

	"github.com/galax-io/parsec/gatling/binary"
	"github.com/galax-io/parsec/gatling/text"
)

// TestACallersBufioIsNotAdopted covers the one way a codec's fixed read buffer
// could become the caller's: bufio.NewReaderSize hands back its argument
// unchanged when that argument is already a *bufio.Reader of at least the size
// asked for, so a caller who buffers its own file would otherwise set both
// codecs' documented buffer — and, for the binary codec, the point at which a
// single large value starts reporting a complete log as cut short — without
// knowing it. Both codecs are asserted by the one test, per #87's acceptance.
func TestACallersBufioIsNotAdopted(t *testing.T) {
	t.Parallel()

	t.Run("text", func(t *testing.T) {
		t.Parallel()

		caller := bufio.NewReaderSize(open(t, repoPath("testdata", "corpus", "gatling", "3.11.5", "simulation.log")), 64<<10)

		if _, err := text.NewReader(caller); err != nil {
			t.Fatalf("NewReader: %v", err)
		}

		if n := caller.Buffered(); n != 0 {
			t.Fatalf("the caller's bufio.Reader holds %d buffered bytes after NewReader; "+
				"the codec read through it instead of buffering its own", n)
		}
	})

	t.Run("binary", func(t *testing.T) {
		t.Parallel()

		caller := bufio.NewReaderSize(open(t, binaryLog()), 64<<10)

		if _, err := binary.NewReader(caller); err != nil {
			t.Fatalf("NewReader: %v", err)
		}

		if n := caller.Buffered(); n != 0 {
			t.Fatalf("the caller's bufio.Reader holds %d buffered bytes after NewReader; "+
				"the codec read through it instead of buffering its own", n)
		}
	})
}
