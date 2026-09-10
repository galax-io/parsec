# Contract 2 — The binary reader's budget, buffer and gate

**Applies to**: `binary.NewReader`, `binary.NewRunReader`, `binary.Reader`, `binary.MaxStringLen`;
and, for the gate's order, `text.NewReader` and `text.NewRunReader`.
**Status**: the budget is what `binary.Reader` already documents; the ceilings are how it becomes
true. Issues #75, #76, #87.

## The budget, as `Reader`'s documentation will state it

Peak *retained* heap stays under **32 MiB** for any log this codec accepts, whatever its length. It
is bounded by:

- a read buffer of 64 KiB, allocated by the codec — a caller's own `*bufio.Reader` is read
  *through*, never adopted (#87);
- a scratch buffer of at most `MaxStringLen` (1 MiB) for the value being decoded;
- three tables held for the life of the read, each bounded in bytes as its entries arrive, counting
  each entry's content plus a 16-byte header:

| Table | Ceiling |
|---|---|
| assertion payloads | 8 MiB (and at most 65,536 of them) |
| scenario names | 1 MiB (and at most 65,536 of them) |
| the string table — every distinct string the log introduces | 12 MiB |

A log that exceeds a ceiling is refused as damaged, with a `*gatling.SyntaxError` at the entry that
crossed it, naming the total and the ceiling. No log Gatling writes for an ordinary simulation
approaches any of them: scenario names are class names, assertion payloads run to tens of kilobytes,
and the distinct strings of a run are its request and group names plus its failure messages, which
Gatling truncates. A simulation whose checks put a per-session value into every failure message can
introduce more than 12 MiB of distinct text over a long run; such a log is now refused where it was
previously accepted at a cost the documentation denied.

## The gate, first

`NewReader` reads the version string, judges it, and only then reads the rest of the run record: a
log naming a version below the supported range — or naming no release at all — is refused with a
`*gatling.VersionError` whatever follows the version, and the bytes pulled from the source before
that refusal are at most one read-buffer fill, whatever the log's tables claim. A version string
longer than 64 bytes is refused as damaged before its bytes are read, so a corrupt length cannot make
the refusal cost more, or quote a megabyte back. The text codec judges the version on its `RUN` line
before validating the run start, and rules on an `ASSERTION` line with too few fields only after
that, so both codecs return the same error type for the same fault. One fault is the exception, and
the text format forces it: its assertions come before the version, so an assertion table past its
8 MiB ceiling is refused there as damage. What the gate *decides* — the range, the warning above it, strict mode — is
unchanged (see [data-model.md](../data-model.md) §2 for the full precedence).

## The buffer, the codec's

Handed any `io.Reader`, including a `*bufio.Reader` of any size, the codec allocates its own 64 KiB
buffer. A caller's buffered reader of the codec's size or smaller has nothing buffered after
`NewReader`; a larger one may prefetch for itself, which is the caller's `bufio` at work and the
caller's memory.

## What changes for a consumer, in `CHANGELOG.md` terms

**Changed**

- `gatling/binary` bounds what it retains in **bytes** for every table it keeps across a read. The
  scenario names are capped at 1 MiB and the string table at 12 MiB, each counting a 16-byte header
  per entry, beside the assertion payloads' existing 8 MiB; a log past a ceiling is refused as
  damaged. The count-only ceiling on the string table (1,048,576 entries) is gone — the byte ceiling
  binds first. The 32 MiB peak-heap figure `Reader` documents now holds by construction: worst-case
  retention is about 26 MiB, where before it was bounded by nothing but the counts. (#75)
- Both codecs judge the version before the rest of the run header. A binary log naming a version
  below the range is refused with a `*gatling.VersionError` whatever its scenario or assertion
  tables hold, and reads at most one buffer fill before refusing; a text `RUN` line whose version
  is below the range is refused the same way even when its run start is out of bounds or an
  `ASSERTION` line before it has too few fields. Previously the binary codec reported a corrupt count
  after an out-of-range version as damage, and the text codec reported a bad run start or a short
  assertion line before an out-of-range version. A binary version string longer than 64 bytes is
  refused as damaged before it is read. (#76)

**Fixed**

- `gatling/binary` adopted a caller's `*bufio.Reader` as its own read buffer when that buffer was at
  least 64 KiB, so the fixed buffer `Reader` documents was whatever the caller had — a 64 MiB one
  for a caller buffering a large file. The codec now reads through its own buffer, as the text codec
  already does. (#87)

## Verified by

The three `…PeakMemory` tests (distinct scenario names, distinct strings, the tables at their
ceilings — the last one accepted and measured), the boundary cases in `limits_test.go`, the
codec-parity table `TestCodecsGateBeforeTheRestOfTheRunRecord`, the bytes-before-refusal test, and
`TestCodecsReadThroughTheirOwnBuffer`. See [quickstart.md](../quickstart.md) §3–4.
