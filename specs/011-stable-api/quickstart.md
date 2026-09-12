# Quickstart: validating A stable API

**Feature**: 011-stable-api | **Date**: 2026-09-12

How to prove each part of this feature works, from a clean checkout of the branch. Every command is
runnable; nothing here is implementation.

## Prerequisites

```bash
go version   # 1.25 or newer; the toolchain directive pins go1.26.8 for CI parity
gh auth status
git config core.hooksPath .githooks   # once per clone: the local tag gate
```

No corpus is recorded for this feature. The existing entries under
`testdata/corpus/gatling/{3.11.5,3.12.0,3.13.1,3.14.9,3.15.1}` are what it uses.

## The whole gate

```bash
gofmt -l . && go vet ./... && go build ./... && go test -race -shuffle=on ./...
```

```bash
go mod tidy && git diff --exit-code
```

The second must produce no diff: Principle IV, and this feature adds no module.

## 1 — The surface is a list (User Story 1)

The inventory is a golden file; a drift is a red test.

```bash
go test -run TestExportedSurface ./...
```

To see the list the test compares against, and its count:

```bash
tail -1 testdata/api/surface.txt        # TOTAL 275
grep -c '^    ' testdata/api/surface.txt
```

The two withdrawals are absent from the tool's own view:

```bash
go doc -all ./gatling | grep -E '\bGate\b|MaxRunStart'   # expect no match
```

A consumer compiles against the promised surface alone. From a scratch module outside this tree:

```bash
go mod edit -replace github.com/galax-io/parsec=/path/to/parsec && go build ./...
```

## 2 — One name, one rule, one copy (User Story 2)

```bash
grep -rn 'Tool = "gatling"' --include='*.go' .      # exactly one declaration, in gatling/
```

```bash
go test -run 'OutOfRange|ZeroValue' ./model/ ./gatling/ ./gatling/run/
```

The eleven enums at an out-of-range value render as the type name and the number; at the zero value
they render their own name, unchanged.

```bash
diff <(sed -n '/^func NewRunReader/,/^}/p' gatling/text/model.go) \
     <(sed -n '/^func NewRunReader/,/^}/p' gatling/binary/model.go)
```

Nothing may differ but the constructor call and the capability set.

## 3 — The wrong reader says so (User Story 3)

```bash
go test -run 'Refuses(ABinaryLog|ATextLog)|StillReportsDamage|DamagedTextLog' ./gatling/text/ ./gatling/binary/
```

The cases are the corpus logs themselves: `3.14.9/simulation.log` into `text.NewReader`, and
`3.11.5/simulation.log` into `binary.NewReader`. Each must yield a `*gatling.UnsupportedFormatError`
naming the other format — and a damaged log of the reader's own format must still yield a
`*gatling.SyntaxError`.

## 4 — The span never ends before what it counted (User Story 4)

```bash
go test -run 'Bounds|NoRecordedEnd|EndIsNeverBefore' ./model/
```

The case that decides it: a sample at 00:00:10 lasting 5 s and a sample at 00:00:20 with no recorded
end. `End()` reports 00:00:20.

## 5 — What is published is true (User Story 5)

```bash
go test -run 'TestNoDocComment|TestEveryReaderStates|TestTheTwoWarnings|TestPackageOverviews' .
```

No exported identifier's doc comment renders a literal `//` or a collapsed paragraph.

```bash
for p in . ./model ./gatling ./gatling/text ./gatling/binary ./gatling/simlog ./gatling/run; do
  echo "=== $p"; go doc "$p" | head -30
done
```

Read as a newcomer would: each overview names where to start, none claims a shipped package or type
is future work, and the root lists every package including `gatling/run`.

```bash
go doc ./gatling Warning && go doc ./model Warning
```

Each names the other and says how a value crosses.

```bash
go doc ./gatling/simlog RecordReader && go doc ./gatling/text Reader
```

Each states that a value may be used by one goroutine at a time, and what a concurrent call costs.

## 6 — Every test can go red (User Story 6)

This one is not verified by running the suite green. For each of the six, break the production code
it names, run it, and watch it fail:

| Test | Break |
|---|---|
| `TestTheGroupPathIsReusedBetweenRecords` | make `Groups` a fresh `slices.Clone(path)` in `gatling/binary/record.go` |
| `TestASampleThatLostItsOutcomeIsNotASuccess` | move `OutcomeSuccess` to iota 0 in `model/sample.go` |
| `TestCorpusFailurePresenceMatchesTheOutcome` | make `wire.failure` attach a `Failure` to every sample |
| `TestKindDecidesWhichFieldIsRead` | make `Bounds.Extend` read `Sample` for an `ItemGroup` |
| `TestACountTooLargeToBeADurationIsAbsent` (`internal/wire`) | drop the millisecond ceiling in `millisDuration` |
| `TestFindNeverOpensTheLog` | make `run.newest` skip candidates whose log cannot be opened |

```bash
sudo docker run --rm -v "$PWD":/src -w /src golang:1.26 go test ./gatling/run/
```

Run as root, `TestFindNeverOpensTheLog` must skip through `requireUnixPermissions` or still test both
fixtures — never pass vacuously.

## 7 — A stranger can work from the README (User Story 7)

```bash
go test -run Example ./gatling/simlog/
```

The README's first program is this example's body, so `go test` compiles and output-checks the code
the README publishes.

```bash
grep -c '```' README.md          # not 0
grep -n 'go get' README.md
grep -n '3.13.1' README.md       # the table, with the reason 3.13.0 is refused
grep -rn 'v0\.0\.' README.md     # no per-package version stamps
```

```bash
gh api repos/galax-io/parsec --jq '{description, homepage}'
```

The description must not say "statistics"; the homepage points at pkg.go.dev.

```bash
sed -n '/0.0.9/,/0.0.8/p' CHANGELOG.md | grep -n 'modification time'
```

The ordering rule appears once and reads: modification time, then the run id's own UTC stamp, then
the directory name.

## 8 — The release path (User Story 8)

```bash
bash scripts/check-pins.sh
```

Needs `python3` with PyYAML — the gate parses the workflows rather than matching their lines, and says
so with exit 2 if the parser is missing. `verify.yml` installs it pinned.

Holds both rules of contract 5: every reference to code the repository does not contain — a `uses:`, a
reusable workflow, a job `container:`, a `services.<id>.image:` — is pinned to a digest with the version
in a trailing comment, and no `${{ … }}` substitution of **any** context reaches a `run:` script, a
github-script `script:` input or a `docker://` step's `args:`. A key the gate does not recognise is
refused rather than skipped.

A bare `grep` over `uses: ` is not enough, and neither was the line matcher this replaced: a regular
expression cannot tell a `uses:` key from the same text inside a scalar, so it refused a step whose name
mentioned one and passed `? uses` / `: …`, which is the explicit-key spelling of a real one.

```bash
gh workflow run fuzz-nightly.yml -f fuzztime='10m"; echo pwned'
```

Must fail at the validation step, before any shell runs the value.

```bash
for t in scripts/*_test.sh .claude/hooks/*_test.sh .githooks/*_test.sh; do bash "$t"; done
```

## Before the tag

```bash
bash scripts/check-linkage.sh
```

Every PR merged since the previous tag carries milestone **#11 v0.1.0 A stable API**, and every issue
in it whose fix is on `main` is closed. Then the release branch, per `AGENTS.md`:

```bash
git checkout -b release/0.1.0 main && git push -u origin release/0.1.0
```

```bash
git tag v0.1.0 && git push origin v0.1.0
```
