#!/usr/bin/env bash
#
# check-pins_test.sh — tests for check-pins.sh (issue #93).
#
# Hermetic: every case writes a synthetic workflow directory, so nothing here reads the
# repository's own workflows or touches the network.
#
# It does need what the gate needs. Without PyYAML every workflow case reports the gate's
# own exit 2, and the suite printed 37 failures for one missing package — which reads as a
# broken gate rather than an unmet prerequisite, on the loop AGENTS.md documents as the
# local gate. Said once, here, instead.

set -uo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
script="$here/check-pins.sh"

if ! command -v python3 > /dev/null 2>&1 || ! python3 -c 'import yaml' 2>/dev/null; then
  echo "check-pins_test: skipped — check-pins.sh parses the workflows, so this needs" >&2
  echo "                 python3 with PyYAML (pip install PyYAML)" >&2
  exit 0
fi
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

pass=0; fail=0
ok()  { printf '  ✓ %s\n' "$1"; pass=$((pass + 1)); }
bad() { printf '  ✗ %s\n     %s\n' "$1" "$2"; fail=$((fail + 1)); }

# workflow writes one file into a fresh directory and echoes the directory.
workflow() {
  local dir; dir="$(mktemp -d "$tmp/wf.XXXXXX")"
  cat > "$dir/w.yml"
  echo "$dir"
}

sha=0123456789abcdef0123456789abcdef01234567

# ---- a SHA with a version comment passes ------------------------------------
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: owner/action@$sha # v1.2.3
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 0 ] && ok "a SHA with a version comment passes" \
  || bad "a SHA with a version comment passes" "rc=$rc: $out"

# ---- a tag is refused, whoever owns the action ------------------------------
# The rule is about code this repository did not choose, and actions/* is held to it
# too: a half-pinned file invites the next contributor to copy the unpinned line.
for action in owner/action@v4 actions/checkout@v7; do
  d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: $action
YAML
)
  out=$(bash "$script" --dir "$d" 2>&1); rc=$?
  [ $rc -eq 1 ] && grep -q "$action" <<<"$out" \
    && ok "a tag is refused and named ($action)" \
    || bad "a tag is refused and named ($action)" "rc=$rc: $out"
done

# ---- a SHA with no version comment is refused -------------------------------
# The comment is what tells a reader, and Dependabot, which release the SHA is.
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: owner/action@$sha
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && ok "a SHA with no version comment is refused" \
  || bad "a SHA with no version comment is refused" "rc=$rc: $out"

# ---- a local reusable workflow is exempt ------------------------------------
d=$(workflow <<'YAML'
jobs:
  gates:
    uses: ./.github/workflows/verify.yml
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 0 ] && ok "a local reusable workflow needs no SHA" \
  || bad "a local reusable workflow needs no SHA" "rc=$rc: $out"

# ---- an unguarded dispatch input in a run: body is refused ------------------
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: owner/action@$sha # v1.2.3
      - run: |
          go test -fuzztime '\${{ inputs.fuzztime }}' ./...
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && grep -q 'fuzztime' <<<"$out" \
  && ok "an unguarded dispatch input in a run: body is refused" \
  || bad "an unguarded dispatch input in a run: body is refused" "rc=$rc: $out"

# ---- the same value routed through env: passes ------------------------------
# This is the guard record-corpus.yml already carries, and the only one this
# repository uses.
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: owner/action@$sha # v1.2.3
      - env:
          FUZZTIME: \${{ inputs.fuzztime }}
        run: |
          set -euo pipefail
          printf '%s' "\$FUZZTIME" | grep -qE '^[0-9]+[smh]\$' || exit 1
          go test -fuzztime "\$FUZZTIME" ./...
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 0 ] && ok "a value routed through env: passes" \
  || bad "a value routed through env: passes" "rc=$rc: $out"

# ---- a job named "run" is not a run: body -----------------------------------
# Found the hard way: a job id is indistinguishable from a step key by shape, and
# this repository has a job named run.
d=$(workflow <<YAML
jobs:
  run:
    strategy:
      matrix:
        version: \${{ fromJSON(inputs.versions) }}
    steps:
      - uses: owner/action@$sha # v1.2.3
        with:
          key: cache-\${{ matrix.version }}
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 0 ] && ok "a job named run, and a with: key, are not a script" \
  || bad "a job named run, and a with: key, are not a script" "rc=$rc: $out"

