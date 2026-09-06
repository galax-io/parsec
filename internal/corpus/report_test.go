package corpus_test

import (
	"cmp"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/galax-io/parsec/internal/corpus"
)

// repoPath resolves a path against the repository root. A test runs with its
// own package directory as the working directory, so the corpus is two levels
// up. Spelled the way gatling/binary/helpers_test.go spells it, so the two
// agree when parsec#59 merges the corpus helpers into one place.
func repoPath(parts ...string) string {
	return filepath.Join(append([]string{"..", ".."}, parts...)...)
}

func corpusDir(version string) string {
	return repoPath("testdata", "corpus", "gatling", version)
}

// row is one expected line of a run's own report, addressed the way a decoded
// log addresses it: by the names of its enclosing groups, never by Gatling's
// internal row hash.
type row struct {
	path  string // ancestor names, "/"-joined; empty at the root
	name  string
	kind  corpus.NodeKind
	total int64
	ok    int64
	ko    int64
}

// The tree each recording's own report states.
//
// Two shapes, because the probe changed. The 3.11.5 and 3.12.0 entries predate
// the v0.0.5 probe and say so in their own RECORDING.md: 36 requests, no
// Cyrillic request name, and the group recorded as "inner  with comma" with two
// spaces, because a text simulation.log separates a group path with commas and
// Gatling substitutes a space before writing. The binary versions record the
// declared name with its comma intact. Both spellings are correct for their own
// format, and this table is where that stops being a surprise.
var (
	oldProbe = []row{
		{"", "All Requests", corpus.KindRoot, 36, 18, 18},
		{"", "outer", corpus.KindGroup, 6, 0, 6},
		{"outer", "inner  with comma", corpus.KindGroup, 6, 0, 6},
		{"outer/inner  with comma", "GET /fail", corpus.KindRequest, 6, 0, 6},
		{"outer/inner  with comma", "GET /slow", corpus.KindRequest, 6, 6, 0},
		{"outer", "GET /ok", corpus.KindRequest, 6, 6, 0},
		{"", "connect refused", corpus.KindRequest, 6, 0, 6},
		{"", "GET /ok", corpus.KindRequest, 6, 6, 0},
		{"", "unknown host", corpus.KindRequest, 6, 0, 6},
	}

	currentProbe = []row{
		{"", "All Requests", corpus.KindRoot, 102, 84, 18},
		{"", "outer", corpus.KindGroup, 6, 0, 6},
		{"outer", "inner, with comma", corpus.KindGroup, 6, 0, 6},
		{"outer/inner, with comma", "GET /fail", corpus.KindRequest, 6, 0, 6},
		{"outer/inner, with comma", "GET /slow", corpus.KindRequest, 6, 6, 0},
		{"outer", "GET /ok", corpus.KindRequest, 66, 66, 0},
		{"", "connect refused", corpus.KindRequest, 6, 0, 6},
		{"", "GET /ok", corpus.KindRequest, 6, 6, 0},
		{"", "Проверка /ok", corpus.KindRequest, 6, 6, 0},
		{"", "unknown host", corpus.KindRequest, 6, 0, 6},
	}
)

// rowsOf renders a report as the sorted set of rows above, so a comparison is
// against what the report says rather than against the order it happened to say
// it in.
func rowsOf(rep corpus.Report) []row {
	out := make([]row, 0, len(rep.Nodes))

	for _, n := range rep.Nodes {
		out = append(out, row{
			path:  strings.Join(rep.Path(n), "/"),
			name:  n.Name,
			kind:  n.Kind,
			total: n.Requests.Total,
			ok:    n.Requests.OK,
			ko:    n.Requests.KO,
		})
	}

	return sorted(out)
}

func sorted(rows []row) []row {
	out := slices.Clone(rows)
	slices.SortFunc(out, func(a, b row) int {
		return cmp.Or(
			cmp.Compare(a.path, b.path),
			cmp.Compare(a.name, b.name),
			cmp.Compare(a.kind, b.kind),
		)
	})

	return out
}

// compareRows reports every difference rather than the first, so one run of the
// test says everything that disagreed.
func compareRows(t *testing.T, what string, got, want []row) {
	t.Helper()

	if len(got) != len(want) {
		t.Errorf("%s: %d rows, want %d", what, len(got), len(want))
	}

	for i := 0; i < len(got) && i < len(want); i++ {
		if got[i] != want[i] {
			t.Errorf("%s: row %d is\n  got  %+v\n  want %+v", what, i, got[i], want[i])
		}
	}
}

