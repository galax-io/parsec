#!/usr/bin/env bash
#
# check-linkage_test.sh — regression tests for the tag→milestone resolution in
# check-linkage.sh (constitution Principle III: a bug fix ships with a test that
# fails without the fix).
#
# `gh` is replaced by a stub answering from fixtures, so the tests are hermetic:
# no network, no repository state, same result on a laptop and on a runner.

set -uo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
script="$here/check-linkage.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

pass=0; fail=0
ok()   { printf '  ✓ %s\n' "$1"; pass=$((pass + 1)); }
bad()  { printf '  ✗ %s\n     %s\n' "$1" "$2"; fail=$((fail + 1)); }

# ---- the gh stub -----------------------------------------------------------
mkdir -p "$tmp/bin"
cat > "$tmp/bin/gh" <<'STUB'
#!/usr/bin/env bash
# Test double for gh: answers only what check-linkage.sh asks, from $GH_FIXTURE.
args="$*"
expr=""; prev=""
for a in "$@"; do
  case "$prev" in --jq|-q) expr="$a" ;; esac
  prev="$a"
done
emit() { if [ -n "$expr" ]; then jq -r "$expr"; else cat; fi; }

# gh --slurp wraps each page in an outer array; without it the body is returned
# as-is. Fixtures are stored flat and wrapped here, so a caller that forgets
# --slurp sees exactly what the real gh would give it.
page() { case "$args" in *--slurp*) jq -c '[.]' < "$1" ;; *) cat "$1" ;; esac | emit; }

case "$args" in
  "pr view 77 "*) emit < "$GH_FIXTURE/pr77.json" ;;
  "pr view 99 "*) emit < "$GH_FIXTURE/pr99.json" ;;
  *"/milestones?state="*) page "$GH_FIXTURE/milestones.json" ;;
  *"/milestones/"*)
      n="${args##*/milestones/}"; n="${n%% *}"
      jq -c --argjson n "$n" 'map(select(.number == $n)) | .[0]' < "$GH_FIXTURE/milestones.json" | emit ;;
  *"/releases/latest"*)
      if [ -f "$GH_FIXTURE/latest.json" ]; then emit < "$GH_FIXTURE/latest.json"
      else printf '{"message":"Not Found","status":"404"}\n'; exit 1; fi ;;
  *"/issues?milestone="*) page "$GH_FIXTURE/items.json" ;;
  *"/compare/"*)
      if [ -f "$GH_FIXTURE/compare.json" ]; then page "$GH_FIXTURE/compare.json"
      else printf '{"commits":[]}\n' | page /dev/stdin; fi ;;
  *"/git/ref/tags/"*)
      # The tag exists only when the fixture says so, which is what makes the
      # "audit up to HEAD" branch reachable in a test.
      if [ -f "$GH_FIXTURE/tagref.json" ]; then emit < "$GH_FIXTURE/tagref.json"; else exit 1; fi ;;
  *"/commits/"*"/pulls"*)
      sha="${args##*/commits/}"; sha="${sha%%/pulls*}"
      f="$GH_FIXTURE/pulls-$sha.json"
      if [ -f "$f" ]; then emit < "$f"; else printf '[]\n' | emit; fi ;;
  "pr view "*)   emit < "$GH_FIXTURE/pr.json" ;;
  "issue view "*)
      # A per-issue fixture (issue-<owner>_<repo>-<n>.json) wins over the one shared answer,
      # so a test can put an issue of another repository in a milestone of its own.
      set -- $args; n="$3"; r=""; prev=""
      for a in "$@"; do [ "$prev" = "--repo" ] && r="$a"; prev="$a"; done
      f="$GH_FIXTURE/issue-${r//\//_}-$n.json"
      if [ -f "$f" ]; then emit < "$f"; else emit < "$GH_FIXTURE/issue.json"; fi ;;
  *) printf 'gh stub: unhandled call: %s\n' "$args" >&2; exit 1 ;;
esac
STUB
chmod +x "$tmp/bin/gh"

# ---- fixture builder -------------------------------------------------------
# milestones are wrapped one level deep: `gh api --paginate --slurp` returns pages.
fixture() { # fixture <name> <milestones-json-array> [items-json-array]
  local d="$tmp/$1"; mkdir -p "$d"
  printf '%s\n' "$2" > "$d/milestones.json"
  printf '%s\n' "${3:-[]}" > "$d/items.json"
  printf '%s' "$d"
}

