package binary

import (
	"io"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/internal/wire"
	"github.com/galax-io/parsec/model"
)

// RunReader reads a Gatling binary simulation.log as canonical results.
//
// It is the model-facing counterpart of [Reader]: the same log, the same version
// gate, the same bounded memory, but it yields [model.Item] values rather than
// the log's own wire records. Reach for this unless you need to see what the log
// contained, which is what Reader is for.
//
// It produces the same values the text codec produces for an equivalent run. A
// report written against the model cannot tell which format it was reading, and
// that is the point of the milestone rather than a coincidence: the conversion
// is one function in internal/wire that both codecs call.
type RunReader struct {
	rd  *Reader
	run model.Run
}

// NewRunReader reads the run record and gates on the version it names, failing
// for the same reasons [NewReader] does.
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
			// The binary log records no run identifier of its own, so this is
			// the simulation class: every run of one simulation carries the same
			// string, and Start is what tells two apart.
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

// Run is everything about the run that does not grow with its length: what the
// header named, what this source cannot record, and any version warning. It is
// complete before the first item and never changes.
func (x *RunReader) Run() model.Run { return x.run }

// Next returns the next item of the run, or [io.EOF] at the end.
//
// Any other error ends the read, and the items already delivered are not a
// result. The returned item's Groups slice is only valid until the next call —
// copy it to keep it, for the reason [Reader.Next] gives.
func (x *RunReader) Next() (model.Item, error) {
	for {
		rec, err := x.rd.Next()
		if err != nil {
			return model.Item{}, err
		}

		var it model.Item
		if wire.Item(&it, &rec) {
			return it, nil
		}
	}
}
