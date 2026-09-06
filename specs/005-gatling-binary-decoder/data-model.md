# Phase 1 Data Model: Reading the Binary simulation.log

**Feature**: `005-gatling-binary-decoder` | **Date**: 2026-09-06 | **Spec**: [spec.md](./spec.md)

This feature adds no field to `model/` and no record kind to `gatling/`. Both already exist, and
that is the point: the binary codec produces the same values the text codec does. What is new is the
on-disk grammar below, the table that resolves its strings, and the state a reader carries.

---

## 1. The grammar

Read out of Gatling's writer at both bounds of the supported range and verified byte for byte
against a real 3.15.1 log (research [R1](./research.md)). Every integer is big-endian.

### Primitives

| Name | Encoding | Notes |
|---|---|---|
| `byte` | 1 byte | record kinds |
| `bool` | 1 byte | `1` true, `0` false; any other value is malformed |
| `int32` | 4 bytes, big-endian, signed | counts, offsets, lengths, cache indices |
| `int64` | 8 bytes, big-endian, signed | the run's start, epoch milliseconds |
| `string` | `int32 n`; **if n is 0 the field ends there**; otherwise `n` bytes then a 1-byte coder | `n` counts bytes of the JVM's internal array, not characters |
| `cached` | `int32 i`; if `i > 0` a new entry follows as a `string`; if `i < 0` it refers to entry `-i` | `i` is never 0 |
| `blob` | `int32 n` then `n` bytes | assertion payloads, carried through unread |

**Validation.** `n < 0` is malformed. `n` above the documented cap is malformed and MUST be rejected
before allocating. A coder other than 0 or 1 is malformed. A `cached` index of 0, or a negative index
naming an entry that was never introduced, is malformed.

### Records

Each begins with its kind byte. The kinds are fixed by Gatling's `RecordHeader` and unchanged across
the supported range.

| Kind | Byte | Fields, in order |
|---|---|---|
| Run | `0` | `string` version · `string` simulation class · `int64` start · `string` description · `int32` scenario count · that many `string` · `int32` assertion count · that many `blob` |
| Request | `1` | `int32` depth · that many `cached` group names · `cached` name · `int32` start offset · `int32` end offset · `bool` ok · `cached` message |
| User | `2` | `int32` scenario index · `bool` isStart · `int32` timestamp offset |
| Group | `3` | `int32` depth · that many `cached` group names · `int32` start offset · `int32` end offset · `int32` cumulated response time · `bool` ok |
| Error | `4` | `cached` message · `int32` timestamp offset |

**The Run record is not optional and is not repeatable.** It is the first record, it carries the
version the gate reads, and every later record is meaningless without its `start` and its scenario
list. A second Run record is malformed.

---

## 2. String table

The writer's cache, rebuilt by the reader in the writer's order.

| Property | Value |
|---|---|
| First index | 1 — zero is excluded because a hit is written as the negation of an index and `-0 == 0` |
| Grows by | one entry each time a `cached` field carries a positive index |
| Never contains | the Run record's version, simulation class, description or **scenario names** — all written as plain `string` |
| Lifetime | the whole read; it is why the stream must start at byte 0 |
| Bounded by | the number of distinct strings the simulation declares, not the number of records |

**State transitions**: append-only. An entry is never replaced or removed, and a positive index that
is not the next expected one is malformed — the writer's counter is strictly sequential, so a gap
means the stream desynchronised earlier.

**Failure**: a reference to an entry that does not exist ends the read with an offset. It never
yields an empty string or the nearest entry (FR-012).

---

## 3. Reader state

What a reader holds between records, and what bounds it.

| Field | Bounded by | Notes |
|---|---|---|
| the run header | fixed | decoded once, available before the first record |
| scenario names | the simulation's scenario count | indexed by user records |
| string table | distinct strings in the simulation | §2 |
| group path scratch | the deepest group nesting | reused between records; the contract says the caller must copy it |
| a read buffer | a fixed size | peak memory does not grow with the log |
| the terminal error | one | after a failure every later call returns the same error |

---

## 4. Timestamps

| In the file | Meaning | Resolution |
|---|---|---|
| Run `start` | `int64`, epoch milliseconds | the absolute anchor |
| every other time | `int32`, milliseconds after `start` | `start + offset`, as an instant in UTC |

The signed 32-bit offset covers about **24.8 days**. Past that Gatling's own writer overflows, so the
format cannot represent a longer run. The run's `start` is bounded when it is read — non-negative,
and low enough that any offset stays inside an `int64` — so no resolved time can wrap; an offset that
would resolve before the run's start is reported as **absent** rather than as a wrapped value — the same refusal to invent a number the text
codec already makes for a duration it cannot compute (research [R3](./research.md)).

---

## 5. Strings

| Coder | Encoding | Bytes per character |
|---|---|---|
| `0` | Latin-1 | 1 |
| `1` | UTF-16, the writing JVM's native byte order | 2 |
| anything else | malformed | — |

The file records nothing about which byte order the writing JVM used. Little-endian is assumed,
documented, and unprovable from any corpus this project can record (research [R4](./research.md)).

---

## 6. What reaches a consumer

Unchanged from the text codec, which is the requirement rather than a coincidence.

| From this format | Wire record | Canonical model |
|---|---|---|
| Run | `gatling.Header` | `model.Run` |
| Request | `gatling.Record{Kind: KindRequest}` | `model.Item{Kind: ItemSample}` |
| User | `KindUser` | `ItemUser` |
| Group | `KindGroup` | `ItemGroup` |
| Error | `KindError` | `ItemError` |
| assertion blob | `Header`/`Run.Assertions` | carried verbatim |

A scenario index becomes a scenario **name** before it reaches either level: the index is a
representation detail of this format, and `model` never sees one.

**Capabilities**: expected identical to the text codec's, and asserted rather than assumed
(research [R10](./research.md)). Comparing the field lists, the binary format records the same things
and omits the same things.

---

## 7. Errors

All existing types; this feature adds none.

| Cause | Type | Carries |
|---|---|---|
| a malformed byte, length, coder or cache reference | `*gatling.SyntaxError` | the byte offset |
| version below the range, or not a release | `*gatling.VersionError` | the version and the range |
| version above the range under strictness | `*gatling.UnverifiedError` | the version and the range |
| the format is not this one | `*gatling.FormatError` | raised by detection, before this codec |

`SyntaxError.Line` names a line for a text log and a **byte offset** here. That the same field serves
both is a naming question for the contract, not a second type.
