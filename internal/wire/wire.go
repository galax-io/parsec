// Package wire converts a Gatling wire record into the canonical model.
//
// It is one function of one input, shared by the codecs rather than written
// twice. The mapping is a property of the records, not of the format they were
// read from: two copies could disagree about what a record means while both
// looked correct, and a report written against the model would then depend on
// which log the numbers came from.
//
// It is internal because it has never belonged to the public API. A consumer
// asks a RunReader for model values; how a record becomes one is this module's
// business.
package wire

import (
	"math"
	"time"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/model"
)

// Item fills it from one wire record. The result is false for a record that is
// not an event of the run.
//
// Both sides are pointers because Item is large — it carries a slot for every
// kind — and Next is called once per record of a multi-gigabyte log, so passing
// either by value copies hundreds of bytes per item to no end. The shape is a
// consumer-facing choice; the copies are not, and this is where they are paid.
func Item(it *model.Item, rec *gatling.Record) bool {
	*it = model.Item{}

	switch rec.Kind {
	case gatling.KindRequest:
		it.Kind = model.ItemSample
		it.Sample = model.Sample{
			Groups:   rec.Groups,
			Name:     rec.Name,
			Start:    Millis(rec.Start),
			Duration: span(rec.Start, rec.End),
			Outcome:  outcome(rec.Status),
			Failure:  failure(rec.Status, rec.Message),
		}

		return true

	case gatling.KindGroup:
		it.Kind = model.ItemGroup
		it.Group = model.GroupSample{
			Groups: rec.Groups,
			Start:  Millis(rec.Start),
			// Two different quantities, and the record carries both: Duration
			// is wall clock across the traversal, pauses included, and
			// CumulatedDuration is the sum of the durations of the requests
			// inside it. Neither is derived from the other.
			Duration:          span(rec.Start, rec.End),
			CumulatedDuration: millisDuration(rec.CumulatedResponseTime),
			Outcome:           outcome(rec.Status),
		}

		return true

	case gatling.KindUser:
		it.Kind = model.ItemUser
		it.User = model.UserEvent{
			Scenario: rec.Scenario,
			Kind:     userEvent(rec.Event),
			At:       Millis(rec.Timestamp),
		}

		return true

	case gatling.KindError:
		it.Kind = model.ItemError
		it.Error = model.RunError{Message: rec.Message, At: Millis(rec.Timestamp)}

		return true

	// An assertion written among the events rather than ahead of them. Run
	// carries the ones the preamble held and is fixed by the time any item is
	// read, so this one is yielded instead of being added to it — dropping it
	// would lose a payload the wire path preserves.
	case gatling.KindAssertion:
		it.Kind = model.ItemAssertion
		it.Assertion = rec.Payload

		return true

	// Neither can reach here: the decoder refuses a second run header, and it
	// never produces an unknown kind. Listed so that a kind added to the wire
	// records later fails to compile rather than being dropped in silence.
	case gatling.KindRun, gatling.KindUnknown:
		return false
	}

	return false
}

// Millis reads a Gatling timestamp — milliseconds since the Unix epoch — as an
// instant in UTC. The conversion is exact; UTC only fixes how it prints, so the
// same run reads the same way on every machine.
//
// Exported because a codec needs it for the run header as well as for the
// records: both formats store the run's start the same way, and a second copy
// of one line is still a second place for the two to disagree.
func Millis(ms int64) time.Time { return time.UnixMilli(ms).UTC() }

// maxMillis is the largest millisecond count that survives conversion to a
// time.Duration, which counts nanoseconds in an int64. Anything above it wraps,
// and wraps to a small plausible-looking negative rather than to obvious
// garbage: (1<<63 - 1) ms comes out as -1ms.
const maxMillis = int64(math.MaxInt64) / int64(time.Millisecond)

// millisDuration converts a recorded millisecond count into a duration, and is
// unset when the count cannot be one.
//
// The log is untrusted input — parseTimestamp accepts any non-negative int64 —
// so the bound is checked rather than assumed. A value past it is reported
// absent, never wrapped: a consumer dividing by a negative duration is the
// failure this exists to prevent.
func millisDuration(ms int64) model.Opt[time.Duration] {
	if ms < 0 || ms > maxMillis {
		return model.Opt[time.Duration]{}
	}

	return model.Some(time.Duration(ms) * time.Millisecond)
}

// span is the duration between two recorded timestamps, and is unset when there
// is none to be had.
//
// Gatling's own reader branches on an end equal to the minimum signed 64-bit
// integer, treating it as an event that never completed. Whether a 3.11.5 or
// 3.12.0 run can produce one is unconfirmed, so nothing here assumes the end is
// at or after the start: an end before the start yields no duration, and so
// does a span too large to be one.
func span(start, end int64) model.Opt[time.Duration] {
	if end < start {
		return model.Opt[time.Duration]{}
	}

	return millisDuration(end - start)
}

func outcome(s gatling.Status) model.Outcome {
	switch s {
	case gatling.StatusOK:
		return model.OutcomeSuccess
	case gatling.StatusKO:
		return model.OutcomeFailure
	case gatling.StatusUnknown:
	}

	return model.OutcomeUnknown
}

// failure is set if and only if the record failed. Type stays empty: Gatling
// writes free text rather than a classification, and this package does not
// invent a taxonomy — Capabilities declares FieldSampleFailureType absent.
func failure(s gatling.Status, message string) model.Opt[model.Failure] {
	if s != gatling.StatusKO {
		return model.Opt[model.Failure]{}
	}

	return model.Some(model.Failure{Message: message})
}

func userEvent(e gatling.Event) model.UserEventKind {
	switch e {
	case gatling.EventStart:
		return model.UserStart
	case gatling.EventEnd:
		return model.UserEnd
	case gatling.EventUnknown:
	}

	return model.UserEventUnknown
}
