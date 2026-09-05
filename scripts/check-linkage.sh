#!/usr/bin/env bash
#
# check-linkage.sh — verify the issue ↔ PR ↔ milestone contract (see AGENTS.md "Milestones (ALWAYS)").
# Run `scripts/check-linkage.sh --help` for full usage.

set -euo pipefail

usage() {
  cat <<'EOF'
check-linkage.sh — verify the issue <-> PR <-> milestone contract (see AGENTS.md "Milestones (ALWAYS)").

What each entity owes (this script enforces it):
  Issue      belongs to exactly one milestone; closed only when its fix is on main.
  PR         carries its issue's milestone + a real closing link (Closes #<issue>);
             the linked issue sits in the same milestone; one issue per PR.
  Milestone  one release (vX.Y.Z); tag only when every issue is closed and every PR merged.

Usage:
  scripts/check-linkage.sh --pr <N>          # GATE one PR: milestone + Closes #issue + same milestone
  scripts/check-linkage.sh --for-tag vX.Y.Z  # GATE a release: tag-readiness of that version's milestone
                                             #   resolves the milestone titled vX.Y.Z, else vX.Y.0
  scripts/check-linkage.sh [milestone]       # audit a milestone (default: lowest-numbered open)
  scripts/check-linkage.sh --tag [ms]        # also assert tag-readiness (all issues closed, all PRs merged)
  scripts/check-linkage.sh --help

Env:
  REPO=owner/name   # override repo (default: gh repo view of the current checkout)

Exit: 0 all rules hold | 1 at least one violation | 2 usage/prereq error.
EOF
}

for bin in gh jq; do
  command -v "$bin" >/dev/null 2>&1 || { echo "error: '$bin' not found on PATH" >&2; exit 2; }
done

REPO="${REPO:-$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null || true)}"
[ -n "$REPO" ] || { echo "error: cannot determine repo — run inside a GitHub checkout or set REPO=owner/name" >&2; exit 2; }

TAG_MODE=0
MS=""
MILESTONES=""
PR_NUM=""
FOR_TAG=""
while [ $# -gt 0 ]; do
  case "$1" in
    --tag)      TAG_MODE=1 ;;
    --pr)       shift; PR_NUM="${1:-}"
                [[ "$PR_NUM" =~ ^[0-9]+$ ]] || { echo "error: --pr needs a numeric PR id" >&2; exit 2; } ;;
    --for-tag)  shift; FOR_TAG="${1:-}"
                [[ "$FOR_TAG" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "error: --for-tag needs vX.Y.Z" >&2; exit 2; } ;;
    --help|-h)  usage; exit 0 ;;
    [0-9]*)     MS="$1" ;;
    *)          echo "error: unknown argument '$1'" >&2; usage; exit 2 ;;
  esac
  shift
done

# Gate one PR (the merge gate). Fails if the PR is missing a milestone, closes no
# issue, or closes an issue in a different milestone. Strict: requires a registered
# GitHub closing link (no body-text fallback — that lenient path is audit-mode only).
if [ -n "$PR_NUM" ]; then
  pj=$(gh pr view "$PR_NUM" --repo "$REPO" --json number,title,state,milestone,closingIssuesReferences) \
    || { echo "error: PR #$PR_NUM not found in $REPO" >&2; exit 2; }
  p_title=$(jq -r '.title' <<<"$pj")
  p_ms=$(jq -r '.milestone.title // ""' <<<"$pj")
  p_closes=$(jq -r '.closingIssuesReferences[]?.number' <<<"$pj")
  e=0
  printf 'PR #%s  %s\n' "$PR_NUM" "$p_title"
  if [ -z "$p_ms" ]; then printf '  ✗ no milestone — assign one (gh pr edit %s --milestone "…")\n' "$PR_NUM"; e=1
  else printf '  ✓ milestone: %s\n' "$p_ms"; fi
  if [ -z "$p_closes" ]; then
    printf '  ✗ closes no issue — add "Closes #<issue>" to the PR body\n'; e=1
  else
    for i in $p_closes; do
      i_ms=$(gh issue view "$i" --repo "$REPO" --json milestone -q '.milestone.title // ""' 2>/dev/null || echo "")
      if [ "$i_ms" = "$p_ms" ]; then printf '  ✓ closes #%s (same milestone)\n' "$i"
      else printf '  ✗ closes #%s but its milestone is "%s", not "%s"\n' "$i" "${i_ms:-none}" "$p_ms"; e=1; fi
    done
  fi
  if [ "$e" = 0 ]; then printf 'PASS: PR #%s is well-formed.\n' "$PR_NUM"; exit 0; fi
  printf 'FAIL: PR #%s is malformed — fix the above before merge.\n' "$PR_NUM"; exit 1
