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
  Feature    an issue in the backlog repository whose milestone says "Release: business"
             on a line of its own. It sits in that business milestone, not in the version
             milestone of the PR that does a part of it, so a link to a feature skips the
             same-milestone rule. The PR still carries its own version milestone.
  Feature    every PR of a product repository links a feature of the backlog: "Part of
             <backlog>#N" (or a closing link) to an issue whose milestone says "Release: business".
             A bot's PR (a bot author, or a dependabot/ or update/ branch) and the backlog's own
             PRs are exempt. Maintenance with no feature of its own links the standing feature
             (the issue with the label "standing") of the active business milestone.
  Milestone  one release (vX.Y.Z); tag only when every issue is closed and every PR merged.

Usage:
  scripts/check-linkage.sh --pr <N>          # GATE one PR: milestone + Closes #issue + same milestone
  scripts/check-linkage.sh --for-tag vX.Y.Z  # GATE a release: tag-readiness of that version's milestone
                                             #   resolves the milestone titled vX.Y.Z, else vX.Y.0
  scripts/check-linkage.sh [milestone]       # audit a milestone (default: the open one with the lowest version)
  scripts/check-linkage.sh --tag [ms]        # also assert tag-readiness (all issues closed, all PRs merged)
  scripts/check-linkage.sh --help

Env:
  REPO=owner/name          # override repo (default: gh repo view of the current checkout)
  BACKLOG_REPO=owner/name  # the repository whose business milestones hold features
                           # (default: galax-io/galaxio-sdlc-backlog)
  BACKLOG_READ_TOKEN=token # a read-only token for the backlog repository (Issues and Metadata). The
                           # backlog is private, so the job's own token cannot read it; this one is used
                           # for calls about the backlog and for nothing else.
  FEATURE_LINK=required|optional   # required (default): a PR without a feature of the backlog fails
                           # the gate. optional only for a repository not yet on the rule.

Exit: 0 all rules hold | 1 at least one violation | 2 usage/prereq error.
EOF
}

for bin in gh jq; do
  command -v "$bin" >/dev/null 2>&1 || { echo "error: '$bin' not found on PATH" >&2; exit 2; }
done

REPO="${REPO:-$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null || true)}"
[ -n "$REPO" ] || { echo "error: cannot determine repo — run inside a GitHub checkout or set REPO=owner/name" >&2; exit 2; }

BACKLOG_REPO="${BACKLOG_REPO:-galax-io/galaxio-sdlc-backlog}"
FEATURE_LINK="${FEATURE_LINK:-required}"
case "$FEATURE_LINK" in required|optional) ;; *) echo "error: FEATURE_LINK must be required or optional, not '$FEATURE_LINK'" >&2; exit 2 ;; esac

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

