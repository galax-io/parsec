package binary

// MaxAssertionBytes re-exports the ceiling on what a run record's assertion
// payloads come to in total, for the external test package.
//
// The constant is unexported because it is not a promise to a consumer, but two
// tests need to size a payload set against it, and a copied literal drifts
// silently: raise the real value and a test that copied it keeps asserting
// against the old one with no compile error. This is the route MaxStringLen
// already takes, minus the promise.
const MaxAssertionBytes = maxAssertionBytes
