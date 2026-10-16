#!/bin/sh
set -eu

# Twelve cases on a fixture repository. Cases 1-2 pin that the check measures the Git index rather
# than the worktree: a violation present only on disk passes, and a violation staged while the disk
# copy is clean fails and is named. Cases 3-5 are ARTIFACT-V0-008 shapes: the compliant workflow
# passes, a `name:`-first checkout step is still held to `persist-credentials: false`, and a
# job-level permissions block broader than `contents: read` fails. Cases 6-9 pin V1-1469: a
# workflow other than ci.yml is checked at both levels, a declared per-job exception passes only
# for its exact permission set, and an exception that matches no block fails. Cases 10-12 pin parser
# bypasses found in review: a blank line inside the workflow-level block, a quoted `permissions`
# key, and a quoted job id following a declared job.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-ci-least-privilege-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir -p "$test_root/script" "$test_root/.github/workflows"
cp "$source_root/script/check-ci-least-privilege.sh" "$test_root/script/"
git -C "$test_root" init -q

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

workflow() {
    printf '%s\n' 'name: ci' 'on: pull_request' 'permissions:' "  contents: $1" 'jobs:' '  test:' \
        '    runs-on: ubuntu-latest' '    steps:' \
        '      - uses: actions/checkout@0123456789abcdef0123456789abcdef01234567' \
        '        with:' '          persist-credentials: false' \
        > "$test_root/.github/workflows/ci.yml"
}

run() {
    status=0
    output=$(cd "$test_root" && script/check-ci-least-privilege.sh 2>&1) || status=$?
    printf '%s' "$output"
    return $status
}

# 1. A violation that exists only in the worktree is not what a clone contains.
workflow read
git -C "$test_root" add .github/workflows/ci.yml
workflow write
output=$(run) || fail "an unstaged worktree violation failed the gate: $output"

# 2. A staged violation fails even though the worktree copy is clean.
git -C "$test_root" add .github/workflows/ci.yml
workflow read
if output=$(run); then
    fail "a staged violation passed because the worktree copy is clean"
fi
case $output in
    *"must be exactly \`contents: read\` or a declared exception, found: contents: write"*) ;;
    *) fail "the failure did not name the staged violation: $output" ;;
esac

pin=9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0


# run_steps WORKFLOW_STEPS: stages a workflow with those steps and runs the check on it.
run_steps() {
    {
        printf 'name: ci\npermissions:\n  contents: read\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n'
        printf '%s\n' "$1"
    } > "$test_root/.github/workflows/ci.yml"
    git -C "$test_root" add .github/workflows/ci.yml
    (cd "$test_root" && script/check-ci-least-privilege.sh 2>&1)
}

# 3. The compliant shape, in both step spellings, passes.
output=$(run_steps "      - uses: actions/checkout@$pin
        with:
          persist-credentials: false
      - name: Checkout again
        uses: actions/checkout@$pin
        with:
          persist-credentials: false
      - run: echo ok") || fail "a compliant workflow did not pass: $output"

# 4. A `name:`-first checkout step without persist-credentials: false fails.
if output=$(run_steps "      - name: Checkout
        uses: actions/checkout@$pin
      - uses: actions/setup-go@$pin
        with:
          persist-credentials: false"); then
    fail "a name-first checkout step without persist-credentials: false passed"
fi
case $output in
    *persist-credentials*) ;;
    *) fail "the failure did not name persist-credentials: $output" ;;
esac

# 5. A job-level permissions block broader than `contents: read` fails.
cat > "$test_root/.github/workflows/ci.yml" <<WORKFLOW
name: ci
permissions:
  contents: read
jobs:
  test:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - run: echo ok
WORKFLOW
git -C "$test_root" add .github/workflows/ci.yml
if output=$(cd "$test_root" && script/check-ci-least-privilege.sh 2>&1); then
    fail "a job-level contents: write grant passed"
fi
case $output in
    *job-level*) ;;
    *) fail "the failure did not name the job-level permissions block: $output" ;;
esac

# Restore a compliant ci.yml; the remaining cases vary a second workflow and the exception list.
run_steps "      - run: echo ok" >/dev/null || fail "the compliant ci.yml did not pass"