# ---- any substitution in a run: body is refused, whatever context it names ---
# The first version of this rule listed the contexts it distrusted, inputs. and
# matrix., and every other one walked past it: a pull request title, a head ref, a
# step output. The hazard is textual substitution into a script, so the rule is
# about where the value lands, never about what it is called.
for ctx in 'github.event.pull_request.title' 'github.head_ref' 'steps.assemble.outputs.entry' 'needs.build.outputs.tag'; do
  d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: owner/action@$sha # v1.2.3
      - run: |
          echo "\${{ $ctx }}"
YAML
)
  out=$(bash "$script" --dir "$d" 2>&1); rc=$?
  [ $rc -eq 1 ] && printf '%s' "$out" | grep -q "$ctx" \
    && ok "a run: body is refused whatever it interpolates ($ctx)" \
    || bad "a run: body is refused whatever it interpolates ($ctx)" "rc=$rc: $out"
done

# ---- the run: line is a script too ------------------------------------------
# The rule recognised the run: key and then skipped the line it was on, so the
# one-line form of the very payload the block form refused walked through.
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: owner/action@$sha # v1.2.3
      - run: go test -fuzztime '\${{ inputs.fuzztime }}' ./...
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'fuzztime' \
  && ok "a substitution on the run: line itself is refused" \
  || bad "a substitution on the run: line itself is refused" "rc=$rc: $out"

# ---- a step whose first key is run: still owns its env: ---------------------
# The false positive that matters most, because it refused the guard the rule asks
# for: with the dash on the run: line every sibling key sits further right than the
# dash, and measuring the line instead of the key read env: as script.
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: owner/action@$sha # v1.2.3
      - run: |
          set -euo pipefail
          printf '%s' "\$FUZZTIME" | grep -qE '^[0-9]+[smh]\$' || exit 1
        env:
          FUZZTIME: \${{ inputs.fuzztime }}
        if: always()
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 0 ] && ok "a step written run:-first keeps its own env: and if:" \
  || bad "a step written run:-first keeps its own env: and if:" "rc=$rc: $out"

# ---- a uses: the line anchor could not see ----------------------------------
# All three are legal YAML that GitHub runs. A flow-style steps: was the worst:
# it failed the anchor, so insteps was never set and rule 2 went quiet for the
# whole job while rule 1 passed the unpinned action inside it.
while IFS='|' read -r what line; do
  [ -n "$what" ] || continue
  d=$(workflow <<YAML
jobs:
  build:
    steps:
$line
YAML
)
  out=$(bash "$script" --dir "$d" 2>&1); rc=$?
  [ $rc -eq 1 ] && ok "an unpinned action is refused when written as $what" \
    || bad "an unpinned action is refused when written as $what" "rc=$rc: $out"
done <<CASES
a flow mapping|      - {uses: owner/action@v4}
a quoted key|      - "uses": owner/action@v4
a key with a space before the colon|      - uses : owner/action@v4
CASES

# ---- a decoy SHA in a comment is not a pin ----------------------------------
# The regex looked for 40 hex anywhere on the line, so a note recording what the
# pin used to be read as the pin itself — the one shape a reviewer skims past.
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: owner/action@main # was @$sha # v1.2.3
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && ok "a 40-hex decoy in a trailing comment does not pass as a pin" \
  || bad "a 40-hex decoy in a trailing comment does not pass as a pin" "rc=$rc: $out"

# ---- an explicit-key mapping is a mapping ------------------------------------
# The line-matching version of this gate required a key and its colon on one line,
# so `? uses` / `: actions/evil@main` — YAML's explicit-key notation, which PyYAML,
# Psych, go-yaml and actionlint all resolve to exactly the same mapping as the
# ordinary spelling — passed both rules at once: an unpinned action AND an
# untrusted commit message reaching a shell, reported as "all pinned and guarded".
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - ? uses
        : owner/action@v4
      - ? run
        : echo "\${{ github.event.head_commit.message }}"
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'owner/action@v4' \
  && printf '%s' "$out" | grep -q 'head_commit' \
  && ok "an explicit-key step is read, by both rules" \
  || bad "an explicit-key step is read, by both rules" "rc=$rc: $out"

