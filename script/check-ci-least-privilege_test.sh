#!/bin/sh
set -eu

# Five cases on a fixture repository. Cases 1-2 pin that the check measures the Git index rather
# than the worktree: a violation present only on disk passes, and a violation staged while the disk
# copy is clean fails and is named. Cases 3-5 are ARTIFACT-V0-008 shapes: the compliant workflow
# passes, a `name:`-first checkout step is still held to `persist-credentials: false`, and a
# job-level permissions block broader than `contents: read` fails.

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
    *"must be exactly \`contents: read\`, found: contents: write"*) ;;
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

printf 'ci least-privilege test: 5 cases passed\n'
