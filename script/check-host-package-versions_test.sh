#!/bin/sh
set -eu

# AHI-020: fixture histories for Gemini CLI, Pi and OpenCode. The commit that adds a package
# passes (it bumps and ships in the same commit), a later commit that changes content without
# touching the version
# field fails and names the package, and a subsequent commit that bumps the version field
# passes again. Packages with no files in this fixture report
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

# Clear the earlier fixture's stale state before exercising the Pi shipped modules.
printf '{"name":"corvint","version":"0.1.4"}\n' >"$test_root/integrations/gemini-cli/gemini-extension.json"
mkdir -p "$test_root/integrations/pi"
printf '{"version":"0.3.0","corvintIntegration":{"adapterVersion":"0.3.0"}}\n' >"$test_root/integrations/pi/package.json"
printf 'entry\n' >"$test_root/integrations/pi/index.ts"
commit "introduce Pi fixture" 2026-01-01T00:07:00
output=$(run) || fail "Pi introduction did not pass: $output"
i=0
for module in process core tasks operations cockpit workflow; do
    i=$((i + 1))
    printf 'new shipped module\n' >"$test_root/integrations/pi/$module.js"
    commit "add Pi $module without bump" "2026-01-01T01:0${i}:00"
    if output=$(run); then fail "Pi $module shipped without a bump"; fi
    case $output in *pi*package.json*) ;; *) fail "Pi failure not named: $output" ;; esac
    printf '{"version":"0.3.%s","corvintIntegration":{"adapterVersion":"0.3.%s"}}\n' "$i" "$i" >"$test_root/integrations/pi/package.json"
    commit "bump Pi after $module" "2026-01-01T02:0${i}:00"
    output=$(run) || fail "Pi bump did not clear $module: $output"
done
# AHI-954 / V1-3409: a Pi README-only edit needs no bump, though the files list names README.md.
printf '# pi fixture\n' >"$test_root/integrations/pi/README.md"
commit "edit Pi README only" 2026-01-01T03:00:00
output=$(run) || fail "a Pi README.md-only change failed the gate: $output"

# AHI-020 / V1-0452: OpenCode's explicit shipped set must cover its package.json files list,
# including a skills-only addition and an mcp.example.json-only edit.
mkdir -p "$test_root/integrations/opencode/src"
cat >"$test_root/integrations/opencode/package.json" <<'EOF'
{"version":"0.1.0","files":["src","README.md","opencode.example.json","mcp.example.json","skills"]}
EOF
printf 'entry\n' >"$test_root/integrations/opencode/src/index.ts"
printf '{}\n' >"$test_root/integrations/opencode/mcp.example.json"
commit "introduce OpenCode fixture" 2026-01-02T00:00:00
output=$(run) || fail "OpenCode introduction did not pass: $output"
i=0
for shipped in skills/added/SKILL.md mcp.example.json; do
    i=$((i + 1))
    mkdir -p "$test_root/integrations/opencode/$(dirname "$shipped")"
    printf 'changed shipped content\n' >"$test_root/integrations/opencode/$shipped"
    commit "change OpenCode $shipped without bump" "2026-01-02T00:${i}0:00"
    if output=$(run); then fail "OpenCode $shipped shipped without a bump"; fi
    case $output in
        *'opencode: integrations/opencode/package.json last bumped'*) ;;
        *) fail "OpenCode failure not named: $output" ;;
    esac
    jq --arg version "0.1.$i" '.version = $version' \
        "$test_root/integrations/opencode/package.json" >"$test_root/package.json.tmp"
    mv "$test_root/package.json.tmp" "$test_root/integrations/opencode/package.json"
    commit "bump OpenCode after $shipped" "2026-01-02T00:${i}1:00"
    output=$(run) || fail "OpenCode bump did not clear $shipped: $output"
done

# Files outside the published list do not require a package bump.
printf 'development-only test\n' >"$test_root/integrations/opencode/fixture.test.ts"
commit "change unpublished OpenCode test only" 2026-01-02T01:00:00
output=$(run) || fail "unpublished OpenCode change failed the gate: $output"
# AHI-954 / V1-3409: an OpenCode README-only edit needs no bump either.
printf '# opencode fixture\n' >"$test_root/integrations/opencode/README.md"
commit "edit OpenCode README only" 2026-01-02T01:01:00
output=$(run) || fail "an OpenCode README.md-only change failed the gate: $output"
# AHI-020 / V1-2184: the Spec Kit extension reads its version from extension.yml's extension block,
# and the failure names the bump command (AHI-173).
mkdir -p "$test_root/protocol/integrations/speckit/corvint/commands"
speckit_version() {
    printf 'schema_version: "1.0"\nextension:\n  id: corvint\n  version: "%s"\nrequires:\n  version: "%s"\n' "$1" "$2" \
        >"$test_root/protocol/integrations/speckit/corvint/extension.yml"
}
speckit_version 0.1.0 9.9.9
printf 'context\n' >"$test_root/protocol/integrations/speckit/corvint/commands/context.md"
commit "introduce Spec Kit fixture" 2026-01-03T00:00:00
output=$(run) || fail "Spec Kit introduction did not pass: $output"
printf 'changed context\n' >"$test_root/protocol/integrations/speckit/corvint/commands/context.md"
commit "change a Spec Kit command without a bump" 2026-01-03T00:01:00
if output=$(run); then fail "a Spec Kit command change with no version bump passed the gate"; fi
case $output in
    *'speckit: protocol/integrations/speckit/corvint/extension.yml last bumped'*'run make host-package-version-bump PKG=speckit'*) ;;
    *) fail "Spec Kit failure did not name the package and the bump command: $output" ;;
esac
speckit_version 0.1.1 9.9.9
commit "bump Spec Kit" 2026-01-03T00:02:00
output=$(run) || fail "Spec Kit bump did not clear the change: $output"
# Another block's version key is not the package version.
printf 'changed again\n' >"$test_root/protocol/integrations/speckit/corvint/commands/context.md"
commit "change a Spec Kit command" 2026-01-03T00:03:00
speckit_version 0.1.1 9.9.10
commit "change only requires.version" 2026-01-03T00:04:00
if output=$(run); then fail "a requires.version edit was read as the Spec Kit package bump"; fi

# The OpenSpec skill is its own version file: a body edit without a front-matter bump fails, a
# body edit that lands with the bump passes.
speckit_version 0.1.2 9.9.10
mkdir -p "$test_root/protocol/integrations/openspec/skills/corvint-openspec"
skill() {
    printf -- '---\nname: corvint-openspec\nmetadata:\n  author: fixture\n  version: "%s"\n---\n\n%s\n' "$1" "$2" \
        >"$test_root/protocol/integrations/openspec/skills/corvint-openspec/SKILL.md"
}
skill 0.1.0 body
commit "introduce OpenSpec fixture and clear Spec Kit" 2026-01-04T00:00:00
output=$(run) || fail "OpenSpec introduction did not pass: $output"
skill 0.1.0 "changed body"
commit "change the OpenSpec skill body without a bump" 2026-01-04T00:01:00
if output=$(run); then fail "an OpenSpec skill body change with no version bump passed the gate"; fi
case $output in
    *'openspec: protocol/integrations/openspec/skills/corvint-openspec/SKILL.md last bumped'*) ;;
    *) fail "OpenSpec failure not named: $output" ;;
esac
skill 0.1.1 "changed body again"
commit "change the OpenSpec skill body with a bump" 2026-01-04T00:02:00
output=$(run) || fail "an OpenSpec body change with its bump did not pass: $output"
printf 'check-host-package-versions_test: 19 cases passed\n'
