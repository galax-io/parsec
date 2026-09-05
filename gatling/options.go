package gatling

// readOptions is what a caller may vary about one read. Its zero value is the
// default, and the default is lenient.
//
// It is unexported on purpose. A codec forwards the options it was handed
// without inspecting them, so nothing outside this package needs to read the
// resolved value, and a consumer able to define its own option would be
// defining behaviour this package then has to honour.
type readOptions struct {
	// strict refuses a version above the range any recording covers, instead of
	// decoding it and raising a warning.
	strict bool
}

// Option varies one read. Every Gatling codec accepts the same options, so a
// caller states its policy once, whichever format the log turns out to be in.
type Option func(*readOptions)

// WithStrict makes a read refuse a Gatling version that no recording covers,
// rather than decoding it and raising a warning. Reach for it where an
// unverified number is worth less than no number at all: a release gate, an
// automated comparison between runs.
//
// It changes nothing else. A version inside the supported range reads
// identically with it and without it, and a version below the range is refused
// either way — strictness only ever tightens the gate.
func WithStrict() Option {
	return func(o *readOptions) { o.strict = true }
}

// resolve folds opts onto the default.
//
// A nil Option is skipped rather than called. Returning one is ordinary Go —
// `func policyFor(c Config) Option { if !c.Strict { return nil }; ... }` — and
// calling it would panic, which a package whose job is refusing malformed input
// must never do.
func resolve(opts []Option) readOptions {
	var o readOptions

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		opt(&o)
	}

	return o
}
