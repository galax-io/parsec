package simlog

import (
	"bytes"
	"fmt"
	"io"

	"github.com/galax-io/parsec/gatling"
	"github.com/galax-io/parsec/gatling/text"
	"github.com/galax-io/parsec/model"
)

// RecordReader yields a log's own wire records: the events the file contains,
// rather than the canonical result model. [text.Reader] satisfies it, and so
// will the binary codec.
//
// Reach for [RunReader] unless you need to see what the log actually held.
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
	// Any other error ends the read: there is no next record after it, the same
	// error is returned on every later call, and the records already delivered
	// are not a result — no total may be derived from them.
	//
	// The returned record's Groups slice is only valid until the next call.
	// Copy it to keep it; retaining it aliases a slice the codec refills in
	// place, so a caller collecting records ends up with every one of them
	// reporting the last record's group path.
	Next() (gatling.Record, error)
}

// RunReader yields canonical results: the same log as [RecordReader], decoded
// into the model every source shares. [text.RunReader] satisfies it.
type RunReader interface {
	// Run is everything about the run that does not grow with its length,
	// including what the source cannot record and any version warning.
	Run() model.Run
	// Next returns the next item of the run, or io.EOF at the end.
	//
	// Any other error ends the read, and the items already delivered are not a
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
	// Known, and not readable until the codec lands in v0.0.5. Listed rather
	// than omitted: a caller handed a binary log needs to be told what it is
	// holding.
	{format: gatling.FormatBinary},
}

// NewReader identifies the format of the log in r and returns a reader for its
// wire records. The records are identical to those the codec for that format
// yields when handed the same log directly: this package adds identification
// and forwarding, and nothing else.
//
// It reads Gatling 3.11.5 through 3.12.0, the range the text codec's golden
// corpus covers; [Supported] reports it without a decode. A binary log — every
// Gatling from 3.13.0 — is identified and refused with a
// *gatling.UnsupportedFormatError until that codec lands.
//
// It returns a *gatling.FormatError when r is not a Gatling simulation.log. The
// version gate belongs to the codec and is applied once: a version below the
// supported range is refused with a *gatling.VersionError, and one above it
// decodes with exactly one warning, or is refused with a
// *gatling.UnverifiedError under [gatling.WithStrict].
//
// On success the codec sees the stream from its first byte. On a refusal the
// bytes identification read are gone from r; they are handed back on the error
// — [gatling.FormatError.Head] and [gatling.UnsupportedFormatError.Head] — so a
// caller holding a stream it cannot rewind can still spool the log aside whole.
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

	// Compared with == rather than errors.Is, deliberately, and matching
	// io.ReadFull's own contract: it converts a source EOF with == too. An error
	// that merely wraps io.EOF is the source reporting a failure of its own — a
	// truncated decompressor, a closed transport — and reporting that as bytes
	// we did not recognise would send a user to inspect a file that is fine.
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF { //nolint:errorlint // deliberate: ReadFull's contract is identity, not wrapping
		return gatling.FormatUnknown, nil, nil, fmt.Errorf("gatling: reading the start of the stream: %w", err)
	}

	head := bytes.Clone(buf[:n])

	format, err := gatling.Detect(head)
	if err != nil {
		return gatling.FormatUnknown, head, nil, err
	}

	return format, head, io.MultiReader(bytes.NewReader(head), r), nil
}

// maxEmptyReads is how many times a stalled reader is given the benefit of the
// doubt. It is bufio's own figure, for the same reason.
const maxEmptyReads = 100

// readHead fills buf from r and follows io.ReadFull's contract: nil when buf is
// filled, io.EOF when nothing was read, io.ErrUnexpectedEOF when the stream
// ended part-way, and the source's own error otherwise.
//
// It exists because io.ReadFull has no guard against a reader that makes no
// progress. A Read returning (0, nil) is legal — the io.Reader contract says so
// explicitly, and says it must not be taken for EOF — and io.ReadFull loops on
// it forever, wedging the caller with no error and nothing to cancel. bufio has
// the same guard, but in fill(), which its Read does not use; wrapping in one
// would also over-read past the window and put bytes beyond reach of the error
// a refusal returns.
//
// It reads no more than len(buf) bytes, so a caller handed back the head and
// the untouched reader holds the whole stream between them.
func readHead(r io.Reader, buf []byte) (int, error) {
	n, empty := 0, 0

	for n < len(buf) {
		read, err := r.Read(buf[n:])
		n += read

		switch {
		case err != nil:
			if err == io.EOF && n > 0 {
				return n, io.ErrUnexpectedEOF
			}

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