# The cases that test other rules run with the feature requirement off (FEATURE_LINK=optional);
# the cases at the end use run_strict, which leaves it at its default (required).
run() { # run <fixture-dir> <args...>
  GH_FIXTURE="$1" PATH="$tmp/bin:$PATH" REPO="acme/thing" BACKLOG_REPO="acme/backlog" FEATURE_LINK=optional bash "$script" "${@:2}" 2>&1
}
run_strict() { # run_strict <fixture-dir> <args...>
  GH_FIXTURE="$1" PATH="$tmp/bin:$PATH" REPO="acme/thing" BACKLOG_REPO="acme/backlog" bash "$script" "${@:2}" 2>&1
}

MS='[{"number":1,"title":"v0.0.1 Scaffold","state":"open"},
     {"number":2,"title":"v0.0.10 Finding the run","state":"open"},
     {"number":3,"title":"v1.2.0 Minor","state":"open"},
     {"number":4,"title":"v2.0.0. Big rewrite","state":"open"},
     {"number":5,"title":"v3.0.0-rc1 Preview","state":"open"},
     {"number":6,"title":"v4.0.0.1 Odd","state":"open"}]'

# One merged PR + one closed issue, so a milestone is non-empty and tag-ready.
ITEMS='[{"number":10,"pull_request":{},"state":"closed"},
        {"number":11,"state":"closed"}]'

f=$(fixture sel "$MS" "$ITEMS")
cp /dev/null "$tmp/sel/pr.json"; printf '{"number":10,"title":"t","state":"MERGED","milestone":{"title":"x"},"closingIssuesReferences":[{"number":11}],"body":""}\n' > "$tmp/sel/pr.json"
printf '{"milestone":{"title":"x"}}\n' > "$tmp/sel/issue.json"

selects() { # selects <tag> <expected milestone title or NONE>
  local out got
  out=$(run "$f" --for-tag "$1")
  got=$(sed -n 's/^Milestone: #[0-9]*  \(.*\)  (.*)$/\1/p' <<<"$out" | head -1)
  [ -n "$got" ] || got="NONE"
  if [ "$got" = "$2" ]; then ok "$1 → $2"; else bad "$1 → expected '$2'" "got '$got'"; fi
}

printf 'tag → milestone resolution\n'
selects v0.0.1  "v0.0.1 Scaffold"          # prefix collision: must not take v0.0.10
selects v0.0.10 "v0.0.10 Finding the run"
selects v1.2.0  "v1.2.0 Minor"
selects v1.2.7  "v1.2.0 Minor"             # patch falls back to its minor
selects v2.0.0  "v2.0.0. Big rewrite"      # punctuation directly after the version
selects v3.0.0  NONE                       # a prerelease milestone is not the release
selects v4.0.0  NONE                       # v4.0.0.1 does not satisfy v4.0.0
selects v9.9.9  NONE

printf '\nan empty milestone is not tag-ready\n'
e=$(fixture empty '[{"number":1,"title":"v0.0.1 Scaffold","state":"open"}]' '[]')
out=$(run "$e" --for-tag v0.0.1); rc=$?
if [ "$rc" -ne 0 ] && grep -q "no PRs carry milestone" <<<"$out"; then
  ok "refuses a milestone nobody filed work against"
else
  bad "empty milestone should be refused in tag mode" "exit=$rc"
fi
if grep -qE "^Milestone #[0-9]+ is tag-ready\.$" <<<"$out"; then
  bad "empty milestone must not be reported tag-ready" "$(grep 'is tag-ready' <<<"$out")"
else
  ok "does not print 'tag-ready' for an empty milestone"
fi

