#!/usr/bin/env bash
#
# check-compat.sh — read a gorelease report and decide whether the change may merge.
# Run `scripts/check-compat.sh --help` for full usage.

set -euo pipefail

usage() {
  cat <<'USAGE'
check-compat.sh — hold a change to the public API contract (constitution Principle V).

gorelease compares the module against its last tagged version and lists every exported
identifier that was removed or changed under "## incompatible changes". Below major
version 1 it reports and does not fail — its exit status ignores incompatible changes by
design — so this script is where the report becomes a gate.

  no incompatible changes                       -> pass
  incompatible changes, no --allow-breaking      -> fail, naming them
  incompatible changes, --allow-breaking         -> pass only when CHANGELOG.md records the
                                                   change under [Unreleased] as ### Changed
                                                   or ### Removed
  an empty report, or one without "# summary"    -> exit 2: the gate did not run. An empty
                                                   report is a broken gate, not a clean one.

Usage:
  scripts/check-compat.sh [--allow-breaking] [--changelog FILE] REPORT

  REPORT               a gorelease report; "-" reads stdin
  --allow-breaking     the pull request carries the `breaking` label: a deliberate MINOR
                       bump, which Principle V permits with a changelog entry
  --changelog FILE     the changelog to check (default: CHANGELOG.md)

Writes a line to $GITHUB_STEP_SUMMARY when that is set.

Exit: 0 may merge | 1 must not | 2 usage error, or the report is not a report.
USAGE
}

ALLOW=0
CHANGELOG=CHANGELOG.md
REPORT=""

while [ $# -gt 0 ]; do
  case "$1" in
    --allow-breaking) ALLOW=1 ;;
    --changelog) CHANGELOG="${2:-}"; shift ;;
    -h|--help) usage; exit 0 ;;
    -) REPORT="-" ;;
    -*) echo "check-compat: unknown option $1" >&2; usage >&2; exit 2 ;;
    *) REPORT="$1" ;;
  esac
  shift
done

[ -n "$REPORT" ] || { usage >&2; exit 2; }

if [ "$REPORT" = "-" ]; then
  tmp="$(mktemp)"
  trap 'rm -f "$tmp"' EXIT
  cat > "$tmp"
  REPORT="$tmp"
fi

# incompatible prints "<package>: <change>" for every line gorelease filed under
# "## incompatible changes". A package header is "# <import path>"; "# summary" ends
# the per-package part of the report.
incompatible() {
  awk '
    /^# summary/               { pkg = ""; f = 0; next }
    /^# /                      { pkg = substr($0, 3); f = 0; next }
    /^## incompatible changes/ { f = 1; next }
    /^## /                     { f = 0; next }
    f && NF                    { print pkg ": " $0 }
  ' "$1"
}

# recorded succeeds when the changelog's [Unreleased] section has at least one bullet
# under a "### Changed" or "### Removed" heading. An entry under a released version is
# someone else's release; a heading with nothing under it records nothing.
recorded() {
  awk '
    /^## \[Unreleased\]/     { u = 1; next }
    /^## \[/                 { u = 0 }
    !u                       { next }
    /^### (Changed|Removed)/ { s = 1; next }
    /^### /                  { s = 0; next }
    s && /^- /               { found = 1 }
    END                      { exit found ? 0 : 1 }
  ' "$1"
}

note() { # note <markdown line> — to the job summary, when there is one
  if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
    printf '%s\n' "$1" >> "$GITHUB_STEP_SUMMARY"
  fi
}

if [ ! -s "$REPORT" ]; then
  echo "check-compat: the report is empty — the gate did not run, and an empty report is a broken gate, not a clean one" >&2
  exit 2
fi

if ! grep -q '^# summary' "$REPORT"; then
  echo "check-compat: the report carries no '# summary' line — this is not a gorelease report" >&2
  exit 2
fi

breaking="$(incompatible "$REPORT")"

if [ -z "$breaking" ]; then
  echo "check-compat: no incompatible change to the public API"
  note "**compat:** no incompatible API change against the base."
  exit 0
fi

echo "check-compat: incompatible changes to the public API:"
printf '%s\n' "$breaking" | sed 's/^/  /'

if [ "$ALLOW" -ne 1 ]; then
  cat >&2 <<'MSG'
check-compat: from v0.1.0 a change to an exported identifier is a breaking change
(constitution Principle V). A deliberate one is a MINOR bump: label the pull request
`breaking` and record the change in CHANGELOG.md under [Unreleased] as ### Changed or
### Removed. Without the label this gate fails on any incompatible change.
MSG
  note "**compat:** incompatible API change without the \`breaking\` label:"
  printf '%s\n' "$breaking" | sed 's/^/- /' | while IFS= read -r line; do note "$line"; done
  exit 1
fi

if [ ! -f "$CHANGELOG" ]; then
  echo "check-compat: $CHANGELOG not found" >&2
  exit 2
fi

if recorded "$CHANGELOG"; then
  echo "check-compat: allowed — the pull request is labelled breaking and $CHANGELOG records the change under [Unreleased]"
  note "**compat:** deliberate incompatible change, labelled and recorded in \`$CHANGELOG\`."
  exit 0
fi

echo "check-compat: labelled breaking, but $CHANGELOG has no bullet under ### Changed or ### Removed in [Unreleased] — the change is not recorded" >&2
note "**compat:** labelled \`breaking\` but not recorded in \`$CHANGELOG\` under [Unreleased]."
exit 1
