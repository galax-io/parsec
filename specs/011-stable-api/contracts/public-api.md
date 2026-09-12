# Contract 1 — the public API, frozen

**Applies to**: every exported identifier in `model`, `gatling`, `gatling/text`, `gatling/binary`,
`gatling/simlog` and `gatling/run`.
**Status**: this is the freeze. Issues [#13](https://github.com/galax-io/parsec/issues/13) (the
promise) and [#107](https://github.com/galax-io/parsec/issues/107) (its concrete scope),
[#77](https://github.com/galax-io/parsec/issues/77) (one `Tool`).

## The promise

From the `v0.1.0` tag:

- Changing the signature or the observable behaviour of any identifier listed below, or any format
  this module writes, is a **breaking change**. It is called out in a spec, approved before
  implementation, recorded in `CHANGELOG.md`, and released as a new **MINOR** version while the
  module is at v0.x.
- A superseded identifier keeps working for at least one MINOR release and carries a
  `// Deprecated:` comment naming its replacement. Removal without that window is available only
  below v0.1.0 — which is the window this feature works in, and it closes at the tag.
- Nothing outside the list is public. `internal/` is not importable and carries no promise.
- The supported Gatling range is part of the promise: **3.11.5 through 3.12.0** for the text format,
  **3.13.1 through 3.15.1** for the binary one. A 3.13.0 log is refused, although the codec could
  read it: that version writes the format and cannot generate a report, so no run of it can carry the
  second account of its own numbers a corpus entry needs (Principle III). Read the range from
  `simlog.Supported()` rather than from prose.
- `simlog.RecordReader` and `simlog.RunReader` are frozen **two-sided**: their method sets are final,
  because consumers' test doubles implement them and an added method breaks an implementer.

## What this feature withdraws, before the tag

| Identifier | Why it is not frozen | Changelog |
|---|---|---|
| `gatling.Gate` | its only production caller is `Policy.Apply`, documented as "the single place the outcomes are decided"; a second exported entry to one rule contradicts that | **Removed** |
| `gatling.MaxRunStart` | read only by the two codecs; no consumer computes with it. Moves to `internal/wire` | **Removed** |
| `gatling/text.Tool` | one value, two names, neither placement the convention | **Removed** |
| `gatling/binary.Tool` | as above | **Removed** |
| — | `gatling.Tool` replaces both, beside `Header` and `Version` | **Added** |

Each carries the `breaking` label and an `[Unreleased]` bullet, or `scripts/check-compat.sh` refuses
the merge.

## The three decisions #13 left open

| Question | Answer | Where it is stated |
|---|---|---|
| `UnsupportedFormatError` — keep or remove? | **Keep, and give it producers.** #84 needs an error meaning "a Gatling log this reader does not decode"; overloading `FormatError` would make a consumer's "not a Gatling file" branch wrong for every wrong-codec case | the type's doc comment; [wrong-format.md](./wrong-format.md) |
| `SyntaxError` — three position fields or one? | **Three, as they are.** Folding `Line` and `Offset` into one loses the type-level distinction, and `Format` is the discriminator two legitimately-zero positions need | `SyntaxError`'s doc comment |
| `Record.Line` = 0 for a binary log — contract or omission? | **The contract.** A binary record's offset serves no seek — the format cannot be resumed — and only a failure needs a position, which `SyntaxError.Offset` carries. An `Offset` field can be added compatibly later if a need appears | `Record.Line`'s doc comment |

## How the list is kept honest

`testdata/api/surface.txt` holds this list, and `api_test.go` regenerates it with `go/parser` and
fails on any difference. `gorelease` (through `scripts/check-compat.sh`) decides whether a change is
*compatible*; the golden file decides whether the surface is *what this contract says*. An addition
passes the first and fails the second, which is why both exist.

## The frozen surface — 275 identifiers

Kinds: `func` package-level function · `method T.M` method on an exported type · `imethod I.M`
interface method · `type` · `const` · `var` · `field T.F` exported struct field.

```text
## model  (111)
    const FieldConnectTiming
    const FieldDNSTiming
    const FieldGroupCumulatedDuration
    const FieldGroupDuration
    const FieldGroupOutcome
    const FieldIntervalSeries
    const FieldRequirements
    const FieldSampleBytesReceived
    const FieldSampleBytesSent
    const FieldSampleDuration
    const FieldSampleFailureType
    const FieldSampleResponseCode
    const FieldSampleScenario
    const FieldSampleUserIdentity
    const FieldTLSTiming
    const FieldUnknown
    const ItemAssertion
    const ItemError
    const ItemGroup
    const ItemSample
    const ItemUnknown
    const ItemUser
    const OutcomeFailure
    const OutcomeSuccess
    const OutcomeUnknown
    const PositionGroup
    const PositionSample
    const PositionUnknown
    const UserEnd
    const UserEventUnknown
    const UserStart
    field Failure.Message
    field Failure.Type
    field GroupSample.CumulatedDuration
    field GroupSample.Duration
    field GroupSample.Groups
    field GroupSample.Outcome
    field GroupSample.Start
    field Item.Assertion
    field Item.Error
    field Item.Group
    field Item.Kind
    field Item.Sample
    field Item.User
    field Run.Assertions
    field Run.Capabilities
    field Run.Description
    field Run.ID
    field Run.Name
    field Run.Start
    field Run.Tool
    field Run.ToolVersion
    field Run.Warnings
    field RunError.At
    field RunError.Message
    field Sample.BytesReceived
    field Sample.BytesSent
    field Sample.Duration
    field Sample.Failure
    field Sample.Groups
    field Sample.Name
    field Sample.Outcome
    field Sample.ResponseCode
    field Sample.Scenario
    field Sample.Start
    field UserEvent.At
    field UserEvent.Kind
    field UserEvent.Scenario
    field Warning.Reason
    field Warning.Version
    func FieldsKnown
    func NewCapabilities
    func NewGroupPosition
    func NewSamplePosition
    func Some
    method Bounds.End
    method Bounds.Extend
    method Bounds.Start
    method Capabilities.Absent
    method Capabilities.Provides
    method Field.String
    method GroupSample.Position
    method ItemKind.String
    method Opt.Get
    method Opt.IsSet
    method Outcome.String
    method Position.Groups
    method Position.Kind
    method Position.Name
    method Position.String
    method PositionKind.String
    method Sample.Position
    method UserEventKind.String
    method Warning.String
    type Bounds
    type Capabilities
    type Failure
    type Field
    type GroupSample
    type Item
    type ItemKind
    type Opt
    type Outcome
    type Position
    type PositionKind
    type Run
    type RunError
    type Sample
    type UserEvent
    type UserEventKind
    type Warning
## gatling  (106)
    const AbsentTimestamp
    const DetectSize
    const EventEnd
    const EventStart
    const EventUnknown
    const FormatBinary
    const FormatText
    const FormatUnknown
    const KindAssertion
    const KindError
    const KindGroup
    const KindRequest
    const KindRun
    const KindUnknown
    const KindUser
    const StatusKO
    const StatusOK
    const StatusUnknown
    const Tool
    const VerdictAccepted
    const VerdictRefused
    const VerdictUnknown
    const VerdictUnverified
    field FormatError.Head
    field FormatError.Short
    field Header.Description
    field Header.RunID
    field Header.SimulationClass
    field Header.Start
    field Header.Version
    field Policy.Max
    field Policy.Min
    field Record.CumulatedResponseTime
    field Record.End
    field Record.Event
    field Record.Groups
    field Record.Kind
    field Record.Line
    field Record.Message
    field Record.Name
    field Record.Payload
    field Record.Scenario
    field Record.Start
    field Record.Status
    field Record.Timestamp
    field SyntaxError.Expected
    field SyntaxError.Format
    field SyntaxError.Found
    field SyntaxError.Line
    field SyntaxError.Offset
    field TruncationError.Dropped
    field TruncationError.Expected
    field TruncationError.Format
    field TruncationError.Line
    field TruncationError.Offset
    field UnsupportedFormatError.Format
    field UnsupportedFormatError.Head
    field UnverifiedError.Max
    field UnverifiedError.Min
    field UnverifiedError.Version
    field Version.Major
    field Version.Minor
    field Version.Patch
    field VersionError.Found
    field VersionError.Max
    field VersionError.Min
    field VersionError.Parsed
    field VersionError.Version
    field Warning.Max
    field Warning.Min
    field Warning.Version
    func Detect
    func ParseVersion
    func WithStrict
    method Event.String
    method Format.String
    method FormatError.Error
    method Kind.String
    method Policy.Apply
    method Status.String
    method SyntaxError.Error
    method TruncationError.Error
    method UnsupportedFormatError.Error
    method UnverifiedError.Error
    method Verdict.String
    method Version.Compare
    method Version.String
    method VersionError.Error
    method Warning.String
    type Event
    type Format
    type FormatError
    type Header
    type Kind
    type Option
    type Policy
    type Record
    type Status
    type SyntaxError
    type TruncationError
    type UnsupportedFormatError
    type UnverifiedError
    type Verdict
    type Version
    type VersionError
    type Warning
## gatling/text  (13)
    const MaxLineLen
    func Capabilities
    func NewReader
    func NewRunReader
    func SupportedVersions
    method Reader.Assertions
    method Reader.Header
    method Reader.Next
    method Reader.Warnings
    method RunReader.Next
    method RunReader.Run
    type Reader
    type RunReader
## gatling/binary  (13)
    const MaxStringLen
    func Capabilities
    func NewReader
    func NewRunReader
    func SupportedVersions
    method Reader.Assertions
    method Reader.Header
    method Reader.Next
    method Reader.Warnings
    method RunReader.Next
    method RunReader.Run
    type Reader
    type RunReader
## gatling/simlog  (16)
    field Support.Format
    field Support.Newest
    field Support.Oldest
    field Support.Readable
    func NewReader
    func NewRunReader
    func Supported
    imethod RecordReader.Assertions
    imethod RecordReader.Header
    imethod RecordReader.Next
    imethod RecordReader.Warnings
    imethod RunReader.Next
    imethod RunReader.Run
    type RecordReader
    type RunReader
    type Support
## gatling/run  (16)
    const DefaultResultsRoot
    const FoundByLastRun
    const FoundByNewest
    const FoundByPath
    const FoundByUnknown
    field Location.Dir
    field Location.Found
    field Location.Log
    field NotFoundError.Dir
    func Find
    method FoundBy.String
    method NotFoundError.Error
    type FoundBy
    type Location
    type NotFoundError
    var ErrNoPath
TOTAL 275
```

Counted from the tree at `0a55f09` (278) with this feature's four withdrawals and one addition
applied. The figure that binds is the one `api_test.go` reports at the tag.
