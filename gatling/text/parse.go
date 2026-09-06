package text

import (
	"bytes"
	"math"
	"strconv"

	"github.com/galax-io/parsec/gatling"
)

// minVersion and maxVersion bound what this codec accepts without a warning.
// They equal the range the golden corpus covers and are read through
// SupportedVersions: a caller must not be able to widen the gate.
var (
	minVersion = gatling.Version{Major: 3, Minor: 11, Patch: 5}
	maxVersion = gatling.Version{Major: 3, Minor: 12, Patch: 0}
)

// SupportedVersions returns the oldest and newest Gatling release this codec
// accepts without a warning. The range equals the versions covered by the
// golden corpus; widening it means recording a new corpus entry first.
func SupportedVersions() (oldest, newest gatling.Version) {
	return minVersion, maxVersion
}

// versionPolicy is this codec's gate, stated once. The decision it applies
// lives in gatling, so that the binary codec inherits the same one rather than
// growing a second copy of it.
var versionPolicy = gatling.Policy{Min: minVersion, Max: maxVersion}

const (
	tab            = '\t'
	groupSeparator = ','
)

// The literals a record opens with, and the values its fields may hold.
// Comparing string(b) against a constant does not allocate.
const (
	kindRun       = "RUN"
	kindUser      = "USER"
	kindRequest   = "REQUEST"
	kindGroup     = "GROUP"
	kindError     = "ERROR"
	kindAssertion = "ASSERTION"
	statusOK      = "OK"
	statusKO      = "KO"
	eventStart    = "START"
	eventEnd      = "END"
	// absent is how Gatling writes an empty description or message.
	absent = " "
	// neverCompleted is the end timestamp Gatling reserves for an event that
	// never completed: the minimum signed 64-bit integer, written in decimal.
	neverCompleted = "-9223372036854775808"
)

// Field counts per kind, the kind literal included. Inside the covered range
// they are exact; above it they are minimums (FR-008a). The error record is
// the exception: its message may span separators (FR-008b).
const (
	runFields       = 6
	userFields      = 4
	requestFields   = 7
	groupFields     = 6
	errorFields     = 3
	assertionFields = 2
)

// parser decodes event records. It owns its scratch slices and the name table
// so that decoding a record allocates only the first time a value is seen.
type parser struct {
	isLenient bool
	fields    [][]byte
	groups    []string
	names     *interner
}

func newParser(isLenient bool) *parser {
	return &parser{
		isLenient: isLenient,
		fields:    make([][]byte, 0, requestFields),
		groups:    make([]string, 0, 4),
		names:     newInterner(),
	}
}

// quoteMax bounds how much of an offending value an error may carry. A line
// runs to MaxLineLen and a field can be nearly all of it, so quoting one whole
// would put a megabyte in the error — and quoting escapes, so a binary field
// costs four bytes per byte. The head is what identifies the damage.
const quoteMax = 64

// quote renders b for an error message, shortened when it is long enough to
// matter. The ellipsis sits outside the quotes so the value stays unambiguous.
func quote(b []byte) string {
	if len(b) <= quoteMax {
		return strconv.Quote(string(b))
	}

	return strconv.Quote(string(b[:quoteMax])) + "\u2026 (" + strconv.Itoa(len(b)) + " bytes)"
}

// kindOf returns the literal that opens the line.
func kindOf(line []byte) []byte {
	kind, _, _ := bytes.Cut(line, []byte{tab})

	return kind
}

// split appends the first want fields of line to dst and returns them with the
// total number of fields the line has. It walks the line once: on fields ten
// bytes long a per-field bytes.IndexByte spent more in call overhead than in
// searching, and the single pass measured 12% faster end to end (benchstat,
// n=6, p=0.002).
func split(dst [][]byte, line []byte, want int) ([][]byte, int) {
	dst = dst[:0]
	total := 1
	start := 0

	for i, c := range line {
		if c != tab {
			continue
		}

		if total <= want {
			dst = append(dst, line[start:i])
		}

		total++
		start = i + 1
	}

	if total <= want {
		dst = append(dst, line[start:])
	}

	return dst, total
}

func fieldCountError(lineNo int, kind string, want, got int) error {
	return &gatling.SyntaxError{
		Format:   gatling.FormatText,
		Line:     lineNo,
		Expected: kind + " with " + strconv.Itoa(want) + " fields",
		Found:    strconv.Itoa(got) + " fields",
	}
}

// parseHeader decodes a RUN line. It returns the number of fields the line
// carried so the caller can apply the exact-count rule once the version, and
// therefore the verdict, is known. A version that is not a plain release is a
// *gatling.VersionError quoting it.
func parseHeader(line []byte, lineNo int) (gatling.Header, int, error) {
	fields, n := split(make([][]byte, 0, runFields), line, runFields)
	if n < runFields {
		return gatling.Header{}, n, fieldCountError(lineNo, kindRun, runFields, n)
	}

	start, err := parseTimestamp(fields[3], lineNo, "start")
	if err != nil {
		return gatling.Header{}, n, err
	}

	version, err := gatling.ParseVersion(string(fields[5]))
	if err != nil {
		return gatling.Header{}, n, &gatling.VersionError{Found: string(fields[5]), Min: minVersion, Max: maxVersion}
	}

	return gatling.Header{
		SimulationClass: string(fields[1]),
		RunID:           string(fields[2]),
		Start:           start,
		Description:     text(fields[4]),
		Version:         version,
	}, n, nil
}