fi

# Find the milestone a release version belongs to. Anchored on a version boundary,
# never a bare prefix: "v0.0.1" must not select "v0.0.10 Finding the run".
milestone_for() { # milestone_for <vX.Y.Z> -> milestone number, or empty
  local want="$1" re
  re="^$(printf '%s' "$want" | sed 's/[.]/\\./g')"'([^0-9.-]|\.[^0-9]|$)'
  printf '%s' "$MILESTONES" \
    | jq -r --arg re "$re" 'map(select(.title | test($re))) | .[0].number // empty'
}

# Map a release version (vX.Y.Z) to its milestone, then assert tag-readiness on it.
#
# The exact version is tried first, and only then vX.Y.0. Both shapes are real:
# during 0.0.x every patch carries its own milestone ("v0.0.1 Scaffold"), so asking
# only for vX.Y.0 looks for a milestone nobody created and refuses the release for
# a reason that has nothing to do with the release. Once a minor owns a milestone
# and its patches ship under it, the fallback finds it (v1.23.1 -> "v1.23.0 …").
if [ -n "$FOR_TAG" ]; then
  v="${FOR_TAG#v}"
  exact="v$v"
  minor="v${v%.*}.0"

  MILESTONES=$(gh api --paginate --slurp "repos/$REPO/milestones?state=all&per_page=100" | jq 'add')

  MS=$(milestone_for "$exact")
  if [ -z "$MS" ] && [ "$minor" != "$exact" ]; then
    MS=$(milestone_for "$minor")
  fi

  if [ -z "$MS" ]; then
    if [ "$minor" = "$exact" ]; then
      echo "error: no milestone titled '$exact' for tag $FOR_TAG" >&2
    else
      echo "error: no milestone for tag $FOR_TAG - looked for '$exact', then '$minor'" >&2
    fi
    exit 2
  fi
  TAG_MODE=1
fi

# Default to the lowest-numbered open milestone (the "active" one).
if [ -z "$MS" ]; then
  MS=$(gh api --paginate --slurp "repos/$REPO/milestones?state=open&per_page=100" \
        | jq -r 'add | sort_by(.number) | .[0].number // empty')
  [ -n "$MS" ] || { echo "error: no open milestone found in $REPO" >&2; exit 2; }
fi

# Reuse the list already in hand when --for-tag fetched it; only the default path
# has to ask again.
if [ -n "${MILESTONES:-}" ]; then
  ms_json=$(jq --argjson n "$MS" 'map(select(.number == $n)) | .[0]' <<<"$MILESTONES")
else
  ms_json=$(gh api "repos/$REPO/milestones/$MS") || { echo "error: milestone #$MS not found in $REPO" >&2; exit 2; }
fi
ms_title=$(jq -r '.title' <<<"$ms_json")
ms_state=$(jq -r '.state' <<<"$ms_json")

errors=0
warns=0
err()  { printf '  ✗ %s\n' "$1"; errors=$((errors + 1)); }
warn() { printf '  ! %s\n' "$1"; warns=$((warns + 1)); }
ok()   { printf '  ✓ %s\n' "$1"; }

