package gatling_test

import (
	"testing"

	"github.com/galax-io/parsec/gatling"
)

// The tool name has one spelling. It was declared twice, once per codec, for one
// value: a consumer branching on the tool had two names to choose between and no
// reason to prefer one, and a comparison against either silently meant "that
// codec's spelling of gatling". From v0.1.0 both would have been frozen.
//
// This file imports gatling and nothing else, which is the other half of the
// claim: naming the tool must not cost an import of a codec the caller does not
// otherwise need.
func TestToolIsNamedOnce(t *testing.T) {
	t.Parallel()

	if gatling.Tool != "gatling" {
		t.Errorf("Tool = %q, want %q", gatling.Tool, "gatling")
	}
}
