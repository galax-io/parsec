#!/usr/bin/env bash
#
# e2e-inventory.sh — turn `go test -json` into the executed-case inventory, and
# refuse a run in which nothing executed.
# Run `scripts/e2e-inventory.sh --help` for full usage.

set -euo pipefail

usage() {
  cat <<'EOF'
e2e-inventory.sh — read `go test -json` on stdin and report which end-to-end cases ran.

  scripts/e2e-inventory.sh [--summary] < test.json

  --summary   also append the inventory to $GITHUB_STEP_SUMMARY

The count is of cases that EXECUTED, not cases that passed: a failing case still
registers, so a run with one failure reports it as one case and the failure is
attributed to that case rather than being reported as an empty run. That is what
keeps "no end-to-end case executed" meaning the corpus is missing, and never
doubling as the message for a case that ran and failed.

An empty inventory exits 1. This is the second reading of the same fact, not the
load-bearing one: `set -o pipefail` plus `go test`'s own status is what fails the
job for a failing test. This one also catches a suite that failed to build and so
never ran a case to have an opinion about.

Exit: 0 at least one case executed | 1 none did | 2 usage error.
EOF
}

SUMMARY=0
while [ $# -gt 0 ]; do
  case "$1" in
    --summary) SUMMARY=1; shift ;;
    --help|-h) usage; exit 0 ;;
    *)         echo "error: unknown argument '$1'" >&2; usage >&2; exit 2 ;;
  esac
done

command -v jq >/dev/null 2>&1 || { echo "error: 'jq' not found on PATH" >&2; exit 2; }

events="$(cat)"

# A case is a top-level test that reached a terminal action. Subtests carry a "/"
# and are the same case seen more often, so they are not counted twice.
inventory="$(jq -rs '
  map(select(.Action == "pass" or .Action == "fail")
      | select(.Test != null and (.Test | contains("/") | not))
      | {pkg: .Package, test: .Test, action: .Action})
  | group_by(.pkg)
  | map("  \(.[0].pkg)  cases=\(length)  failed=\(map(select(.action == "fail")) | length)")
  | .[]
' <<<"$events")"

count="$(jq -rs '
  map(select(.Action == "pass" or .Action == "fail")
      | select(.Test != null and (.Test | contains("/") | not)))
  | length
' <<<"$events")"

report="end-to-end cases executed: $count"
[ -n "$inventory" ] && report="$report"$'\n'"$inventory"

printf '%s\n' "$report"

if [ "$SUMMARY" -eq 1 ] && [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
  {
    printf '%s\n' '```text'
    printf '%s\n' "$report"
    printf '%s\n' '```'
  } >> "$GITHUB_STEP_SUMMARY"
fi

if [ "$count" -eq 0 ]; then
  echo "no end-to-end case executed: the corpus is missing, not optional" >&2
  exit 1
fi
