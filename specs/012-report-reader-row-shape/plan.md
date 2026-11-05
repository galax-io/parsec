# Implementation Plan: Unrecognised report rows are errors

**Branch**: `fix/report-reader-unrecognized-rows` | **Date**: 2026-10-10

**Spec**: [spec.md](spec.md) | **Issue**: [#105](https://github.com/galax-io/parsec/issues/105)

## Approach

Keep `statisticsTable` and the strict `reportRow` matcher. In the zero-match
branch, remove every paired `thead` block from each bounded table with a
case-insensitive, multiline, non-greedy regular expression. Check the remaining
markup for a row opening with an explicit tag boundary. A row in either table
produces an ordinary shape error; otherwise retain the existing sentinel.

Use two small, precompiled standard-library regular expressions. Do not require
`tbody`, add an HTML framework or broaden the successful parsing path. Update
the reader's comment to state the error distinction. `Accounts` already returns
non-sentinel errors and needs regression coverage rather than a logic change.

## Technical context and constitution check

The module declares Go 1.25 with toolchain Go 1.26.8 and has no third-party
requirements. Tests use stdlib `testing`, `errors.Is` and the recorded corpus.
The throughput and peak-memory goals for decoder features do not apply: this
change affects an internal report-account helper's error path, not a decoder.

Against constitution **2.4.0**:

- **I / II**: no model, statistics computation, codec or version-gate changes.
- **III**: derive the regression from the real 3.15.1 HTML in a temporary,
  explicitly named fixture; leave recordings untouched. Keep the 3.13.1 empty
  report and 3.14.9/3.15.1 totals/tree controls. Run race and coverage checks.
- **IV / V**: no new dependencies or published API changes.
- **VI**: local error classification, errors as values and no speculative
  abstraction. Required `golang-error-handling` and `golang-testing` skills are
  unavailable; follow Principles I–VI as the constitution permits.
- **Workflow**: milestone v0.2.0 (#17); commit these three artifacts as
  `docs(speckit)` before the single green issue-fix commit.

No constitution exception is needed.

## Tests and validation

The primary regression changes all data-row openings in a temporary copy of the
recorded 3.15.1 HTML, including ROOT. Assert a non-nil error and
`!errors.Is(err, ErrNoFigures)` before changing production code. With that HTML
and the recorded console account, assert that `Accounts` fails rather than
silently returning the remaining account.

Table-driven boundaries cover unfamiliar rows in either statistics table,
direct-table rows, multiline headings with attributes and uppercase tags,
multiple paired headings, unclosed headings, `<track>`, and row-like text outside
the tables. Empty data areas retain the sentinel. Existing missing-table,
malformed-row and partial-tree checks remain in place.

Run the focused corpus tests first, then build, vet, the full race/shuffle suite,
integration tests and enforced coverage with the existing scripts. Check
`gofmt`, the configured `golangci-lint fmt --diff`, lint and unchanged module
metadata. State the actual tool versions and any differences from CI pins.
Automatic toolchain/module downloads are disabled for local verification;
checks requiring additional tools or network data must be reported separately.

## Files

Only `internal/corpus/report_html.go`, its existing report tests and these three
spec artifacts need changes. No recorded fixture, workflow or lint configuration
change is planned.
