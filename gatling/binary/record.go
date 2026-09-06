package binary

import (
	"strconv"

	"github.com/galax-io/parsec/gatling"
)

// The record kinds the format writes, as the first byte of every record. They
// are fixed by Gatling's RecordHeader and unchanged across the supported range.
const (
	kindRun     = 0
	kindRequest = 1
	kindUser    = 2
	kindGroup   = 3
	kindError   = 4
)

// Every count read from the file has its own ceiling, named for what it bounds
// and justified on its own. A count sizes an allocation — a slice of string
// headers, sixteen bytes each — so a corrupt one must be stopped before the
// allocator sees it. That is what a ceiling is for, and it is not policy:
// raising one moves nothing else.
const (
	// maxGroupDepth caps the group nesting one request or group record may
	// claim. Gatling's own DSL nests far below it.
	maxGroupDepth = 1024
	// maxScenarios caps the scenario names a run record declares. A simulation
	// declares its scenarios in code, a few at most; at this ceiling a corrupt
	// count can force at most 1 MiB of headers, against a 32 MiB budget.
	maxScenarios = 1 << 16
	// maxAssertions caps the assertion payloads a run record carries. A
	// generated suite may run to thousands — two thousand is the floor the spec
	// fixes — and the same 1 MiB bound holds on the headers.
	maxAssertions = 1 << 16
	// maxAssertionBytes caps what those payloads come to in total. The count
	// bounds the headers; this bounds what the reader keeps, and the two are not
	// the same ceiling: a payload is retained for the life of the read and
	// copied again by Assertions, so a count alone would let 65,536 payloads
	// read under MaxStringLen hold hundreds of megabytes against the budget this
	// package documents. Every recorded run's assertions come to a few hundred
	// bytes.
	maxAssertionBytes = 8 << 20
	// initialCountCap is how much of an untrusted count is reserved up front.
	// The count comes from the file, so the slice grows with what is actually
	// decoded rather than with what the file claims, and a corrupt count costs
	// nothing until the elements behind it read.
	initialCountCap = 64
	// initialPathCap is the first capacity of the reused group-path scratch: a
	// sizing hint for the usual nesting, not a ceiling.
	initialPathCap = 8
)

// runHeader is what the run record carries: the header the version gate reads,
// the scenario names user records index into, and the assertion payloads.
type runHeader struct {
	header     gatling.Header
	scenarios  []string
	assertions []string
}

// readRun decodes the run record, which is the first record of the log and
// occurs exactly once.
//
// The scenario names are written as plain strings and not through the cache,
// which matters: reading them as cached would introduce entries the writer never
// made and shift every later index by as many.
func readRun(r *reader) (runHeader, error) {
	var out runHeader

	version, err := r.str("the Gatling version")
	if err != nil {
		return out, err
	}

	simulation, err := r.str("the simulation class")
	if err != nil {
		return out, err
	}

	startAt := r.off

	start, err := r.i64("the run start")
	if err != nil {
		return out, err
	}

	// Every later record resolves against this, so it is checked once here
	// rather than on each addition. The bounds are gatling.MaxRunStart's, which
	// the text codec applies to the same field: keeping them in one place is
	// what stops a run start being readable by one codec and refused by the
	// other.
	if start < 0 || start > gatling.MaxRunStart {
		return out, r.syntax(startAt, "the run start",
			"an epoch millisecond of "+strconv.FormatInt(start, 10))
	}

	description, err := r.str("the run description")
	if err != nil {
		return out, err
	}

	if out.scenarios, err = readStrings(r, maxScenarios, "the scenario count", "a scenario name"); err != nil {
		return out, err
	}

	if out.assertions, err = readBlobs(r); err != nil {
		return out, err
	}

	// The version is parsed here rather than by the gate so that a string which
	// is not a release is refused with what was actually written, quoted.
	parsed, err := gatling.ParseVersion(version)
	if err != nil {
		return out, &gatling.VersionError{Found: version, Min: minVersion, Max: maxVersion}
	}

	out.header = gatling.Header{
		SimulationClass: simulation,
		// The binary run record carries no separate run identifier. The text
		// format wrote Gatling's normalised simulation id beside the class name
		// and this one does not, so the class is the only identifier the log
		// gives and it is what RunID reports.
		//
		// Deriving the normalised form — lowercasing the simple class name, as
		// Gatling names the run directory — would be inventing a field the
		// source does not record, which is exactly what this module does not do.
		RunID:       simulation,
		Start:       start,
		Description: description,
		Version:     parsed,
	}

	return out, nil
}

