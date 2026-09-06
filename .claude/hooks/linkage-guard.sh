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
is_tag=0
printf '%s' "$cmd" | grep -qE '\bgit[[:space:]]+tag[[:space:]]+v?[0-9]+\.[0-9]+\.[0-9]+' && is_tag=1
# A push is a release only when it carries a tag. Pushing a release/* branch was
# once treated as one too, which was wrong twice over: publishing is what the tag
# does — a branch push publishes nothing — and the version was then read out of
# the branch name. A release branch serves a whole minor line (release/1.2.0 ->
# v1.2.0, v1.2.1, …), so for every patch release the branch name and the tag
# version differ by design, and the guard went looking for a milestone that has
# never existed. It blocked the documented release procedure at its second step.
#
# The three patterns below still cover every way a tag reaches the remote:
# --tags, an explicit refs/tags/ ref, and a bare vX.Y.Z argument.
if printf '%s' "$cmd" | grep -qE '\bgit[[:space:]]+push\b' \
   && printf '%s' "$cmd" | grep -qE '(--tags|refs/tags/|[[:space:]](origin[[:space:]]+)?v[0-9]+\.[0-9]+\.[0-9]+([[:space:]]|$))'; then
  is_tag=1
fi
printf '%s' "$cmd" | grep -qE '\bgit[[:space:]]+(commit|log|show)\b' && is_tag=0
[ "$is_tag" = 1 ] || exit 0
[ -x "$checker" ] || block "checker missing ($checker) — release cannot be verified"
# The v is required. Tags here are always vX.Y.Z, while a bare X.Y.Z is as
# likely to be a branch name, a path or a pinned dependency — matching one of
# those is how the guard used to resolve the wrong milestone.
ver=$(printf '%s' "$cmd" | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -1)
[ -n "$ver" ] || block "tag/release push without explicit vX.Y.Z — verify: scripts/check-linkage.sh --for-tag <version>"
out=$("$checker" --for-tag "$ver" 2>&1) || block "$out"
exit 0
