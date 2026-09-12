package simlog

import (
	"bytes"
	"errors"
	"io"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/binary"
	"github.com/galax-io/parsec/gatling/text"
	"github.com/galax-io/parsec/internal/source"
	"github.com/galax-io/parsec/model"
)

// RecordReader yields a log's own wire records: the events the file contains,
// rather than the canonical result model. [text.Reader] and [binary.Reader] both
// satisfy it without an adapter: the two codecs share these method sets.
//
// Reach for [RunReader] unless you need to see what the log actually held.
//
// The method set is frozen at v0.1.0, on both sides. A consumer's test double
// implements this interface, so adding a method breaks the implementer and not
// only the caller — which makes an addition a breaking change with no
// deprecation path, and is why the set is final rather than merely stable.
type RecordReader interface {
	// Header is the run header, available before the first record.
	Header() gatling.Header
	// Assertions is the opaque payloads written ahead of the header.
	Assertions() []string
	// Warnings is what the version gate raised. A codec raises one for a
	// version no recording covers; how many, and for what, is the codec's to
	// say.
	Warnings() []gatling.Warning
	// Next returns the next record, or io.EOF at the end of the log.
	//
	// Any other error ends the read: there is no next record after it, and the
	// same error is returned on every later call. A *gatling.TruncationError
	// says the log was cut short — the bytes ran out inside a record, which is
	// how a killed run ends — and the records already delivered are what it
	// recorded. Anything else says the read failed, and no total may be derived
	// from what was delivered.
	//
	// The returned record's Groups slice is only valid until the next call.
	// Copy it to keep it; retaining it aliases a slice the codec refills in
	// place, so a caller collecting records ends up with every one of them
	// reporting the last record's group path.
	Next() (gatling.Record, error)
}

// RunReader yields canonical results: the same log as [RecordReader], decoded
// into the model every source shares. [text.RunReader] and [binary.RunReader]
// both satisfy it.
//
// Its method set is frozen at v0.1.0 on both sides, for the reason
// [RecordReader] gives.
type RunReader interface {
	// Run is everything about the run that does not grow with its length,
	// including what the source cannot record and any version warning.
	Run() model.Run
	// Next returns the next item of the run, or io.EOF at the end.
	//
	// Any other error ends the read. A *gatling.TruncationError says the log was
	// cut short and that the items already delivered are the ones the run
	// recorded; anything else says the read failed and that they are not a
	// result. The returned item's Groups slice is only valid until the next
	// call — copy it to keep it, for the reason [RecordReader.Next] gives.
	Next() (model.Item, error)
}

// codec is what this module can do with one log format: how to open it, and the
// range it accepts without a warning. A format with no constructors is one this
// module knows about and cannot read yet.
//
// Dispatch and [Supported] both read this table, so "which formats we read" is
// stated once. Stating it twice is how a module ends up reading a format while
// telling every consumer it cannot.
type codec struct {
	format   gatling.Format
	versions func() (oldest, newest gatling.Version)
	records  func(io.Reader, ...gatling.Option) (RecordReader, error)
	run      func(io.Reader, ...gatling.Option) (RunReader, error)
}

// codecs is in the order a consumer should see them: oldest format first.
var codecs = [...]codec{
	{
		format:   gatling.FormatText,
		versions: text.SupportedVersions,
		// Assigned and returned rather than returned directly, in both
		// adapters: a nil *text.Reader inside a non-nil interface would defeat
		// every nil check a caller writes, and putting the guard here keeps it
		// in one place rather than once per constructor.
		records: func(r io.Reader, opts ...gatling.Option) (RecordReader, error) {
			rd, err := text.NewReader(r, opts...)
			if err != nil {
				return nil, err
			}

			return rd, nil
		},
		run: func(r io.Reader, opts ...gatling.Option) (RunReader, error) {
			rd, err := text.NewRunReader(r, opts...)
			if err != nil {
				return nil, err
			}

			return rd, nil
		},
	},
	{
		format:   gatling.FormatBinary,
		versions: binary.SupportedVersions,
		records: func(r io.Reader, opts ...gatling.Option) (RecordReader, error) {
			rd, err := binary.NewReader(r, opts...)
			if err != nil {
				return nil, err
			}

			return rd, nil
		},
		run: func(r io.Reader, opts ...gatling.Option) (RunReader, error) {
			rd, err := binary.NewRunReader(r, opts...)
			if err != nil {
				return nil, err
			}

			return rd, nil
		},
	},
}

