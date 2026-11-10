#!/bin/sh
set -eu

# AHI-020: fixture histories for the Codex and Claude Code packages (the Gemini CLI, Pi and
# OpenCode packages were removed by decision 0594). The commit that adds a package
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

mkdir -p "$test_root/script" "$test_root/integrations/codex/plugins/corvint/hooks" "$test_root/integrations/codex/plugins/corvint/.codex-plugin"
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
cat >"$test_root/integrations/codex/plugins/corvint/.codex-plugin/plugin.json" <<'EOF'
{
  "name": "corvint",
  "version": "0.1.0",
  "description": "fixture"
}
EOF
printf '{"hooks":{}}\n' >"$test_root/integrations/codex/plugins/corvint/hooks/hooks.json"
commit "add codex package at 0.1.0" 2026-01-01T00:00:00
output=$(run) || fail "the introducing commit did not pass: $output"
test -z "$output" || fail "a passing run printed output: $output"

# 2. A later commit changes shipped content without touching the version field: fails, and
# names the package and the stale version file.
printf '{"hooks":{"changed":true}}\n' >"$test_root/integrations/codex/plugins/corvint/hooks/hooks.json"
commit "change hooks.json only" 2026-01-01T00:01:00
if output=$(run); then
    fail "a content-only change with no version bump passed the gate"
fi
case $output in
    *codex*.codex-plugin/plugin.json*) ;;
    *) fail "the failure did not name the stale package/file: $output" ;;
esac

# 3. Bumping the version field afterward passes again.
cat >"$test_root/integrations/codex/plugins/corvint/.codex-plugin/plugin.json" <<'EOF'
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
printf '# fixture\n' >"$test_root/integrations/codex/plugins/corvint/README.md"
commit "edit README only" 2026-01-01T00:03:00
output=$(run) || fail "a README.md-only change failed the gate: $output"
mkdir -p "$test_root/integrations/codex/plugins/corvint/skills/added"
printf 'skill\n' >"$test_root/integrations/codex/plugins/corvint/skills/added/SKILL.md"
commit "add a skill without a bump" 2026-01-01T00:04:00
if output=$(run); then
    fail "a newly added shipped file with no version bump passed the gate"
fi

# 5. AHI-020: a version bump and a later shipped-content change that land in the same
# committer second (as a fast rebase can produce) must still fail, because the content commit
# is strictly the later one in history -- committer-second equality must not be read as "the
# bump covers it". Bump first to clear case 4's stale state, then bump again and change content
# in one forced committer second.
cat >"$test_root/integrations/codex/plugins/corvint/.codex-plugin/plugin.json" <<'EOF'
{
  "name": "corvint",
  "version": "0.1.2",
  "description": "fixture"
}
EOF
commit "bump to 0.1.2" 2026-01-01T00:05:00
output=$(run) || fail "the run after clearing the missed bump did not pass: $output"

cat >"$test_root/integrations/codex/plugins/corvint/.codex-plugin/plugin.json" <<'EOF'
{
  "name": "corvint",
  "version": "0.1.3",
  "description": "fixture"
}
EOF
commit "bump to 0.1.3" 2026-01-01T00:06:00
printf '{"hooks":{"changed_again":true}}\n' >"$test_root/integrations/codex/plugins/corvint/hooks/hooks.json"
commit "change hooks.json in the bump commit's committer second" 2026-01-01T00:06:00
if output=$(run); then
    fail "a content change sharing the bump commit's committer second passed the gate"
fi