// readStrings reads a count and that many plain strings, refusing a count above
// the ceiling the caller names for it.
func readStrings(r *reader, limit int32, countExpected, itemExpected string) ([]string, error) {
	at := r.off

	n, err := r.i32(countExpected)
	if err != nil {
		return nil, err
	}

	if n < 0 || n > limit {
		return nil, r.syntax(at, countExpected, "a count of "+strconv.Itoa(int(n)))
	}

	if n == 0 {
		return nil, nil
	}

	out := make([]string, 0, min(int(n), initialCountCap))

	for range n {
		s, err := r.str(itemExpected)
		if err != nil {
			return nil, err
		}

		out = append(out, s)
	}

	return out, nil
}

// readBlobs reads the assertion count and that many payloads, verbatim. Nothing
// here decodes, validates or interprets one: a payload is Gatling's own encoding
// of an assertion and this module carries it through untouched.
func readBlobs(r *reader) ([]string, error) {
	at := r.off

	n, err := r.i32("the assertion count")
	if err != nil {
		return nil, err
	}

	if n < 0 || n > maxAssertions {
		return nil, r.syntax(at, "the assertion count", "a count of "+strconv.Itoa(int(n)))
	}

	if n == 0 {
		return nil, nil
	}

	out := make([]string, 0, min(int(n), initialCountCap))

	var total int

	for range n {
		blobAt := r.off

		size, err := r.i32("an assertion payload length")
		if err != nil {
			return nil, err
		}

		buf, err := r.sized(size, blobAt, "an assertion payload")
		if err != nil {
			return nil, err
		}

		// The payloads are held for the whole read, so what they come to is
		// checked as they arrive rather than left to the count alone.
		if total += len(buf); total > maxAssertionBytes {
			return nil, r.syntax(blobAt, "the assertion payloads",
				strconv.Itoa(total)+" bytes, past the ceiling of "+strconv.Itoa(maxAssertionBytes))
		}

		out = append(out, string(buf))
	}

	return out, nil
}

// groups reads a depth and that many cached group names into the reusable
// scratch slice. The result is valid until the next record, which is what keeps
// a run of millions of records from allocating a path each.
func (r *Reader) groups() ([]string, error) {
	at := r.rd.off

	depth, err := r.rd.i32("a group depth")
	if err != nil {
		return nil, err
	}

	if depth < 0 || depth > maxGroupDepth {
		return nil, r.rd.syntax(at, "a group depth", "a depth of "+strconv.Itoa(int(depth)))
	}

	if r.path == nil {
		// Allocated once so that a record with no groups always hands out an
		// empty, non-nil slice. Slicing a nil scratch at [:0] would yield nil
		// for the first such record and an empty non-nil slice for every later
		// one, and `Groups == nil` and `len(Groups) == 0` would then disagree
		// depending on what was read before.
		r.path = make([]string, 0, initialPathCap)
	}

	path := r.path[:0]

	for range depth {
		name, err := r.cache.read(&r.rd, "a group name")
		if err != nil {
			return nil, err
		}

		path = append(path, name)
	}

	r.path = path

	// Handed out with no spare capacity. A caller appending to a group path —
	// which reads as a copy — would otherwise write into the reader's own array
	// and see the next record's groups appear in its slice.
	return path[:len(path):len(path)], nil
}

// readRequest decodes a REQUEST record: the group path it ran under, its name,
// its start and end, its outcome and its failure message.
func (r *Reader) readRequest(rec *gatling.Record) error {
	path, err := r.groups()
	if err != nil {
		return err
	}

	name, err := r.cache.read(&r.rd, "a request name")
	if err != nil {
		return err
	}

	start, err := r.instant("a request start")
	if err != nil {
		return err
	}

	end, err := r.instant("a request end")
	if err != nil {
		return err
	}

	ok, err := r.rd.boolean("a request outcome")
	if err != nil {
		return err
	}

	message, err := r.cache.read(&r.rd, "a request message")
	if err != nil {
		return err
	}

	*rec = gatling.Record{
		Kind:    gatling.KindRequest,
		Groups:  path,
		Name:    name,
		Start:   start,
		End:     end,
		Status:  status(ok),
		Message: message,
	}

	return nil
}