# signal_workflow TOP JOB: stages release-gates.yml with workflow-level permission TOP and
# job-level permission lines JOB for its `signal` job.
signal_workflow() {
    {
        printf 'name: release gates\npermissions:\n  contents: read\n'
        [ -z "$1" ] || printf '  %s\n' "$1"
        printf 'jobs:\n  signal:\n    runs-on: ubuntu-latest\n    permissions:\n'
        printf '      %s\n' $2
        printf '    steps:\n      - run: echo ok\n'
    } | sed 's/=/: /' > "$test_root/.github/workflows/release-gates.yml"
    git -C "$test_root" add .github/workflows/release-gates.yml
}

exceptions() {
    printf '%s\n' "$@" > "$test_root/script/ci-least-privilege-exceptions.tsv"
    git -C "$test_root" add script/ci-least-privilege-exceptions.tsv
}
row=$(printf '.github/workflows/release-gates.yml\tjob:signal\tcontents:read issues:write\tfixture reason')

# 6. Workflow-level `issues: write` in a workflow other than ci.yml fails, even with the job declared.
exceptions "$row"
signal_workflow 'issues=write' 'contents=read issues=write'
if output=$(run); then
    fail "a workflow-level issues: write in release-gates.yml passed"
fi
case $output in
    *"release-gates.yml:2: workflow-level"*"issues: write"*) ;;
    *) fail "the failure did not name release-gates.yml's workflow-level grant: $output" ;;
esac

# 7. An undeclared job-level write in another workflow fails.
exceptions '# no exceptions'
signal_workflow '' 'contents=read issues=write'
if output=$(run); then
    fail "an undeclared job-level issues: write passed"
fi
case $output in
    *"job-level \`permissions:\` (job:signal)"*"issues: write"*) ;;
    *) fail "the failure did not name the undeclared job-level grant: $output" ;;
esac

# 8. The declared exception passes for its exact set, and fails once the job widens past it.
exceptions "$row"
output=$(run) || fail "a declared job-level exception did not pass: $output"
signal_workflow '' 'contents=read issues=write pull-requests=write'
if output=$(run); then
    fail "a job that widened past its declared exception passed"
fi
case $output in
    *"job:signal"*"pull-requests: write"*) ;;
    *) fail "the failure did not name the widened job: $output" ;;
esac

# 9. An exception that matches no block fails as stale.
signal_workflow '' 'contents=read'
if output=$(run); then
    fail "a stale exception passed"
fi
case $output in
    *"exception for .github/workflows/release-gates.yml job:signal matches no permissions block"*) ;;
    *) fail "the failure did not name the stale exception: $output" ;;
esac

# stage_release_gates: stages stdin as release-gates.yml and expects the check to fail naming $1.
stage_release_gates() {
    cat > "$test_root/.github/workflows/release-gates.yml"
    git -C "$test_root" add .github/workflows/release-gates.yml
    if output=$(run); then
        fail "$2 passed"
    fi
    case $output in
        *"$1"*) ;;
        *) fail "the failure for $2 did not name $1: $output" ;;
    esac
}
exceptions "$row"

# 10. A blank line inside the workflow-level block does not hide a later write.
stage_release_gates 'issues: write' 'a write after a blank line in the workflow-level block' <<'WORKFLOW'
name: release gates
permissions:
  contents: read

  issues: write
jobs:
  signal:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      issues: write
    steps:
      - run: echo ok
WORKFLOW

# 11. A quoted `permissions` key is still a job-level block.
stage_release_gates 'pull-requests: write' 'a quoted job-level permissions key' <<'WORKFLOW'
name: release gates
permissions:
  contents: read
jobs:
  signal:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      issues: write
    steps:
      - run: echo ok
  other:
    runs-on: ubuntu-latest
    "permissions":
      pull-requests: write
    steps:
      - run: echo ok
WORKFLOW

# 12. A quoted job id after the declared job does not inherit that job's exception.
stage_release_gates '(not a job)' 'a quoted job id after a declared job' <<'WORKFLOW'
name: release gates
permissions:
  contents: read
jobs:
  signal:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      issues: write
    steps:
      - run: echo ok
  "other":
    runs-on: ubuntu-latest
    permissions:
      contents: read
      issues: write
    steps:
      - run: echo ok
WORKFLOW

printf 'ci least-privilege test: 12 cases passed\n'
