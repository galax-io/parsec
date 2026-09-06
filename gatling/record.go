package gatling

import (
	"math"
	"strconv"
)

// Kind identifies which record a simulation.log line carries. KindUnknown is
// the zero value, so a Record that was never decoded cannot pass for one.
type Kind uint8

// The record kinds a Gatling 3.11.5–3.12.0 text simulation.log contains, in
// the order Gatling names them, behind the unknown sentinel.
const (
	KindUnknown   Kind = iota // never written by Gatling; the zero value
	KindRun                   // RUN: the run header, exactly one per log
	KindUser                  // USER: a virtual user starting or ending a scenario
	KindRequest               // REQUEST: one request attempt
	KindGroup                 // GROUP: one group closing
	KindError                 // ERROR: a crash message with its timestamp
	KindAssertion             // ASSERTION: one encoded assertion, written ahead of the header
)

// unknownName renders every enum's zero value.
const unknownName = "unknown"

var kindNames = [...]string{unknownName, "RUN", "USER", "REQUEST", "GROUP", "ERROR", "ASSERTION"}

// String returns the literal that opens a line of this kind.
func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}

	return "Kind(" + strconv.Itoa(int(k)) + ")"
}

// Status is a request's or group's recorded outcome, preserved as written and
// never inferred from the presence of a message. StatusUnknown is the zero
// value.
type Status uint8

// The two outcomes Gatling records, behind the unknown sentinel.
const (
	StatusUnknown Status = iota // never written by Gatling; the zero value
	StatusOK                    // OK
	StatusKO                    // KO
)

// String returns the literal Gatling writes for this status.
func (s Status) String() string {
	switch s {
	case StatusUnknown:
		return unknownName
	case StatusOK:
		return "OK"
	case StatusKO:
		return "KO"
	default:
		return "Status(" + strconv.Itoa(int(s)) + ")"
	}
}

// Event marks whether a user record opens or closes a scenario. EventUnknown
// is the zero value.
type Event uint8

// The two user events Gatling records, behind the unknown sentinel.
const (
	EventUnknown Event = iota // never written by Gatling; the zero value
	EventStart                // START
	EventEnd                  // END
)

// String returns the literal Gatling writes for this event.
func (e Event) String() string {
	switch e {
	case EventUnknown:
		return unknownName
	case EventStart:
		return "START"
	case EventEnd:
		return "END"
	default:
		return "Event(" + strconv.Itoa(int(e)) + ")"
	}
}

// AbsentTimestamp is the value a Record carries in Start, End or Timestamp for a
// time the log could not resolve: a negative offset in a binary log, a negative
// value in a text one, and the sentinel Gatling itself writes for a request that
// never completed, which is this very value. The canonical model reports such a
// time as the zero time.Time.
//
// A resolved time never collides with it. Both codecs refuse a run that starts
// before the epoch, so every timestamp a codec resolves is zero or above, and
// the negative half of the number line means absence and nothing else.
const AbsentTimestamp int64 = math.MinInt64

// MaxRunStart is the latest run start either codec accepts.
//
// Every event time is resolved against the run start, and the binary format
// stores each one as a 32-bit offset from it, so a start above this could not
// carry an offset without running past the end of the int64 range. A log naming
// a later start is refused rather than decoded into instants that would wrap —
// and both codecs bound it here, so a value both formats can express cannot be
// read by one and refused by the other.
const MaxRunStart int64 = math.MaxInt64 - math.MaxInt32

// Header is the run header every log carries exactly once. It is decoded from
// the RUN record and is what the version gate reads.
type Header struct {
	// SimulationClass is the simulation's fully qualified class name.
	SimulationClass string
	// RunID is the run's identifier, as written.
	RunID string
	// Start is the wall-clock start of the run in milliseconds since the Unix
	// epoch, exactly as recorded.
	Start int64
	// Description is the free-text run description. It is empty when the log
	// wrote a lone space, which is how Gatling records an absent description.
	Description string
	// Version is the Gatling release that wrote the log.
	Version Version
}

// Record is one decoded record of a simulation.log. Kind says which of the other
// fields are meaningful; the rest hold their zero value.
//
//	Kind          Meaningful fields
//	KindUser      Scenario, Event, Timestamp
//	KindRequest   Groups, Name, Start, End, Status, Message
//	KindGroup     Groups, Start, End, CumulatedResponseTime, Status
//	KindError     Message, Timestamp
//	KindAssertion Payload
//
// Line is meaningful for every kind above, but only for a text log; see its own
// documentation.
//
// Groups is backed by a slice the reader reuses between calls: it is valid until
// the next call to Next, and a caller keeping it must copy it. String fields are
// shared between records that carry the same value and are immutable, so they
// may be kept freely.
type Record struct {
	// Kind says which record this is.
	Kind Kind
	// Line is the 1-based line number the record was decoded from, for a text
	// simulation.log. A binary log has no lines and leaves it 0; the position of
	// a record in one is a byte offset, which only a failure needs and which
	// SyntaxError.Offset carries.
	Line int
	// Groups is the ordered path of enclosing group names, outermost first. It
	// is empty for a request outside any group.
	Groups []string
	// Name is the request name.
	Name string
	// Scenario is the scenario a user event belongs to.
	Scenario string
	// Event says whether a user event opens or closes the scenario.
	Event Event
	// Start is a request's or group's start timestamp in milliseconds since the
	// Unix epoch, or AbsentTimestamp for one the log could not resolve.
	Start int64
	// End is a request's or group's end timestamp in milliseconds since the Unix
	// epoch, or AbsentTimestamp: Gatling writes that value for a request that
	// never completed, and a codec reports it for an end it could not resolve.
	// Nothing may assume End >= Start.
	End int64
	// Timestamp is a user event's or error's time in milliseconds since the Unix
	// epoch, or AbsentTimestamp for one the log could not resolve.
	Timestamp int64
	// Status is a request's or group's outcome.
	Status Status
	// Message is a request's failure text or an error's message, exactly as
	// written. It is empty when the log wrote a lone space.
	Message string
	// CumulatedResponseTime is the sum of the response times of the requests
	// inside a group, in milliseconds, exactly as written. A negative value is
	// carried through, and the canonical model reports it unset.
	CumulatedResponseTime int64
	// Payload is an assertion's encoded blob, verbatim and uninterpreted.
	Payload string
}
