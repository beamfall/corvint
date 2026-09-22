#!/bin/sh
set -eu

# Five cases, on a fixture repository shaped like the gemini-cli host package (the simplest of
# the four, one version file): the commit that adds the package passes (it bumps and ships in
# the same commit), a later commit that changes shipped content without touching the version
# field fails and names the package, and a subsequent commit that bumps the version field
# passes again. The other three packages have no files in this fixture, so their checks report
# "no shipped files tracked" and skip, exercising that path too.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-host-package-versions-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir -p "$test_root/script" "$test_root/integrations/gemini-cli/hooks"
cp "$source_root/script/check-host-package-versions.sh" "$test_root/script/"

git -C "$test_root" init -q
git -C "$test_root" config user.email fixture@example.test
git -C "$test_root" config user.name Fixture

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

run() {
    status=0
    output=$(cd "$test_root" && script/check-host-package-versions.sh 2>&1) || status=$?
    printf '%s' "$output"
    return $status
}

commit() {
    msg=$1 when=$2
    git -C "$test_root" add -A
    GIT_AUTHOR_DATE=$when GIT_COMMITTER_DATE=$when git -C "$test_root" commit -q -m "$msg"
}

# 1. The commit that introduces the package bumps and ships its content together, so it passes.
cat >"$test_root/integrations/gemini-cli/gemini-extension.json" <<'EOF'
{
  "name": "corvint",
  "version": "0.1.0",
  "description": "fixture"
}
EOF
printf '{"hooks":{}}\n' >"$test_root/integrations/gemini-cli/hooks/hooks.json"
commit "add gemini-cli package at 0.1.0" 2026-01-01T00:00:00
output=$(run) || fail "the introducing commit did not pass: $output"
test -z "$output" || fail "a passing run printed output: $output"

# 2. A later commit changes shipped content without touching the version field: fails, and
# names the package and the stale version file.
printf '{"hooks":{"changed":true}}\n' >"$test_root/integrations/gemini-cli/hooks/hooks.json"
commit "change hooks.json only" 2026-01-01T00:01:00
if output=$(run); then
    fail "a content-only change with no version bump passed the gate"
fi
case $output in
    *gemini-cli*gemini-extension.json*) ;;
    *) fail "the failure did not name the stale package/file: $output" ;;
esac

# 3. Bumping the version field afterward passes again.
cat >"$test_root/integrations/gemini-cli/gemini-extension.json" <<'EOF'
{
  "name": "corvint",
  "version": "0.1.1",
  "description": "fixture"
}
EOF
commit "bump to 0.1.1" 2026-01-01T00:02:00
output=$(run) || fail "the run after the version bump did not pass: $output"
test -z "$output" || fail "a passing run printed output: $output"

# 4. The shipped set is the package directory, not a list of files known when the gate was
# written: a newly added shipped file (here a second skill) without a version bump fails, while a
# README.md edit alone still passes.
printf '# fixture\n' >"$test_root/integrations/gemini-cli/README.md"
commit "edit README only" 2026-01-01T00:03:00
output=$(run) || fail "a README.md-only change failed the gate: $output"
mkdir -p "$test_root/integrations/gemini-cli/skills/added"
printf 'skill\n' >"$test_root/integrations/gemini-cli/skills/added/SKILL.md"
commit "add a skill without a bump" 2026-01-01T00:04:00
if output=$(run); then
    fail "a newly added shipped file with no version bump passed the gate"
fi

# 5. AHI-020: a version bump and a later shipped-content change that land in the same
# committer second (as a fast rebase can produce) must still fail, because the content commit
# is strictly the later one in history -- committer-second equality must not be read as "the
# bump covers it". Bump first to clear case 4's stale state, then bump again and change content
# in one forced committer second.
cat >"$test_root/integrations/gemini-cli/gemini-extension.json" <<'EOF'
{
  "name": "corvint",
  "version": "0.1.2",
  "description": "fixture"
}
EOF
commit "bump to 0.1.2" 2026-01-01T00:05:00
output=$(run) || fail "the run after clearing the missed bump did not pass: $output"

cat >"$test_root/integrations/gemini-cli/gemini-extension.json" <<'EOF'
{
  "name": "corvint",
  "version": "0.1.3",
  "description": "fixture"
}
EOF
commit "bump to 0.1.3" 2026-01-01T00:06:00
printf '{"hooks":{"changed_again":true}}\n' >"$test_root/integrations/gemini-cli/hooks/hooks.json"
commit "change hooks.json in the bump commit's committer second" 2026-01-01T00:06:00
if output=$(run); then
    fail "a content change sharing the bump commit's committer second passed the gate"
fi

printf 'check-host-package-versions_test: 5 cases passed\n'
