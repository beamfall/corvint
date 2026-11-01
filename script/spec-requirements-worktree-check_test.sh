#!/bin/sh
set -eu

# Covers SRG-V0-800 (V1-3379): spec-requirements-check fails when an unstaged spec edit would
# change REQUIREMENTS.tsv, INDEX.json or README.md, names the edit and the fix, and leaves the real
# index untouched; a staged-and-regenerated edit, a prose-only unstaged edit and a private
# GIT_INDEX_FILE all pass. SRG-V0-923 (V1-3362): a regenerated generated file left unstaged fails.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
project=$(mktemp -d "${TMPDIR:-/tmp}/corvint-spec-worktree-test.XXXXXX")
trap 'rm -rf "$project"' EXIT INT TERM
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

mkdir -p "$project/script" "$project/docs/specs"
cp "$source_root/script/gen-spec-requirements.sh" "$source_root/script/spec-requirements-worktree-check.sh" "$project/script/"
git -C "$project" init -q -b main
g() { git -C "$project" -c user.name=t -c user.email=t@example.invalid "$@"; }
spec() {
  printf '# One spec\n\nIntent status: proposed\nDelivery status: not-started\n\n## Agent digest\n- Claim: One does the first thing.\n- Status: proposed / not-started\n\n- `ONE-V0-001`: %s\n%s' "$1" "${2:-}" > "$project/docs/specs/one.md"
}
regenerate() {
  out=$(mktemp -d "$project/.out.XXXXXX")
  (cd "$project" && script/gen-spec-requirements.sh --all "$out") 2>/dev/null || fail "fixture: the generator failed"
  for file in REQUIREMENTS.tsv INDEX.json README.md; do cp "$out/$file" "$project/docs/specs/$file"; done
  rm -rf "$out"
}
check() { (cd "$project" && script/spec-requirements-worktree-check.sh); }

spec 'The first clause.'
printf '[]\n' > "$project/docs/specs/INDEX.json"
printf '# Specs\n\n| Scope | Contract | Intent | Delivery | Binding evidence or next gate |\n| --- | --- | --- | --- | --- |\n| One | [`one.md`](one.md) | proposed | not-started | One does the first thing. |\n' > "$project/docs/specs/README.md"
g add -A
regenerate
g add -A
g commit -q -m base
check || fail "a clean checkout failed the worktree check"

# The 2026-10-31 case: the spec edit is unstaged, so make spec-requirements changes nothing and the
# staged-view check passes; the worktree check must fail and name the edit.
spec 'The amended first clause.'
regenerate
g diff --quiet -- docs/specs/REQUIREMENTS.tsv || fail "fixture: regenerating from the index changed the TSV"
before=$(g ls-files -s)
if err=$(check 2>&1); then fail "an unstaged requirement edit passed the worktree check"; fi
case $err in *"docs/specs/{REQUIREMENTS.tsv} are stale against the working-tree specs"*"docs/specs/one.md"*"fix: git add the spec, then rerun make spec-requirements"*) ;;
  *) fail "the refusal did not name the stale file, the edit and the fix: $err" ;; esac
test "$(g ls-files -s)" = "$before" || fail "the worktree check changed the real index"
GIT_INDEX_FILE="$(g rev-parse --absolute-git-dir)/index" check || fail "a private GIT_INDEX_FILE was not skipped"

# Staging and regenerating is the fix, once the regenerated files are staged too. SRG-V0-923
# (V1-3362): regenerated but unstaged, they are what CI would not see, so the check fails.
sed 's/^- Claim: One does the first thing\.$/- Claim: One does the amended thing./' "$project/docs/specs/one.md" > "$project/one.md" &&
  mv "$project/one.md" "$project/docs/specs/one.md"
g add docs/specs/one.md
regenerate
g diff --quiet -- docs/specs/INDEX.json && fail "fixture: the regeneration did not change INDEX.json"
if err=$(check 2>&1); then fail "an unstaged regenerated INDEX.json passed the worktree check"; fi
case $err in *'generated spec index file(s) differ between the working tree and the index'*'docs/specs/INDEX.json'*'fix: git add them'*'(SRG-V0-923)'*) ;;
  *) fail "the unstaged-generated refusal did not name INDEX.json and the fix: $err" ;; esac
g add docs/specs
check || fail "a staged and regenerated edit failed the worktree check"
spec 'The amended first clause.'
g add docs/specs/one.md && regenerate && g add docs/specs

# A prose-only unstaged edit changes no generated file, so it passes.
spec 'The amended first clause.' 'Unindexed prose.
'
check || fail "a prose-only unstaged edit failed the worktree check"

printf 'ok\n'
