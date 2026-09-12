package parsec_test

import (
	"strings"
	"testing"

	"github.com/galax-io/parsec/internal/exports"
)

// From v0.1.0 these comments are what three consumers read on pkg.go.dev, and
// Principle V freezes each one with the identifier it documents. The checks in
// the docs_*_test.go files are the ones a review found a defect for; each fails
// if that defect returns.

// A stray "//" inside a doc comment destroys the paragraph break around it and
// is published verbatim. It happened once, on the constructor of one of the two
// readers this module promises — the first thing a consumer evaluating it reads.
// The defect was corrected while the version gate moved; this is what keeps it
// corrected.
func TestNoDocCommentPublishesACommentMarker(t *testing.T) {
	t.Parallel()

	for _, pkg := range packages {
		t.Run(pkg, func(t *testing.T) {
			t.Parallel()

			docs, err := exports.Docs(pkg)
			if err != nil {
				t.Fatal(err)
			}

			for name, text := range docs {
				if strings.Contains(text, "//") {
					t.Errorf("%s.%s renders a literal // :\n%s", pkg, name, text)
				}
			}
		})
	}
}

// A fact stated twice inside one comment leaves a reader unsure which statement
// is current. No general detector is possible — the two statements are usually
// paraphrases, not repeats — so this pins the ones review found, by the phrase
// that carries the fact.
func TestNoDocCommentStatesOneFactTwice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		pkg, ident, phrase string
	}{
		// TruncationError stated the boundary-cut fact in two paragraphs of one
		// comment: that a binary log cut exactly on a record boundary is a
		// shorter valid log and ends the read cleanly. It is true and important,
		// and it was stated once too many.
		{pkg: "gatling", ident: "type TruncationError", phrase: "record boundary"},
	}

	for _, tt := range tests {
		t.Run(tt.pkg+" "+tt.ident, func(t *testing.T) {
			t.Parallel()

			docs, err := exports.Docs(tt.pkg)
			if err != nil {
				t.Fatal(err)
			}

			text, ok := docs[tt.ident]
			if !ok {
				t.Fatalf("%s has no doc comment in %s", tt.ident, tt.pkg)
			}

			if n := strings.Count(text, tt.phrase); n > 1 {
				t.Errorf("%s states %q %d times in one comment:\n%s", tt.ident, tt.phrase, n, text)
			}
		})
	}
}