# Issues may live in another repository (the SDLC backlog), so a link is owner/repo#N and
# is looked up in its own repository; a bare number means this one.
closing_refs() { # pr-json -> registered GitHub closing links as owner/repo#N
  jq -r --arg repo "$REPO" '.closingIssuesReferences[]?
    | if .repository then "\(.repository.owner.login)/\(.repository.name)#\(.number)" else "\($repo)#\(.number)" end' <<<"$1"
}
part_of_refs() { # pr-json -> owner/repo#N of each "Part of" line of the body
  jq -r '.body // ""' <<<"$1" \
    | grep -iE '^[[:space:]]*part of +([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)#[0-9]+' \
    | grep -oE '[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+#[0-9]+' || true
}
gh_for() { # owner/repo, gh arguments: gh, with BACKLOG_READ_TOKEN when the repository is the backlog
  local repo="$1"; shift
  if [ "$repo" = "$BACKLOG_REPO" ] && [ -n "${BACKLOG_READ_TOKEN:-}" ]; then GH_TOKEN="$BACKLOG_READ_TOKEN" gh "$@"; else gh "$@"; fi
}
BACKLOG_SEEN=""
backlog_readable() { # whether this job can read the backlog repository (asked once)
  if [ -z "$BACKLOG_SEEN" ]; then
    if gh_for "$BACKLOG_REPO" api "repos/$BACKLOG_REPO" --silent >/dev/null 2>&1; then BACKLOG_SEEN=yes; else BACKLOG_SEEN=no; fi
  fi
  [ "$BACKLOG_SEEN" = yes ]
}
cannot_read_backlog() { # text of the failure that names what is missing
  printf 'cannot read the backlog %s with this job'"'"'s token, so a feature link cannot be checked: give the job BACKLOG_READ_TOKEN, a read-only token for %s (Issues and Metadata)' "$BACKLOG_REPO" "$BACKLOG_REPO"
}
issue_ms() { # owner/repo#N -> the issue's milestone title
  gh_for "${1%#*}" issue view "${1##*#}" --repo "${1%#*}" --json milestone -q '.milestone.title // ""' 2>/dev/null || echo ""
}
feature_ms() { # owner/repo#N -> the business milestone's title when the issue is a feature of the backlog
  [ "${1%#*}" = "$BACKLOG_REPO" ] || return 0
  gh_for "${1%#*}" issue view "${1##*#}" --repo "${1%#*}" --json milestone -q '
    select(.milestone != null and ((.milestone.description // "")
      | test("(^|\n)[ \t]*Release:[ \t]*business[ \t]*\r?(\n|$)"; "i"))) | .milestone.title' 2>/dev/null || true
}
is_bot_pr() { # pr-json -> succeeds when a bot opened it: a bot author, or Dependabot's or Scala Steward's branch
  jq -e '(.author.is_bot == true) or ((.author.login // "") | test("\\[bot\\]$|^app/")) or ((.headRefName // "") | test("^(dependabot|update)/"))' <<<"$1" >/dev/null 2>&1
}
feature_refs() { # pr-json -> every issue it links (closing links and Part of lines) that is a feature of the backlog
  local i
  { closing_refs "$1"; part_of_refs "$1"; } | awk 'NF && !seen[$0]++' | while read -r i; do
    [ -z "$(feature_ms "$i")" ] || echo "$i"
  done
}
feature_candidates() { # pr-json -> the links it writes into the backlog (closing links and Part of lines)
  { closing_refs "$1"; part_of_refs "$1"; } | awk -v b="$BACKLOG_REPO" 'NF && index($0, b "#") == 1 && !seen[$0]++'
}
show_ref() { printf '%s' "${1#"$REPO"}"; } # this repository's own issue reads as #N

# Which conventional-commit types must close a tracked issue.
#
# The constitution says "Every issue a PR fixes MUST be closed when the PR lands
# on main" — a rule about the issues a PR has, not a requirement that it have
# one. Some work legitimately has none, and the same document creates it: spec
# artifacts land as `docs(speckit): …` before any feat commit, a constitution
# amendment is its own PR, and a refactor outside an issue's scope is explicitly
# sent to a PR of its own. Demanding a link from those means opening a
# placeholder issue to satisfy a check, which is bookkeeping wearing the costume
# of traceability.
#
# What does need one is the work a milestone is a manifest of — a feature, a fix,
# a performance change — because "1 issue = 1 commit" maps those onto tracked
# issues one for one, and a release whose notes miss one is a release that lies
# about what shipped.
#
# Every PR still needs a milestone, without exception. That rule is unconditional
# in the constitution and stays unconditional here.
needs_closing_link() { # needs_closing_link <pr title> -> 0 when a link is required
  local type=${1%%[:(]*}
  case "$type" in
    feat|fix|perf) return 0 ;;
    *)             return 1 ;;
  esac
}

# Gate one PR (the merge gate). Fails if the PR is missing a milestone, closes no
# issue, or closes an issue in a different milestone. Strict: requires a registered
# GitHub closing link (no body-text fallback — that lenient path is audit-mode only).
if [ -n "$PR_NUM" ]; then
  pj=$(gh pr view "$PR_NUM" --repo "$REPO" --json number,title,state,milestone,closingIssuesReferences,body,author,headRefName) \
    || { echo "error: PR #$PR_NUM not found in $REPO" >&2; exit 2; }
  p_title=$(jq -r '.title' <<<"$pj")
  p_ms=$(jq -r '.milestone.title // ""' <<<"$pj")
  p_closes=$(closing_refs "$pj")
  p_features=""   # "Part of" lines that name a feature; they are reported, never required
  for r in $(part_of_refs "$pj"); do
    [ -z "$(feature_ms "$r")" ] || p_features="$p_features$r "
  done
  e=0
  printf 'PR #%s  %s\n' "$PR_NUM" "$p_title"
  if [ -z "$p_ms" ]; then printf '  ✗ no milestone — assign one (gh pr edit %s --milestone "…")\n' "$PR_NUM"; e=1
  else printf '  ✓ milestone: %s\n' "$p_ms"; fi
  if [ -z "$p_closes" ]; then
    if needs_closing_link "$p_title"; then
      printf '  ✗ closes no issue — add "Closes #<issue>" to the PR body\n'; e=1
    else
      printf '  ✓ closes no issue (not required for this change type)\n'
    fi
  else
    for i in $p_closes; do
      i_feature=$(feature_ms "$i")
      i_ms=$(issue_ms "$i")
      if [ -n "$i_feature" ]; then printf '  ✓ links %s (feature of business milestone "%s")\n' "$i" "$i_feature"
      elif [ "${i%#*}" = "$BACKLOG_REPO" ] && ! backlog_readable; then printf '  ✗ links %s, but %s\n' "$i" "$(cannot_read_backlog)"; e=1
      elif [ "$i_ms" = "$p_ms" ]; then printf '  ✓ closes %s (same milestone)\n' "$(show_ref "$i")"
      else printf '  ✗ closes %s but its milestone is "%s", not "%s"\n' "$(show_ref "$i")" "${i_ms:-none}" "$p_ms"; e=1; fi
    done
  fi
  for i in $p_features; do
    printf '  ✓ links %s (feature of business milestone "%s")\n' "$i" "$(feature_ms "$i")"
  done
  # Every PR belongs to a feature of the backlog; the backlog's own PRs and bots' are exempt.
  if [ "$FEATURE_LINK" = required ] && [ "$REPO" != "$BACKLOG_REPO" ]; then
    if is_bot_pr "$pj"; then
      printf '  ✓ a bot'"'"'s PR needs no feature of the backlog\n'
    elif [ -z "$(feature_refs "$pj")" ] && [ -n "$(feature_candidates "$pj")" ] && ! backlog_readable; then
      for c in $(feature_candidates "$pj"); do
        case " $p_closes " in *" $c "*) ;; *) printf '  ✗ links %s, but %s\n' "$c" "$(cannot_read_backlog)"; e=1 ;; esac   # closing links said it above
      done
      e=1
    elif [ -z "$(feature_refs "$pj")" ]; then
      printf '  ✗ links no feature of the backlog — add "Part of %s#<feature>" (maintenance with no feature of its own: the standing feature of the active business milestone, the issue with the label standing)\n' "$BACKLOG_REPO"; e=1
    fi
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

