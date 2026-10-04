#!/bin/sh
set -eu

# Fixtures reproduce both 2026-10-04 contradictions (V1-0721): an AGENTS.md that required draft
# pull requests against a PreToolUse hook refusing the draft flag, and an AGENTS.md that required
# approval to merge against a memory that arms auto-merge and a global file preauthorizing merges.
# The resolved wording must report no conflict, and a missing or unreadable host input, or no
# host input at all, must report UNKNOWN rather than a pass.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
check="$source_root/script/check-instruction-conflicts.sh"
fixtures=$(mktemp -d "${TMPDIR:-/tmp}/corvint-instruction-conflicts-test.XXXXXX")
cleanup() { chmod -R u+rwx "$fixtures" 2>/dev/null || true; rm -rf "$fixtures"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

fail() { printf 'FAIL: %s\n%s\n' "$1" "$output" >&2; exit 1; }

run() {
	status=0
	output=$("$check" "$@" 2>&1) || status=$?
}

run_copy() {
	status=0
	output=$("$fixtures/repo/script/check-instruction-conflicts.sh" "$@" 2>&1) || status=$?
}

expect_status() { [ "$status" = "$1" ] || fail "$2: exit $status, want $1"; }
expect_line() { printf '%s\n' "$output" | grep -Eq "$1" || fail "$2: no line matching $1"; }
refuse_line() { printf '%s\n' "$output" | grep -Eq "$1" && fail "$2: unexpected line matching $1" || true; }

# The pre-resolution AGENTS.md wording, wrapped as the file wraps it.
cat >"$fixtures/old-AGENTS.md" <<'EOF'
## Repository etiquette

For requested Corvint issue work, automatically push verified task branches to this repository's
configured origin and open or update draft pull requests. Merging, force-pushing, deleting remote
branches, and publishing releases still require explicit approval.
EOF

# The resolved wording.
cat >"$fixtures/new-AGENTS.md" <<'EOF'
For requested Corvint issue work, automatically push verified task branches and open or update
ready (non-draft) pull requests; a machine PreToolUse hook refuses draft PR creation. After
opening a task PR, also arm GitHub auto-merge with the merge-commit method. Merging by hand,
force-pushing, deleting remote branches, and publishing releases still require explicit approval.
EOF

# The host hook, as the settings file stores it: one JSON line.
cat >"$fixtures/settings.json" <<'EOF'
{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"printf '%s' \"$cmd\" | grep -Eq 'gh pr create.*--draft' && printf '{\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"Draft PRs are disabled by user setting. Re-run gh pr create without --draft.\"}'"}]}]}}
EOF

mkdir "$fixtures/memory"
cat >"$fixtures/memory/pr-auto-merge.md" <<'EOF'
---
name: pr-auto-fix-auto-merge-default
---

After opening a PR, turn on Auto-fix and auto-merge without offering first.
EOF
cat >"$fixtures/memory/stacked.md" <<'EOF'
Do not arm auto-merge on a stacked PR. Never enable auto-merge through gh.
EOF

cat >"$fixtures/global-AGENTS.md" <<'EOF'
- Standing owner instruction: PR merges are preauthorized across projects. After required checks
  and review pass, merge task-owned pull requests without asking again.
EOF

# 1. Both 2026-10-04 contradictions are reported, each naming its project and host sources.
run --project "$fixtures/old-AGENTS.md" --host "$fixtures/settings.json" \
	--host "$fixtures/memory" --host "$fixtures/global-AGENTS.md"
expect_status 1 "old wording"
expect_line '^CONFLICT +pr-draft +draft-required: project [^ ]*old-AGENTS.md:3 \| draft-refused: host [^ ]*settings.json:1$' "draft rule against hook"
expect_line '^CONFLICT +pr-auto-merge +allowed: host [^ ]*memory/pr-auto-merge.md:5 \| needs-approval: project [^ ]*old-AGENTS.md:4$' "merge approval against memory"
expect_line '^CONFLICT +pr-manual-merge +needs-approval: project [^ ]*old-AGENTS.md:4 \| preauthorized: host [^ ]*global-AGENTS.md:1$' "merge approval against global file"
expect_line '^result: CONFLICT \(3\)$' "old wording result"
expect_line 'local, uncertain and non-authoritative' "provenance label"

# 2. The resolved wording agrees with the hook and the memory. Scoped negations in the stacked
#    memory neither claim nor contradict, and the absent global file is not consulted.
run --project "$fixtures/new-AGENTS.md" --host "$fixtures/settings.json" --host "$fixtures/memory"
expect_status 0 "new wording"
refuse_line '^CONFLICT' "new wording"
refuse_line 'stacked.md' "scoped negation"
expect_line '^NO_CONFLICT_FOUND +pr-draft +draft-refused: project [^ ]*new-AGENTS.md:1, project [^ ]*new-AGENTS.md:2, host [^ ]*settings.json:1$' "ready rule"
expect_line '^NO_CONFLICT_FOUND +pr-auto-merge +allowed: project [^ ]*new-AGENTS.md:2, host [^ ]*memory/pr-auto-merge.md:5$' "arm auto-merge"
expect_line '^NO_CONFLICT_FOUND +pr-manual-merge +needs-approval: project [^ ]*new-AGENTS.md:3$' "merge by hand"
expect_line '^result: NO_CONFLICT_FOUND$' "new wording result"

