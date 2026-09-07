# Contract 2 — the blocking-source contract

**Status**: new statement of behaviour that already holds. Measured on 2026-09-07 against all five
recordings, both codecs, with no source file changed (research [R6](../research.md)). Nothing here
asks for code; it asks for the promise to be written down and held by a test, so that the next
change to a buffer cannot remove it in silence.

**Audience**: the comet sidecar ([comet#3](https://github.com/galax-io/comet/issues/3)), and anyone
else following a log while it is being written.

## What this module promises

1. **A source whose `Read` blocks is supported.** The readers make no assumption that bytes are
   already there. A read that returns fewer bytes than asked for is a wait, not an end.
2. **A record split across separate reads is delivered exactly once**, when its last byte arrives,
   and is identical to what a whole-file read of the same log yields.
3. **The end of input is the caller's statement, taken as given.** This module never polls, reopens,
   stats or otherwise infers whether the writer is still alive — it has no way to know, and for a
   binary log cut on a record boundary no reader could know.
4. **Memory held while waiting is bounded** and does not grow with the records already delivered:
   the fixed read buffer, the deepest group nesting, and the capped table of distinct strings the
   log introduces.
5. **A source failure is reported as a failure**, including one that merely wraps `io.EOF`, and is
   never reported as the end of the log.

## What the follower must do

| Requirement | Why |
|---|---|
| Read from the first byte of the file | The binary codec rebuilds its string cache as it goes; a record can name a string introduced megabytes earlier. Starting anywhere else is silently wrong. |
| **Block** when there are no new bytes — never return `(0, nil)` in a loop | `(0, nil)` is legal and must not be read as an end, so a spinning source wedges a reader. Both guards that exist (`simlog.readHead`'s empty-read count, `bufio`'s own) end such a read with `io.ErrNoProgress` rather than hanging, but that is a backstop, not a supported mode. |
| Return `io.EOF` only when the follow is over | Any earlier `io.EOF` inside a record is a cut short, and this module will report it as one. |
| Make a blocked read returnable | Cancellation belongs to the follower: the reader is blocked inside `Read`, and only the source can end that. |
| Report a file that shrank or was replaced as an **error**, not as an end | This module cannot see it. An error is propagated with its cause; an `io.EOF` would be read as the log ending normally. |

## The one thing neither side can do

A binary log cut exactly on a record boundary is a shorter valid log. No reader can tell it from a
complete one, so the follower — which can see whether the writer is alive — owns that judgement
([comet#4](https://github.com/galax-io/comet/issues/4)).

## How it is held

A test that delivers each recording 300 bytes at a time from a source that blocks between appends,
and compares the result with a whole-file read of the same file, for every supported version and
both formats. It fails if any future change breaks promise 1, 2 or 4.
