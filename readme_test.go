package parsec_test

import (
	"os"
	"strings"
	"testing"
)

// README.md is the landing page for a library three codebases pin. A stranger
// with a simulation.log must be able to assemble a working program from it
// without opening another file, and must be told the truth about which versions
// it reads.
//
// It held no code at all, no install line, a compatibility table wrong at both
// ends, and — as the one read call it named — the codec that cannot read the
// format the file opened by promising to rescue.
func TestReadmeAnswersWhatALanderAsks(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}

	readme := string(raw)

	musts := []struct{ what, text string }{
		{what: "how to install it", text: "go get github.com/galax-io/parsec"},
		{what: "the minimum Go version", text: "Go 1.25"},
		{what: "the entry point, which reads either format", text: "simlog.NewRunReader"},
		{what: "where the run comes from", text: "run.Find"},
		{what: "the binary range, correctly", text: "3.13.1 through 3.15.1"},
		{what: "the text range, correctly", text: "3.11.5 through 3.12.0"},
		{what: "why 3.13.0 is refused", text: "3.13.0 is refused"},
		{what: "how to read the range from the API", text: "simlog.Supported()"},
		{what: "what becomes stable, and when", text: "v0.1.0"},
	}

	for _, m := range musts {
		if !strings.Contains(readme, m.text) {
			t.Errorf("README does not say %s (looked for %q)", m.what, m.text)
		}
	}

	if !strings.Contains(readme, "```go") {
		t.Error("README carries no Go program; a stranger cannot assemble one from prose")
	}

	// The status log and the per-package version stamps answered a question
	// nobody landing here is asking, and CHANGELOG.md is where that lives.
	if strings.Contains(readme, "(v0.0.") {
		t.Error("README carries per-package version stamps; CHANGELOG.md is where release history lives")
	}

	// Both were wrong: 3.13.0 is refused, and 3.15.2 and later decode with a
	// warning rather than being "supported".
	for _, stale := range []string{"3.13.0 …", "3.15.x"} {
		if strings.Contains(readme, stale) {
			t.Errorf("README still claims %q; the accepted range is 3.13.1 through 3.15.1", stale)
		}
	}
}