// NewReader identifies the format of the log in r and returns a reader for its
// wire records. The records are identical to those the codec for that format
// yields when handed the same log directly: this package adds identification
// and forwarding, and nothing else.
//
// It reads both formats Gatling has written: the tab-separated text log through
// 3.12.0, and the binary one from 3.13.1. [Supported] reports each range without
// a decode, read from the same table this dispatches on, so a format cannot be
// readable in one and not the other.
//
// The source may still be being written: see the package documentation for what
// a follower may rely on and what it must do in return.
//
// A stream too short to identify — which is what a sidecar attaching in a run's
// first milliseconds sees — is refused with a *gatling.FormatError whose Short
// field is set, and not with the *gatling.TruncationError the codec
// constructors return for those same bytes. What is missing here is the format,
// and until that is known there is no codec whose grammar a cut could be
// described against. The two mean the same thing operationally — come back with
// more bytes — so a follower that classifies endings must look for both.
//
// It returns a *gatling.FormatError when r is not a Gatling simulation.log. The
// version gate belongs to the codec and is applied once: a version below the
// supported range is refused with a *gatling.VersionError, and one above it
// decodes with exactly one warning, or is refused with a
// *gatling.UnverifiedError under [gatling.WithStrict].
//
// On success the codec sees the stream from its first byte. When identification
// itself refuses — the bytes are not a Gatling simulation.log — the bytes it read
// are handed back on [gatling.FormatError.Head], so a caller holding a stream it
// cannot rewind can still spool the log aside whole.
//
// A refusal from the codec *after* identification does not carry them. A version
// outside the supported range is refused with a *gatling.VersionError, which
// names the version and the range and has nowhere to put bytes, and by then the
// codec has consumed the head. A caller that must keep such a stream should tee
// it rather than rely on the error.
func NewReader(r io.Reader, opts ...gatling.Option) (RecordReader, error) {
	c, stream, head, err := dispatch(r)
	if err != nil {
		return nil, err
	}

	if c.records == nil {
		return nil, &gatling.UnsupportedFormatError{Format: c.format, Head: head}
	}

	return c.records(stream, opts...)
}

// NewRunReader is [NewReader] for canonical results: the same identification,
// the same gate, the same accepted versions, [model.Item] values instead of
// wire records.
func NewRunReader(r io.Reader, opts ...gatling.Option) (RunReader, error) {
	c, stream, head, err := dispatch(r)
	if err != nil {
		return nil, err
	}

	if c.run == nil {
		return nil, &gatling.UnsupportedFormatError{Format: c.format, Head: head}
	}

	return c.run(stream, opts...)
}

// dispatch identifies the stream and finds the codec for it. It returns the
// stream repositioned at byte 0 and the leading bytes it examined.
func dispatch(r io.Reader) (codec, io.Reader, []byte, error) {
	format, head, stream, err := identify(r)
	if err != nil {
		return codec{}, nil, nil, err
	}

	for _, c := range codecs {
		if c.format == format {
			return c, stream, head, nil
		}
	}

	// Unreachable: Detect returns only the formats this table lists, and the
	// test that walks Supported against Detect keeps it that way.
	return codec{format: format}, stream, head, nil
}

