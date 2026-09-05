package text

import (
	"io"
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
// release above the range decodes, and the warning is on the returned run.
//
// The gate is not re-implemented here: a log the decoder refuses never reaches
// the conversion, and the error is the decoder's own.
func NewRunReader(r io.Reader) (*RunReader, error) {
	rd, err := NewReader(r)
	if err != nil {
		return nil, err
	}

	h := rd.Header()

	warnings := rd.Warnings()
	carried := make([]model.Warning, 0, len(warnings))

	for _, w := range warnings {
		carried = append(carried, model.Warning{Version: w.Version.String(), Reason: w.String()})
	}

	return &RunReader{
		rd: rd,
		run: model.Run{
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
// are read.
func (x *RunReader) Run() model.Run { return x.run }

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
	for {
		rec, err := x.rd.Next()
		if err != nil {
			return model.Item{}, err
		}

		it, ok := convert(rec)
		if ok {
			return it, nil
		}
		// A kind that is not an event of the run — the header and the assertion
		// payloads both live on Run, and neither is an item. The decoder yields
		// them before any event, so this loop advances rather than ends.
	}
}

// convert maps one wire record onto one item. The second result is false for a
// record that is not an event of the run.
func convert(rec gatling.Record) (model.Item, bool) {
	switch rec.Kind {
	case gatling.KindRequest:
		return model.Item{
			Kind: model.ItemSample,
			Sample: model.Sample{
				Groups:   rec.Groups,
				Name:     rec.Name,
				Start:    millis(rec.Start),
				Duration: span(rec.Start, rec.End),
				Outcome:  outcome(rec.Status),
				Failure:  failure(rec.Status, rec.Message),
			},
		}, true

	case gatling.KindGroup:
		return model.Item{
			Kind: model.ItemGroup,
			Group: model.GroupSample{
				Groups: rec.Groups,
				Start:  millis(rec.Start),
				// Two different quantities, and the record carries both:
				// Duration is wall clock across the traversal, pauses
				// included, and CumulatedDuration is the sum of the durations
				// of the requests inside it. Neither is derived from the other.
				Duration:          span(rec.Start, rec.End),
				CumulatedDuration: model.Some(time.Duration(rec.CumulatedResponseTime) * time.Millisecond),
				Outcome:           outcome(rec.Status),
			},
		}, true

	case gatling.KindUser:
		return model.Item{
			Kind: model.ItemUser,
			User: model.UserEvent{
				Scenario: rec.Scenario,
				Kind:     userEvent(rec.Event),
				At:       millis(rec.Timestamp),
			},
		}, true

	case gatling.KindError:
		return model.Item{
			Kind:  model.ItemError,
			Error: model.RunError{Message: rec.Message, At: millis(rec.Timestamp)},
		}, true

	case gatling.KindRun, gatling.KindAssertion, gatling.KindUnknown:
		return model.Item{}, false

	default:
		return model.Item{}, false
	}
}

// millis reads a Gatling timestamp — milliseconds since the Unix epoch — as an
// instant in UTC. The conversion is exact; UTC only fixes how it prints, so the
// same run reads the same way on every machine.
func millis(ms int64) time.Time { return time.UnixMilli(ms).UTC() }

// span is the duration between two recorded timestamps, and is unset when there
// is none to be had.
//
// Gatling's own reader branches on an end equal to the minimum signed 64-bit
// integer, treating it as an event that never completed. Whether a 3.11.5 or
// 3.12.0 run can produce one is unconfirmed, so nothing here assumes the end is
// at or after the start: any end before the start yields no duration rather
// than a negative or enormous one, which a consumer could divide by.
func span(start, end int64) model.Opt[time.Duration] {
	if end < start {
		return model.Opt[time.Duration]{}
	}

	return model.Some(time.Duration(end-start) * time.Millisecond)
}

func outcome(s gatling.Status) model.Outcome {
	switch s {
	case gatling.StatusOK:
		return model.OutcomeSuccess
	case gatling.StatusKO:
		return model.OutcomeFailure
	case gatling.StatusUnknown:
		return model.OutcomeUnknown
	default:
		return model.OutcomeUnknown
	}
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
		return model.UserEventUnknown
	default:
		return model.UserEventUnknown
	}
}
