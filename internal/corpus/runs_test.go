package corpus_test

import (
	"os"
	"strings"
	"testing"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/internal/corpus"
)

// The canary reads its run directories from one environment variable, and a
// malformed entry has to fail rather than be skipped: a skipped entry is a
// version nobody tested, wearing the same green as one that passed.
func TestParseRuns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec string
		want []corpus.Run
		fail string
	}{
		{
			name: "one pair",
			spec: "3.15.1=/runs/v3.15.1",
			want: []corpus.Run{{Version: gatling.Version{Major: 3, Minor: 15, Patch: 1}, Dir: "/runs/v3.15.1"}},
		},
		{
			name: "both formats in one value, which is how the cross-format comparison gets its runs",
			spec: "3.12.0=/runs/text;3.15.1=/runs/binary",
			want: []corpus.Run{
				{Version: gatling.Version{Major: 3, Minor: 12}, Dir: "/runs/text"},
				{Version: gatling.Version{Major: 3, Minor: 15, Patch: 1}, Dir: "/runs/binary"},
			},
		},
		{
			name: "a trailing separator is not an entry",
			spec: "3.15.1=/runs/v3.15.1;",
			want: []corpus.Run{{Version: gatling.Version{Major: 3, Minor: 15, Patch: 1}, Dir: "/runs/v3.15.1"}},
		},
		{
			name: "surrounding space is not part of the version or the path",
			spec: " 3.15.1 = /runs/v3.15.1 ",
			want: []corpus.Run{{Version: gatling.Version{Major: 3, Minor: 15, Patch: 1}, Dir: "/runs/v3.15.1"}},
		},
		{
			name: "an entry with no separator fails rather than being skipped",
			spec: "3.15.1",
			fail: "is not version=dir",
		},
		{
			name: "a version that is not a version fails",
			spec: "not-a-version=/runs/x",
			fail: corpus.RunsEnv,
		},
		{
			name: "one bad entry fails the whole value, so no run is silently dropped",
			spec: "3.15.1=/runs/ok;garbage",
			fail: "is not version=dir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := corpus.ParseRuns(tt.spec)

			if tt.fail != "" {
				if err == nil {
					t.Fatalf("ParseRuns(%q) = %v, nil; want an error naming %q", tt.spec, got, tt.fail)
				}

				if !strings.Contains(err.Error(), tt.fail) {
					t.Errorf("the error is %q; it must name %q", err, tt.fail)
				}

				return
			}

			if err != nil {
				t.Fatalf("ParseRuns(%q): %v", tt.spec, err)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("ParseRuns(%q) returned %d runs, want %d", tt.spec, len(got), len(tt.want))
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("run %d is %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// An empty value is not an error: the caller decides what an unset canary means,
// and for a test that answer is to skip with a reason rather than to fail.
func TestParseRunsAcceptsAnEmptyValue(t *testing.T) {
	t.Parallel()

	got, err := corpus.ParseRuns("")
	if err != nil || len(got) != 0 {
		t.Fatalf("ParseRuns(\"\") = %v, %v; want no runs and no error", got, err)
	}
}

// Summarize writes to the runner's job summary when there is one and is a no-op
// when there is not, so a local run is not a special case in the caller.
func TestSummarizeWritesTheJobSummaryWhenThereIsOne(t *testing.T) {
	path := t.TempDir() + "/summary.md"
	t.Setenv("GITHUB_STEP_SUMMARY", path)

	corpus.Summarize("Gatling 3.15.1: matched its own report")
	corpus.Summarize("Gatling 3.14.9: matched its own report")

	raw, err := readFile(path)
	if err != nil {
		t.Fatalf("the summary was not written: %v", err)
	}

	if want := "- Gatling 3.15.1: matched its own report\n- Gatling 3.14.9: matched its own report\n"; raw != want {
		t.Errorf("the summary is %q, want %q", raw, want)
	}
}

func TestSummarizeIsSilentWithoutAJobSummary(t *testing.T) {
	t.Setenv("GITHUB_STEP_SUMMARY", "")
	corpus.Summarize("this goes nowhere and must not panic")
}

func readFile(path string) (string, error) {
	b, err := os.ReadFile(path) //nolint:gosec // a path this test just created
	return string(b), err
}
