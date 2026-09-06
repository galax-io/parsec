package model

import (
	"encoding/binary"
	"strings"
)

// PositionKind says what a Position addresses.
type PositionKind uint8

const (
	// PositionUnknown is the zero value. The zero Position addresses nothing,
	// and no constructor produces it.
	PositionUnknown PositionKind = iota
	// PositionSample addresses a sample: a group path and a name.
	PositionSample
	// PositionGroup addresses a group traversal: a group path alone, whose
	// last element is the group's own name.
	PositionGroup
)

// String returns "sample", "group" or "unknown".
func (k PositionKind) String() string {
	switch k {
	case PositionSample:
		return "sample"
	case PositionGroup:
		return "group"
	case PositionUnknown:
	}

	return unknownName
}

// Position is where in a run something was recorded: the ordered path of
// enclosing groups and, for a sample, its name. It is the definition every
// consumer buckets by, so that two consumers agree on which rows a run has
// without agreeing on a spelling.
//
// It is one comparable value: use it directly as a map key. Two positions are
// equal exactly when they address the same kind of thing at the same path with
// the same name — a group traversal never equals a sample, even where the
// group's path reads as the sample's path plus its name — and distinct paths
// never collide, whatever characters the names contain.
//
// A Position taken from an item stays valid after the reader advances. The
// Groups slice on a [Sample] or [GroupSample] is backed by storage the reader
// reuses; a Position is not, so it may be kept for the whole run without
// copying anything.
//
// The zero Position addresses nothing and equals no position a constructor
// returns.
type Position struct {
	// key encodes the kind, every group name and the sample name, each
	// length-prefixed. It is the identity and it is internal: nothing exported
	// reads it, so the encoding may change.
	key string
}

// NewSamplePosition returns the position of a sample recorded under the given
// groups, outermost first, with the given name. groups may be empty or nil for
// a sample outside any group; the slice is not retained.
func NewSamplePosition(groups []string, name string) Position {
	return Position{key: encode(PositionSample, groups, name)}
}

// NewGroupPosition returns the position of a group traversal at the given path,
// outermost first, the group's own name last. The slice is not retained.
func NewGroupPosition(groups []string) Position {
	return Position{key: encode(PositionGroup, groups, "")}
}

// Position returns where the sample was recorded: its groups and its name, as
// one value a consumer buckets by. Unlike Groups, the result stays valid after
// the next call to Next.
func (s Sample) Position() Position { return NewSamplePosition(s.Groups, s.Name) }

// Position returns where the traversal was recorded: its path, the group's own
// name last, as one value a consumer buckets by. Unlike Groups, the result stays
// valid after the next call to Next.
func (g GroupSample) Position() Position { return NewGroupPosition(g.Groups) }

// Kind reports whether the position addresses a sample or a group traversal.
func (p Position) Kind() PositionKind {
	if p.key == "" {
		return PositionUnknown
	}

	// Only the two constructors write a key, and each opens it with its kind.
	return PositionKind(p.key[0])
}

// Groups returns the ordered path of enclosing groups the position was made
// from — for a group traversal, ending with the group's own name. The slice is
// the caller's own: empty and non-nil for a position with no groups, nil only
// for the zero Position.
func (p Position) Groups() []string {
	if p.key == "" {
		return nil
	}

	segments := p.segments()
	if p.Kind() == PositionSample {
		// The name is the last segment and is not part of the path.
		segments = segments[:len(segments)-1]
	}

	return segments
}

// Name returns the sample's name. It is empty for a group traversal and for the
// zero Position; a sample may also legitimately have an empty name, and Kind
// tells the two apart.
func (p Position) Name() string {
	if p.Kind() != PositionSample {
		return ""
	}

	segments := p.segments()

	return segments[len(segments)-1]
}

// String renders the position for display, as the groups and the name joined
// by " / ", with a group traversal marked as one.
//
// The mark is not decoration. A group traversal at a / b and a sample named b
// inside group a are different positions, and without it they would render
// alike, so a report would show two distinct rows under one label — collapsing
// in display the distinction the type guarantees in equality.
//
// It is still not the identity: a name containing the separator renders
// ambiguously and compares correctly regardless. The zero Position renders
// empty.
func (p Position) String() string {
	if p.key == "" {
		return ""
	}

	rendered := strings.Join(p.segments(), " / ")

	if p.Kind() == PositionGroup {
		return rendered + " [group]"
	}

	return rendered
}

// encode builds the key: the kind byte, then every group name and — for a
// sample — the name, each as its length in unsigned varint form followed by its
// bytes. Length prefixes make the encoding prefix-free, so two different paths
// never share a key whatever their names contain, and it decodes back exactly.
// One allocation: the builder is grown once to an upper bound of the size.
func encode(kind PositionKind, groups []string, name string) string {
	size := 1 + binary.MaxVarintLen64 + len(name)
	for _, g := range groups {
		size += binary.MaxVarintLen64 + len(g)
	}

	var b strings.Builder

	b.Grow(size)
	b.WriteByte(byte(kind))

	for _, g := range groups {
		writeSegment(&b, g)
	}

	if kind == PositionSample {
		writeSegment(&b, name)
	}

	return b.String()
}

func writeSegment(b *strings.Builder, s string) {
	var prefix [binary.MaxVarintLen64]byte

	n := binary.PutUvarint(prefix[:], uint64(len(s)))
	b.Write(prefix[:n])
	b.WriteString(s)
}

// segments decodes every length-prefixed segment after the kind byte into a
// fresh, non-nil slice. The key is always well-formed — this package is the only
// writer — so decoding cannot fail.
func (p Position) segments() []string {
	rest := p.key[1:]
	segments := []string{}

	for len(rest) > 0 {
		n, width := uvarint(rest)
		rest = rest[width:]
		segments = append(segments, rest[:n])
		rest = rest[n:]
	}

	return segments
}

// uvarint reads an unsigned varint from the front of s without copying it,
// returning the value and the bytes it occupied.
func uvarint(s string) (value uint64, width int) {
	var shift uint

	for i := range len(s) {
		c := s[i]
		if c < 0x80 {
			return value | uint64(c)<<shift, i + 1
		}

		value |= uint64(c&0x7f) << shift
		shift += 7
	}

	return value, len(s)
}
