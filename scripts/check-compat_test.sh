#!/usr/bin/env bash
#
# check-compat_test.sh — tests for check-compat.sh (constitution Principle V).
#
# Hermetic: every case feeds a synthetic gorelease report and a synthetic changelog, so
# nothing here compiles Go, resolves a module or touches the network.

set -uo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
script="$here/check-compat.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

pass=0; fail=0
ok()  { printf '  ✓ %s\n' "$1"; pass=$((pass + 1)); }
bad() { printf '  ✗ %s\n     %s\n' "$1" "$2"; fail=$((fail + 1)); }

clean="$tmp/clean.txt"
printf '# summary\nSuggested version: v0.0.10\n' > "$clean"

broken="$tmp/broken.txt"
cat > "$broken" <<'REPORT'
# github.com/galax-io/parsec/gatling
## incompatible changes
WithStrict: removed
## compatible changes
Strict: added

# github.com/galax-io/parsec/model
## compatible changes
Foo: added

# summary
Suggested version: v0.1.0
REPORT

recorded="$tmp/recorded.md"
cat > "$recorded" <<'MD'
# Changelog

## [Unreleased]

### Removed

- `gatling.WithStrict`, replaced by `gatling.Strict`.

## [0.0.9] - 2026-09-09

### Added

- something released
MD

heading_only="$tmp/heading-only.md"
cat > "$heading_only" <<'MD'
## [Unreleased]

### Removed

## [0.0.9] - 2026-09-09

### Removed

- an old entry that belongs to a shipped release
MD

# ---- an empty report is a broken gate, not a clean one --------------------------
: > "$tmp/empty.txt"
out=$(bash "$script" "$tmp/empty.txt" 2>&1); rc=$?
if [ "$rc" -eq 2 ] && grep -q "broken gate" <<<"$out"; then
  ok "an empty report exits 2 and says the gate did not run"
else
  bad "an empty report exits 2 and says the gate did not run" "rc=$rc out=$out"
fi

# ---- a file with no summary is not a report --------------------------------------
printf 'go: downloading something\n' > "$tmp/noise.txt"
out=$(bash "$script" "$tmp/noise.txt" 2>&1); rc=$?
if [ "$rc" -eq 2 ]; then
  ok "a report without '# summary' exits 2"
else
  bad "a report without '# summary' exits 2" "rc=$rc out=$out"
fi

# ---- nothing incompatible passes ------------------------------------------------
out=$(bash "$script" "$clean" 2>&1); rc=$?
if [ "$rc" -eq 0 ]; then
  ok "a report with no incompatible changes passes"
else
  bad "a report with no incompatible changes passes" "rc=$rc out=$out"
fi

# ---- an incompatible change without the label fails and names the identifier -----
out=$(bash "$script" "$broken" 2>&1); rc=$?
if [ "$rc" -eq 1 ] && grep -q "gatling: WithStrict: removed" <<<"$out" && ! grep -q "Foo" <<<"$out"; then
  ok "an incompatible change without --allow-breaking fails and names it, not the compatible ones"
else
  bad "an incompatible change without --allow-breaking fails and names it, not the compatible ones" "rc=$rc out=$out"
fi

# ---- labelled and recorded passes -------------------------------------------------
out=$(bash "$script" --allow-breaking --changelog "$recorded" "$broken" 2>&1); rc=$?
if [ "$rc" -eq 0 ] && grep -q "allowed" <<<"$out"; then
  ok "labelled breaking with a Removed bullet under [Unreleased] passes"
else
  bad "labelled breaking with a Removed bullet under [Unreleased] passes" "rc=$rc out=$out"
fi

# ---- labelled but only a released version records it: not this change's entry -----
out=$(bash "$script" --allow-breaking --changelog "$heading_only" "$broken" 2>&1); rc=$?
if [ "$rc" -eq 1 ] && grep -q "not recorded" <<<"$out"; then
  ok "labelled breaking with an empty heading under [Unreleased] fails — a shipped release's entry does not count"
else
  bad "labelled breaking with an empty heading under [Unreleased] fails — a shipped release's entry does not count" "rc=$rc out=$out"
fi

# ---- the report may arrive on stdin ------------------------------------------------
out=$(bash "$script" - < "$clean" 2>&1); rc=$?
if [ "$rc" -eq 0 ]; then
  ok "a report on stdin is read"
else
  bad "a report on stdin is read" "rc=$rc out=$out"
fi

# ---- a missing changelog with the label is a prerequisite error, not a verdict -----
out=$(bash "$script" --allow-breaking --changelog "$tmp/absent.md" "$broken" 2>&1); rc=$?
if [ "$rc" -eq 2 ]; then
  ok "a missing changelog exits 2"
else
  bad "a missing changelog exits 2" "rc=$rc out=$out"
fi

# ---- a baseline subtracts what the base branch already reports ---------------------
# gorelease compares against the last TAG, so from the moment a deliberate breaking
# change merges until the tag is cut, every later pull request inherits it. Without
# this the gate asks each of them for a label for somebody else's change — which is
# exactly what happened to the first pull request after the v0.1.0 freeze merged.
out=$(bash "$script" --baseline "$broken" "$broken" 2>&1); rc=$?
if [ "$rc" -eq 0 ] && grep -q "not this change" <<<"$out"; then
  ok "a change that adds no breakage of its own passes, and the inherited one is named"
else
  bad "a change that adds no breakage of its own passes, and the inherited one is named" "rc=$rc out=$out"
fi

# ---- but a NEW incompatible change is still the change's own ----------------------
more="$tmp/more.txt"
cat > "$more" <<'REPORT'
# github.com/galax-io/parsec/gatling
## incompatible changes
WithStrict: removed
Gate: removed

# summary
Suggested version: v0.1.0
REPORT
out=$(bash "$script" --baseline "$broken" "$more" 2>&1); rc=$?
# The verdict list is what follows "incompatible changes to the public API:" — the
# inherited one is also printed, in its own section, and that is not the list.
verdict=$(awk '/incompatible changes to the public API:/ {f=1; next} f && /^  / {print} f && !/^  / {exit}' <<<"$out")
if [ "$rc" -eq 1 ] && grep -q "Gate: removed" <<<"$verdict" && ! grep -q "WithStrict" <<<"$verdict"; then
  ok "an incompatible change the base does not have still fails, and only it is named"
else
  bad "an incompatible change the base does not have still fails, and only it is named" "rc=$rc out=$out"
fi

# ---- and it passes with the label, as before --------------------------------------
out=$(bash "$script" --allow-breaking --baseline "$broken" --changelog "$recorded" "$more" 2>&1); rc=$?
if [ "$rc" -eq 0 ]; then
  ok "a new incompatible change passes when labelled and recorded"
else
  bad "a new incompatible change passes when labelled and recorded" "rc=$rc out=$out"
fi

# ---- an empty or bogus baseline is a prerequisite error, not a pass ---------------
# A baseline the gate cannot read would silently subtract nothing — or everything.
: > "$tmp/empty.txt"
out=$(bash "$script" --baseline "$tmp/empty.txt" "$broken" 2>&1); rc=$?
if [ "$rc" -eq 2 ]; then
  ok "an empty baseline exits 2 rather than judging without it"
else
  bad "an empty baseline exits 2 rather than judging without it" "rc=$rc out=$out"
fi

printf '\n%d passed, %d failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
