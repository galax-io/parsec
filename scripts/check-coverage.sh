#!/usr/bin/env bash
#
# check-coverage.sh — per-package coverage from one profile, against the floors.
# Run `scripts/check-coverage.sh --help` for full usage.

set -euo pipefail

usage() {
  cat <<'EOF'
check-coverage.sh — build the per-package coverage table from a Go coverage profile
and, with --enforce, hold each package to its floor.

Floors (constitution Principle III):
  decoder packages (model/, gatling/...)   90%
  the module overall                       80%
Every other package is reported, never enforced: the module floor already covers it,
and a second per-package floor would silently become the stricter of the two.

A package with no statements reports n/a, not 0%. "Nothing was measured" and "nothing
was covered" are different facts and only one of them is a reason to fail a build.

Usage:
  scripts/check-coverage.sh [--enforce] [--packages "p1 p2 ..."] [profile]

  --enforce            fail when a package or the module is below its floor
  --packages "..."     packages to report (default: go list ./...)
  profile              a go test -coverprofile file (default: cover.out)

Writes a markdown table to $GITHUB_STEP_SUMMARY when that is set.

Exit: 0 floors hold (or reporting only) | 1 a floor was breached | 2 usage/prereq error.
EOF
}

ENFORCE=0
PACKAGES=""
PROFILE=""

while [ $# -gt 0 ]; do
  case "$1" in
    --enforce)  ENFORCE=1; shift ;;
    --packages) PACKAGES="${2-}"; shift 2 ;;
    --help|-h)  usage; exit 0 ;;
    -*)         echo "error: unknown option '$1'" >&2; usage >&2; exit 2 ;;
    *)          PROFILE="$1"; shift ;;
  esac
done

PROFILE="${PROFILE:-cover.out}"
[ -f "$PROFILE" ] || { echo "error: coverage profile '$PROFILE' not found" >&2; exit 2; }

if [ -z "$PACKAGES" ]; then
  command -v go >/dev/null 2>&1 || { echo "error: 'go' not found on PATH and --packages not given" >&2; exit 2; }
  PACKAGES="$(go list ./... 2>/dev/null)"
fi
[ -n "$PACKAGES" ] || { echo "error: no packages to report" >&2; exit 2; }

# Aggregate the profile into "package statements covered". A profile line is
#   <import path>/<file>.go:<start>,<end> <numStmt> <count>
# so the package is the path with the file name removed. Blocks are counted, not
# lines: that is what "% of statements" means and what the floors are written in.
#
# A block is keyed and counted once however often it appears. Under -coverpkg=./...
# every test binary emits an entry for every block it could have reached, so one
# block covered by one package's tests also appears uncovered under each of the
# others; summing the duplicates instead of merging them reads as a fraction of the
# real figure, which is exactly the direction that fails a build wrongly.
counts="$(awk '
  NR == 1 && $0 ~ /^mode:/ { next }
  NF == 3 {
    if (!($1 in stmt)) { stmt[$1] = $2; hit[$1] = 0 }
    if ($3 + 0 > 0) hit[$1] = 1
  }
  END {
    for (k in stmt) {
      split(k, a, ":")
      file = a[1]
      sub(/\/[^\/]*$/, "", file)
      stmts[file] += stmt[k]
      if (hit[k]) covered[file] += stmt[k]
    }
    for (p in stmts) printf "%s %d %d\n", p, stmts[p], covered[p]
  }
' "$PROFILE")"

# floor_for prints the per-package floor, or 0 when the package is report-only.
# The match is on a path segment so that a package named "mymodel" is not mistaken
# for one under model/.
floor_for() {
  case "/$1/" in
    */model/*|*/gatling/*) echo 90 ;;
    *)                     echo 0  ;;
  esac
}

summary() { [ -n "${GITHUB_STEP_SUMMARY:-}" ] && printf '%s\n' "$1" >> "$GITHUB_STEP_SUMMARY"; return 0; }

summary "| package | coverage | floor |"
summary "|---|---|---|"

total_stmts=0
total_covered=0
violations=0

for pkg in $PACKAGES; do
  read -r stmts covered <<<"$(awk -v p="$pkg" '$1 == p { print $2, $3; found = 1 } END { if (!found) print 0, 0 }' <<<"$counts")"

  total_stmts=$((total_stmts + stmts))
  total_covered=$((total_covered + covered))

  floor="$(floor_for "$pkg")"
  floor_text=$([ "$floor" -gt 0 ] && echo "${floor}%" || echo "—")

  if [ "$stmts" -eq 0 ]; then
    # No statements at all: report it and move on. Enforcing a floor here would
    # fail a package for existing.
    printf '%-60s %s\n' "$pkg" "n/a"
    summary "| \`$pkg\` | n/a | $floor_text |"
    continue
  fi

  pct="$(awk -v c="$covered" -v s="$stmts" 'BEGIN { printf "%.1f", 100 * c / s }')"
  printf '%-60s %s%%\n' "$pkg" "$pct"
  summary "| \`$pkg\` | ${pct}% | $floor_text |"

  if [ "$ENFORCE" -eq 1 ] && [ "$floor" -gt 0 ]; then
    awk -v p="$pct" -v f="$floor" 'BEGIN { exit (p + 0 < f) ? 1 : 0 }' || {
      echo "$pkg coverage ${pct}% is below the ${floor}% floor" >&2
      violations=$((violations + 1))
    }
  fi
done

if [ "$total_stmts" -eq 0 ]; then
  echo "overall                                                      n/a"
  summary "| **overall** | n/a | 80% |"
  exit 0
fi

overall="$(awk -v c="$total_covered" -v s="$total_stmts" 'BEGIN { printf "%.1f", 100 * c / s }')"
printf '%-60s %s%%\n' "overall" "$overall"
summary "| **overall** | ${overall}% | 80% |"

if [ "$ENFORCE" -eq 1 ]; then
  awk -v p="$overall" 'BEGIN { exit (p + 0 < 80) ? 1 : 0 }' || {
    echo "overall coverage ${overall}% is below the 80% floor" >&2
    violations=$((violations + 1))
  }
fi

[ "$violations" -eq 0 ]
