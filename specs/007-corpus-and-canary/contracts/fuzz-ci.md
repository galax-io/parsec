# Contract 4 — the fuzz legs

**Status**: new. Enforces the constitution's Principle II — *a decoder MUST NOT panic on any input* —
which today no workflow exercises (spec User Story 3, [parsec#60](https://github.com/galax-io/parsec/issues/60)).

## Two legs

| Leg | Trigger | Budget per target | On a finding |
|---|---|---|---|
| `fuzz` | every pull request — **and only a pull request** | bounded, starting point **90 s** | job fails, crasher uploaded |
| `fuzz-nightly` | schedule | several minutes | same, plus the crasher is the artefact to open an issue with |

The pull-request leg is what #60 requires: nightly alone would have let the `math.MinInt32` defect
merge and be found the next morning.

## Where the leg lives, and why it must not gate a release

`verify.yml` is reached by `workflow_call` from **both** paths — `ci.yml` on push and pull request,
and `release.yml` on a `v*.*.*` tag. A job added there therefore runs on releases too, and **fuzzing
must not**: it is nondeterministic, so the same tree can pass one run and fail the next on a finding
that depends on the seed. The constitution's release rules make that expensive — a tag must not be
deleted once the deployment has started, and a version number is never reused — so a release blocked
by a chance finding has no cheap way out.

Moving the job to `ci.yml` is the wrong fix: `verify.yml`'s own header states that a gate is added
there **and nowhere else**, and `ci.yml` states that its `gates` job carries the only `if:` on the
gate set. Both rules stay intact by giving `verify.yml` an input:

```yaml
# .github/workflows/verify.yml
on:
  workflow_call:
    inputs:
      fuzz:
        description: >-
          Run the fuzzing leg. Pull requests only: fuzzing is nondeterministic, and a
          release must not be gated on a finding that depends on the seed.
        type: boolean
        default: false
```

```yaml
# .github/workflows/ci.yml — the gates job
    uses: ./.github/workflows/verify.yml
    with:
      fuzz: ${{ github.event_name == 'pull_request' }}
```

`release.yml` needs **no change**: the default is `false`, so a release cannot turn the leg on by
omission. The `fuzz-targets` and `fuzz` jobs carry `if: inputs.fuzz`.

A push to `main` does not fuzz either. That is intended — the pull request that produced the merge
was fuzzed, and FR-013 asks for the pull request.

## Target discovery

Targets are **discovered, never listed** (spec FR-013):

```bash
go test -list '^Fuzz' ./...
```

Its output interleaves package lines with target names; the matrix is generated from it. A fuzz
target added tomorrow is fuzzed tomorrow — a hard-coded list would silently stop covering it.

Three targets exist today:

| Package | Target |
|---|---|
| `github.com/galax-io/parsec/gatling` | `FuzzDetect` |
| `github.com/galax-io/parsec/gatling/binary` | `FuzzDecode` |
| `github.com/galax-io/parsec/gatling/text` | `FuzzReader` |

## Invocation

```bash
go test -run '^$' -fuzz '^<Target>$' -fuzztime <budget> ./<package>/
```

- `-run '^$'` — without it the ordinary tests run first and spend the budget.
- `-fuzz` takes **one** target in **one** package per invocation. That is what makes this a matrix.
- **One job per target.** Measured here, a single fuzzer saturates every core it is given (862% CPU
  on a ten-core machine). Three fuzzers sharing a four-core runner would each receive a third of the
  budget the flag appears to grant.

## Budget

90 s is a starting point, from #60's report that `FuzzDecode` finds the `math.MinInt32` crasher in
about ninety seconds. **It is confirmed on the runner, not inferred** (spec FR-014): a ten-core
machine here reached 4.97 M executions in 30 s; a four-core runner will not.

The acceptance test is behavioural, not a number: with the v0.0.5 `MinInt32` fix reverted, the
pull-request leg must fail within its budget. With the fix in place, it must pass.

## Crashers

`go test` writes a failing input to `testdata/fuzz/<Target>/<name>` inside the package directory and
fails the run.

| Requirement | How |
|---|---|
| preserved as a downloadable artefact (FR-015) | `testdata/fuzz/**` uploaded on failure |
| fails the job (FR-015) | the non-zero exit is the job's |
| never committed (FR-016) | the job has no write permission and the workflow grants none |
| the generated corpus stays out of the tree (FR-016) | Go writes it to `$GOCACHE/fuzz`, never to the repository |

## Relationship to the existing `test` job

`go test -race -shuffle=on ./...` already runs every fuzz target's **seed corpus** as ordinary tests.
That is unchanged, and it is not fuzzing: it replays committed inputs. These legs are the part that
generates new ones.