// identify names the format from the leading bytes and hands back a stream that
// still begins at byte 0.
//
// Replaying rather than consuming is a correctness requirement, not a
// convenience: the binary codec rebuilds its string cache from the first byte
// of the file and would be quietly wrong if one were missing.
func identify(r io.Reader) (gatling.Format, []byte, io.Reader, error) {
	var buf [gatling.DetectSize]byte

	n, err := readHead(r, buf[:])

	// Two endings alone mean "the head is what we have", and both are the
	// stream's own: io.EOF straight from the source with nothing read, and
	// errShortHead, which readHead raises when the source's io.EOF came after
	// some bytes. Compared with == rather than errors.Is, deliberately. An
	// error that merely wraps io.EOF is the source reporting a failure of its
	// own — a truncated decompressor, a closed transport — and so is
	// io.ErrUnexpectedEOF, which gzip, flate and zlib return by identity when
	// their compressed input was cut. Reporting either as bytes we did not
	// recognise would send a user to inspect a file that is fine; reporting it
	// as a head that has not arrived yet would have a follower retry forever.
	if err != nil && err != io.EOF && err != errShortHead { //nolint:errorlint // deliberate: identity, not wrapping; see above
		return gatling.FormatUnknown, nil, nil, sourceFailed(err)
	}

	head := bytes.Clone(buf[:n])

	format, err := gatling.Detect(head)
	if err != nil {
		return gatling.FormatUnknown, head, nil, err
	}

	return format, head, io.MultiReader(bytes.NewReader(head), r), nil
}

// sourceFailed reports a stream that broke before it could be identified. The
// cause is kept: a caller told its bytes were not a Gatling log would go and
// inspect a file that is fine, when the pipe was the problem.
//
// A cause whose chain holds io.EOF must not satisfy errors.Is(err, io.EOF): a
// caller whose loop breaks on the clean end of a log — an ingest handler does —
// would book a torn upload as an empty run, with nothing decoded yet to
// contradict it. source.Failed hides io.EOF and keeps every other cause
// reachable, as gatling/binary's sourceFailed and gatling/text's readError do,
// and the package documentation promises it to a follower.
func sourceFailed(err error) error {
	return source.Failed("gatling: reading the start of the stream", err)
}

// maxEmptyReads is how many times a stalled reader is given the benefit of the
// doubt. It is bufio's own figure, for the same reason.
const maxEmptyReads = 100

// errShortHead is what readHead returns when the stream ended inside the
// detection window: some bytes arrived, then the source's own io.EOF. It exists
// so that this package's conversion of a short head can be told from
// io.ErrUnexpectedEOF, which compress/gzip, flate and zlib all return by
// identity when their *compressed* input was cut. That is a failure of the
// source, not a stream that has yet to arrive, and answering "come back with
// more bytes" to it would leave a follower retrying an archive that never gets
// longer. gatling/binary's errCutShort is the same device for the same reason.
var errShortHead = errors.New("the stream ended inside the detection window")

// readHead fills buf from r, and differs from io.ReadFull in three ways. A head
// the stream ends inside is reported as errShortHead rather than
// io.ErrUnexpectedEOF, so only this loop can claim the head ran short. A read
// that fills buf and ends the stream succeeds, but one that arrives with any
// other error returns it, where io.ReadFull would drop it: the head is read
// straight from the source, so a failure dropped here is not seen again.
// gatling/binary's readFull holds the same rule. And a source that keeps
// returning (0, nil) ends the read with io.ErrNoProgress
// rather than spinning: that return is legal — the io.Reader contract says so,
// and says it must not be taken for EOF — and io.ReadFull loops on it forever,
// wedging the caller with no error and nothing to cancel. bufio has the same
// guard, but in fill(), which its Read does not use; wrapping in one would also
// over-read past the window and put bytes beyond reach of the error a refusal
// returns.
//
// Otherwise it returns io.EOF when nothing was read and the source's own error,
// untouched, when the source failed. It reads no more than len(buf) bytes, so
// a caller handed back the head and the untouched reader holds the whole stream
// between them.
func readHead(r io.Reader, buf []byte) (int, error) {
	n, empty := 0, 0

	for n < len(buf) {
		read, err := r.Read(buf[n:])
		n += read

		// The count says whether the head is complete, and the stream ending
		// beside its last bytes does not make it less so. Any other error is
		// the source's failure, and is returned below: the head is read
		// straight from the source, so a failure dropped here is gone.
		if n == len(buf) && err == io.EOF {
			return n, nil
		}

		switch {
		// Identity: only the stream itself ending, never a source wrapping io.EOF.
		case err == io.EOF:
			if n > 0 {
				return n, errShortHead
			}

			return n, io.EOF

		case err != nil:
			return n, err

		case read > 0:
			empty = 0

		default:
			if empty++; empty >= maxEmptyReads {
				return n, io.ErrNoProgress
			}
		}
	}

	return n, nil
}
