package text

import (
	"io"
	"math"
	"slices"
	"time"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/model"
)

// RunReader reads a Gatling text simulation.log as canonical results.
//
// It is the model-facing counterpart of [Reader]: the same log, the same
// version gate, the same bounded memory, but it yields [model.Item] values
// rather than the log's own wire records. Reach for this unless you need to see
// what the log contained, which is what Reader is for.
//
// The whole run is never resident. [RunReader.Run] is complete as soon as
// NewRunReader returns, and items arrive one at a time in file order.
type RunReader struct {
	rd  *Reader
	run model.Run
}

// NewRunReader reads the preamble and the run header from r, applies the
// version gate, and returns a reader positioned before the first event.
//
// It accepts Gatling 3.11.5 through 3.12.0, the range the golden corpus covers.
// A log written by an older version is refused with a *gatling.VersionError
// naming the version found and the range supported; so is a version string that
// is not a plain MAJOR.MINOR.PATCH release, quoting what was found. A plain
// release above the range decodes, and the warning is on the returned run;
// under [gatling.WithStrict] it is refused with a *gatling.UnverifiedError
// instead.
//
// The gate is not re-implemented here: a log the decoder refuses never reaches
// the conversion, and the error is the decoder's own.
func NewRunReader(r io.Reader, opts ...gatling.Option) (*RunReader, error) {
	rd, err := NewReader(r, opts...)
	if err != nil {
		return nil, err
	}

	h := rd.Header()

	// Nil when there is nothing to say, matching Assertions: a caller reading
	// `len(run.Warnings) == 0` and one reading `run.Warnings == nil` must agree.
	var carried []model.Warning

	oldest, newest := SupportedVersions()

	for _, w := range rd.Warnings() {
		// The reason names neither the version nor this package: Warning.Version
		// already carries the first, and the second belongs to no tool-agnostic
		// type. A report printing Warning.String() gets the version once.
		carried = append(carried, model.Warning{
			Version: w.Version.String(),
			Reason: "no recording covers it — the verified range is " +
				oldest.String() + " through " + newest.String() + ", so the records decode unverified",
		})
	}

	return &RunReader{
		rd: rd,
		run: model.Run{
			// Gatling's run identifier is the simulation's, so every run of one
			// simulation carries the same string; Start is what tells two apart.
			ID:           h.RunID,
			Name:         h.SimulationClass,
			Description:  h.Description,
			Start:        millis(h.Start),
			Tool:         Tool,
			ToolVersion:  h.Version.String(),
			Capabilities: Capabilities(),
			Warnings:     carried,
			Assertions:   rd.Assertions(),
		},
	}, nil
}

// Tool is what this source is called in [model.Run.Tool].
const Tool = "gatling"

// Run returns everything about this run that does not grow with its length: its
// identity, the tool and version, what the source can and cannot record, any
// version warning, and the opaque assertion payloads.
//
// It is complete as soon as NewRunReader returns and does not change as items
// are read. The slices are the caller's own — mutating them cannot disturb
// another caller or this reader — which is the rule [Reader.Assertions] and
// [Reader.Warnings] already keep.
func (x *RunReader) Run() model.Run {
	run := x.run
	run.Warnings = slices.Clone(x.run.Warnings)
	run.Assertions = slices.Clone(x.run.Assertions)

	return run
}

// Next returns the next item of the run, or io.EOF at the end.
//
// A log that cannot be read in full yields a *gatling.SyntaxError naming the
// line and what was expected there, and no item after it: a partial read cannot
// produce counts that match the tool's own report, so it is refused rather than
// reported. Every later call returns that same error.
//
// The returned item's Groups slice is valid until the next call; copy it to
// keep it.
func (x *RunReader) Next() (model.Item, error) {
	var it model.Item

	for {
		rec, err := x.rd.Next()
		if err != nil {
			return model.Item{}, err
		}

		if convert(&it, &rec) {
			return it, nil
		}
		// Unreachable in practice: every kind the decoder can yield converts.
		// The loop stays so that a kind added to the wire records without a
		// mapping here is skipped rather than returned as a zero Item.
	}
}

// convert fills it from one wire record. The result is false for a record that
// is not an event of the run.
//
// Both sides are pointers because Item is large — it carries a slot for every
// kind — and Next is called once per record of a multi-gigabyte log, so passing
// either by value copies hundreds of bytes per item to no end. The shape is a
// consumer-facing choice; the copies are not, and this is where they are paid.
func convert(it *model.Item, rec *gatling.Record) bool {
	*it = model.Item{}

	switch rec.Kind {
	case gatling.KindRequest:
		it.Kind = model.ItemSample
		it.Sample = model.Sample{
			Groups:   rec.Groups,
			Name:     rec.Name,
			Start:    millis(rec.Start),
			Duration: span(rec.Start, rec.End),
			Outcome:  outcome(rec.Status),
			Failure:  failure(rec.Status, rec.Message),
		}

		return true

	case gatling.KindGroup:
		it.Kind = model.ItemGroup
		it.Group = model.GroupSample{
			Groups: rec.Groups,
			Start:  millis(rec.Start),
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
			At:       millis(rec.Timestamp),
		}

		return true

	case gatling.KindError:
		it.Kind = model.ItemError
		it.Error = model.RunError{Message: rec.Message, At: millis(rec.Timestamp)}

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

// millis reads a Gatling timestamp — milliseconds since the Unix epoch — as an
// instant in UTC. The conversion is exact; UTC only fixes how it prints, so the
// same run reads the same way on every machine.
func millis(ms int64) time.Time { return time.UnixMilli(ms).UTC() }

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