# ---- a key spelled with an escape is still that key -------------------------
# "uses" is "uses". A parser resolves the escape; a regular expression sees
# a different word and walks past the action.
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - "\\u0075ses": owner/action@v4
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && ok "a uses: key written with a unicode escape is still checked" \
  || bad "a uses: key written with a unicode escape is still checked" "rc=$rc: $out"

# ---- a flow mapping is read, not refused ------------------------------------
# Legal YAML that GitHub runs, so the gate reads it: pinned passes, unpinned does
# not. The version comment sits outside the braces, where YAML allows a comment.
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - {name: pinned, uses: owner/action@$sha} # v1.2.3
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 0 ] && ok "a correctly pinned step in a flow mapping passes" \
  || bad "a correctly pinned step in a flow mapping passes" "rc=$rc: $out"

# ---- the text "uses:" inside a scalar is not an action ----------------------
# The regular-expression version refused a workflow because a step was NAMED
# "assert that nothing uses: a floating tag", reporting the English phrase as an
# unpinned action. A gate that refuses correct work gets switched off.
d=$(workflow <<YAML
name: prose
on: push
env:
  HINT: "write uses: owner/action@<sha> # vX.Y.Z"
jobs:
  build:
    steps:
      - name: "assert that nothing uses: a floating tag"
        run: |
          cat > fixture.yml <<'EOF'
          steps:
            - uses: actions/checkout@v4
          EOF
      - uses: owner/action@$sha # v1.2.3
        with:
          uses: "a with: input may be called anything"
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 0 ] && ok "uses: in a step name, an env string, a heredoc and a with: key is not an action" \
  || bad "uses: in a step name, an env string, a heredoc and a with: key is not an action" "rc=$rc: $out"

# ---- carriage returns do not hide a workflow --------------------------------
# A file with lone \r line endings is one line to a line reader and a whole
# workflow to a parser.
d="$(mktemp -d "$tmp/cr.XXXXXX")"
printf 'name: cr\ron: push\rjobs:\r  build:\r    steps:\r      - uses: owner/action@v4\r' > "$d/w.yml"
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'owner/action@v4' \
  && ok "lone carriage returns do not hide an unpinned action" \
  || bad "lone carriage returns do not hide an unpinned action" "rc=$rc: $out"

# ---- github-script runs its input, so its input is a script -----------------
# actions/github-script evaluates with.script as JavaScript on the runner. The
# hazard is the same as a run: body's; only the interpreter differs.
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: actions/github-script@$sha # v7.0.1
        with:
          script: |
            core.info("\${{ github.event.pull_request.title }}")
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'pull_request.title' \
  && ok "a substitution in github-script's script: input is refused" \
  || bad "a substitution in github-script's script: input is refused" "rc=$rc: $out"

# ---- a file the parser cannot read is refused, not skipped ------------------
# The whole reason to parse: a gate that reports success on input it never read
# is worse than no gate.
d="$(mktemp -d "$tmp/bad.XXXXXX")"
printf 'jobs:\n  build:\n    steps: [ {uses: owner/a@v4\n' > "$d/w.yml"
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'cannot read' \
  && ok "a file that is not YAML is refused as unreadable" \
  || bad "a file that is not YAML is refused as unreadable" "rc=$rc: $out"

# ---- the job's container is code too ----------------------------------------
# Found by attacking the parser: the walk read jobs.<id>.{uses,steps} and nothing
# else, so `container: alpine:latest` — the filesystem and interpreter every run:
# in the job executes inside — passed as "all pinned and guarded", while the same
# image reached through `uses: docker://alpine:latest` was refused. A service is
# the same hazard with a second name.
# Each case is its own block rather than a row in a table: a table would have to fold
# a multi-line YAML body onto one line, and an expansion that degrades to invalid YAML
# would still be refused — the test would pass without ever reaching the rule. Every
# case names the image it expects in the output for the same reason.
image_refused() { # image_refused <claim> <expected text in output>
  local out rc
  out=$(bash "$script" --dir "$1" 2>&1); rc=$?
  [ $rc -eq 1 ] && printf '%s' "$out" | grep -q "$3" \
    && ok "an unpinned image is refused in $2" \
    || bad "an unpinned image is refused in $2" "rc=$rc: $out"
}