printf 'Repo:      %s\n' "$REPO"
printf 'Milestone: #%s  %s  (%s)\n' "$MS" "$ms_title" "$ms_state"
[ "$TAG_MODE" = 1 ] && printf 'Mode:      tag-readiness\n'
printf '\n'

# All issues + PRs carrying this milestone (REST returns both; PRs carry a .pull_request key).
items=$(gh api --paginate --slurp "repos/$REPO/issues?milestone=$MS&state=all&per_page=100" | jq 'add')

pr_numbers=$(jq -r '.[] | select(.pull_request != null) | .number' <<<"$items")
issue_numbers=$(jq -r '.[] | select(.pull_request == null) | .number' <<<"$items")

linked_issues=" "   # space-delimited set of issue numbers a PR points at

printf 'Pull requests\n'
if [ -z "$pr_numbers" ]; then
  # In tag mode this is a refusal, not a note. A milestone nobody filed work
  # against is trivially "ready", so warning here is how an empty or mistakenly
  # created milestone shadows the real one and a release ships unverified.
  if [ "$TAG_MODE" = 1 ]; then
    err "no PRs carry milestone #$MS — nothing to release under it, or the work was filed elsewhere"
  else
    warn "no PRs carry milestone #$MS yet"
  fi
fi
for pr in $pr_numbers; do
  pr_json=$(gh pr view "$pr" --repo "$REPO" --json number,title,state,milestone,closingIssuesReferences,body)
  pr_state=$(jq -r '.state' <<<"$pr_json")
  pr_title=$(jq -r '.title' <<<"$pr_json")

  # Real GitHub closing links, plus a text fallback for Closes/Fixes/Resolves #N in the body.
  ref_nums=$(jq -r '.closingIssuesReferences[]?.number' <<<"$pr_json")
  if [ -z "$ref_nums" ]; then
    ref_nums=$(jq -r '.body // ""' <<<"$pr_json" \
      | grep -oiE '(close[sd]?|fix(e[sd])?|resolve[sd]?) +#[0-9]+' \
      | grep -oE '[0-9]+' || true)
    [ -n "$ref_nums" ] && warn "PR #$pr links via body text only (not a registered GitHub closing link): $pr_title"
  fi

  if [ -z "$ref_nums" ]; then
    err "PR #$pr ($pr_state) closes no issue — add 'Closes #<issue>': $pr_title"
    continue
  fi

  for ri in $ref_nums; do
    linked_issues="$linked_issues$ri "
    ri_ms=$(gh issue view "$ri" --repo "$REPO" --json milestone -q '.milestone.title // ""' 2>/dev/null || echo "")
    if [ "$ri_ms" != "$ms_title" ]; then
      err "PR #$pr closes issue #$ri but that issue's milestone is '${ri_ms:-none}', not '$ms_title'"
    fi
  done

  if [ "$TAG_MODE" = 1 ] && [ "$pr_state" != "MERGED" ]; then
    err "PR #$pr is $pr_state — must be MERGED before tagging: $pr_title"
  else
    ok "PR #$pr ($pr_state) → closes #$(echo "$ref_nums" | paste -sd, -)"
  fi
done

printf '\nIssues\n'
if [ -z "$issue_numbers" ]; then
  if [ "$TAG_MODE" = 1 ]; then
    err "no issues carry milestone #$MS — an empty milestone is not a tag-ready one"
  else
    warn "no issues carry milestone #$MS"
  fi
fi
for is in $issue_numbers; do
  is_state=$(jq -r --argjson n "$is" '.[] | select(.number == $n) | .state' <<<"$items")
  case "$linked_issues" in
    *" $is "*) linked="linked" ;;
    *)         linked="" ;;
  esac

  if [ "$TAG_MODE" = 1 ] && [ "$is_state" != "closed" ]; then
    err "issue #$is is $is_state — must be closed before tagging"
  elif [ -z "$linked" ]; then
    warn "issue #$is ($is_state) has no PR closing it"
  else
    ok "issue #$is ($is_state) ← linked"
  fi
