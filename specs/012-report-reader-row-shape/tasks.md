# Tasks: Unrecognised report rows are errors

**Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)

All tasks are sequential; the change has one implementation concern.

- [x] T001 [tier:deep] Verify the unchanged base with the corpus race/shuffle
  tests, `go build ./...` and the full race/shuffle test suite.
- [ ] T002 [tier:deep] Commit `spec.md`, `plan.md` and `tasks.md` under
  `specs/012-report-reader-row-shape/` as
  `docs(speckit): add 012-report-reader-row-shape spec/plan/tasks` before code.
- [ ] T003 [tier:deep] In `internal/corpus/report_test.go`, derive a temporary
  row-shape fixture from the recorded 3.15.1 report and observe the runtime
  failure of non-sentinel and `Accounts` propagation assertions on the base.
- [ ] T004 [tier:deep] Add the bounded-table, heading and tag-boundary cases to
  `internal/corpus/report_test.go`, retaining recorded-report controls.
- [ ] T005 [tier:deep] Change only the zero-match classification in
  `internal/corpus/report_html.go`, document it and run the focused tests green.
- [ ] T006 [tier:deep] Run formatting, build, vet, lint, race, integration and
  coverage checks; record actual versions and any unexecuted gates.
- [ ] T007 [tier:deep] Review the final scope and regression evidence, then make
  the single green commit
  `fix(corpus): report unrecognised statistics rows (#105)`.