printf '\nPRs merged since the previous tag\n'
r=$(fixture range '[{"number":1,"title":"v0.0.2 Second","state":"open"}]' "$ITEMS")
printf '{"number":10,"title":"t","state":"MERGED","milestone":{"title":"v0.0.2 Second"},"closingIssuesReferences":[{"number":11}],"body":""}\n' > "$r/pr.json"
printf '{"milestone":{"title":"v0.0.2 Second"}}\n' > "$r/issue.json"
printf '{"tag_name":"v0.0.1"}\n' > "$r/latest.json"
printf '{"id":"x"}\n' > "$r/tagref.json"
printf '{"commits":[{"sha":"aaa","commit":{"message":"feat(x): thing (#99)"}},{"sha":"bbb","commit":{"message":"fix(y): other (#77)"}}]}\n' > "$r/compare.json"
# The commit subjects name issues #99 and #77 — this repository's convention puts
# the issue number there. Commit aaa belongs to no pull request at all, so an audit
# that read its subject would look up #99, find no milestone on it, and refuse a
# release for a pull request that does not exist. Commit bbb really is in PR #77,
# which really does lack a milestone, and that one must still be caught.
printf '[]\n' > "$r/pulls-aaa.json"
printf '{"number":99,"title":"an issue, not a PR","state":"MERGED","milestone":null,"closingIssuesReferences":[],"body":""}\n' > "$r/pr99.json"
printf '[{"number":77}]\n' > "$r/pulls-bbb.json"
printf '{"number":77,"title":"stray","state":"MERGED","milestone":null,"closingIssuesReferences":[],"body":""}\n' > "$r/pr77.json"
out=$(GH_FIXTURE="$r" PATH="$tmp/bin:$PATH" REPO="acme/thing" FEATURE_LINK=optional bash "$script" --for-tag v0.0.2 2>&1)
if grep -q "PR #77 merged since v0.0.1" <<<"$out"; then
  ok "sees a PR merged in the range that the milestone query cannot"
else
  bad "range audit missed an unmilestoned merged PR" "$(tail -4 <<<"$out")"
fi

# The regression this release exposed: the audit used to read "(#NNN)" out of the
# commit subjects, but AGENTS.md puts the *issue* number there, so every issue was
# reported as a pull request carrying no milestone and no correct release could pass.
if grep -q "PR #99 merged since" <<<"$out"; then
  bad "range audit reads issue numbers out of commit subjects as PRs" "$(grep 'PR #99 merged since' <<<"$out")"
else
  ok "does not mistake an issue number in a commit subject for a pull request"
fi

printf '\nthe active milestone is the lowest version, not the lowest GitHub number\n'
# Numbers and versions deliberately disagree, as they do after milestones are
# renamed: #6 carries v0.0.8, #7 carries v0.0.6, #12 carries v0.0.7. A title with
# no version at all is never a release and must never be picked.
A='[{"number":6,"title":"v0.0.8 An incomplete record","state":"open"},
    {"number":7,"title":"v0.0.6 The primitives a consumer folds","state":"open"},
    {"number":12,"title":"v0.0.7 The corpus and the canary","state":"open"},
    {"number":13,"title":"retired — percentiles and ranges (was v0.0.8)","state":"open"}]'
a=$(fixture active "$A" "$ITEMS")
printf '{"number":10,"title":"t","state":"MERGED","milestone":{"title":"x"},"closingIssuesReferences":[{"number":11}],"body":""}\n' > "$a/pr.json"
printf '{"milestone":{"title":"x"}}\n' > "$a/issue.json"
active() { # active <fixture-dir> <expected title>
  local out got
  out=$(run "$1")
  got=$(sed -n 's/^Milestone: #[0-9]*  \(.*\)  (.*)$/\1/p' <<<"$out" | head -1)
  if [ "$got" = "$2" ]; then ok "default → $2"; else bad "default → expected '$2'" "got '$got'"; fi
}
active "$a" "v0.0.6 The primitives a consumer folds"

# With v0.0.6 gone the next version up is active, still regardless of number.
B='[{"number":6,"title":"v0.0.8 An incomplete record","state":"open"},
    {"number":12,"title":"v0.0.7 The corpus and the canary","state":"open"},
    {"number":13,"title":"retired — percentiles and ranges (was v0.0.8)","state":"open"}]'
b=$(fixture active2 "$B" "$ITEMS")
cp "$a/pr.json" "$a/issue.json" "$b/"
active "$b" "v0.0.7 The corpus and the canary"

# Only a version-less milestone open: not a release, so nothing is active.
C='[{"number":13,"title":"retired — percentiles and ranges (was v0.0.8)","state":"open"}]'
c=$(fixture active3 "$C" "$ITEMS")
out=$(run "$c"); rc=$?
if [ "$rc" -eq 2 ] && grep -q "no open vX.Y.Z milestone" <<<"$out"; then
  ok "refuses to treat a version-less milestone as active"
else
  bad "a version-less milestone must not be active" "exit=$rc $(head -2 <<<"$out")"
fi

