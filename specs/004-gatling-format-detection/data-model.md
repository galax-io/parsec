# Phase 1 Data Model: Telling Which Gatling Wrote a simulation.log

**Feature**: `004-gatling-format-detection` | **Date**: 2026-09-05 | **Spec**: [spec.md](./spec.md)

This feature adds no field to `model/` and no record kind to `gatling/`. What it adds are the types
that answer three questions — *which format*, *what does the policy say*, *why was this refused* —
and the two interfaces a dispatched read hands back. Entities below map to the Key Entities section
of the spec.

---

## 1. Format — `gatling.Format`

The on-disk shape of a `simulation.log`. A property of the bytes, never of the file's name (FR-002).

| Value | Meaning |
|---|---|
| `FormatUnknown` | zero value; nothing has been detected. Never returned by a successful `Detect`. |
| `FormatText` | tab-separated text log, written through Gatling 3.12.0 |
| `FormatBinary` | binary stream, written from Gatling 3.13.0 |

`String()` returns `"unknown"`, `"text"`, `"binary"`, and `Format(n)` for anything else — the
sentinel-behind-zero convention `Kind`, `Status`, `Event` and `Verdict` already follow in
`gatling/record.go` and `gatling/version.go`.

**Detection rule** (R3, R4, R5):

| Leading bytes | Result |
|---|---|
| `0x00` … | `FormatBinary` |
| `RUN\t` … | `FormatText` |
| `ASSERTION\t` … | `FormatText` |
| a proper prefix of `ASSERTION\t`, then end of input | `*FormatError{Short: true}` |
| anything else | `*FormatError{Short: false}` |

`DetectSize = 10` is the fixed window (`len("ASSERTION\t")`). Detection reads no more, whatever the
size of the log.

**Validation rules**: the tab is part of every text rule — a literal alone does not classify.
`0x00` is tested first and cannot collide with an ASCII literal. `USER`, `REQUEST`, `GROUP` and
`ERROR` are *not* accepted: they cannot legally open a log, and accepting them would classify a
mid-file fragment as a whole one.

---

## 2. Read options — `gatling.Option`, `gatling.WithStrict`

What a caller may vary about a read. Absence of options is the default, and the default is lenient
(FR-020).

| Option | Meaning |
|---|---|
| `WithStrict()` | refuse a version above the covered range instead of decoding it with a warning |

`Option` is `func(*readOptions)` over an **unexported** configuration: a codec forwards the options
untouched to `Policy.Apply` and never inspects them, and a consumer cannot define an option this
package would then have to honour. `WithStrict` is the only one this feature defines; the `With`
prefix is the Go convention for functional options. Every constructor takes `opts ...gatling.Option`
and `simlog` forwards them untouched.

**State transitions**: none. Options are applied once, before the first read, and never change for
the life of a reader.

---

## 3. Version policy — `gatling.Policy`

One codec's version policy: the range its golden corpus covers.

| Field | Meaning |
|---|---|
| `Min` | oldest release the corpus covers |
| `Max` | newest release the corpus covers |

`Apply(found Version, opts ...Option) (Warning, error)` is the whole policy for one log, and the only
place the outcomes are decided (FR-012). It is called once, before any record is decoded (FR-018).

| `found` vs range | `WithStrict` | Warning | error |
|---|---|---|---|
| below `Min` | either | zero | `*VersionError` (`Parsed: true`) |
| `Min` … `Max` | either | zero | `nil` |
| above `Max` | absent | the warning to surface | `nil` |
| above `Max` | given | zero | `*UnverifiedError` |

A version string that is not a plain release never reaches `Apply`: the codec refuses it while
parsing the header, with `*VersionError{Parsed: false}` quoting the string (FR-017, unchanged from
spec 002).

**Invariant (FR-021)**: strictness can only turn the third row into the fourth. It cannot reach the
first two, which is why it is not a `Verdict` — see below.

---

## 4. Verdict — `gatling.Verdict` *(existing, unchanged)*

