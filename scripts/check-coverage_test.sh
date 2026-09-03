#!/usr/bin/env bash
#
# check-coverage_test.sh — tests for check-coverage.sh (constitution Principle III).
#
# Hermetic: every case feeds a synthetic coverage profile and an explicit package
# list, so nothing here compiles Go, reads the module or touches the network.

set -uo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
script="$here/check-coverage.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

pass=0; fail=0
ok()  { printf '  ✓ %s\n' "$1"; pass=$((pass + 1)); }
bad() { printf '  ✗ %s\n     %s\n' "$1" "$2"; fail=$((fail + 1)); }

# profile writes a coverage profile from "file numStmt count" triples.
profile() {
  local path="$1"; shift
  echo "mode: set" > "$path"
  local n=1
  for spec in "$@"; do
    set -- $spec
    echo "$1:$n.1,$((n + 1)).2 $2 $3" >> "$path"
    n=$((n + 2))
  done
}

M=github.com/galax-io/parsec

# ---- a package with no statements reports n/a, never 0% ---------------------
# A 0% reading and "there was nothing to measure" are different facts, and only
# one of them is a reason to fail a build.
p="$tmp/empty.out"
profile "$p" "$M/gatling/text/parse.go 10 1"
out=$(bash "$script" --packages "$M/gatling/text $M/model" "$p" 2>&1); rc=$?

if grep -qE "$M/model[^0-9]+n/a" <<<"$out"; then
  ok "a package with no statements reports n/a"
else
  bad "a package with no statements reports n/a" "got: $(grep "$M/model" <<<"$out" || echo '<no row>')"
fi

if grep -qE "$M/model[^%]*0(\.0)?%" <<<"$out"; then
  bad "n/a is not reported as 0%" "the model row reads as a zero percentage"
else
  ok "n/a is not reported as 0%"
fi

if [ "$rc" -eq 0 ]; then
  ok "a package with no statements does not fail the run"
else
  bad "a package with no statements does not fail the run" "exit $rc"
fi

# ---- --enforce fails a decoder package below its floor ----------------------
p="$tmp/low.out"
profile "$p" "$M/gatling/text/parse.go 10 1" "$M/gatling/text/scan.go 10 0"   # 50%
bash "$script" --enforce --packages "$M/gatling/text" "$p" >"$tmp/low.log" 2>&1; rc=$?

if [ "$rc" -ne 0 ]; then
  ok "--enforce fails a package below the 90% floor"
else
  bad "--enforce fails a package below the 90% floor" "exit 0 on 50% coverage"
fi

if grep -q "90" "$tmp/low.log"; then
  ok "the refusal names the floor it applied"
else
  bad "the refusal names the floor it applied" "$(tail -2 "$tmp/low.log")"
fi

# ---- the same input without --enforce reports and exits 0 -------------------
# The gate is report-only until a maintainer decides otherwise, so the identical
# input must be a green run that merely states the number.
bash "$script" --packages "$M/gatling/text" "$p" >"$tmp/report.log" 2>&1; rc=$?

if [ "$rc" -eq 0 ]; then
  ok "without --enforce the same input exits 0"
else
  bad "without --enforce the same input exits 0" "exit $rc"
fi

if grep -q "50" "$tmp/report.log"; then
  ok "report-only still prints the percentage"
else
  bad "report-only still prints the percentage" "$(cat "$tmp/report.log")"
fi

# ---- a package at or above its floor passes under --enforce ----------------
p="$tmp/high.out"
profile "$p" "$M/gatling/text/parse.go 19 1" "$M/gatling/text/scan.go 1 0"    # 95%
if bash "$script" --enforce --packages "$M/gatling/text" "$p" >/dev/null 2>&1; then
  ok "--enforce passes a package above its floor"
else
  bad "--enforce passes a package above its floor" "95% was refused"
fi

# ---- the overall floor is 80%, not the per-package 90% ---------------------
# A non-decoder package sits under the module floor only; holding it to 90%
# would make the two floors one and quietly raise the looser of them.
p="$tmp/overall.out"
profile "$p" "$M/internal/tool/run.go 17 1" "$M/internal/tool/x.go 3 0"       # 85%
if bash "$script" --enforce --packages "$M/internal/tool" "$p" >/dev/null 2>&1; then
  ok "a non-decoder package is held to the 80% floor, not 90%"
else
  bad "a non-decoder package is held to the 80% floor, not 90%" "85% was refused"
fi

# ---- a block repeated across test binaries is merged, not summed -----------
# go test -coverpkg=./... emits every block once per test binary that could have
# reached it, so a block covered by one package's tests appears uncovered under
# each of the others. Summing those duplicates reads as a fraction of the real
# figure and fails a build that should pass.
p="$tmp/dup.out"
{
  echo "mode: set"
  echo "$M/gatling/text/parse.go:1.1,2.2 10 1"
  echo "$M/gatling/text/parse.go:1.1,2.2 10 0"
  echo "$M/gatling/text/parse.go:1.1,2.2 10 0"
} > "$p"

out=$(bash "$script" --packages "$M/gatling/text" "$p" 2>&1)
if grep -q "100.0%" <<<"$out"; then
  ok "a block repeated across binaries counts once, covered"
else
  bad "a block repeated across binaries counts once, covered" "got: $out"
fi

# ---- a missing profile is a usage error, not a silent pass -----------------
bash "$script" "$tmp/nope.out" >/dev/null 2>&1; rc=$?
if [ "$rc" -eq 2 ]; then
  ok "a missing profile exits 2"
else
  bad "a missing profile exits 2" "exit $rc"
fi

printf '\n%d passed, %d failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
