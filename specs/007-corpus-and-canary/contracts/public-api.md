# Contract 1 — `gatling/binary.MaxStringLen`

**Status**: **approved 2026-09-06.** The sign-off spec FR-028 and AGENTS.md (*Ask first: changing
public API signatures / observable behavior*) require was given on the measurements in
[research R8](../research.md), after they were presented with the alternative below.

## The change

```go
// before
const MaxStringLen = 8 << 20 // 8 MiB

// after
const MaxStringLen = 1 << 20 // 1 MiB
```

Signature unchanged: still an untyped integer constant, still exported, still the ceiling on one
string or assertion payload in bytes.

## What a caller observes

| Field size on the wire | Before | After |
|---|---|---|
| ≤ 1 MiB | decodes | decodes |
| 1 MiB … 8 MiB | decodes | **refused** — `*gatling.SyntaxError` carrying the offset |
| > 8 MiB | refused | refused |

A caller reading `binary.MaxStringLen` gets a different number. A caller decoding a real Gatling log
sees no change: the longest field in the corpus is a 51-byte assertion payload, and Gatling truncates
failure messages far below either ceiling.

## Why

The exported ceiling is the only lever that keeps the module's documented peak-memory bound true. At
8 MiB the bound is false — measured, not argued:

| Ceiling | Worst peak, one field of each encoding in one log | Documented budget |
|---:|---:|---|
| 8 MiB | 52.3 MiB | 32 MiB — **fails** |
| 1 MiB | 6.8 MiB | 32 MiB — holds |

Full measurements, method and the rejection of 2 MiB and 4 MiB are in [research R8](../research.md).

The constant's stated purpose is unchanged and unimpaired: it exists so that a corrupt length prefix
cannot ask the allocator for gigabytes. 1 MiB refuses that as flatly as 8 MiB does.

## Compatibility handling

- **Principle V, pre-v0.1.0**: exported identifiers may change between releases, and every such
  change is recorded. A `CHANGELOG.md` **Changed** entry lands in the same PR.
- **No deprecation window applies** — the identifier is not superseded, its value is corrected.
- **Doc comments restated in the same change**: `MaxStringLen`'s own comment, which today explains
  that decoding one field at the ceiling costs a small multiple of it, and the memory paragraph on
  `Reader`, which states the budget. After this change the multiple is stated as measured and both
  places name the same 32 MiB budget (FR-026).
- **Downstream**: galaxio-cli, the comet sidecar and the Galaxio backend consume decoded records;
  none is known to read this constant. The change is called out in the release notes regardless.

## Alternative considered and rejected

Keep 8 MiB and restate the documented budget to the measured worst case — at least 56 MiB to leave
any margin. FR-028 permits this outcome; it was put alongside the recommendation and not taken. The
budget is the number consumers size a process against, and raising it by 75% to accommodate a field
no real log contains trades a real guarantee for an unused one.
