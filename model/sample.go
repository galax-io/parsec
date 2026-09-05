package model

import "time"

// Outcome is whether one recorded operation succeeded or failed, as the source
// recorded it.
//
// It is read from what the source wrote and never inferred from whether some
// other field is set, so a failure cannot become a success by losing its
// message.
type Outcome uint8

const (
	// OutcomeUnknown is the zero value. No adapter produces it, so a sample
	// carrying it lost its outcome on the way rather than succeeding quietly.
	OutcomeUnknown Outcome = iota
	// OutcomeSuccess is an operation the source recorded as having succeeded.
	OutcomeSuccess
	// OutcomeFailure is an operation the source recorded as having failed.
	OutcomeFailure
)

// String returns "success", "failure" or "unknown".
func (o Outcome) String() string {
	switch o {
	case OutcomeSuccess:
		return "success"
	case OutcomeFailure:
		return "failure"
	case OutcomeUnknown:
	}

	return unknownName
}

// Failure is what the source recorded about an operation that did not succeed.
//
// Its presence is what marks a sample failed — [Sample.Failure] is set if and
// only if the outcome is [OutcomeFailure] — and presence is also what a
// requirement written as a fraction of failed operations tests.
type Failure struct {
	// Type is what the source called this failure. It is never invented here: a
	// source that records free text rather than a classification leaves it
	// empty and declares FieldSampleFailureType absent.
	Type string
	// Message is what the source recorded, character for character.
	Message string
}

// Sample is one recorded operation — a request, in every tool surveyed so far.
type Sample struct {
	// Groups is the enclosing groups, outermost first, and empty for an
	// operation taken outside any group. It is the OpenNFR attribute
	// loadtest.group.name.
	//
	// It is backed by a slice a reader reuses between calls: it is valid until
	// the next call to Next, and a caller keeping it must copy it.
	Groups []string
	// Name is what the source called this operation. It is the OpenNFR
	// attribute loadtest.request.name.
	Name string
	// Start is when the operation began, in UTC, exactly as the source recorded
	// it: not rounded, and not re-based against the run's start.
	Start time.Time
	// Duration is how long the operation took, and is the OpenNFR metric
	// loadtest.request.duration. It is unset — never negative — when the source
	// recorded no usable end.
	Duration Opt[time.Duration]
	// Outcome is success or failure as the source recorded it.
	Outcome Outcome
	// Failure is set if and only if Outcome is OutcomeFailure.
	Failure Opt[Failure]
	// Scenario is the scenario this operation ran under, where the source
	// records one on the operation itself.
	Scenario Opt[string]
	// ResponseCode is the protocol's own result code. A string, because not
	// every protocol's code is a number.
	ResponseCode Opt[string]
	// BytesSent is the request body size.
	BytesSent Opt[int64]
	// BytesReceived is the response body size.
	BytesReceived Opt[int64]
}

// GroupSample is one traversal of a group, closing.
type GroupSample struct {
	// Groups is the group's own path, outermost first, its own name last. It is
	// backed by a slice a reader reuses between calls: valid until the next
	// call to Next, and a caller keeping it must copy it.
	Groups []string
	// Start is when the traversal began, in UTC, exactly as recorded.
	Start time.Time
	// Duration is wall clock across the traversal, including any pause inside
	// it.
	//
	// CumulatedDuration is a different quantity: the sum of the durations of
	// the operations the traversal enclosed, and the OpenNFR metric
	// loadtest.group.duration. Neither is derived from the other, and a run
	// that pauses inside a group is not charged for the pause in the second.
	Duration Opt[time.Duration]
	// CumulatedDuration is the sum of the durations of the operations the
	// traversal enclosed, and the OpenNFR metric loadtest.group.duration. It is
	// not derived from Duration and does not include a pause.
	CumulatedDuration Opt[time.Duration]
	// Outcome is the group's own, not the conjunction of what it enclosed: a
	// group can fail while every operation inside it succeeded.
	Outcome Outcome
}

// UserEventKind is which end of a virtual user's life an event marks.
type UserEventKind uint8

const (
	// UserEventUnknown is the zero value and is never produced by an adapter.
	UserEventUnknown UserEventKind = iota
	// UserStart is a virtual user beginning its scenario.
	UserStart
	// UserEnd is a virtual user finishing its scenario.
	UserEnd
)

// String returns "start", "end" or "unknown".
func (k UserEventKind) String() string {
	switch k {
	case UserStart:
		return "start"
	case UserEnd:
		return "end"
	case UserEventUnknown:
	}

	return unknownName
}

// UserEvent is one virtual user starting or ending a scenario.
//
// These bound the span of a run that every derived rate divides by, so they are
// load-bearing rather than decoration: a user event can set either end of that
// span, and mishandling one shifts every rate computed from the run.
type UserEvent struct {
	// Scenario is what the source called the scenario this user ran.
	Scenario string
	// Kind is whether the user is starting or ending it.
	Kind UserEventKind
	// At is when it happened, in UTC, exactly as recorded.
	At time.Time
}

// RunError is a failure the source recorded that belongs to no sample — an
// operation that never reached the wire, say, and so produced no record of its
// own.
//
// It arrives as a stream item rather than on [Run] because a run may hold any
// number of them.
type RunError struct {
	// Message is what the source recorded, character for character.
	Message string
	// At is when it happened, in UTC, exactly as recorded.
	At time.Time
}
