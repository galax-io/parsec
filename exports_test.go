package parsec_test

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/galax-io/parsec/internal/exports"
)

var update = flag.Bool("update", false, "rewrite testdata/api/surface.txt from the module's exported surface")

// packages is the module's public surface, in the order the contract table
// lists them: the canonical model first, then what a Gatling source is decoded
// through. internal/ is absent because it is not importable and carries no
// promise.
var packages = []string{
	"model",
	"gatling",
	"gatling/text",
	"gatling/binary",
	"gatling/simlog",
	"gatling/run",
}

// golden is the contract table this module freezes at v0.1.0, held as data so
// that the freeze is a list and not a description.
const golden = "testdata/api/surface.txt"

// From v0.1.0 every identifier below is a promise: changing its signature or
// its observable behaviour is a breaking change, and removing it costs a
// deprecation window and a MINOR release. The list is generated rather than
// typed, because a typed one agrees on the day it is written and drifts on the
// first merge.
//
// gorelease, which scripts/check-compat.sh reads, decides whether a change is
// compatible. It never reports what the surface *is*, so an addition passes it
// silently — which is the second question a reviewer of this module has to
// answer, and the older one: this module computes no statistic, and a count, a
// mean, an extreme, a percentile, a range or a series could arrive as a
// function, a method or a struct field. Every one of those is a line here, and
// an addition fails this test until the golden is rewritten on purpose.
func TestExportedSurfaceIsGolden(t *testing.T) {
	t.Parallel()

	got, err := exports.Render(packages)
	if err != nil {
		t.Fatal(err)
	}

	if *update {
		if err := exports.Write(golden, got); err != nil {
			t.Fatal(err)
		}

		t.Logf("rewrote %s — review every changed line before committing it", golden)

		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v — generate it with -update, then review it", err)
	}

	if got != string(want) {
		t.Fatalf("the exported surface differs from %s; if that is intended, rerun with -update and review the diff:\n%s",
			golden, exports.Diff(got, string(want)))
	}
}

// A count in the golden that does not match the lines beneath it would let a
// hand-edit add an identifier without saying so. The totals are what the
// contract table and CHANGELOG.md quote, so they are checked as numbers rather
// than trusted as prose.
func TestExportedSurfaceCountsAreRecorded(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("%v — generate it with -update, then review it", err)
	}

	var (
		counted, declared, total int
		header                   string
		seen                     []string
	)

	closeHeader := func() {
		if header != "" && counted != declared {
			t.Errorf("%s: header says %d identifiers, %d are listed", header, declared, counted)
		}
	}

	for line := range strings.SplitSeq(strings.TrimRight(string(data), "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "## "):
			closeHeader()

			header, declared = parseHeader(t, line)
			counted = 0

			seen = append(seen, header)
		case strings.HasPrefix(line, "TOTAL "):
			closeHeader()

			header = ""

			total = parseTotal(t, line)
		case strings.TrimSpace(line) != "":
			counted++
		}
	}

	if !slices.Equal(seen, packages) {
		t.Errorf("golden lists packages %v, want %v", seen, packages)
	}

	if lines := strings.Count(string(data), "\n    "); total != lines {
		t.Errorf("TOTAL = %d, %d identifiers are listed", total, lines)
	}
}

// Gate and MaxRunStart were exported and are not frozen: Gate is a second
// entrance to the rule Policy.Apply documents itself as the only place for, and
// MaxRunStart is a ceiling the two codecs read and no consumer computes with.
// Their absence is a line in CHANGELOG.md under Removed, and this is what keeps
// it true.
func TestWithdrawnIdentifiersAreGone(t *testing.T) {
	t.Parallel()

	surface, err := exports.Surface("gatling")
	if err != nil {
		t.Fatal(err)
	}

	for _, withdrawn := range []string{"func Gate", "const MaxRunStart"} {
		if slices.Contains(surface, withdrawn) {
			t.Errorf("gatling still exports %q; it was withdrawn before the v0.1.0 freeze", withdrawn)
		}
	}
}

func parseHeader(t *testing.T, line string) (string, int) {
	t.Helper()

	name, count, ok := strings.Cut(strings.TrimPrefix(line, "## "), "  (")
	if !ok {
		t.Fatalf("malformed package header %q", line)
	}

	n, err := strconv.Atoi(strings.TrimSuffix(count, ")"))
	if err != nil {
		t.Fatalf("malformed count in %q: %v", line, err)
	}

	return name, n
}

func parseTotal(t *testing.T, line string) int {
	t.Helper()

	var n int
	if _, err := fmt.Sscanf(line, "TOTAL %d", &n); err != nil {
		t.Fatalf("malformed total %q: %v", line, err)
	}

	return n
}
