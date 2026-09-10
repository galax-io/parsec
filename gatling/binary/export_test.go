package binary

import "io"

// EndsWithItsLastBytes re-exports the source that hands over its final bytes
// together with the error that ends it — io.EOF when err is nil — in one call,
// so the external test package reads through the same shape rather than a copy.
func EndsWithItsLastBytes(data []byte, err error) io.Reader {
	return &endsWithItsLastBytes{data: data, err: err}
}

// MaxAssertionBytes re-exports the ceiling on what a run record's assertion
// payloads come to in total, for the external test package.
//
// The constant is unexported because it is not a promise to a consumer, but two
// tests need to size a payload set against it, and a copied literal drifts
// silently: raise the real value and a test that copied it keeps asserting
// against the old one with no compile error. This is the route MaxStringLen
// already takes, minus the promise.
const MaxAssertionBytes = maxAssertionBytes

// ReadBufferSize re-exports the size of the codec's read buffer, for the same
// reason and by the same route: a test bounds what a refusal may pull from the
// source by it, and a copied literal would drift.
const ReadBufferSize = readBufferSize