d=$(workflow <<YAML
jobs:
  build:
    container:
      image: alpine:latest
    steps:
      - uses: owner/action@$sha # v1.2.3
YAML
)
image_refused "$d" "container (mapping)" 'alpine:latest in container.image'

d=$(workflow <<YAML
jobs:
  build:
    container: alpine:latest
    steps:
      - uses: owner/action@$sha # v1.2.3
YAML
)
image_refused "$d" "container (shorthand)" 'alpine:latest in container'

d=$(workflow <<YAML
jobs:
  build:
    services:
      db:
        image: postgres:latest
    steps:
      - uses: owner/action@$sha # v1.2.3
YAML
)
image_refused "$d" "a service" 'postgres:latest in services.db.image'

d=$(workflow <<YAML
jobs:
  build:
    container:
      image: ghcr.io/x/y:\${{ inputs.tag }}
    steps:
      - uses: owner/action@$sha # v1.2.3
YAML
)
image_refused "$d" "a container substitution" 'a substitution cannot be pinned'

# ---- a docker:// action pins by digest, and that counts as pinned -----------
# A uses: may be either form; an image may only be a digest. Folding the two pin
# checks into one helper briefly passed a single form for both, which refused a
# correctly digest-pinned docker:// action — no test covered it, so nothing said so.
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: docker://alpine@sha256:$(printf '0123456789abcdef%.0s' 1 2 3 4) # v3.18
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 0 ] && ok "a digest-pinned docker:// action passes" \
  || bad "a digest-pinned docker:// action passes" "rc=$rc: $out"