// readUser decodes a USER record. The format stores the scenario as an index
// into the run record's list; the name is resolved here, so that no index ever
// reaches a wire record or the canonical model — an index is a representation
// detail of this format and nothing outside it should have to know one.
func (r *Reader) readUser(rec *gatling.Record) error {
	at := r.rd.off

	index, err := r.rd.i32("a scenario index")
	if err != nil {
		return err
	}

	if index < 0 || int(index) >= len(r.run.scenarios) {
		return r.rd.syntax(at, "a scenario index", "index "+strconv.Itoa(int(index))+
			", where the run declared "+strconv.Itoa(len(r.run.scenarios))+" scenarios")
	}

	isStart, err := r.rd.boolean("a user event")
	if err != nil {
		return err
	}

	when, err := r.instant("a user event time")
	if err != nil {
		return err
	}

	event := gatling.EventEnd
	if isStart {
		event = gatling.EventStart
	}

	*rec = gatling.Record{
		Kind:      gatling.KindUser,
		Scenario:  r.run.scenarios[index],
		Event:     event,
		Timestamp: when,
	}

	return nil
}

// readGroup decodes a GROUP record. It carries two different durations and the
// record holds both: End - Start is wall clock across the traversal, pauses
// included, and CumulatedResponseTime is the sum of the requests inside it.
func (r *Reader) readGroup(rec *gatling.Record) error {
	path, err := r.groups()
	if err != nil {
		return err
	}

	start, err := r.instant("a group start")
	if err != nil {
		return err
	}

	end, err := r.instant("a group end")
	if err != nil {
		return err
	}

	cumulated, err := r.rd.i32("a group cumulated response time")
	if err != nil {
		return err
	}

	ok, err := r.rd.boolean("a group outcome")
	if err != nil {
		return err
	}

	*rec = gatling.Record{
		Kind:                  gatling.KindGroup,
		Groups:                path,
		Start:                 start,
		End:                   end,
		Status:                status(ok),
		CumulatedResponseTime: int64(cumulated),
	}

	return nil
}

// readError decodes an ERROR record: a crash message and when it happened.
func (r *Reader) readError(rec *gatling.Record) error {
	message, err := r.cache.read(&r.rd, "an error message")
	if err != nil {
		return err
	}

	when, err := r.instant("an error time")
	if err != nil {
		return err
	}

	*rec = gatling.Record{Kind: gatling.KindError, Message: message, Timestamp: when}

	return nil
}

// instant resolves one of the format's 32-bit millisecond offsets against the
// run's start, into the absolute epoch millisecond the wire records carry.
//
// The offset is signed and Gatling's own writer overflows past about 24.8 days,
// so the format cannot represent a longer run. A negative offset would place the
// event before the run began, which the writer never intends — it stores a delta
// from the start — and none of the recorded runs contains one. It is reported as
// absent rather than refused: FR-009 asks for a value that cannot be resolved to
// be reported absent and never wrapped or guessed, and one such field in a
// ten-million-record log should not end the read.
//
// The value reported is gatling.AbsentTimestamp, the one sentinel both codecs
// write, so a consumer has one thing to check rather than two. readRun's bound on
// the run start is what keeps a resolved time from colliding with it: a
// non-negative start plus a non-negative offset is never negative, and the
// addition cannot overflow because that bound keeps start plus any int32 offset
// inside an int64.
func (r *Reader) instant(expected string) (int64, error) {
	offset, err := r.rd.i32(expected)
	if err != nil {
		return 0, err
	}

	if offset < 0 {
		return gatling.AbsentTimestamp, nil
	}

	return r.run.header.Start + int64(offset), nil
}

func status(ok bool) gatling.Status {
	if ok {
		return gatling.StatusOK
	}

	return gatling.StatusKO
}
