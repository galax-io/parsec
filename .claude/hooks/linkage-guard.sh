#!/usr/bin/env bash
# PreToolUse(Bash) guard — gate ONLY release tagging. ~0 tokens. Normal push/merge untouched.
set -uo pipefail
input=$(cat)
cmd=$(printf '%s' "$input" | jq -r '.tool_input.command // ""' 2>/dev/null || true)
[ -n "$cmd" ] || exit 0
case "$cmd" in *"git "*) ;; *) exit 0 ;; esac
root="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
checker="$root/scripts/check-linkage.sh"
block() { printf 'BLOCKED by linkage-guard:\n%s\n' "$1" >&2; exit 2; }

# Split the command into the invocations it actually runs.
#
# The guard used to grep the whole command string, which cannot tell an
# invocation from the same words sitting inside an argument to something else.
# Filing the bug report about that was itself blocked, because the report quoted
# the failing command; testing the fix needed the version assembled at runtime.
# The workaround for a false positive is to phrase a command so the guard cannot
# recognise it — a habit of evading a safety check, learned by doing, that
# transfers to the case where the guard is right.
#
# So: split at separators that are OUTSIDE quotes, and stop at a heredoc, since
# everything after << is data being written rather than commands being run. A
# quoted body or a heredoc mentioning a release is then just text, while
# `make build && git push origin v1.2.3` is still two invocations and the second is still judged.
segments=$(printf '%s' "$cmd" | awk '
{
  seg = ""; inS = 0; inD = 0; n = length($0)
  for (i = 1; i <= n; i++) {
    c = substr($0, i, 1)
    if (inS)          { if (c == "\x27") inS = 0; seg = seg c; continue }
    if (inD)          { if (c == "\\") { seg = seg c substr($0, i+1, 1); i++; continue }
                        if (c == "\"") inD = 0; seg = seg c; continue }
    if (c == "\x27")  { inS = 1; seg = seg c; continue }
    if (c == "\"")    { inD = 1; seg = seg c; continue }
    if (c == "<" && substr($0, i+1, 1) == "<") { print seg; exit }
    if (c == ";" || c == "|" || c == "&") { print seg; seg = ""; continue }
    seg = seg c
  }
  print seg
}')

is_tag=0
ver=""
while IFS= read -r seg; do
  # Strip leading whitespace, then any VAR=value assignments, then judge the
  # command word. Anything that is not git runs no refs and is not a release.
  seg="${seg#"${seg%%[![:space:]]*}"}"
  while printf '%s' "$seg" | grep -qE '^[A-Za-z_][A-Za-z0-9_]*='; do
    seg=$(printf '%s' "$seg" | sed -E 's/^[A-Za-z_][A-Za-z0-9_]*=("[^"]*"|'"'"'[^'"'"']*'"'"'|[^[:space:]]*)[[:space:]]*//')
  done
  printf '%s' "$seg" | grep -qE '^([^[:space:]]*/)?git([[:space:]]|$)' || continue

  # A tag is created, or a push carries one. A release branch push is not a
  # release: publishing is what the tag does (#46).
  if printf '%s' "$seg" | grep -qE '^([^[:space:]]*/)?git[[:space:]]+tag[[:space:]]+v?[0-9]+\.[0-9]+\.[0-9]+'; then
    is_tag=1
  elif printf '%s' "$seg" | grep -qE '\bpush\b' \
    && printf '%s' "$seg" | grep -qE '(--tags|refs/tags/|[[:space:]](origin[[:space:]]+)?v[0-9]+\.[0-9]+\.[0-9]+([[:space:]]|$))'; then
    is_tag=1
  else
    continue
  fi

  # The v is required. A bare X.Y.Z is as likely to be a branch name, a path or
  # a pinned dependency, and matching one is how the guard resolved the wrong
  # milestone for a release branch push (#46).
  [ -n "$ver" ] || ver=$(printf '%s' "$seg" | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -1)
done <<< "$segments"

[ "$is_tag" = 1 ] || exit 0
[ -x "$checker" ] || block "checker missing ($checker) — release cannot be verified"
[ -n "$ver" ] || block "tag/release push without explicit vX.Y.Z — verify: scripts/check-linkage.sh --for-tag <version>"
out=$("$checker" --for-tag "$ver" 2>&1) || block "$out"
exit 0