printf '\na link to a feature of the backlog needs no same milestone\n'
# A feature is an issue in BACKLOG_REPO whose milestone description has the line
# "Release: business". The PR keeps its own version milestone; the feature sits in the
# business one, so the two cannot match.
BL=acme/backlog
BUSINESS='Release: business'
bl_issue() { # fixture-dir, number, milestone title, [milestone description]
  jq -nc --arg ms "$3" --arg d "${4:-}" \
    '{milestone: (if $ms == "" then null else {title: $ms, description: $d} end)}' \
    > "$1/issue-${BL//\//_}-$2.json"
}
gate_fixture() { # name, pr body, closing-links JSON array, [pr milestone title]
  local d; d=$(fixture "$1" '[]' '[]')
  jq -nc --arg body "$2" --argjson links "$3" --arg ms "${4-v0.2.0 The gates hold}" \
    --arg author "${PR_AUTHOR:-jigarkhwar}" --arg bot "${PR_BOT:-0}" --arg head "${PR_HEAD:-feat/x}" \
    '{number:3,title:"fix(x): y",state:"OPEN",baseRefName:"main",body:$body,closingIssuesReferences:$links,
      author:{login:$author,is_bot:($bot == "1")},headRefName:$head,
      milestone:(if $ms == "" then null else {title:$ms} end)}' > "$d/pr.json"
  printf '{"milestone":{"title":"v0.1.0 Earlier"}}\n' > "$d/issue.json"
  printf '%s' "$d"
}
BL_LINK='[{"number":30,"repository":{"name":"backlog","owner":{"login":"acme"}}}]'

g=$(gate_fixture gate-feature-closes '' "$BL_LINK"); bl_issue "$g" 30 'M1 — first run' "$BUSINESS"
out=$(run "$g" --pr 3); rc=$?
if [ "$rc" -eq 0 ] && grep -qF "✓ links $BL#30 (feature of business milestone \"M1 — first run\")" <<<"$out"; then
  ok "gate: a closing link to a backlog feature passes and names the business milestone"
else bad "gate: closing link to a feature should pass" "rc=$rc $out"; fi

g=$(gate_fixture gate-feature-partof "Part of $BL#30" '[]'); jq '.title="chore(x): y"' "$g/pr.json" > "$g/pr2" && mv "$g/pr2" "$g/pr.json"; bl_issue "$g" 30 'M1 — first run' "$BUSINESS"
out=$(run "$g" --pr 3); rc=$?
if [ "$rc" -eq 0 ] && grep -qF "✓ links $BL#30 (feature of business milestone" <<<"$out"; then
  ok "gate: 'Part of' a backlog feature is reported as a feature link"
else bad "gate: Part of a feature should be reported" "rc=$rc $out"; fi

g=$(gate_fixture gate-feature-multiline '' "$BL_LINK"); bl_issue "$g" 30 'M1 — first run' $'First real run.\nrelease:  Business \nDue: autumn'
out=$(run "$g" --pr 3); rc=$?
if [ "$rc" -eq 0 ]; then ok "gate: the marker on its own line of a longer description counts, any case"
else bad "gate: multi-line description should count" "rc=$rc $out"; fi

g=$(gate_fixture gate-backlog-plain '' "$BL_LINK"); bl_issue "$g" 30 'v0.1.0' 'Tooling of the backlog itself.'
out=$(run "$g" --pr 3); rc=$?
if [ "$rc" -eq 1 ] && grep -qF "✗ closes $BL#30 but its milestone is \"v0.1.0\", not \"v0.2.0 The gates hold\"" <<<"$out"; then
  ok "gate: a backlog issue in an ordinary milestone still has to match"
else bad "gate: ordinary backlog milestone should fail" "rc=$rc $out"; fi

g=$(gate_fixture gate-marker-elsewhere '' '[{"number":30,"repository":{"name":"other","owner":{"login":"acme"}}}]')
jq -nc --arg d "$BUSINESS" '{milestone:{title:"M1 — first run",description:$d}}' > "$g/issue-acme_other-30.json"
out=$(run "$g" --pr 3); rc=$?
if [ "$rc" -eq 1 ]; then ok "gate: the marker outside the backlog repository does not make a feature"
else bad "gate: marker elsewhere must not exempt" "rc=$rc $out"; fi

g=$(gate_fixture gate-backlog-no-ms '' "$BL_LINK"); bl_issue "$g" 30 ""
out=$(run "$g" --pr 3); rc=$?
if [ "$rc" -eq 1 ]; then ok "gate: a backlog issue without a milestone is not a feature"
else bad "gate: milestone-less backlog issue should fail" "rc=$rc $out"; fi

