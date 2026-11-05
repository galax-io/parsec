# Feature Specification: Unrecognised report rows are errors

**Feature Branch**: `fix/report-reader-unrecognized-rows`

**Created**: 2026-10-10

**Issue**: [parsec #105](https://github.com/galax-io/parsec/issues/105)

**Milestone**: [v0.2.0 (#17)](https://github.com/galax-io/parsec/milestone/17)

## Context

`internal/corpus.FromReportHTML` reads figures from the report's two statistics
tables. When its strict row matcher finds nothing, it currently returns
`ErrNoFigures` even if those tables contain rows with unfamiliar markup.
`Accounts` deliberately skips that sentinel, because a recorded Gatling 3.13.1
report genuinely stores its figures in JavaScript rather than in the tables.

This change corrects that error classification. It does not teach the reader a
new row format or change the requirement for a complete per-request tree.

## User story and acceptance scenarios

As a contributor checking recorded runs, I need an unfamiliar statistics-row
shape to fail the account read so I can distinguish it from an empty HTML account.

1. Given both statistics tables contain rows but none match the supported shape,
   `FromReportHTML` returns an error that does not match `ErrNoFigures` through
   `errors.Is`; `Accounts` propagates that error.
2. Given the recorded 3.15.1 report with every data-row opening, including ROOT,
   changed from `<tr id=` to `<tr class="row" id=`, the same classification holds.
3. Given only table headings and empty data areas, the reader returns
   `ErrNoFigures`. Rows or row-shaped JavaScript elsewhere on the page do not
   change that result. The recorded 3.13.1 report retains this behaviour.
4. Given the unchanged 3.14.9 or 3.15.1 report, its totals and complete tree remain
   readable. Existing direct-table rows without a `tbody` remain supported.

## Requirements

- Inspect only the already bounded head and body statistics tables, and only
  when the supported row matcher finds zero rows across both tables.
- Exclude paired `thead` blocks from this additional check. Support multiline
  headings, heading attributes and uppercase tags; an unclosed heading must not
  hide subsequent rows.
- Recognise row openings by their tag boundary, including whitespace, `>` or
  `/>`; `<track>` is not a row. Either table may establish an unfamiliar shape.
- Return a contextual ordinary error for unfamiliar rows. Reserve
  `ErrNoFigures` for the absence of data rows.
- Preserve the supported-row parsing and validation paths, recorded corpus,
  dependencies, published APIs and decoder behaviour.

## Success criteria

The unfamiliar-row regression fails on the current implementation and passes
after the change. Error-identity assertions, account propagation tests and the
existing recorded-report controls pass with the race detector enabled.