// parse decodes one event line.
func (p *parser) parse(line []byte, lineNo int) (gatling.Record, error) {
	if len(line) == 0 {
		return gatling.Record{}, &gatling.SyntaxError{Format: gatling.FormatText, Line: lineNo, Expected: "a record", Found: "an empty line"}
	}

	kind := kindOf(line)

	switch string(kind) {
	case kindUser:
		return p.parseUser(line, lineNo)
	case kindRequest:
		return p.parseRequest(line, lineNo)
	case kindGroup:
		return p.parseGroup(line, lineNo)
	case kindError:
		return p.parseError(line, lineNo)
	case kindAssertion:
		return p.parseAssertion(line, lineNo)
	case kindRun:
		return gatling.Record{}, &gatling.SyntaxError{
			Format:   gatling.FormatText,
			Line:     lineNo,
			Expected: "an event record",
			Found:    "RUN, a second run header",
		}
	default:
		return gatling.Record{}, &gatling.SyntaxError{
			Format:   gatling.FormatText,
			Line:     lineNo,
			Expected: "a record kind",
			Found:    quote(kind),
		}
	}
}

// fieldsOf splits the line and applies the count rule for its kind.
func (p *parser) fieldsOf(line []byte, lineNo int, kind string, want int) ([][]byte, error) {
	fields, n := split(p.fields, line, want)
	p.fields = fields

	if n < want || (n > want && !p.isLenient) {
		return nil, fieldCountError(lineNo, kind, want, n)
	}

	return fields, nil
}

func (p *parser) parseUser(line []byte, lineNo int) (gatling.Record, error) {
	f, err := p.fieldsOf(line, lineNo, kindUser, userFields)
	if err != nil {
		return gatling.Record{}, err
	}

	event, err := parseEvent(f[2], lineNo)
	if err != nil {
		return gatling.Record{}, err
	}

	ts, err := parseTimestamp(f[3], lineNo, "timestamp")
	if err != nil {
		return gatling.Record{}, err
	}

	return gatling.Record{
		Kind:      gatling.KindUser,
		Line:      lineNo,
		Scenario:  p.names.intern(f[1]),
		Event:     event,
		Timestamp: ts,
	}, nil
}

func (p *parser) parseRequest(line []byte, lineNo int) (gatling.Record, error) {
	f, err := p.fieldsOf(line, lineNo, kindRequest, requestFields)
	if err != nil {
		return gatling.Record{}, err
	}

	start, err := parseTimestamp(f[3], lineNo, "start")
	if err != nil {
		return gatling.Record{}, err
	}

	// The end may be the sentinel Gatling reserves for an event that never
	// completed, which is the one negative timestamp the format admits.
	end, err := parseEnd(f[4], lineNo)
	if err != nil {
		return gatling.Record{}, err
	}

	status, err := parseStatus(f[5], lineNo)
	if err != nil {
		return gatling.Record{}, err
	}

	return gatling.Record{
		Kind:    gatling.KindRequest,
		Line:    lineNo,
		Groups:  p.parseGroups(f[1]),
		Name:    p.names.intern(f[2]),
		Start:   start,
		End:     end,
		Status:  status,
		Message: p.message(f[6]),
	}, nil
}

func (p *parser) parseGroup(line []byte, lineNo int) (gatling.Record, error) {
	f, err := p.fieldsOf(line, lineNo, kindGroup, groupFields)
	if err != nil {
		return gatling.Record{}, err
	}

	start, err := parseTimestamp(f[2], lineNo, "start")
	if err != nil {
		return gatling.Record{}, err
	}

	end, err := parseTimestamp(f[3], lineNo, "end")
	if err != nil {
		return gatling.Record{}, err
	}

	cumulated, err := parseTimestamp(f[4], lineNo, "cumulated response time")
	if err != nil {
		return gatling.Record{}, err
	}

	status, err := parseStatus(f[5], lineNo)
	if err != nil {
		return gatling.Record{}, err
	}

	return gatling.Record{
		Kind:                  gatling.KindGroup,
		Line:                  lineNo,
		Groups:                p.parseGroups(f[1]),
		Start:                 start,
		End:                   end,
		CumulatedResponseTime: cumulated,
		Status:                status,
	}, nil
}

