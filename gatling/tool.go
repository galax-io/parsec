package gatling

// Tool is what a Gatling source is called in
// [github.com/galax-io/parsec/model.Run].Tool.
//
// It is a property of the tool, not of either log format, so it lives here
// rather than in a codec: a consumer that wants to branch on the tool can name
// it without importing gatling/text or gatling/binary, and there is one
// spelling to compare against rather than two that were equal only because the
// same string had been typed twice.
const Tool = "gatling"