# Default to the open milestone carrying the lowest version in its title (the
# "active" one). By version, not by GitHub's milestone number: that number is
# assigned at creation and never changes, so it only tracks release order for as
# long as milestones are created in release order and never renamed. A milestone
# whose title carries no vX.Y.Z is not a release and is never the active one.
if [ -z "$MS" ]; then
  MS=$(gh api --paginate --slurp "repos/$REPO/milestones?state=open&per_page=100" \
        | jq -r 'add
          | map(select(.title | test("^v[0-9]+\\.[0-9]+\\.[0-9]+")))
          | map(. + {v: (.title
              | capture("^v(?<a>[0-9]+)\\.(?<b>[0-9]+)\\.(?<c>[0-9]+)")
              | [(.a|tonumber), (.b|tonumber), (.c|tonumber)])})
          | sort_by(.v) | .[0].number // empty')
  [ -n "$MS" ] || { echo "error: no open vX.Y.Z milestone found in $REPO" >&2; exit 2; }
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
  pr_json=$(gh pr view "$pr" --repo "$REPO" --json number,title,state,milestone,closingIssuesReferences,body,author,headRefName)
  pr_state=$(jq -r '.state' <<<"$pr_json")
  pr_title=$(jq -r '.title' <<<"$pr_json")

  # A PR of a product repository belongs to a feature of the backlog (a warning here, the gate fails it).
  if [ "$FEATURE_LINK" = required ] && [ "$REPO" != "$BACKLOG_REPO" ] && ! is_bot_pr "$pr_json" \
     && [ -z "$(feature_refs "$pr_json")" ]; then
    warn "PR #$pr links no feature of the backlog (Part of $BACKLOG_REPO#N): $pr_title"
  fi

  # Real GitHub closing links, plus a text fallback for Closes/Fixes/Resolves #N in the body.
  ref_nums=$(closing_refs "$pr_json")
  if [ -z "$ref_nums" ]; then
    ref_nums=$(jq -r '.body // ""' <<<"$pr_json" \
      | grep -oiE '(close[sd]?|fix(e[sd])?|resolve[sd]?) +#[0-9]+' \
      | grep -oE '[0-9]+' | sed "s|^|$REPO#|" || true)
    [ -n "$ref_nums" ] && warn "PR #$pr links via body text only (not a registered GitHub closing link): $pr_title"
  fi

  if [ -z "$ref_nums" ]; then
    if needs_closing_link "$pr_title"; then
      err "PR #$pr ($pr_state) closes no issue — add 'Closes #<issue>': $pr_title"
    fi

    # Still has to be merged before the tag, and still has to carry the
    # milestone; only the closing link is waived. Reported either way, so a
    # waived PR reads as checked rather than as one the audit forgot.
    if [ "$TAG_MODE" = 1 ] && [ "$pr_state" != "MERGED" ]; then
      err "PR #$pr is $pr_state — must be MERGED before tagging: $pr_title"
    elif needs_closing_link "$pr_title"; then
      : # already reported above
    else
      ok "PR #$pr ($pr_state) → closes no issue (not required for this change type)"
    fi

    continue
  fi

  for ri in $ref_nums; do
    [ "${ri%#*}" != "$REPO" ] || linked_issues="$linked_issues${ri##*#} "
    [ -z "$(feature_ms "$ri")" ] || continue   # a feature sits in its business milestone
    ri_ms=$(issue_ms "$ri")
    if [ "$ri_ms" != "$ms_title" ]; then
      err "PR #$pr closes issue $(show_ref "$ri") but that issue's milestone is '${ri_ms:-none}', not '$ms_title'"
    fi
  done

  if [ "$TAG_MODE" = 1 ] && [ "$pr_state" != "MERGED" ]; then
    err "PR #$pr is $pr_state — must be MERGED before tagging: $pr_title"
  else
    ok "PR #$pr ($pr_state) → closes $(for r in $ref_nums; do show_ref "$r"; done | paste -sd, -)"
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
