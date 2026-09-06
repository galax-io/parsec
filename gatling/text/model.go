package text

import (
	"io"
	"slices"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/internal/wire"
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
			Start:        wire.Millis(h.Start),
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

		if wire.Item(&it, &rec) {
			return it, nil
		}
		// Unreachable in practice: every kind the decoder can yield converts.
		// The loop stays so that a kind added to the wire records without a
		// mapping here is skipped rather than returned as a zero Item.
	}
}
