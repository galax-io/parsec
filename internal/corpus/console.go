package corpus

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// The Global Information request count, in the two shapes Gatling has written
// it. It reworded the console summary at 3.14.0:
//
//	3.13.x   > request count       102 (OK=84     KO=18    )
//	3.14.0+  > request count   |   102 |    84 |    18
var (
	consoleOld = regexp.MustCompile(`request count\s+(\d+) \(OK=(\d+)\s+KO=(\d+)`)
	consoleNew = regexp.MustCompile(`request count\s*\|\s*([\d,]+)\s*\|\s*([\d,]+)\s*\|\s*([\d,]+)`)
)

// FromConsole reads the run total out of a captured console summary.
//
// It states the run total and nothing else — no per-request rows, no rate — and
// that is the artefact's own limit, not this reader's. From 3.14.0, when
// Gatling stopped writing stats.json, the console became one of only two
// accounts a run gives of its own numbers, and the only one that exists purely
// because standard output was redirected while the run happened. It cannot be
// recovered from a run that has already finished, which is why a recording
// captures it or never has it.
func FromConsole(path string) (Report, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // a path inside a recorded run directory, chosen by the caller
	if err != nil {
		return Report{}, fmt.Errorf("opening the console summary: %w", err)
	}

	text := string(raw)

	for _, re := range []*regexp.Regexp{consoleNew, consoleOld} {
		m := re.FindStringSubmatch(text)
		if m == nil {
			continue
		}

		requests, err := consoleTriple(path, m[1], m[2], m[3])
		if err != nil {
			return Report{}, err
		}

		return Report{
			Source: path,
			Nodes: []Node{{
				ID:       rootID,
				Name:     "All Requests",
				Kind:     KindRoot,
				Requests: requests,
			}},
		}, nil
	}

	return Report{}, fmt.Errorf(
		"%s: no Global Information request count in either shape Gatling has written it", path)
}

// consoleTriple parses the three counts of the summary line.
func consoleTriple(path, total, ok, ko string) (Triple, error) {
	out := Triple{}

	for _, c := range []struct {
		name string
		text string
		into *int64
	}{
		{"total", total, &out.Total},
		{"ok", ok, &out.OK},
		{"ko", ko, &out.KO},
	} {
		// The 3.14.0 summary groups thousands with a comma.
		n, err := strconv.ParseInt(strings.ReplaceAll(c.text, ",", ""), 10, 64)
		if err != nil {
			return Triple{}, fmt.Errorf("%s: the %s count is %q, which is not an integer", path, c.name, c.text)
		}

		*c.into = n
	}

	return out, nil
}
