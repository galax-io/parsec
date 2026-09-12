package parsec_test

import (
	"strings"
	"testing"

	"github.com/galax-io/parsec/internal/exports"
)

// The package overviews are the pages pkg.go.dev shows first, and every likely
// landing page sent a newcomer away from the intended path: gatling said the
// canonical model was future work two milestones after it shipped, the root
// omitted gatling/run entirely, gatling/text never named its own RunReader, and
// no overview named the entry point.
func TestPackageOverviewsRouteToTheEntryPoint(t *testing.T) {
	t.Parallel()

	t.Run("none claims a shipped thing is future work", func(t *testing.T) {
		t.Parallel()

		for _, pkg := range append([]string{"."}, packages...) {
			docs, err := exports.Docs(pkg)
			if err != nil {
				t.Fatal(err)
			}

			overview := docs["package"]
			for _, claim := range []string{"a later milestone", "arrive in", "will arrive"} {
				if strings.Contains(overview, claim) {
					t.Errorf("%s's overview says %q; model/ and both RunReaders shipped in v0.0.3 and v0.0.5",
						pkg, claim)
				}
			}
		}
	})

	t.Run("the root lists every package", func(t *testing.T) {
		t.Parallel()

		docs, err := exports.Docs(".")
		if err != nil {
			t.Fatal(err)
		}

		for _, pkg := range packages {
			if !strings.Contains(docs["package"], pkg) {
				t.Errorf("the root overview does not list %s", pkg)
			}
		}
	})

	t.Run("the entry point is named where a reader lands", func(t *testing.T) {
		t.Parallel()

		// run.Find, then simlog.NewRunReader, then a fold over Next. A reader
		// landing on the root or on model must be told where to start; a reader
		// landing on a codec must be told that both entry points exist.
		named := map[string][]string{
			".":              {"run.Find", "simlog.NewRunReader"},
			"model":          {"run.Find", "simlog.NewRunReader"},
			"gatling":        {"simlog"},
			"gatling/text":   {"RunReader"},
			"gatling/binary": {"RunReader"},
			"gatling/simlog": {"RunReader"},
			"gatling/run":    {"simlog"},
		}

		for pkg, wants := range named {
			docs, err := exports.Docs(pkg)
			if err != nil {
				t.Fatal(err)
			}

			for _, want := range wants {
				if !strings.Contains(docs["package"], want) {
					t.Errorf("%s's overview does not name %s", pkg, want)
				}
			}
		}
	})
}
