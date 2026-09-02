package gatling

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
)

// Version is a plain MAJOR.MINOR.PATCH Gatling release, ordered by component.
type Version struct {
	Major, Minor, Patch int
}

// ParseVersion reads a plain release number such as "3.11.5". A version string
// carrying anything else — a snapshot, milestone, nightly or vendor suffix, a
// leading "v", surrounding space, or fewer than three components — is not a
// release and is rejected, because a build that is not a release cannot be
// placed against any recording.
func ParseVersion(s string) (Version, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("gatling: %q is not a major.minor.patch release version", s)
	}

	var n [3]int

	for i, part := range parts {
		if part == "" || strings.IndexFunc(part, notDigit) >= 0 {
			return Version{}, fmt.Errorf("gatling: %q is not a major.minor.patch release version", s)
		}

		v, err := strconv.Atoi(part)
		if err != nil {
			return Version{}, fmt.Errorf("gatling: %q is not a major.minor.patch release version: %w", s, err)
		}

		n[i] = v
	}

	return Version{Major: n[0], Minor: n[1], Patch: n[2]}, nil
}

func notDigit(r rune) bool { return r < '0' || r > '9' }

// String returns the release number in MAJOR.MINOR.PATCH form.
func (v Version) String() string {
	return strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Patch)
}

// Compare orders two versions by major, then minor, then patch. It returns -1
// when v is older than o, 0 when they are equal and +1 when v is newer.
func (v Version) Compare(o Version) int {
	if c := cmp.Compare(v.Major, o.Major); c != 0 {
		return c
	}

	if c := cmp.Compare(v.Minor, o.Minor); c != 0 {
		return c
	}

	return cmp.Compare(v.Patch, o.Patch)
}

// Verdict is the outcome of the version gate for a log. VerdictUnknown is the
// zero value and is never returned by Gate.
type Verdict uint8

// The outcomes of the version gate, behind the unknown sentinel.
const (
	// VerdictUnknown is the zero value: no gate has run.
	VerdictUnknown Verdict = iota
	// VerdictRefused means the version is below the covered range, or the log
	// named no release version at all. Nothing is decoded.
	VerdictRefused
	// VerdictAccepted means the version lies inside the range the golden corpus
	// covers. Records decode without a warning.
	VerdictAccepted
	// VerdictUnverified means the version is above the covered range. Records
	// decode, and a Warning is raised, because no recording proves them.
	VerdictUnverified
)

// String names the verdict.
func (v Verdict) String() string {
	switch v {
	case VerdictUnknown:
		return unknownName
	case VerdictRefused:
		return "refused"
	case VerdictAccepted:
		return "accepted"
	case VerdictUnverified:
		return "unverified"
	default:
		return "Verdict(" + strconv.Itoa(int(v)) + ")"
	}
}

// Gate applies the version gate: a version below lo is refused, a version
// above hi is unverified, and anything from lo through hi inclusive is
// accepted. A codec's lo and hi equal the range its golden corpus covers.
func Gate(found, lo, hi Version) Verdict {
	switch {
	case found.Compare(lo) < 0:
		return VerdictRefused
	case found.Compare(hi) > 0:
		return VerdictUnverified
	default:
		return VerdictAccepted
	}
}
