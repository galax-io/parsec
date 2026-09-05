# Sample, not a corpus entry: Gatling 3.15.1 binary simulation.log

**`3.15.1-head.bin` is a 64-byte fragment. It is not a recording of a run, and nothing may compare
a decoder against it.** It holds no complete run, no report and no statistics; it exists to prove
one thing only — that a binary `simulation.log` is identified as binary (spec 004, FR-031a). The
complete binary recordings, with everything a later comparison needs, belong to milestone v0.0.5
alongside the codec they prove.

## Provenance

| Fact | Value |
|---|---|
| Captured | 2026-09-05 |
| Gatling version | 3.15.1 |
| Source | a throwaway one-request simulation written for this capture, **not** the corpus probe |
| Build | `sbt -batch "Gatling/testOnly BinProbe"`, `gatling-sbt` 4.19.2, Scala 2.13.18 |
| Machine | macOS (Darwin 25.6.0), arm64 |
| JVM | Homebrew OpenJDK 17.0.10 |
| Whole log | 154 bytes; this file is its first 64 |

## Why not the corpus probe

`testdata/corpus/gatling/simulation/` pins `gatling-picatinny`, which renders the probe's OpenNFR
requirements into Gatling assertions. Picatinny has **no release targeting the 3.14.x or 3.15.x
line** — the artefact-per-line table reads `none` — so the probe cannot run under 3.15.1 at all.
A throwaway simulation with no picatinny was used instead; the sample needs a real binary log, not
that probe in particular.

## Why 64 bytes and not more

The plan said 256. The whole log turned out to be 154 bytes, so 256 would have been the entire file
— exactly the shape FR-031a forbids this sample from having, because a complete log invites being
read as a recording. 64 bytes is six times what detection examines and still unmistakably a
fragment.

## What the bytes showed

The leading bytes settle the claim spec 004 carried unverified. Issue #6 read the layout as records
opening with a kind byte, `0` for the run record; nothing in this repository had ever read a binary
log to check.

```
00 | 00 00 00 06 | 33 2e 31 35 2e 31 | 00 | 00 00 00 08 | 42 69 6e 50 72 6f 62 65 | ...
^^   ^^^^^^^^^^^   "3.15.1"            ^^                 "BinProbe"
kind length 6                          coder
```

- **The first byte is `0x00`.** That is all `gatling.Detect` needs, and it is now evidence rather
  than a reading of an issue.
- A string is a 4-byte big-endian length, the bytes, then a coder byte — not the shape issue #6
  described in prose. Recorded here for v0.0.5; nothing in this milestone decodes it.
- The version string sits at offset 5, so reading a binary log's version would be easy. This
  milestone still does not (FR-031b), and the reason is evidence rather than difficulty: one sample
  from 3.15.1 proves nothing about 3.13.5 or 3.14.9, and Principle II binds a codec's range to its
  corpus.

## What 3.15.1 does and does not generate

The run produced an HTML report — `index.html`, `js/`, `style/` — but **no `js/global_stats.json`
and no `js/stats.json`**, confirming the constitution's account that Gatling stopped generating
them in 3.13.5. `js/stats.js` exists but is interface code, not data, so it is not a substitute.
Milestone v0.0.5 will have to take its tolerance figures from somewhere other than those two files,
and that is worth settling before its corpus is recorded, because a run archived without its
numbers can never prove a decoder's afterwards.