# Clear the earlier fixture's stale state before exercising the Claude Code shipped modules.
printf '{"name":"corvint","version":"0.1.4"}\n' >"$test_root/integrations/codex/plugins/corvint/.codex-plugin/plugin.json"
mkdir -p "$test_root/integrations/claude-code/plugins/corvint/.claude-plugin" "$test_root/integrations/claude-code/.claude-plugin"
cc_version() {
    printf '{"name":"corvint","version":"%s"}\n' "$1" >"$test_root/integrations/claude-code/plugins/corvint/.claude-plugin/plugin.json"
    printf '{"plugins":[{"name":"corvint","version":"%s"%s}]}\n' "$1" "${2:-}" >"$test_root/integrations/claude-code/.claude-plugin/marketplace.json"
}
cc_version 0.3.0
printf 'entry\n' >"$test_root/integrations/claude-code/plugins/corvint/index.ts"
commit "introduce Claude Code fixture" 2026-01-01T00:07:00
output=$(run) || fail "Claude Code introduction did not pass: $output"
i=0
for module in process core tasks operations cockpit workflow; do
    i=$((i + 1))
    printf 'new shipped module\n' >"$test_root/integrations/claude-code/plugins/corvint/$module.js"
    commit "add Claude Code $module without bump" "2026-01-01T01:0${i}:00"
    if output=$(run); then fail "Claude Code $module shipped without a bump"; fi
    case $output in *claude-code*plugin.json*) ;; *) fail "Claude Code failure not named: $output" ;; esac
    cc_version "0.3.$i"
    commit "bump Claude Code after $module" "2026-01-01T02:0${i}:00"
    output=$(run) || fail "Claude Code bump did not clear $module: $output"
done
# AHI-954 / V1-3409: a README-only edit needs no bump.
printf '# claude-code fixture\n' >"$test_root/integrations/claude-code/plugins/corvint/README.md"
commit "edit Claude Code README only" 2026-01-01T03:00:00
output=$(run) || fail "a Claude Code README.md-only change failed the gate: $output"

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

# LVT-V0-005 (V1-4252): a version file's history is read in one batch, so twenty more commits that
# touch Claude Code's marketplace.json without changing either version field cost no more git spawns, and the
# verdict is unchanged: the last bump is still the commit that changed the fields, so the gate
# passes, and a later shipped change with no bump still fails.
spawn_dir=$(mktemp -d "${TMPDIR:-/tmp}/corvint-host-package-versions-spawns.XXXXXX")
trap 'rm -rf "$test_root" "$spawn_dir"' EXIT
real_git=$(command -v git)
printf '#!/bin/sh\necho git >> "%s/log"\nexec "%s" "$@"\n' "$spawn_dir" "$real_git" > "$spawn_dir/git"
chmod +x "$spawn_dir/git"
count_spawns() { : > "$spawn_dir/log"; (cd "$test_root" && PATH="$spawn_dir:$PATH" script/check-host-package-versions.sh > /dev/null 2>&1) || true; grep -c . "$spawn_dir/log"; }
cc=$(jq -r .version "$test_root/integrations/claude-code/plugins/corvint/.claude-plugin/plugin.json")
i=0
while [ "$i" -lt 22 ]; do
    i=$((i + 1))
    cc_version "$cc" ",\"description\":\"edit $i\""
    commit "describe Claude Code $i" "2026-01-05T00:$(printf '%02d' "$i"):00"
    [ "$i" -ne 2 ] || few=$(count_spawns)
done
output=$(run) || fail "edits that keep the Claude Code version did not pass: $output"
many=$(count_spawns)
[ "$many" = "$few" ] || fail "git spawns grew with the version file's history: $few after two commits, $many after twenty more"
printf 'changed\n' >"$test_root/integrations/claude-code/plugins/corvint/index.ts"
commit "change Claude Code shipped content after the description edits" 2026-01-06T00:00:00
if output=$(run); then fail "a Claude Code shipped change after twenty unchanged-version commits passed the gate"; fi
case $output in
    *'claude-code: integrations/claude-code/plugins/corvint/.claude-plugin/plugin.json last bumped'*) ;;
    *) fail "the late Claude Code shipped change was not named: $output" ;;
esac
# Fault injection: a broken batch reader must use the full legacy history algorithm.
sed '2i\
if [ "${LANE_BAD_BATCH:-}" = 1 ] && [ "$1 $2" = "cat-file --batch" ]; then printf "invalid batch\\n"; exit 1; fi
' "$spawn_dir/git" > "$spawn_dir/newgit"
mv "$spawn_dir/newgit" "$spawn_dir/git"; chmod +x "$spawn_dir/git"
if output=$(cd "$test_root" && LANE_BAD_BATCH=1 PATH="$spawn_dir:$PATH" script/check-host-package-versions.sh 2>&1); then
 fail "a broken batch fallback passed an unbumped shipped change"
