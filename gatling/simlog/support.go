package simlog

import "github.com/galax-io/parsec/gatling"

// Support is what this module does with one Gatling log format.
type Support struct {
	// Format is the format described.
	Format gatling.Format
	// Readable says whether this module has a codec for it. A format that is
	// known and not readable is a different answer from an unknown one, and a
	// consumer can say so to a user.
	Readable bool
	// Oldest and Newest bound what that codec accepts without a warning. They
	// hold their zero value when Readable is false.
	Oldest, Newest gatling.Version
}

// Supported returns one entry per Gatling log format this module knows about,
// in a fixed order, so a consumer can report what parsec reads without naming a
// format or a version of its own — a hard-coded range goes stale the first time
// this module widens one, and nobody notices until a user is told their
// supported log is unsupported.
//
// It is read from the same table dispatch uses, so a format cannot become
// readable in one and not the other. The ranges are the codecs' own, bound to
// the golden corpus; a caller cannot widen one, and the slice is freshly built
// so mutating it changes nothing.
func Supported() []Support {
	out := make([]Support, 0, len(codecs))

	for _, c := range codecs {
		s := Support{Format: c.format, Readable: c.records != nil}
		if s.Readable {
			s.Oldest, s.Newest = c.versions()
		}

		out = append(out, s)
	}

	return out
}