# 2b. Negated or refusing wording keeps its polarity: no draft requirement, no auto-merge
#     allowance beside its own approval rule, and no merge preauthorization.
cat >"$fixtures/neutral.md" <<'EOF'
Keep changes small.
EOF
cat >"$fixtures/negations.md" <<'EOF'
Never open draft pull requests. A hook refuses draft pull requests.
Draft pull requests are not allowed.

Arming auto-merge requires explicit approval.

Do not merge without asking.
EOF
run --project "$fixtures/neutral.md" --host "$fixtures/negations.md"
expect_status 0 "negated wording"
refuse_line 'draft-required' "negated draft wording"
refuse_line 'allowed' "auto-merge approval rule"
refuse_line 'preauthorized' "negated merge wording"
expect_line '^NO_CONFLICT_FOUND +pr-draft +draft-refused: host [^ ]*negations.md:1, host [^ ]*negations.md:2$' "draft refusals"
expect_line '^NO_CONFLICT_FOUND +pr-auto-merge +needs-approval: host [^ ]*negations.md:4$' "arming needs approval"
expect_line '^NO_CLAIM +pr-manual-merge$' "do not merge without asking"
printf 'Do not wait for review; open draft pull requests.\n' >"$fixtures/clause.md"
run --project "$fixtures/clause.md" --host "$fixtures/settings.json"
expect_status 1 "negation in an earlier clause"
expect_line '^CONFLICT +pr-draft +draft-required: project [^ ]*clause.md:1 ' "negation in an earlier clause"

# 3. A host file that is absent is UNKNOWN, not a pass.
run --project "$fixtures/new-AGENTS.md" --host "$fixtures/settings.json" --host "$fixtures/missing.json"
expect_status 3 "absent host file"
expect_line '^UNKNOWN +host [^ ]*missing.json absent or unreadable$' "absent host file"
expect_line '^result: UNKNOWN$' "absent host file result"

# 4. A host directory with nothing to read is UNKNOWN.
mkdir "$fixtures/empty"
run --project "$fixtures/new-AGENTS.md" --host "$fixtures/empty"
expect_status 3 "empty host directory"
expect_line '^UNKNOWN +host [^ ]*empty/\*\.md absent or unreadable$' "empty host directory"

# 5. No host input at all leaves the host layer unobserved.
run --project "$fixtures/new-AGENTS.md"
expect_status 3 "no host input"
expect_line '^UNKNOWN +host layer not observed: no --host given$' "no host input"

# 6. An unreadable host file is UNKNOWN. Skipped where permission bits do not bind (root).
cp "$fixtures/settings.json" "$fixtures/locked.json"
chmod 000 "$fixtures/locked.json"
if [ -r "$fixtures/locked.json" ]; then
	printf 'SKIP: unreadable-file case; permission bits do not bind for this user\n'
else
	run --project "$fixtures/new-AGENTS.md" --host "$fixtures/locked.json"
	expect_status 3 "unreadable host file"
	expect_line '^UNKNOWN +host [^ ]*locked.json absent or unreadable$' "unreadable host file"
fi

# 7. A conflict still exits 1 when another input is unknown, and the unknown is still listed.
run --project "$fixtures/old-AGENTS.md" --host "$fixtures/settings.json" --host "$fixtures/missing.json"
expect_status 1 "conflict with unknown"
expect_line '^UNKNOWN +host [^ ]*missing.json' "conflict with unknown"

# 8. A directory given as a file, or a *.md directory inside a host directory, is UNKNOWN rather
#    than an awk error.
run --project "$fixtures/memory" --host "$fixtures/settings.json"
expect_status 3 "directory as project file"
expect_line '^UNKNOWN +project [^ ]*memory absent or unreadable$' "directory as project file"
mkdir -p "$fixtures/dirhost/x.md"
run --project "$fixtures/new-AGENTS.md" --host "$fixtures/dirhost"
expect_status 3 "directory inside host directory"
expect_line '^UNKNOWN +host [^ ]*dirhost/x.md absent or unreadable$' "directory inside host directory"

# 9. Without --project, the repository root's AGENTS.md is read, and its absence is UNKNOWN.
run --host "$fixtures/settings.json"
expect_line 'project AGENTS\.md:[0-9]' "default project file"
mkdir -p "$fixtures/repo/script"
cp "$check" "$fixtures/repo/script/"
run_copy --host "$fixtures/settings.json"
expect_status 3 "absent default project file"
expect_line '^UNKNOWN +project AGENTS\.md absent or unreadable$' "absent default project file"

# 10. Usage errors exit 2.
run --host
expect_status 2 "missing --host value"
run --unknown
expect_status 2 "unknown flag"

printf 'PASS: check-instruction-conflicts\n'