g=$(gate_fixture gate-feature-no-pr-ms '' "$BL_LINK" ""); bl_issue "$g" 30 'M1 — first run' "$BUSINESS"
out=$(run "$g" --pr 3); rc=$?
if [ "$rc" -eq 1 ] && grep -qF "✗ no milestone" <<<"$out"; then
  ok "gate: a feature link does not replace the PR's own milestone"
else bad "gate: PR without milestone must still fail" "rc=$rc $out"; fi

g=$(fixture audit-feature '[{"number":1,"title":"v0.2.0 The gates hold","state":"open"}]' '[{"number":3,"pull_request":{},"state":"open"}]')
jq -nc --argjson links "$BL_LINK" '{number:3,title:"fix(x): y",state:"OPEN",body:"",closingIssuesReferences:$links,milestone:{title:"v0.2.0 The gates hold"}}' > "$g/pr.json"
bl_issue "$g" 30 'M1 — first run' "$BUSINESS"
out=$(run "$g"); rc=$?
if [ "$rc" -eq 0 ]; then ok "audit: a PR of the milestone that closes a backlog feature is not an error"
else bad "audit: feature link should not be an error" "rc=$rc $out"; fi
bl_issue "$g" 30 'v0.1.0' 'plain'
out=$(run "$g"); rc=$?
if [ "$rc" -eq 1 ] && grep -qF "closes issue $BL#30 but that issue's milestone is 'v0.1.0'" <<<"$out"; then
  ok "audit: a backlog issue in an ordinary milestone is still an error"
else bad "audit: ordinary backlog milestone should fail" "rc=$rc $out"; fi

printf '\nevery PR links a feature of the backlog\n'
OWN_LINK='[{"number":5}]'
strict_pr() { # name, pr body, closing links, [title] -> fixture dir; issue #5 of this repo is in the PR's milestone
  local d; d=$(gate_fixture "$1" "$2" "$3"); printf '{"milestone":{"title":"v0.2.0 The gates hold"}}\n' > "$d/issue.json"
  if [ -n "${4:-}" ]; then jq --arg t "$4" '.title=$t' "$d/pr.json" > "$d/pr2" && mv "$d/pr2" "$d/pr.json"; fi
  printf '%s' "$d"
}

g=$(strict_pr strict-none 'Closes #5' "$OWN_LINK")
out=$(run_strict "$g" --pr 3); rc=$?
if [ "$rc" -eq 1 ] && grep -qF '✗ links no feature of the backlog — add "Part of acme/backlog#<feature>" (maintenance with no feature of its own: the standing feature of the active business milestone, the issue with the label standing)' <<<"$out"; then
  ok "strict: a PR that links no feature fails, with the message that says what to add"
else bad "strict: no feature should fail" "rc=$rc $out"; fi

g=$(strict_pr strict-chore '' '[]' 'chore(x): bump')
out=$(run_strict "$g" --pr 3); rc=$?
if [ "$rc" -eq 1 ] && grep -qF '✗ links no feature of the backlog' <<<"$out"; then
  ok "strict: a PR that needs no issue still needs a feature"
else bad "strict: chore without feature should fail" "rc=$rc $out"; fi

g=$(strict_pr strict-part 'Part of acme/backlog#30' '[]' 'chore(x): bump'); bl_issue "$g" 30 'M1 — first run' "$BUSINESS"
out=$(run_strict "$g" --pr 3); rc=$?
if [ "$rc" -eq 0 ] && grep -qF "✓ links $BL#30 (feature of business milestone" <<<"$out"; then
  ok "strict: Part of a feature passes"
else bad "strict: Part of a feature should pass" "rc=$rc $out"; fi

