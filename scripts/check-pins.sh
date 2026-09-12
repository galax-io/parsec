#!/usr/bin/env bash
#
# check-pins.sh — hold the workflows to a pinned supply chain and guarded inputs.
# Run `scripts/check-pins.sh --help` for full usage.

set -euo pipefail

usage() {
  cat <<'USAGE'
check-pins.sh — refuse a workflow that can be moved under the release, or that hands a
substituted value to a shell.

Two rules, both about code this repository did not choose running with the permissions
this repository grants.

  1. Every `uses:` naming a third-party action names a 40-character commit SHA, with the
     version in a trailing comment. A git tag is movable: release.yml grants its publish
     job `contents: write` and a token that can cut releases, so a moved upstream tag runs
     whatever it now points at, with no access to this repository needed. actions/* is held
     to the same rule — a half-pinned file invites the next contributor to copy the
     unpinned line. A `docker://` image pins by digest instead, `@sha256:` and 64 hex, for
     the same reason and with the same comment. A local `./…` call is exempt: it is this
     repository. `../` is not local, and is refused.

     A job's `container:` and every `services.<id>.image:` are held to the same rule, by
     digest. A step's action is one step's code; the job's container is the filesystem and
     the interpreter that every `run:` in the job executes inside, so a movable tag there is
     worth more to an attacker than a movable action, not less.

     The SHA is read from the parsed value, so no comment can supply it: a trailing
     `# was @<sha> # v1.2.3` note reads like a pin history and would otherwise launder an
     unpinned line past a reviewer. The version may sit anywhere in the comment — `# v4` and
     `# pin v1.2.3 (renovate)` both count — and hex is accepted in either case, because
     refusing a correctly pinned line is how a gate gets switched off. And no `${{ … }}` may
     appear in a `uses:` or an image value at all, including in the owner/repo part — a SHA
     pins content within a repository, it does not pin which repository.

  2. No `${{ … }}` substitution of any kind appears in a `run:` script, in the `script:`
     input of actions/github-script, which evaluates it as JavaScript, or in the `args:` or
     `entrypoint:` of a `docker://` step, which become argv for the container. Not a narrower
     list of contexts: any of them. GitHub substitutes textually before bash parses the line, so
     a value carrying a quote closes the quoting and runs arbitrary commands on the runner —
     and whether that happens is a property of the *value*, which this gate cannot see,
     never of the context's name. A context named here today as safe (`github.event_name` is
     an enum, `steps.x.outputs.y` is a literal in the step that wrote it) stops being safe in
     a later commit that this line's diff does not touch.

     There is no allowlist because there is nothing an allowlist could buy. Every context
     can be bound in `env:` and read back as a quoted `$VAR`, which crosses into the shell
     through a slot the parser never re-reads; many are already exported by the runner
     (`$GITHUB_EVENT_NAME`, `$RUNNER_OS`, `$RUNNER_TEMP`, `$GITHUB_SHA`) and cost nothing
     at all. An entry would only ever permit a spelling strictly worse than one that always
     exists. `${{ … }}` is still free everywhere it does not reach a shell: `env:`, `with:`,
     `if:`, `name:`, and every job- or workflow-level key. Route it through `env:` and
     validate it, as record-corpus.yml does.

A mapping the gate will not guess about is refused: a duplicate key, because PyYAML takes
the last value and nothing this repository can cite says GitHub does, and a `<<` merge key,
because the node graph does not expand one, so a step reached through it would be invisible.

Both rules read the parsed YAML, not the lines. The first version of this gate matched
`uses:` and `run:` with regular expressions, and a line matcher cannot tell a key from the
same text inside a scalar: it refused a step named "assert that nothing uses: a floating
tag", and it passed `? uses` / `: actions/evil@main`, which is the explicit-key spelling of
the same mapping and which GitHub runs. Both failures are one defect, and parsing is the
only fix for it. A file this gate cannot parse is refused, not skipped.

Scope: the files directly in DIR. A composite action's own `action.yml` elsewhere in the
tree is not read by this gate, and `uses: ./.github/actions/...` is exempt by rule 1, so
a local composite action is unpinned territory. Say so before adding one.

Requires python3 with PyYAML, which is what parsing costs. verify.yml installs it pinned.

Usage:
  scripts/check-pins.sh [--dir DIR]

  --dir DIR   the workflow directory (default: .github/workflows)

Writes a line to $GITHUB_STEP_SUMMARY when that is set.

Exit: 0 every workflow passes | 1 one does not | 2 usage error, no workflow was found, or
the parser is missing.
USAGE
}

DIR=.github/workflows

while [ $# -gt 0 ]; do
  case "$1" in
    # $# is tested before the value is taken: "--dir" as the last argument used to
    # leave the second shift with nothing to remove, which under set -e exits 1
    # silently — a usage error reported as a refusal, with no message at all.
    --dir)
      [ $# -ge 2 ] || { echo "check-pins: --dir needs a directory" >&2; exit 2; }
      DIR="$2"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "check-pins: unknown argument $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

[ -d "$DIR" ] || { echo "check-pins: $DIR is not a directory" >&2; exit 2; }

shopt -s nullglob
files=("$DIR"/*.yml "$DIR"/*.yaml)
shopt -u nullglob

[ ${#files[@]} -gt 0 ] || { echo "check-pins: no workflow found in $DIR" >&2; exit 2; }

command -v python3 > /dev/null 2>&1 \
  || { echo "check-pins: python3 is needed to parse the workflows" >&2; exit 2; }

python3 -c 'import yaml' 2>/dev/null \
  || { echo "check-pins: PyYAML is needed to parse the workflows (pip install PyYAML)" >&2; exit 2; }

# The checker prints its own findings, so that a workflow line containing a tab or a
# newline cannot forge or split a record on the way back through the shell. It exits 0
# or 1, and the summary line below is the only thing this file adds to its verdict.
status=0
python3 - "${files[@]}" <<'PYEOF' || status=$?
import re
import sys

import yaml

# Hex is accepted in either case for a commit SHA: git resolves an upper-case one and
# GitHub serves it, so refusing one refuses a correctly pinned workflow. A sha256 digest
# is not the same: the OCI distribution spec restricts it to lower case, so accepting
# upper case there would green a reference the runner cannot resolve.
SHA_PIN = re.compile(r"^[^@\s]+@[0-9a-fA-F]{40}$")
DIGEST_PIN = re.compile(r"^[^@\s]+@sha256:[0-9a-f]{64}$")

# The version may sit anywhere in the comment, not only at its head: `# pin v1.2.3
# (renovate)` is the same promise as `# v1.2.3`, and a tool that annotates the line it
# maintains must not be read as having removed the version. The `v` is required, and that
# is the whole rule — a bare dotted number is how a date is written, and a date is the
# commonest thing to find in a comment that is not a version. No comment can supply the
# ref itself: that is read from the parsed value.
VERSION_COMMENT = re.compile(r"\bv\d+(?:\.\d+)*\b")
SUBSTITUTION = re.compile(r"\$\{\{")

# PyYAML's scanner ends a line on any of these, and open(newline=None) translates only the
# first three, so splitting on "\n" alone would shift every node mark after the first of
# the others and read a comment off the wrong line.
LINE_BREAK = re.compile("\n|\x85| | ")

# A step's code can reach an interpreter through more than run:. actions/github-script
# evaluates its script: input as JavaScript, and a docker:// step's entrypoint and args
# become argv for the container. Matched case-folded: GitHub repository names are
# case-insensitive, so actions/GitHub-Script is the same action and an exact comparison
# let it through with its script unchecked.
SCRIPT_INPUTS = {"actions/github-script": ("script",)}
ARGV_INPUTS = ("args", "entrypoint")

# The walk fails closed. Every key a job or a step may carry is named here, as either
# checked or inert, and anything else is refused — because the alternative is what this
# gate has already done twice: a sink nobody enumerated (a job container:, a service
# image:, a docker:// step's args:) sat outside the walk and was reported as guarded. A
# key GitHub adds next year now costs one line and a decision, announced by a refusal,
# instead of a silent hole. Inert means the value never reaches a shell, an interpreter or
# a fetch: Actions evaluates it itself, or it is data the runner consumes.
JOB_CHECKED = ("uses", "container", "services", "steps")
JOB_INERT = (
    "concurrency", "continue-on-error", "defaults", "env", "environment", "if", "name",
    "needs", "outputs", "permissions", "runs-on", "secrets", "strategy",
    "timeout-minutes", "with",
)
STEP_CHECKED = ("uses", "run", "with")
STEP_INERT = (
    "continue-on-error", "env", "id", "if", "name", "shell", "timeout-minutes",
    "working-directory",
)

# A mapping this gate will not guess about. PyYAML resolves neither, and what GitHub does
# with them is not written down anywhere this repository can cite: a duplicate key takes
# the last value here and may take the first there, and `<<` is a YAML merge that the node
# graph does not expand, so a step reached through one is invisible to the walk.
MERGE_KEY = "<<"

findings = {"PIN": [], "RUN": [], "SHAPE": []}


def report(kind, path, line, text):
    # Interior whitespace is collapsed, not only trimmed. A scalar carrying a newline would
    # otherwise split into two log records, and the second would read as a finding this
    # gate never made — the forging the single-writer design is meant to rule out.
    findings[kind].append("%s:%d: %s" % (path, line + 1, " ".join(text.split())))


def entries(node):
    """The scalar-keyed entries of a mapping node, as {key: value node}."""
    if not isinstance(node, yaml.MappingNode):
        return {}
    out = {}
    for key, value in node.value:
        if isinstance(key, yaml.ScalarNode):
            out[key.value] = value
    return out


def unreadable(path, node, what, checked=(), inert=()):
    """Refuse a mapping this gate will not guess about, or whose keys it does not know.
    True when it refused. checked and inert together are the complete key vocabulary;
    pass neither to check only for duplicates and merges."""
    if not isinstance(node, yaml.MappingNode):
        return False

    known = set(checked) | set(inert)
    seen, bad = set(), False

    for key, _ in node.value:
        if not isinstance(key, yaml.ScalarNode):
            report("SHAPE", path, key.start_mark.line,
                   "a %s with a key that is not a scalar; this gate cannot read it" % what)
            bad = True
            continue

        name = key.value
        if name == MERGE_KEY:
            report("SHAPE", path, key.start_mark.line,
                   "a %s assembled with a YAML merge key; write the keys out" % what)
            bad = True
        elif name in seen:
            report("SHAPE", path, key.start_mark.line,
                   "a %s with a duplicate %s: key; which one runs is not this gate's to guess"
                   % (what, name))
            bad = True
        elif known and name not in known:
            report("SHAPE", path, key.start_mark.line,
                   "a %s carrying %s:, which this gate does not know; say in check-pins.sh "
                   "whether it can reach an interpreter or a fetch" % (what, name))
            bad = True
        seen.add(name)

    return bad


def comment(lines, node):
    """The trailing comment on the line a scalar ended on, or "".

    Not the remainder of the line: in flow style that remainder is ordinary YAML, and a
    `#` inside a later quoted scalar of the same mapping would otherwise supply the
    version a pin is required to carry. The scan tracks quoting so that only a `#` which
    actually opens a comment counts.
    """
    if node.end_mark.line != node.start_mark.line:
        return ""
    if node.end_mark.line >= len(lines):
        return ""

    tail = lines[node.end_mark.line][node.end_mark.column:]
    quote = ""
    for i, ch in enumerate(tail):
        if quote:
            if ch == quote:
                quote = ""
        elif ch in "\"'":
            quote = ch
        elif ch == "#" and (i == 0 or tail[i - 1] in " \t"):
            return tail[i:]

    return ""


def versioned(lines, node):
    return VERSION_COMMENT.search(comment(lines, node)) is not None


def scalar(path, node, what):
    """The node's text, or None after reporting that the gate cannot read its shape."""
    if isinstance(node, yaml.ScalarNode):
        return node.value.strip()

    report("SHAPE", path, node.start_mark.line,
           "a %s whose value is not a single scalar; this gate cannot read it" % what)
    return None


def check_reference(path, lines, node, what, pins, local):
    """One rule for every reference to code this workflow does not contain: an action, a
    reusable workflow, a container image. pins are the forms that count as pinned — a
    uses: may be either a commit SHA or, for a docker:// action, a digest, while an image
    can only be a digest; local names the prefixes that are this repository and need no
    pin."""
    ref = scalar(path, node, what)
    if ref is None:
        return

    if SUBSTITUTION.search(ref):
        report("PIN", path, node.start_mark.line,
               "%s in %s (a substitution cannot be pinned: a digest fixes content, it does "
               "not fix which thing is fetched)" % (ref, what))
        return

    if local and ref.startswith(local) and ".." not in ref:
        return

    if any(pin.match(ref) for pin in pins) and versioned(lines, node):
        return

    report("PIN", path, node.start_mark.line, "%s in %s" % (ref, what))


def check_script(path, lines, node, what):
    text = scalar(path, node, what)
    if text is None or not SUBSTITUTION.search(text):
        return

    # Reported at the line the substitution is on, not at the key: a block scalar is
    # one node and dozens of lines, and the one that matters is the one to edit.
    first = min(node.start_mark.line, len(lines) - 1)
    last = min(node.end_mark.line, len(lines) - 1)
    hits = [n for n in range(first, last + 1) if SUBSTITUTION.search(lines[n])]
    for n in hits or [first]:
        report("RUN", path, n, lines[n] if hits else "%s: %s" % (what, text))


def check_step(path, lines, step):
    if not isinstance(step, yaml.MappingNode):
        report("SHAPE", path, step.start_mark.line, "a step that is not a mapping")
        return

    if unreadable(path, step, "step", STEP_CHECKED, STEP_INERT):
        return

    keys = entries(step)

    if "uses" in keys:
        check_reference(path, lines, keys["uses"], "uses", (SHA_PIN, DIGEST_PIN), "./")

    if "run" in keys:
        check_script(path, lines, keys["run"], "run")

    action = keys.get("uses")
    if not isinstance(action, yaml.ScalarNode) or "with" not in keys:
        return

    # The with: vocabulary belongs to the action, not to Actions, so it is not enumerated
    # here — only the inputs this gate knows reach an interpreter are read.
    if unreadable(path, keys["with"], "with: mapping"):
        return

    named = action.value.strip().split("@")[0].lower()
    inputs = entries(keys["with"])
    wanted = ARGV_INPUTS if named.startswith("docker://") else SCRIPT_INPUTS.get(named, ())

    for name in wanted:
        if name in inputs:
            check_script(path, lines, inputs[name], "with." + name)


def check_job(path, lines, job):
    if not isinstance(job, yaml.MappingNode):
        report("SHAPE", path, job.start_mark.line, "a job that is not a mapping")
        return

    if unreadable(path, job, "job", JOB_CHECKED, JOB_INERT):
        return

    keys = entries(job)

    # A job-level uses: is a reusable-workflow call and is pinned like any other.
    if "uses" in keys:
        check_reference(path, lines, keys["uses"], "uses", (SHA_PIN, DIGEST_PIN), "./")

    # container: is either the image or a mapping holding it, and every service is one
    # too. A step's action is one step's code; the job's container is the filesystem and
    # the interpreter that every run: in the job executes inside, so a movable tag there
    # is worth more to an attacker than a movable action, not less.
    container = keys.get("container")
    if isinstance(container, yaml.ScalarNode):
        check_reference(path, lines, container, "container", (DIGEST_PIN,), "")
    elif container is not None and not unreadable(path, container, "container"):
        image = entries(container).get("image")
        if image is not None:
            check_reference(path, lines, image, "container.image", (DIGEST_PIN,), "")

    services = keys.get("services")
    if services is not None and not unreadable(path, services, "services: mapping"):
        for name, service in entries(services).items():
            if unreadable(path, service, "service %s" % name):
                continue
            image = entries(service).get("image")
            if image is not None:
                check_reference(path, lines, image, "services.%s.image" % name,
                                (DIGEST_PIN,), "")

    steps = keys.get("steps")
    if isinstance(steps, yaml.SequenceNode):
        for step in steps.value:
            check_step(path, lines, step)
    elif steps is not None:
        report("SHAPE", path, steps.start_mark.line, "a steps: that is not a sequence")


def check(path):
    with open(path, "r", encoding="utf-8", errors="strict", newline=None) as f:
        try:
            text = f.read()
        except UnicodeDecodeError as err:
            report("SHAPE", path, 0,
                   "this file is not UTF-8, so the gate is not reading what the runner "
                   "will: %s" % err)
            return

    lines = LINE_BREAK.split(text)

    try:
        documents = list(yaml.compose_all(text))
    except yaml.YAMLError as err:
        mark = getattr(err, "problem_mark", None)
        report("SHAPE", path, mark.line if mark else 0,
               "this file is not YAML this gate can read: %s" % str(err).replace("\n", " "))
        return

    for document in documents:
        if document is None:
            continue

        # The root and the jobs: mapping are checked for duplicates and merges like every
        # other mapping. They were not, and they are the two levels the walk enters
        # through: a second jobs: key silently dropped every job in the first block, and a
        # `<<` at the root made the whole file invisible while it still counted as passing.
        # Their key vocabularies are not enumerated — a workflow-level key is Actions' own
        # business, and a job id is whatever the author called it.
        if unreadable(path, document, "workflow"):
            continue

        jobs = entries(document).get("jobs")
        if jobs is None:
            report("SHAPE", path, document.start_mark.line,
                   "a workflow with no jobs: mapping; the gate has nothing to read here")
            continue
        if not isinstance(jobs, yaml.MappingNode):
            report("SHAPE", path, jobs.start_mark.line, "a jobs: that is not a mapping")
            continue
        if unreadable(path, jobs, "jobs: mapping"):
            continue

        for _, job in jobs.value:
            check_job(path, lines, job)


for path in sys.argv[1:]:
    # A backstop, not a convenience. check() names the failures it expects, but a gate
    # that dies on one file loses the findings of every file before it — they are printed
    # at the end — and hands a contributor a traceback where a verdict belongs. Anything
    # unexpected becomes a refusal naming the file. RecursionError from a deeply nested
    # flow collection was the one that found this: it is not a yaml.YAMLError, so the
    # except inside check() does not see it.
    try:
        check(path)
    except Exception as err:  # noqa: BLE001 - deliberate: a gate refuses what breaks it
        report("SHAPE", path, 0,
               "this file broke the gate (%s: %s), so the gate will not pass it"
               % (type(err).__name__, str(err).replace("\n", " ")[:120]))

GROUPS = (
    ("SHAPE", "this gate cannot read these lines, so it will not pass them:",
     "  fix: write the workflow as ordinary block YAML, with keys this gate knows"),
    ("PIN", "these references are not pinned to a digest with a version comment:",
     "  fix: uses: owner/action@<40-char sha> # vX.Y.Z\n"
     "       image: name@sha256:<64-char digest> # vX.Y.Z"),
    ("RUN", "these scripts interpolate a value straight into the shell:",
     "  fix: route it through env: and validate it, as record-corpus.yml does;\n"
     "       many contexts the runner already exports ($RUNNER_OS, $GITHUB_EVENT_NAME)"),
)

bad = False
for kind, headline, remedy in GROUPS:
    if not findings[kind]:
        continue
    bad = True
    print("check-pins: " + headline, file=sys.stderr)
    for line in findings[kind]:
        print(line, file=sys.stderr)
    print(remedy, file=sys.stderr)

sys.exit(1 if bad else 0)
PYEOF

[ "$status" -eq 0 ] || [ "$status" -eq 1 ] || {
  echo "check-pins: the parser failed" >&2
  exit 2
}

# The verdict, once, in the words the --help text uses. The trailing `return 0` is what
# keeps an unwritable summary from turning a pass into a failure under set -e, and is the
# shape check-coverage.sh already uses for the same reason.
summary() { [ -n "${GITHUB_STEP_SUMMARY:-}" ] && printf '%s\n' "$1" >> "$GITHUB_STEP_SUMMARY"; return 0; }

verdict="check-pins: ${#files[@]} workflows, every reference pinned and every substitution kept out of the shell"

if [ "$status" -eq 0 ]; then
  summary "$verdict"
  echo "$verdict"
else
  summary "check-pins: refused — see the job log"
fi

exit "$status"
