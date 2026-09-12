# Security policy

## What this module is exposed to

parsec decodes files it does not trust, in a process it does not own.

A `simulation.log` is an artefact of a load test, and a consumer of this library reads whichever one
it is handed: an upload to a server, an entry in an archive, a file a CI job produced. The binary
format carries length prefixes that decide how much memory is allocated, and a corrupt one is
indistinguishable from a large one until it is read. The module is built against that — allocation
ceilings, a bounded read buffer, a bounded string table, a version gate before anything else is
decoded — and it carries four fuzz targets (`FuzzDetect`, `FuzzDecode`, `FuzzReader`, `FuzzLastRun`)
and a nightly fuzz workflow because those are the defences most easily broken by a change that looks
harmless.

If you have found a way to make this module allocate without bound, spin, panic, or read memory it
should not, that is a vulnerability here even though this module opens no socket and holds no
credential.

## Reporting a vulnerability

**Use GitHub's private vulnerability reporting**: open
<https://github.com/galax-io/parsec/security/advisories/new>. It is private to the maintainers until
an advisory is published.

Please do not open a public issue for a vulnerability. A crasher for a decoder is a working exploit
for every consumer that has not upgraded yet.

Include, where you have it:

- the input that triggers it, or a fuzz corpus entry — this is the whole reproduction;
- the module version, from `go list -m github.com/galax-io/parsec`;
- what happens: an out-of-memory, a panic and its stack, a hang, a wrong decode.

**What to expect.** An acknowledgement within 7 days, and an assessment within 14. If it is
accepted, you will be told which release will carry the fix before it ships, and credited in the
advisory unless you ask otherwise.

## Supported versions

Fixes land on `main` and are cherry-picked onto the current release branch, so the supported set is
the latest minor line:

| Version | Supported |
|---|---|
| the latest `0.1.x` | yes |
| earlier `0.1.x` patches | upgrade to the latest patch |
| `0.0.x` | no — these predate the stable API, and upgrading is a `go get -u` |

A fix for an accepted vulnerability is released as a patch on that line. If a fix cannot be made
without a breaking change, it is released as a MINOR and the advisory says so.

## Scope

In scope: anything reachable by handing this module an artefact — a `simulation.log` of either
format, a `lastRun.txt`, a results directory. That includes memory exhaustion, unbounded CPU, a
panic, and a decode that silently reports numbers a file does not contain.

Out of scope: vulnerabilities in Gatling itself, in a consumer of this library, or in the CI of this
repository's forks. A `panic` from a deliberately malformed input is in scope; a `panic` from calling
an API in a way its documentation forbids is a bug — open an issue.
