package corpus

import (
	"fmt"
	"os"
	"strings"

	"github.com/galax-io/parsec/gatling"
)

// RunsEnv is the variable the canary reads its run directories from.
const RunsEnv = "PARSEC_CANARY_RUNS"

// Run is one directory a tool has just written, and the version that wrote it.
type Run struct {
	Version gatling.Version
	Dir     string
}

// ParseRuns reads the RunsEnv spelling: "version=dir" pairs separated by ";".
//
// It is deliberately strict. A canary exists to hold a decoder to a tool that
// ran a minute ago, so a malformed entry has to fail rather than be skipped —
// a skipped entry is a version nobody tested, wearing the same green as one
// that passed.
func ParseRuns(spec string) ([]Run, error) {
	var runs []Run

	for pair := range strings.SplitSeq(spec, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		version, dir, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("%s entry %q is not version=dir", RunsEnv, pair)
		}

		v, err := gatling.ParseVersion(strings.TrimSpace(version))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", RunsEnv, err)
		}

		runs = append(runs, Run{Version: v, Dir: strings.TrimSpace(dir)})
	}

	return runs, nil
}

// Summarize records a line in the job summary when there is one, and returns it
// so a caller can also put it in the test log.
//
// A canary that passes silently says nothing about which versions it tested, and
// "no version was tested" is the failure it most needs to make visible.
func Summarize(line string) {
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		return
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600) //nolint:gosec // the runner's own summary file
	if err != nil {
		return
	}

	defer func() { _ = f.Close() }()

	_, _ = fmt.Fprintf(f, "- %s\n", line)
}