done

# The milestone query above can only see work that already carries the milestone,
# so it is structurally blind to the other half of the rule: "every PR merged since
# the previous tag carries a milestone". That one has to be asked of the commit
# range, not of the milestone.
if [ "$TAG_MODE" = 1 ]; then
  printf '\nMerged since the previous tag\n'

  # gh prints the 404 body on stdout when a repository has no release yet, so the
  # result is shape-checked rather than trusted.
  prev_tag=$(gh api "repos/$REPO/releases/latest" --jq '.tag_name // empty' 2>/dev/null || true)
  case "$prev_tag" in
    v[0-9]*.[0-9]*.[0-9]*) ;;
    *) prev_tag="" ;;
  esac

  if [ -z "$prev_tag" ]; then
    ok "no previous release - $FOR_TAG is the first, nothing precedes it"
  else
    # The range end is the tag when it exists and HEAD when it does not. Before the
    # push there is no tag to compare against, and comparing to a ref that 404s
    # makes this whole section vacuous — which is how the one check that gates the
    # push came to be the one check the pre-push run cannot exercise.
    range_end="$FOR_TAG"
    if ! gh api "repos/$REPO/git/ref/tags/${FOR_TAG#refs/tags/}" >/dev/null 2>&1; then
      range_end=$(git rev-parse HEAD 2>/dev/null || echo "")
      [ -n "$range_end" ] && printf '  (%s does not exist yet; auditing up to HEAD)\n' "$FOR_TAG"
    fi

    # Which pull requests contain each commit, asked of GitHub rather than read out
    # of the commit subjects. The subjects carry "(#NNN)", but this repository's
    # convention (AGENTS.md, "1 issue = 1 commit") makes that the *issue* number —
    # so scanning them reports every issue as a milestone-less pull request, which
    # is a refusal no correct release can avoid.
    range_shas=$(gh api --paginate --slurp "repos/$REPO/compare/$prev_tag...$range_end" 2>/dev/null \
                  | jq -r 'add | .commits[].sha' 2>/dev/null || true)

    range_prs=$(for sha in $range_shas; do
                  gh api "repos/$REPO/commits/$sha/pulls" --jq '.[].number' 2>/dev/null || true
                done | sort -un)

    audited=" $(echo $pr_numbers) "   # collapse newlines; already checked above
    unseen=0

    for pr in $range_prs; do
      case "$audited" in *" $pr "*) continue ;; esac

      unseen=$((unseen + 1))
      pr_ms=$(gh pr view "$pr" --repo "$REPO" --json milestone -q '.milestone.title // ""' 2>/dev/null || echo "")

      if [ -z "$pr_ms" ]; then
        err "PR #$pr merged since $prev_tag carries no milestone"
      elif [ "$pr_ms" != "$ms_title" ]; then
        err "PR #$pr merged since $prev_tag carries milestone '$pr_ms', not '$ms_title'"
      else
        ok "PR #$pr (milestone '$pr_ms') ← merged in range"
        unseen=$((unseen - 1))
      fi
    done

    if [ -z "$range_shas" ]; then
      err "no commits found between $prev_tag and $range_end — the range could not be read, so nothing was audited"
    elif [ -z "$range_prs" ]; then
      warn "commits between $prev_tag and $range_end belong to no pull request"
    elif [ "$unseen" = 0 ]; then
      ok "every PR merged since $prev_tag carries milestone '$ms_title'"
    fi
  fi
fi

printf '\n'
if [ "$errors" -gt 0 ]; then
  printf 'FAIL: %d error(s), %d warning(s).\n' "$errors" "$warns"
  exit 1
fi
printf 'PASS: 0 errors, %d warning(s).\n' "$warns"
[ "$TAG_MODE" = 1 ] && printf 'Milestone #%s is tag-ready.\n' "$MS"
exit 0