fi
case $output in *'claude-code: integrations/claude-code/plugins/corvint/.claude-plugin/plugin.json last bumped'*) ;; *) fail "fallback lost the shipped-change verdict: $output" ;; esac
# LVT-V0-005: a package whose only shipped path contains a newline still exists,
# and deletion does not erase its historical shipped change from the absence filter.
odd_repo=$test_root/odd-history
mkdir -p "$odd_repo/script" "$odd_repo/integrations/codex/plugins/corvint/src" "$odd_repo/integrations/codex/plugins/corvint/.codex-plugin"
cp "$source_root/script/check-host-package-versions.sh" "$odd_repo/script/"
git -C "$odd_repo" init -q
odd_git() { git -C "$odd_repo" -c user.name=Fixture -c user.email=fixture@example.test "$@"; }
odd_name=$(printf 'line\nbreak.ts')
printf '{"version":"0.1.0"}\n' > "$odd_repo/integrations/codex/plugins/corvint/.codex-plugin/plugin.json"
odd_git add -A && odd_git commit -q -m manifest
(cd "$odd_repo" && script/check-host-package-versions.sh) || fail "an unshipped manifest failed"
printf 'first\n' > "$odd_repo/integrations/codex/plugins/corvint/src/$odd_name"
odd_git add -A && odd_git commit -q -m unusual-path
if output=$(cd "$odd_repo" && script/check-host-package-versions.sh 2>&1); then fail "a newline-only shipped path was skipped"; fi
case $output in *'codex: integrations/codex/plugins/corvint/.codex-plugin/plugin.json last bumped'*) ;; *) fail "unusual path failure not named: $output" ;; esac
odd_git rm -q "integrations/codex/plugins/corvint/src/$odd_name" && odd_git commit -q -m delete-unusual-path
if output=$(cd "$odd_repo" && script/check-host-package-versions.sh 2>&1); then fail "deleting the only shipped path erased its history"; fi
# LVT-V0-005: a package whose first shipped file appears only in a merge resolution, while its
# manifest keeps an unchanged version, still enters the inventory and fails the missing bump.
merge_repo=$test_root/merge-history
mkdir -p "$merge_repo/script" "$merge_repo/integrations/codex/plugins/corvint/.codex-plugin"
cp "$source_root/script/check-host-package-versions.sh" "$merge_repo/script/"
git -C "$merge_repo" init -q -b main
merge_git() { git -C "$merge_repo" -c user.name=Fixture -c user.email=fixture@example.test "$@"; }
printf '{"version":"0.1.0"}\n' > "$merge_repo/integrations/codex/plugins/corvint/.codex-plugin/plugin.json"
printf 'base\n' > "$merge_repo/notes.txt"
merge_git add -A && merge_git commit -q -m manifest
merge_git checkout -q -b side
printf 'side\n' > "$merge_repo/side.txt"
merge_git add -A && merge_git commit -q -m side
merge_git checkout -q main
printf 'main\n' >> "$merge_repo/notes.txt"
merge_git add -A && merge_git commit -q -m main
merge_git merge -q --no-ff --no-commit side
mkdir -p "$merge_repo/integrations/codex/plugins/corvint/src"
printf 'resolved\n' > "$merge_repo/integrations/codex/plugins/corvint/src/index.ts"
merge_git add -A && merge_git commit -q -m 'merge side, shipping a resolution-only file'
if output=$(cd "$merge_repo" && script/check-host-package-versions.sh 2>&1); then fail "a shipped file introduced only by a merge resolution skipped the version check"; fi
case $output in *'codex: integrations/codex/plugins/corvint/.codex-plugin/plugin.json last bumped'*) ;; *) fail "merge-only shipped change not named: $output" ;; esac
printf 'check-host-package-versions_test: 20 cases passed (%s git spawns)\n'  "$many"
