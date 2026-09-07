# Quickstart: An incomplete record

How to convince yourself this feature works, and how to make each part of it fail on purpose. Every
command runs from the repository root and needs nothing but the Go toolchain — no Gatling, no JDK,
no network. The five recordings under `testdata/corpus/gatling/` are the only inputs.

## Prerequisites

```bash
go version   # go1.25 or newer; go.mod is authoritative
```

```bash
go build ./... && go test ./...
```

That is the definition of a green tree here. Everything below assumes it passes first.

## 1. A killed run still reports what it recorded

The sweep cuts each recording at every offset inside its final 200 bytes and reads what is left:
1000 reads across five versions and both formats.

```bash
go test -race -run 'Truncat' ./gatling/...
```

Expected: every cut delivers the records that fit, ends with a `*gatling.TruncationError`, and never
panics. An intact recording ends with `io.EOF` and no truncation.

**Make it fail on purpose** — the two mistakes worth guarding against:

```bash
# (a) swallow the cut: end a truncated read with io.EOF instead of the error.
#     The sweep fails on every cut offset, and SC-004 is what catches it.
# (b) lose a record: stop delivering the last complete record before the cut.
#     The comparison against a whole-file read of the same prefix fails (FR-014).
```

## 2. The dropped-byte count and the position

```bash
go test -race -run 'Truncat.*Position|Truncat.*Dropped' ./gatling/...
```

Expected: `Offset + Dropped` is the file's length for a binary log, and `Line` names the
unterminated final line for a text one. The position is where the **incomplete record began**, not
where the stream stopped — see [data-model.md](data-model.md), invariant 3.

## 3. A damaged log is still refused

```bash
go test -race -run 'Syntax|Malformed|Mutation' ./gatling/...
```

Expected: an undefined record kind, an over-large length prefix and an unparseable field each still
end the read with a `*gatling.SyntaxError` naming its position, and none of them is reported as a
truncation. This is the guard that keeps story 3 honest: salvage must not become guessing.

## 4. A source failure is not an ending

```bash
go test -race -run 'SourceFail|WrappedEOF' ./gatling/...
```

Expected: a source returning an error that merely *wraps* `io.EOF` is reported as that failure, with
its cause available through `errors.Is`, and never as a clean end or a truncation. Before this
feature the binary codec converted it into a truncation (research [R5](research.md)).

## 5. Following a log that is still being written

```bash
go test -race -run 'Follow|BlockingSource' ./gatling/simlog/
```

Expected: each recording, delivered 300 bytes at a time by a source that blocks between appends,
yields the record stream a whole-file read yields — 66 records for the two text versions, 132 for
the three binary ones.

**Make it fail on purpose**: have the test's source return `io.EOF` between appends instead of
blocking. The read then ends at the first pause, which is precisely the difference between "wait"
and "the log ended" that [contract 2](contracts/blocking-source.md) exists to state.

## 6. No panic on any input

```bash
go test -run '^$' -fuzz '^FuzzDecode$' -fuzztime 90s ./gatling/binary/
go test -run '^$' -fuzz '^FuzzReader$' -fuzztime 90s ./gatling/text/
```

`-run '^$'` matters: without it the ordinary tests spend the budget. The seed corpora now include
prefixes of the recordings, so the first minute is spent on the input class this feature handles
(research [R8](research.md)).

## 7. The budget did not move

```bash
go test -run '^$' -bench 'Decode|Reader' -benchmem ./gatling/...
```

Expected: throughput and allocations within noise of the recorded v0.0.7 figures, and the peak-memory
tests still green. The change adds one `int64` per record on the binary path and nothing on the text
path.

## Everything, the way CI sees it

```bash
gofmt -l . && go vet ./... && go test -race -shuffle=on ./... && go test -tags=integration ./...
```

Coverage floors (90% decoder packages, 80% module) are what the `coverage` job enforces:

```bash
go test -cover ./gatling/... ./model/...
```
