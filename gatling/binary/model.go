package binary

import (
	"io"
	"slices"

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

	oldest, newest := SupportedVersions()

	return &RunReader{
		rd:  rd,
		run: wire.NewRun(rd.Header(), Capabilities(), wire.Warnings(rd.Warnings(), oldest, newest), rd.Assertions()),
	}, nil
}

// Run is everything about the run that does not grow with its length: what the
// header named, what this source cannot record, and any version warning. It is
// complete before the first item and never changes.
//
// The slices are the caller's own — mutating them cannot disturb another caller
// or this reader, which is the same promise gatling/text makes.
func (x *RunReader) Run() model.Run {
	run := x.run
	run.Warnings = slices.Clone(x.run.Warnings)
	run.Assertions = slices.Clone(x.run.Assertions)

	return run
}

// Next returns the next item of the run, or [io.EOF] at the end.
//
// Any other error ends the read. A *gatling.TruncationError says the log was cut
// short and that the items already delivered are the ones the run recorded;
// anything else says the read failed and that they are not a result. The
// returned item's Groups slice is only valid until the next call — copy it to
// keep it, for the reason [Reader.Next] gives.
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
