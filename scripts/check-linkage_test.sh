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
  "issue view "*) emit < "$GH_FIXTURE/issue.json" ;;
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

run() { # run <fixture-dir> <args...>
  GH_FIXTURE="$1" PATH="$tmp/bin:$PATH" REPO="acme/thing" bash "$script" "${@:2}" 2>&1
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
out=$(GH_FIXTURE="$r" PATH="$tmp/bin:$PATH" REPO="acme/thing" bash "$script" --for-tag v0.0.2 2>&1)
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

printf '\n%d passed, %d failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
