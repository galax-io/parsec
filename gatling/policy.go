package gatling

// Policy is one codec's version policy: the range its golden corpus covers.
// Widening it means recording a new corpus entry first, never assuming a format
// did not change.
//
// It is a value rather than a free function so that the bounds travel together
// with the decision made from them. That is what stops a second codec from
// growing a copy of the rule: a codec states its range and asks, rather than
// deciding for itself.
type Policy struct {
	// Min and Max bound the range this codec accepts without a warning. They
	// equal the versions its golden corpus covers.
	Min, Max Version
}

// Apply resolves the policy for the version a log names. It is the single place
// the outcomes are decided, and a codec calls it once, before any record is
// decoded, so that a refusal never arrives after records have been delivered.
//
// The Verdict it returns is the fact a codec acts on: VerdictUnverified is what
// makes a codec lenient about a record shaped unfamiliarly. Reading that from
// the verdict rather than from whether a Warning came back keeps the two tied
// together, so a warning raised for some future reason cannot silently relax a
// parser.
//
// It returns VerdictAccepted with the zero Warning and a nil error for a
// version inside the range; VerdictUnverified with a Warning and a nil error
// for a version above it; VerdictRefused with a *VersionError for a version
// below it; and VerdictRefused with an *UnverifiedError for a version above it
// under WithStrict.
//
// A version string that is not a plain release never reaches Apply. A codec
// refuses that while reading the header, quoting what it found, because there
// is no release to place against a range.
func (p Policy) Apply(found Version, opts ...Option) (Verdict, Warning, error) {
	// Resolved once, before the switch, so that an option's validity never
	// depends on which branch a log happens to take.
	o := resolve(opts)

	switch verdict := Gate(found, p.Min, p.Max); verdict {
	case VerdictAccepted:
		return verdict, Warning{}, nil

	case VerdictRefused:
		return verdict, Warning{}, &VersionError{
			Found:   found.String(),
			Version: found,
			Parsed:  true,
			Min:     p.Min,
			Max:     p.Max,
		}

	case VerdictUnverified:
		if o.strict {
			return VerdictRefused, Warning{}, &UnverifiedError{Version: found, Min: p.Min, Max: p.Max}
		}

		return verdict, Warning{Version: found, Min: p.Min, Max: p.Max}, nil

	case VerdictUnknown:
	}

	// Gate never returns VerdictUnknown. If one ever reaches here the gate has
	// not run, and a gate that has not run refuses: failing open would decode a
	// log nothing has vouched for.
	return VerdictRefused, Warning{}, &VersionError{
		Found:   found.String(),
		Version: found,
		Parsed:  true,
		Min:     p.Min,
		Max:     p.Max,
	}
}