// Every recording states its own numbers in whatever artefact its version
// wrote, and every one of those must read back as the same tree.
func TestAccountsStateTheTreeTheRunRecorded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version string
		// sources that state the whole tree, and what it is
		tree []row
		// sources that state the run total alone, by design
		rootOnly []string
	}{
		{"3.11.5", oldProbe, []string{"global_stats.json"}},
		{"3.12.0", oldProbe, []string{"global_stats.json"}},
		{"3.13.1", currentProbe, []string{"js/global_stats.json", "console.txt"}},
		{"3.14.9", currentProbe, []string{"console.txt"}},
		{"3.15.1", currentProbe, []string{"console.txt"}},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			t.Parallel()

			accounts, err := corpus.Accounts(corpusDir(tt.version))
			if err != nil {
				t.Fatalf("Accounts: %v", err)
			}

			if len(accounts) == 0 {
				t.Fatal("the recording carries no account of its own numbers, so it proves nothing")
			}

			rootOnly := map[string]bool{}
			for _, s := range tt.rootOnly {
				rootOnly[s] = true
			}

			trees := 0

			for source, rep := range accounts {
				root, ok := rep.Root()
				if !ok {
					t.Errorf("%s states no run total", source)

					continue
				}

				want := tt.tree[0]
				if got := (row{"", root.Name, root.Kind, root.Requests.Total, root.Requests.OK, root.Requests.KO}); got != want {
					t.Errorf("%s: run total is\n  got  %+v\n  want %+v", source, got, want)
				}

				if rootOnly[source] {
					if len(rep.Nodes) != 1 {
						t.Errorf("%s states %d rows; it carries the run total alone", source, len(rep.Nodes))
					}

					continue
				}

				trees++

				compareRows(t, source, rowsOf(rep), sorted(tt.tree))
			}

			if trees == 0 {
				t.Error("no account stated a per-request tree; the recording proves only its own totals")
			}
		})
	}
}

// A recording is not required to carry every artefact — which ones exist is the
// tool version's decision, taken at capture time and unrecoverable afterwards.
// This pins what each version actually left behind, so a recording that loses a
// file fails here rather than quietly being checked against less.
func TestAccountsAreTheOnesTheVersionWrote(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version string
		want    []string
	}{
		{"3.11.5", []string{"global_stats.json", "stats.json"}},
		{"3.12.0", []string{"global_stats.json", "stats.json"}},
		// 3.13.1 also ships an index.html, and it is deliberately absent from
		// this list: its statistics table is filled in by JavaScript at page
		// load, so the file states no figures and Accounts leaves it out.
		{"3.13.1", []string{"console.txt", "js/global_stats.json", "js/stats.json"}},
		{"3.14.9", []string{"console.txt", "index.html"}},
		{"3.15.1", []string{"console.txt", "index.html"}},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			t.Parallel()

			accounts, err := corpus.Accounts(corpusDir(tt.version))
			if err != nil {
				t.Fatalf("Accounts: %v", err)
			}

			got := make([]string, 0, len(accounts))
			for source := range accounts {
				got = append(got, source)
			}

			if strings.Join(sortedStrings(got), " ") != strings.Join(tt.want, " ") {
				t.Errorf("accounts are %v, want %v", sortedStrings(got), tt.want)
			}
		})
	}
}

func sortedStrings(in []string) []string {
	out := slices.Clone(in)
	slices.Sort(out)

	return out
}

// The 3.13.x report is the one shape that exists and states nothing. It must
// come back as ErrNoFigures rather than as an error, because the figures really
// are absent from the file, and rather than as an empty report, because a
// caller that treated it as an account would compare against nothing.
func TestAJavaScriptFilledReportStatesNoFigures(t *testing.T) {
	t.Parallel()

	_, err := corpus.FromReportHTML(filepath.Join(corpusDir("3.13.1"), "index.html"))
	if !errors.Is(err, corpus.ErrNoFigures) {
		t.Fatalf("FromReportHTML = _, %v; want ErrNoFigures", err)
	}
}

// A report whose statistics table this reader does not recognise must fail, not
// come back empty. A reader that silently found nothing in a report it was
// handed cannot be told from one that checked everything and was satisfied —
// which is the failure this whole package exists to prevent.
func TestAnUnrecognisedReportFails(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "index.html")
	if err := os.WriteFile(path, []byte("<html><body><p>a report from some later Gatling</p></body></html>"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := corpus.FromReportHTML(path)
	if err == nil {
		t.Fatal("FromReportHTML accepted a report with no statistics table")
	}

	if errors.Is(err, corpus.ErrNoFigures) {
		t.Fatalf("an unrecognised report read as a known absence: %v", err)
	}
}

// Accounts fails on an artefact it cannot read rather than leaving it out. The
// point of holding a decoder to a report is lost if an unreadable report reads
// as agreement.
func TestAnUnreadableAccountFailsTheCall(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stats.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := corpus.Accounts(dir); err == nil {
		t.Fatal("Accounts accepted a stats.json it could not parse")
	}
}

// A directory with no artefacts is not an error here: what to do about a
// recording that proves nothing differs between a test over the committed
// corpus and a canary over a run that finished a minute ago, so the caller
// decides.
func TestADirectoryWithNoAccountsIsEmptyNotAnError(t *testing.T) {
	t.Parallel()

	accounts, err := corpus.Accounts(t.TempDir())
	if err != nil {
		t.Fatalf("Accounts: %v", err)
	}

	if len(accounts) != 0 {
		t.Fatalf("Accounts found %d accounts in an empty directory", len(accounts))
	}
}
