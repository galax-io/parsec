package parsec_test

import (
	"strings"
	"testing"

	"github.com/galax-io/parsec/internal/exports"
)

// Every exported reader mutates unsynchronised state on each call, and the text
// codec's interner is a map — so a shared reader is not a race that produces a
// wrong number but a runtime throw that recover cannot catch. The documentation
// is meticulous about the neighbouring aliasing rule and said nothing about this
// one, which reads as if concurrency had been considered and found safe.
//
// The simlog interfaces are where it matters most: a follower holds the
// interface, so the concrete type carrying the map is invisible at the call
// site.
func TestEveryReaderStatesTheConcurrencyRule(t *testing.T) {
	t.Parallel()

	surfaces := map[string][]string{
		"gatling/text":   {"type Reader", "type RunReader"},
		"gatling/binary": {"type Reader", "type RunReader"},
		"gatling/simlog": {"type RecordReader", "type RunReader"},
	}

	for pkg, idents := range surfaces {
		t.Run(pkg, func(t *testing.T) {
			t.Parallel()

			docs, err := exports.Docs(pkg)
			if err != nil {
				t.Fatal(err)
			}

			for _, ident := range idents {
				text, ok := docs[ident]
				if !ok {
					t.Errorf("%s in %s has no doc comment", ident, pkg)

					continue
				}

				if !strings.Contains(text, "one goroutine at a time") {
					t.Errorf("%s in %s does not state that a value is used by one goroutine at a time:\n%s",
						ident, pkg, text)
				}
			}
		})
	}
}