`VerdictUnknown` / `VerdictRefused` / `VerdictAccepted` / `VerdictUnverified`, produced by the
existing `Gate(found, lo, hi)`. It stays a fact about where a version sits relative to the corpus
range, and deliberately gains no strict variant: a caller's policy does not change where a version
sits. `Policy.Apply` is built on it.

---

## 5. Errors — all in `gatling`, shared by both codecs (FR-027)

| Type | Fields | Ends a read because | New |
|---|---|---|---|
| `*FormatError` | `Head []byte`, `Short bool` | the bytes are not a Gatling `simulation.log` (`Short: true`: the input ended before it could be told) | yes |
| `*UnsupportedFormatError` | `Format Format` | the format is known and this module has no codec for it yet | yes |
| `*UnverifiedError` | `Version`, `Min`, `Max` | the version is above the covered range and the caller asked for strictness | yes |
| `*VersionError` | `Found`, `Version`, `Parsed`, `Min`, `Max` | the version is below the range, or is not a release string | existing |
| `*SyntaxError` | `Line`, `Expected`, `Found` | a line could not be decoded | existing |

`Head` is bounded by `DetectSize` and is rendered quoted and printable, so an error message from a
gzip stream is readable rather than a spray of bytes.

**Distinguishability (FR-010, FR-022)**: one type per cause, all reachable through `errors.As`. No
caller ever matches on message text. `*UnverifiedError` and `*VersionError` are separate types
precisely because they describe opposite evidence gaps — too old versus too new.

---

## 6. Warning — `gatling.Warning` *(existing, unchanged)*

Raised exactly once per above-range log, whatever path the caller took (FR-016). `simlog` never
reads a version and therefore never raises one; the codec does, once, and `text.NewRunReader` maps it
to `model.Warning` on the run as it already does.

---

## 7. Codec coverage — `simlog.Support`

What this module does with one Gatling log format, so a consumer can report it without hard-coding a
range (FR-023 … FR-026).

| Field | Meaning |
|---|---|
| `Format` | the format described |
| `Readable` | true when this module has a codec for it |
| `Oldest`, `Newest` | the range that codec accepts without a warning; zero when `Readable` is false |

`Supported() []Support` returns one entry per known format, in a fixed order. Today:

| Format | Readable | Oldest | Newest |
|---|---|---|---|
| `FormatText` | true | 3.11.5 | 3.12.0 |
| `FormatBinary` | false | — | — |

`Oldest`/`Newest` come from `text.SupportedVersions()`, which already reads the codec's own
corpus-bound range and cannot be set by a caller (FR-024). The binary row is FR-025's third answer:
known, and not readable yet.

---

## 8. Dispatched readers — `simlog.RecordReader`, `simlog.RunReader`

The two shapes a dispatched read hands back. Both are exactly the methods `text` already has, so
`*text.Reader` and `*text.RunReader` satisfy them with no adapter (R9).

```
RecordReader   Header() gatling.Header
               Assertions() []string
               Warnings() []gatling.Warning
               Next() (gatling.Record, error)

RunReader      Run() model.Run
               Next() (model.Item, error)
```

**Invariant (FR-011, SC-009)**: a reader obtained through `simlog` yields records identical, field
for field, to one obtained by handing the same log straight to its codec. `simlog` adds detection
and forwarding, and nothing else — no re-gating, no re-wrapping of errors that would hide their
type, no buffering that changes the 1 MiB line ceiling.

---

## Relationships

```
Detect(head)  ──▶ Format ──▶ simlog picks a codec
                              │
                              ├─ FormatText   ──▶ text.NewReader / text.NewRunReader
                              │                     │
                              │                     └─▶ Policy.Apply(header version, opts)
                              │                             ├─ nil            ──▶ records
                              │                             ├─ Warning        ──▶ records + one warning
                              │                             ├─ *VersionError  ──▶ refused (too old / not a release)
                              │                             └─ *UnverifiedError ─▶ refused (strict, too new)
                              │
                              └─ FormatBinary ──▶ *UnsupportedFormatError
```

Nothing in this feature computes a count, a mean, a percentile, a range or a series (Principle I).
