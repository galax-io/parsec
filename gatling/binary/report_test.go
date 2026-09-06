package binary_test

import (
	"testing"

	"github.com/galax-io/parsec/internal/corpus"
)

// counts is what a run said about itself: how many requests it recorded, and how
// many of them failed.
type counts struct {
	total, ok, ko int
}

// runAccounts is the run total from every account a recording carries, keyed by
// the artefact it came from.
//
// Which artefacts exist is the tool version's decision. Up to 3.13.x Gatling
// writes machine-readable JSON; from 3.14.0 it stops, and the run's numbers
// survive only in the generated report and in the console summary — which is why
// the corpus keeps both, and why nothing is extracted at recording time: the
// artefacts are kept as Gatling wrote them so a later reader can check what the
// run actually said.
//
// The reading itself lives in internal/corpus, which both codecs share, so the
// two corpora are compared by the same code rather than by two copies of it.
func runAccounts(t *testing.T, dir string) map[string]counts {
	t.Helper()

	accounts, err := corpus.Accounts(dir)
	if err != nil {
		t.Fatalf("reading what the run said about itself: %v", err)
	}

	out := make(map[string]counts, len(accounts))

	for source, rep := range accounts {
		root, ok := rep.Root()
		if !ok {
			t.Errorf("%s states no run total", source)

			continue
		}

		out[source] = counts{
			total: int(root.Requests.Total),
			ok:    int(root.Requests.OK),
			ko:    int(root.Requests.KO),
		}
	}

	return out
}