d=$(workflow <<YAML
jobs:
  build:
    container:
      image: alpine@$sha
    steps:
      - uses: owner/action@$sha # v1.2.3
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && ok "a commit SHA is not a digest, so an image cannot be pinned with one" \
  || bad "a commit SHA is not a digest, so an image cannot be pinned with one" "rc=$rc: $out"

# ---- an action name is case-insensitive, so the check must be too ------------
# GitHub repository names are case-insensitive: actions/GitHub-Script is the same
# action. An exact comparison let it through with its script: input unchecked —
# which github-script evaluates as JavaScript on the runner.
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: actions/GitHub-Script@$sha # v7.0.1
        with:
          script: core.info("\${{ github.event.pull_request.title }}")
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && ok "github-script's script: is checked whatever the action's casing" \
  || bad "github-script's script: is checked whatever the action's casing" "rc=$rc: $out"

# ---- a docker:// step's args are argv, not data -----------------------------
# entrypoint: and args: are handed to the container as argv, so a substitution
# there reaches an interpreter exactly as one in a run: body does.
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: docker://alpine@sha256:$(printf '0123456789abcdef%.0s' 1 2 3 4) # v3.18
        with:
          args: \${{ github.event.pull_request.title }}
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && ok "a substitution in a docker:// step's args: is refused" \
  || bad "a substitution in a docker:// step's args: is refused" "rc=$rc: $out"

# ---- a mapping the gate will not guess about is refused ---------------------
# PyYAML takes the last of two duplicate keys and does not expand a << merge at
# all. Nothing this repository can cite says GitHub agrees, so the gate refuses
# rather than issuing a verdict about a workflow that may not be the one that runs.
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: owner/action@$sha # v1.2.3
        uses: owner/evil@main
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'duplicate' \
  && ok "a duplicate key in a step is refused, not resolved" \
  || bad "a duplicate key in a step is refused, not resolved" "rc=$rc: $out"

d=$(workflow <<YAML
x: &base
  steps:
    - uses: owner/action@v4
jobs:
  build:
    <<: *base
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'merge key' \
  && ok "a job assembled with a merge key is refused, not skipped" \
  || bad "a job assembled with a merge key is refused, not skipped" "rc=$rc: $out"

# ---- a correctly pinned line is not refused for how its comment reads -------
# Three shapes the first parser refused. Every one is a real pin, and a gate that
# refuses correct work is a gate someone switches off: git resolves an upper-case
# SHA, several actions are released as a bare major, and a tool that annotates the
# line it maintains has not removed the version.
while IFS='|' read -r what line; do
  [ -n "$what" ] || continue
  d=$(workflow <<YAML
jobs:
  build:
    steps:
$line
YAML
)
  out=$(bash "$script" --dir "$d" 2>&1); rc=$?
  [ $rc -eq 0 ] && ok "a pin is accepted with $what" \
    || bad "a pin is accepted with $what" "rc=$rc: $out"
done <<CASES
an upper-case SHA|      - uses: owner/action@0123456789ABCDEF0123456789ABCDEF01234567 # v1.2.3
a bare major version|      - uses: owner/action@$sha # v4
an annotated comment|      - uses: owner/action@$sha # pin v1.2.3 (renovate)
CASES

# ---- the two mappings the walk enters through are checked like the rest ------
# Found by reviewing the parser: the duplicate-and-merge guard was wired into every
# job, step, with:, container: and services: mapping and into neither the document
# root nor jobs:. A second jobs: key dropped every job in the first block, and a <<
# at the root made the whole file invisible — both printing "all pinned and guarded".
d=$(workflow <<YAML
jobs:
  evil:
    steps:
      - uses: actions/evil@main
jobs:
  good:
    steps:
      - uses: owner/action@$sha # v1.2.3
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'duplicate jobs' \
  && ok "a duplicate jobs: key at the root is refused, not resolved" \
  || bad "a duplicate jobs: key at the root is refused, not resolved" "rc=$rc: $out"

d=$(workflow <<YAML
base: &b
  jobs:
    evil:
      steps:
        - uses: actions/evil@main
<<: *b
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'merge key' \
  && ok "a merge key at the root is refused, not passed as an empty workflow" \
  || bad "a merge key at the root is refused, not passed as an empty workflow" "rc=$rc: $out"

# ---- a shape no checker can read is refused by every checker -----------------
# check_script used to return silently for a value that was not a single scalar,
# where its two siblings reported it — so the one rule that is "any substitution at
# all" was the one that passed an unreadable shape without a word.
unreadable_refused() { # unreadable_refused <dir> <claim> <expected text in output>
  local out rc
  out=$(bash "$script" --dir "$1" 2>&1); rc=$?
  [ $rc -eq 1 ] && printf '%s' "$out" | grep -q "$3" \
    && ok "an unreadable $2 is refused, not skipped" \
    || bad "an unreadable $2 is refused, not skipped" "rc=$rc: $out"
}

d=$(workflow <<'YAML'
jobs:
  build:
    steps:
      - run:
          - echo hi
YAML
)
unreadable_refused "$d" "run: value" 'a run whose value is not a single scalar'

d=$(workflow <<'YAML'
jobs:
  build:
    - not: a mapping
YAML
)
unreadable_refused "$d" "job" 'a job that is not a mapping'

d=$(workflow <<'YAML'
jobs:
  build:
    steps: not-a-sequence
YAML
)
unreadable_refused "$d" "steps: value" 'a steps: that is not a sequence'

# ---- a key the gate does not know is refused --------------------------------
# The walk fails closed now. Twice a sink nobody had enumerated — a job container:,
# a docker:// step's args: — sat outside it and was reported as guarded, so an
# unknown key costs a refusal and a decision instead of a silent hole.
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: owner/action@$sha # v1.2.3
        exec-in: some-future-sandbox
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'does not know' \
  && ok "a step key the gate does not know is refused, not ignored" \
  || bad "a step key the gate does not know is refused, not ignored" "rc=$rc: $out"

# ---- only a real comment can carry the version ------------------------------
# The version was matched against the whole remainder of the line. In flow style
# that remainder is ordinary YAML, so a # inside a later quoted scalar supplied the
# version a pin is required to carry; and any dotted number counted, which is how a
# date is written.
while IFS='|' read -r what line; do
  [ -n "$what" ] || continue
  d=$(workflow <<YAML
jobs:
  build:
    steps:
$line
YAML
)
  out=$(bash "$script" --dir "$d" 2>&1); rc=$?
  [ $rc -eq 1 ] && ok "a pin is refused when its version comes from $what" \
    || bad "a pin is refused when its version comes from $what" "rc=$rc: $out"
done <<CASES
a quoted scalar in a flow mapping|      - {uses: owner/a@$sha, name: "bump # v1.0.0 later"}
a dotted date|      - uses: owner/a@$sha # pinned 2024.01.15
CASES

# ---- a digest is lower case, a commit SHA is either -------------------------
# The two were widened together, and only one of them should have been: git
# resolves an upper-case SHA, while the OCI spec restricts a sha256 digest to
# lower case, so accepting it greened a reference the runner cannot resolve.
d=$(workflow <<YAML
jobs:
  build:
    container:
      image: alpine@sha256:0123456789ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef # v3.18
    steps:
      - uses: owner/a@$sha # v1.2.3
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && ok "an upper-case sha256 digest is refused" \
  || bad "an upper-case sha256 digest is refused" "rc=$rc: $out"

# ---- a line separator PyYAML counts and split("\n") does not ----------------
# U+2028 ends a line for the scanner — legally, inside a quoted scalar — but not for
# open(newline=None), so every node mark after one was off by one and the comment was
# read off the wrong line. Measured on this fixture: the version comment resolves to
# " # v1.2.3" when the text is split on every break PyYAML counts, and to "" when it
# is split on newline alone, which refuses a correctly pinned action.
d="$(mktemp -d "$tmp/ls.XXXXXX")"
printf 'jobs:\n  build:\n    steps:\n      - name: "two\xe2\x80\xa8lines"\n        run: echo hi\n      - uses: owner/a@%s # v1.2.3\n' "$sha" > "$d/w.yml"
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 0 ] && ok "a U+2028 earlier in the file does not shift the comment lookup" \
  || bad "a U+2028 earlier in the file does not shift the comment lookup" "rc=$rc: $out"

# ---- findings are one record each -------------------------------------------
# report() trimmed its text but kept interior newlines, so a scalar carrying one
# split into a second log record that read as a finding the gate never made.
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: "owner/a@v4\nw.yml:1: owner/forged@v9"
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && [ "$(printf '%s' "$out" | grep -c 'w.yml:')" -eq 1 ] \
  && ok "a newline inside a value cannot forge a second finding" \
  || bad "a newline inside a value cannot forge a second finding" "rc=$rc: $out"

# ---- a file that is not UTF-8 is refused ------------------------------------
# It used to be read with errors="replace", so the gate issued a verdict about a
# string that was not the file — in a script whose stated policy is that what it
# cannot read it refuses.
d="$(mktemp -d "$tmp/enc.XXXXXX")"
printf 'jobs:\n  build:\n    steps:\n      - name: "\xff\xfe"\n        run: echo hi\n' > "$d/w.yml"
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'not UTF-8' \
  && ok "a file that is not UTF-8 is refused, not silently repaired" \
  || bad "a file that is not UTF-8 is refused, not silently repaired" "rc=$rc: $out"

# ---- a file that breaks the parser is a refusal, not a traceback ------------
# Two thousand nested flow collections raise RecursionError, which is not a
# yaml.YAMLError, so the except inside check() never saw it: the contributor got a
# Python traceback where a verdict belongs, and because findings are printed after
# every file is walked, the findings of the files before it were lost with it.
d="$(mktemp -d "$tmp/deep.XXXXXX")"
printf 'jobs:\n  a:\n    steps:\n      - uses: owner/unpinned@v4\n' > "$d/aaa.yml"
python3 -c "
import sys
open(sys.argv[1],'w').write('jobs:\n  build:\n    steps:\n      - name: '+'['*2000+']'*2000+'\n')
" "$d/zzz.yml"
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'broke the gate' \
  && printf '%s' "$out" | grep -q 'owner/unpinned@v4' \
  && ok "a file that breaks the parser is refused, and the other files' findings survive" \
  || bad "a file that breaks the parser is refused, and the other files' findings survive" "rc=$rc: $out"

# ---- a self-referential anchor does not hang the walk -----------------------
# The node graph PyYAML composes can be cyclic. Nothing in the walk counts depth;
# what stops it is that the aliased mapping reaches unreadable() with keys no job
# carries, and a refusal returns before recursing. Pinned, because that is luck
# turned into behaviour and the next edit could spend it.
d=$(workflow <<'YAML'
jobs: &j
  build:
    steps:
      - uses: owner/a@v4
  self: *j
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && ok "a self-referential anchor is refused rather than walked forever" \
  || bad "a self-referential anchor is refused rather than walked forever" "rc=$rc: $out"

# ---- a .yaml workflow is read too, and every file in the directory ----------
# Every case until now wrote exactly one file, always w.yml, so the .yaml arm of
# the glob and the loop over more than one path were asserted nowhere.
d="$(mktemp -d "$tmp/two.XXXXXX")"
printf 'jobs:\n  a:\n    steps:\n      - uses: owner/a@%s # v1.2.3\n' "$sha" > "$d/first.yml"
printf 'jobs:\n  b:\n    steps:\n      - uses: owner/b@v4\n' > "$d/second.yaml"
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'second.yaml' \
  && ok "a .yaml workflow beside a .yml one is read as well" \
  || bad "a .yaml workflow beside a .yml one is read as well" "rc=$rc: $out"

# ---- the verdict goes to stdout and the findings to stderr ------------------
# The script's own design rests on that split, and every case above captures both
# together, so nothing asserted it.
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: owner/action@$sha # v1.2.3
YAML
)
o=$(bash "$script" --dir "$d" 2>/dev/null); rc=$?
[ $rc -eq 0 ] && printf '%s' "$o" | grep -q 'every reference pinned' \
  && ok "the passing verdict goes to stdout" \
  || bad "the passing verdict goes to stdout" "rc=$rc: $o"

d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: owner/action@v4
YAML
)
o=$(bash "$script" --dir "$d" 2>/dev/null); e=$(bash "$script" --dir "$d" 2>&1 >/dev/null)
[ -z "$o" ] && printf '%s' "$e" | grep -q 'not pinned' \
  && ok "findings go to stderr and nothing to stdout" \
  || bad "findings go to stderr and nothing to stdout" "stdout=$o"

# ---- --help and an unknown argument ----------------------------------------
# Neither was asserted, and --help is where the gate's scope limits are written.
out=$(bash "$script" --help 2>&1); rc=$?
[ $rc -eq 0 ] && printf '%s' "$out" | grep -q 'composite action' \
  && ok "--help exits 0 and states the scope the gate does not cover" \
  || bad "--help exits 0 and states the scope the gate does not cover" "rc=$rc"

out=$(bash "$script" --nonsense 2>&1); rc=$?
[ $rc -eq 2 ] && ok "an unknown argument exits 2" \
  || bad "an unknown argument exits 2" "rc=$rc: $out"

# ---- --dir with no value is a usage error, not a silent refusal -------------
# The second shift had nothing to remove, and set -e turned that into exit 1 with
# no output: a usage mistake reported as a failing gate.
out=$(bash "$script" --dir 2>&1); rc=$?
[ $rc -eq 2 ] && printf '%s' "$out" | grep -q 'needs a directory' \
  && ok "--dir with no value exits 2 and says why" \
  || bad "--dir with no value exits 2 and says why" "rc=$rc: $out"

# ---- a uses: inside a heredoc is prose, not a step --------------------------
# verify.yml inlines a Python heredoc inside a run: body, and this file writes
# workflows from heredocs. Rule 1 must not read a step out of either.
d=$(workflow <<YAML
jobs:
  build:
    steps:
      - uses: owner/action@$sha # v1.2.3
      - run: |
          cat > w.yml <<'EOF'
          - uses: owner/other@v4
          EOF
YAML
)
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 0 ] && ok "a uses: inside a heredoc is not read as a step" \
  || bad "a uses: inside a heredoc is not read as a step" "rc=$rc: $out"

# ---- a directory with no workflow is a usage error, not a pass --------------
# An empty run is a broken gate, not a clean one.
d="$(mktemp -d "$tmp/empty.XXXXXX")"
out=$(bash "$script" --dir "$d" 2>&1); rc=$?
[ $rc -eq 2 ] && ok "a directory with no workflow exits 2" \
  || bad "a directory with no workflow exits 2" "rc=$rc: $out"

printf '\ncheck-pins_test: %d passed, %d failed\n' "$pass" "$fail"
[ "$fail" -eq 0 ]
