package corpus

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// The generated report's statistics table, read with the standard library.
//
// An HTML parser would be the obvious tool and is not available: model/ and
// gatling/ are standard-library only (Principle IV), no third-party module is
// pre-approved anywhere, and a test-only import would still land in go.mod,
// which three downstream builds inherit. What makes the standard library enough
// here is that the markup is generated rather than authored: every figure sits
// in a cell classed with its column number, every row carries Gatling's own id
// and a link to its parent, and none of it is hand-edited between releases.
//
// The trade is that an unrecognised shape must fail loudly rather than quietly
// match nothing — see [FromReportHTML].
var (
	// reportRow matches one row of a statistics table. Rows of the assertions
	// table carry no id and are skipped by construction.
	reportRow = regexp.MustCompile(`(?s)<tr id="([^"]*)"([^>]*)>(.*?)</tr>`)

	rowParent = regexp.MustCompile(`data-parent="([^"]*)"`)
	rowName   = regexp.MustCompile(`class="[^"]*\bellipsed-name\b[^"]*"[^>]*>([^<]*)</span>`)
)

// The two tables the figures live in: the run total in one, every request and
// group in the other.
const (
	headTable = `id="container_statistics_head"`
	bodyTable = `id="container_statistics_body"`
)

// statisticsTable returns the markup of one statistics table, from its opening
// tag to the end of the table.
//
// Rows are read from inside these two tables and nowhere else, which is not
// fussiness. A 3.13.x report carries a <script> block that builds its rows at
// page load, and that script contains the literal text
//
//	'<tr id="' + request.pathFormatted + '" data-parent=…
//
// A scan of the whole document matches it and reads a template as a row. Bounding
// the scan by the tables is what tells a figure Gatling wrote from a figure
// Gatling wrote instructions for writing.
func statisticsTable(doc, id string) (string, bool) {
	start := strings.Index(doc, id)
	if start < 0 {
		return "", false
	}

	end := strings.Index(doc[start:], "</table>")
	if end < 0 {
		return "", false
	}

	return doc[start : start+end], true
}

// reportCell builds the matcher for one numbered column of a statistics row.
//
// The class list is written in a different order on different rows — the run
// total says "value total col-2" where an error column says "error-col-2 value
// ok total" — so the column is matched as one word among several rather than as
// a fixed string.
func reportCell(col int) *regexp.Regexp {
	return regexp.MustCompile(`class="[^"]*\bcol-` + strconv.Itoa(col) + `\b[^"]*"[^>]*>([^<]*)</td>`)
}

// The columns this reader takes. The report states response times in col-7
// through col-14; those are measurements this module has nothing to compare
// against, because it computes no statistic.
var (
	cellTotal = reportCell(2)
	cellOK    = reportCell(3)
	cellKO    = reportCell(4)
	cellRate  = reportCell(6)
)

// FromReportHTML reads the per-request tree out of a generated Gatling report.
//
// From 3.14.0 this is the only machine-readable account a run gives of its own
// per-request numbers: Gatling stopped writing stats.json at that release, and
// the console summary states the run total alone.
//
// A 3.13.x report comes back as [ErrNoFigures]. Its statistics table is present
// but empty — the figures are inserted by JavaScript at page load and are
// genuinely not in the file — and the JSON beside it carries them instead.
//
// A report whose statistics table is missing altogether is an error. That is
// the case where the markup has changed in a way this reader does not
// understand, and reporting nothing found would be indistinguishable from
// checking everything and being satisfied.
func FromReportHTML(path string) (Report, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // a path inside a recorded run directory, chosen by the caller
	if err != nil {
		return Report{}, fmt.Errorf("opening the report: %w", err)
	}

	doc := string(raw)

	body, ok := statisticsTable(doc, bodyTable)
	if !ok {
		return Report{}, fmt.Errorf(
			"%s: no statistics table; the report's markup is not the shape this reader knows", path)
	}

	head, _ := statisticsTable(doc, headTable)

	var matches [][]string
	for _, table := range []string{head, body} {
		matches = append(matches, reportRow.FindAllStringSubmatch(table, -1)...)
	}

	if len(matches) == 0 {
		return Report{}, fmt.Errorf("%w: %s states its figures through JavaScript, not in the markup",
			ErrNoFigures, path)
	}

	rep := Report{Source: path}

	for _, m := range matches {
		node, err := reportNodeFrom(m[1], m[2], m[3])
		if err != nil {
			return Report{}, fmt.Errorf("%s: %w", path, err)
		}

		rep.Nodes = append(rep.Nodes, node)
	}

	if err := rep.validate(); err != nil {
		return Report{}, err
	}

	if err := rep.requireTree(); err != nil {
		return Report{}, err
	}

	return rep, nil
}

// reportNodeFrom turns one matched row into a node. id and attrs come from the
// opening tag, body from between the tags.
func reportNodeFrom(id, attrs, body string) (Node, error) {
	kind, err := reportKind(id)
	if err != nil {
		return Node{}, err
	}

	var parent string
	if m := rowParent.FindStringSubmatch(attrs); m != nil {
		parent = m[1]
	}

	if kind == KindRoot {
		parent = ""
	}

	name := ""
	if m := rowName.FindStringSubmatch(body); m != nil {
		name = html.UnescapeString(strings.TrimSpace(m[1]))
	}

	requests, err := reportTriple(id, body)
	if err != nil {
		return Node{}, err
	}

	return Node{
		ID:       id,
		Parent:   parent,
		Name:     name,
		Kind:     kind,
		Requests: requests,
		// The report has one events-per-second column and no per-outcome
		// split, where stats.json states all three. OK and KO stay empty:
		// the absence is the report's, not a zero.
		Rate: Rates{Total: json.Number(cellText(cellRate, body))},
	}, nil
}

// reportKind reads the kind out of Gatling's own row id.
func reportKind(id string) (NodeKind, error) {
	switch {
	case id == rootID:
		return KindRoot, nil
	case strings.HasPrefix(id, "group_"):
		return KindGroup, nil
	case strings.HasPrefix(id, "req_"):
		return KindRequest, nil
	default:
		return 0, fmt.Errorf("row id %q is neither the run total nor a group or request", id)
	}
}

// reportTriple reads the three count cells of a row. All three are required: a
// row that states a total and no outcome split is a shape this reader does not
// understand, and guessing at it would compare against a number nobody wrote.
func reportTriple(id, body string) (Triple, error) {
	var out Triple

	for _, c := range []struct {
		name string
		re   *regexp.Regexp
		into *int64
	}{
		{"total", cellTotal, &out.Total},
		{"ok", cellOK, &out.OK},
		{"ko", cellKO, &out.KO},
	} {
		text := cellText(c.re, body)
		if text == "" {
			return Triple{}, fmt.Errorf("row %q states no %s count", id, c.name)
		}

		// Gatling groups thousands with a comma once a count is large enough.
		n, err := strconv.ParseInt(strings.ReplaceAll(text, ",", ""), 10, 64)
		if err != nil {
			return Triple{}, fmt.Errorf("row %q states a %s count of %q, which is not an integer", id, c.name, text)
		}

		*c.into = n
	}

	return out, nil
}

// cellText returns the trimmed contents of the first cell the matcher finds, or
// the empty string when the row has no such cell.
func cellText(re *regexp.Regexp, body string) string {
	m := re.FindStringSubmatch(body)
	if m == nil {
		return ""
	}

	return strings.TrimSpace(m[1])
}
