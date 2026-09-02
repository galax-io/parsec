# parsec Constitution

parsec is the shared foundation for load-test results in Galaxio: one canonical model, one decoder per tool, one statistics engine. Three consumers depend on it — `galaxio-cli`, the Galaxio backend, and the comet sidecar — and none of them may need a second copy of this code.

**Version**: 1.0.0 · **Ratified**: 2026-09-02 · **Last amended**: 2026-09-02

## Core Principles

### I. The model is the contract

Every source converts its own artefacts into the types in `model`, and every consumer reads only those types. A source that cannot supply a field declares that through `Capabilities`; it never substitutes a zero, an empty string, or a guess. Adding a tool must not change what an existing tool reports.

### II. Decoders depend on the standard library only

`model` and `gatling` import nothing outside the standard library, and CI enforces it. The module as a whole allows one exception, a t-digest implementation for percentile estimation. Any further dependency is a recorded decision with its reasoning, not a convenience.

### III. Measurement truth

Successful and failed samples are accumulated separately, and a failure never contributes to a success statistic. Counts, minima, maxima, means and standard deviations are exact. Percentiles are estimates, documented as such, and the documentation states that they will not match a tool's own figures digit for digit. An event carries its own timestamp; the moment of processing is never substituted for it.

### IV. The log format is external

Gatling's `simulation.log` is undocumented, declared internal by its authors, and has already changed once. Every read is version-gated: below the supported range it is refused with a message naming both versions; above it, decoding proceeds with a warning, and refuses only when strictness is requested. A canary in CI runs against the newest Gatling release so the project learns about a format change before its users do.

### V. Golden corpus or it did not happen

Every decoder and statistics change is verified against recorded runs from real Gatling versions, covering both the text and the binary branch of the format. Expected statistics come from that run's own report, never from this library. Reading a file whole, in arbitrary chunks, and while it is still being written must all produce the same records.

### VI. Stability is declared, not assumed

Below v1.0.0 the public surface is what the README lists as stable; everything else is unexported or internal. A breaking change to that surface requires a minor bump and a changelog entry naming it. The supported Gatling range is part of the contract.

## Code Conventions

Go 1.25. `gofmt` and `gofumpt` clean, `go vet` and `golangci-lint` green, `go test -race -shuffle=on` on every commit. Standard-library testing with table-driven cases and golden files under `testdata`; no assertion library. `context.Context` is the first parameter where it applies. Constructors take functional options. `init()` is not used. Errors wrap with `%w` and are inspected with `errors.Is` and `errors.As`.

Coverage floor: 90 percent for decoder packages, 80 percent for the module.

## Quality Gates

CI runs formatting, tidiness of `go.mod`, vet, lint, race-enabled tests, and the standard-library-only check for `model` and `gatling`. A weekly canary decodes a run produced by the newest Gatling release. A red build is never merged.

## Governance

This constitution governs. A change to it is a pull request that states what changed and why, and bumps the version above: major for a removed or reversed principle, minor for a new one, patch for wording.

Work follows the organisation's spec-driven flow: a milestone states the problem, `/speckit.specify` turns it into a spec, and one issue becomes one commit and one pull request that closes it.