g=$(strict_pr strict-closes-and-part 'Closes #5
Part of acme/backlog#30' "$OWN_LINK"); bl_issue "$g" 30 'M1 — first run' "$BUSINESS"
out=$(run_strict "$g" --pr 3); rc=$?
if [ "$rc" -eq 0 ] && grep -qF "✓ closes #5 (same milestone)" <<<"$out" && grep -qF "✓ links $BL#30 (feature of business milestone" <<<"$out"; then
  ok "strict: a feature is seen next to a closing link to an issue of this repository"
else bad "strict: closing link plus Part of a feature should pass" "rc=$rc $out"; fi

g=$(strict_pr strict-closing-feature '' "$BL_LINK"); bl_issue "$g" 30 'M1 — first run' "$BUSINESS"
out=$(run_strict "$g" --pr 3); rc=$?
if [ "$rc" -eq 0 ] && ! grep -qF 'links no feature' <<<"$out"; then ok "strict: a closing link to a feature satisfies the rule"
else bad "strict: closing link to a feature should pass" "rc=$rc $out"; fi

g=$(strict_pr strict-not-feature 'Part of acme/backlog#31' '[]' 'chore(x): y'); bl_issue "$g" 31 'v0.1.0' 'Tooling of the backlog itself.'
out=$(run_strict "$g" --pr 3); rc=$?
if [ "$rc" -eq 1 ] && grep -qF '✗ links no feature of the backlog' <<<"$out"; then ok "strict: a backlog issue that is no feature does not satisfy the rule"
else bad "strict: non-feature backlog issue should fail" "rc=$rc $out"; fi

g=$(strict_pr strict-bot '' '[]' 'chore(deps): bump')
jq '.author={login:"dependabot[bot]",is_bot:true}' "$g/pr.json" > "$g/p2" && mv "$g/p2" "$g/pr.json"
out=$(run_strict "$g" --pr 3); rc=$?
if [ "$rc" -eq 0 ] && grep -qF "a bot's PR needs no feature" <<<"$out"; then ok "strict: a bot's PR is exempt"
else bad "strict: bot PR should pass" "rc=$rc $out"; fi

for br in dependabot/go_modules/x update/scalafmt-3.9.0; do
  g=$(strict_pr "strict-branch-${br%%/*}" '' '[]' 'chore(deps): bump'); jq --arg b "$br" '.headRefName=$b' "$g/pr.json" > "$g/p2" && mv "$g/p2" "$g/pr.json"
  out=$(run_strict "$g" --pr 3); rc=$?
  if [ "$rc" -eq 0 ]; then ok "strict: the branch $br is exempt"; else bad "strict: $br should be exempt" "rc=$rc $out"; fi
done

g=$(strict_pr strict-human-update '' '[]' 'chore(x): y'); jq '.headRefName="feat/update-docs"' "$g/pr.json" > "$g/p2" && mv "$g/p2" "$g/pr.json"
out=$(run_strict "$g" --pr 3); rc=$?
if [ "$rc" -eq 1 ]; then ok "strict: a human's branch is not exempt"; else bad "strict: human branch should fail" "rc=$rc $out"; fi

g=$(strict_pr strict-backlog-itself '' '[]' 'chore(x): y')
out=$(GH_FIXTURE="$g" PATH="$tmp/bin:$PATH" REPO="acme/backlog" BACKLOG_REPO="acme/backlog" bash "$script" --pr 3 2>&1); rc=$?
if ! grep -qF 'links no feature' <<<"$out"; then ok "strict: a PR of the backlog repository itself is exempt"
else bad "strict: backlog's own PR should be exempt" "rc=$rc $out"; fi

g=$(fixture strict-audit '[{"number":1,"title":"v0.2.0 The gates hold","state":"open"}]' '[{"number":3,"pull_request":{},"state":"open"}]')
jq -nc '{number:3,title:"chore(x): y",state:"OPEN",body:"",closingIssuesReferences:[],author:{login:"jigarkhwar",is_bot:false},headRefName:"feat/x",milestone:{title:"v0.2.0 The gates hold"}}' > "$g/pr.json"
out=$(run_strict "$g"); rc=$?
if [ "$rc" -eq 0 ] && grep -qF '! PR #3 links no feature of the backlog' <<<"$out"; then ok "strict audit: a PR without a feature is a warning, not an error"
else bad "strict audit: should warn" "rc=$rc $out"; fi

g=$(strict_pr strict-optional '' '[]' 'chore(x): y')
out=$(run "$g" --pr 3); rc=$?
if [ "$rc" -eq 0 ]; then ok "FEATURE_LINK=optional lets a PR without a feature pass"; else bad "optional should pass" "rc=$rc $out"; fi
out=$(GH_FIXTURE="$g" PATH="$tmp/bin:$PATH" REPO="acme/thing" FEATURE_LINK=nonsense bash "$script" --pr 3 2>&1); rc=$?
if [ "$rc" -eq 2 ]; then ok "an unknown FEATURE_LINK value is refused with exit 2"; else bad "nonsense should exit 2" "rc=$rc $out"; fi

printf '\n%d passed, %d failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