// parseError takes the message as everything between the kind and the final
// timestamp field, because Gatling writes it unescaped and it may itself
// contain separators (FR-008b).
func (p *parser) parseError(line []byte, lineNo int) (gatling.Record, error) {
	first := bytes.IndexByte(line, tab)
	last := bytes.LastIndexByte(line, tab)

	if first < 0 || last == first {
		_, n := split(nil, line, 0)

		return gatling.Record{}, fieldCountError(lineNo, kindError, errorFields, n)
	}

	// The timestamp closes the record, so inside the covered range it is the
	// field after the last separator. Above the range a newer version may have
	// appended fields behind it (FR-008a) — and the message may hold separators
	// of its own (FR-008b), so the count cannot say how many — which leaves the
	// last field that reads as a timestamp as the only way to find it. Falling
	// back to the last field when none does keeps the error the natural one.
	at, end := last, len(line)

	if p.isLenient {
		for at > first {
			if _, ok := parseDigits(line[at+1 : end]); ok {
				break
			}

			end, at = at, bytes.LastIndexByte(line[:at], tab)
		}

		if at <= first {
			at, end = last, len(line)
		}
	}

	ts, err := parseTimestamp(line[at+1:end], lineNo, "timestamp")
	if err != nil {
		return gatling.Record{}, err
	}

	return gatling.Record{
		Kind:      gatling.KindError,
		Line:      lineNo,
		Message:   p.message(line[first+1 : at]),
		Timestamp: ts,
	}, nil
}

func (p *parser) parseAssertion(line []byte, lineNo int) (gatling.Record, error) {
	f, err := p.fieldsOf(line, lineNo, kindAssertion, assertionFields)
	if err != nil {
		return gatling.Record{}, err
	}

	return gatling.Record{Kind: gatling.KindAssertion, Line: lineNo, Payload: string(f[1])}, nil
}

// parseGroups splits a comma-separated path into the reusable groups slice. An
// empty path is a top-level record and yields an empty, non-nil slice.
func (p *parser) parseGroups(path []byte) []string {
	p.groups = p.groups[:0]

	if len(path) == 0 {
		return p.sealGroups()
	}

	for {
		i := bytes.IndexByte(path, groupSeparator)
		if i < 0 {
			p.groups = append(p.groups, p.names.intern(path))

			return p.sealGroups()
		}

		p.groups = append(p.groups, p.names.intern(path[:i]))
		path = path[i+1:]
	}
}

// sealGroups hands out the scratch slice with no spare capacity, so that a
// caller's append allocates instead of writing into the reader's own array.
// The record's doc says Groups is only valid until the next Next, but append
// reads as a copy and would otherwise silently not be one: the slice it
// returned would still alias the scratch, and the next record would rewrite it.
func (p *parser) sealGroups() []string {
	return p.groups[:len(p.groups):len(p.groups)]
}

// text decodes a free-text field that appears once per log: a lone space is
// how Gatling writes an absent value, and it decodes as empty.
func text(b []byte) string {
	if string(b) == absent {
		return ""
	}

	return string(b)
}

// message decodes a failure or error text the same way, sharing repeated
// values: a run tends to fail the same way many times.
func (p *parser) message(b []byte) string {
	if string(b) == absent {
		return ""
	}

	return p.names.intern(b)
}

func parseStatus(b []byte, lineNo int) (gatling.Status, error) {
	switch string(b) {
	case statusOK:
		return gatling.StatusOK, nil
	case statusKO:
		return gatling.StatusKO, nil
	default:
		return gatling.StatusUnknown, &gatling.SyntaxError{
			Format:   gatling.FormatText,
			Line:     lineNo,
			Expected: "OK or KO",
			Found:    quote(b),
		}
	}
}

func parseEvent(b []byte, lineNo int) (gatling.Event, error) {
	switch string(b) {
	case eventStart:
		return gatling.EventStart, nil
	case eventEnd:
		return gatling.EventEnd, nil
	default:
		return gatling.EventUnknown, &gatling.SyntaxError{
			Format:   gatling.FormatText,
			Line:     lineNo,
			Expected: "START or END",
			Found:    quote(b),
		}
	}
}

// parseTimestamp reads a non-negative decimal integer without allocating.
func parseTimestamp(b []byte, lineNo int, what string) (int64, error) {
	v, ok := parseDigits(b)
	if !ok {
		return 0, &gatling.SyntaxError{
			Format:   gatling.FormatText,
			Line:     lineNo,
			Expected: "a non-negative integer " + what,
			Found:    quote(b),
		}
	}

	return v, nil
}

// parseEnd reads a request end: a non-negative timestamp, or the sentinel that
// marks an event that never completed.
func parseEnd(b []byte, lineNo int) (int64, error) {
	if len(b) > 0 && b[0] == '-' {
		if string(b) == neverCompleted {
			return math.MinInt64, nil
		}

		return 0, &gatling.SyntaxError{
			Format:   gatling.FormatText,
			Line:     lineNo,
			Expected: "a non-negative integer end, or the never-completed sentinel",
			Found:    quote(b),
		}
	}

	return parseTimestamp(b, lineNo, "end")
}

// parseDigits parses ASCII digits into an int64, rejecting anything else and
// any value that overflows.
func parseDigits(b []byte) (int64, bool) {
	if len(b) == 0 || len(b) > 19 {
		return 0, false
	}

	var v int64

	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, false
		}

		v = v*10 + int64(c-'0')
	}

	return v, v >= 0
}
