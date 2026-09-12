package parsec_test

import (
	"strings"
	"testing"

	"github.com/galax-io/parsec/internal/exports"
)

// Warning is an exported type in two packages with incompatible shapes, reachable
// one simlog call apart — and simlog documents that move as the recommended
// direction. Version goes from an ordered comparable struct to a string across
// it, and a consumer that reaches for strings.Compare on the result gets
// "3.9.0" < "3.11.0", the ordering bug gatling.Version exists to prevent. Both
// names freeze at v0.1.0, so the collision is permanent and a sentence each is
// what it costs to explain.
func TestTheTwoWarningsNameEachOther(t *testing.T) {
	t.Parallel()

	tests := []struct{ pkg, ident, mentions string }{
		{pkg: "gatling", ident: "type Warning", mentions: "parsec/model.Warning"},
		{pkg: "model", ident: "type Warning", mentions: "parsec/gatling.Warning"},
		{pkg: "model", ident: "field Warning.Version", mentions: "tool-agnostic"},
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

			if !strings.Contains(text, tt.mentions) {
				t.Errorf("%s in %s does not mention %q:\n%s", tt.ident, tt.pkg, tt.mentions, text)
			}
		})
	}
}
