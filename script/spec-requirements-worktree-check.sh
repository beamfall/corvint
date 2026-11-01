#!/bin/sh
set -eu

# SRG-V0-800 (V1-3379): `make spec-requirements` reads the staged specs (SRG-V0-001), so running it
# before `git add` leaves REQUIREMENTS.tsv, INDEX.json and README.md generated from the old spec
# while `spec-requirements-check` compares them with that same staged view and passes. This check
# regenerates from a private copy of the index with every unstaged tracked docs/specs/*.md edit
# added, and fails when that worktree view differs from the generated files on disk. It writes
# nothing outside its temporary directory and leaves the real index untouched. A caller that
# points GIT_INDEX_FILE at a private snapshot index (dogfood-finish.sh) is already exact, so the
# check is skipped there; untracked specs keep the DCW-V0-352 warning only.

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
cd "$root"
test -z "${GIT_INDEX_FILE:-}" || exit 0

# SRG-V0-923 (V1-3362): CI compares the committed generated files, so a regenerated one left
# unstaged (make spec-requirements run, git add forgotten) passes a worktree comparison here and
# fails there. Each generated file must match its index copy.
unstaged_generated=$(GIT_OPTIONAL_LOCKS=0 git -c core.quotepath=off diff --name-only -- \
  docs/specs/REQUIREMENTS.tsv docs/specs/INDEX.json docs/specs/README.md)
if test -n "$unstaged_generated"; then
  printf '%s: generated spec index file(s) differ between the working tree and the index, so CI would check other bytes:\n' "$0" >&2
  printf '%s\n' "$unstaged_generated" | sed 's/^/  /' >&2
  printf '%s: fix: git add them (after make spec-requirements), or git restore them (SRG-V0-923)\n' "$0" >&2
  exit 1
fi

unstaged=$(GIT_OPTIONAL_LOCKS=0 git -c core.quotepath=off diff --name-only --diff-filter=AM -- 'docs/specs/*.md' |
  grep -v -x docs/specs/README.md || true)
test -n "$unstaged" || exit 0

tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-spec-worktree.XXXXXX")
trap 'rm -rf "$tmp"' EXIT INT TERM
index=$(git rev-parse --git-path index)
cp "$index" "$tmp/index"
mkdir "$tmp/out"
printf '%s\n' "$unstaged" | GIT_INDEX_FILE="$tmp/index" git add --pathspec-from-file=-
GIT_INDEX_FILE="$tmp/index" script/gen-spec-requirements.sh --all "$tmp/out"

stale=
for file in REQUIREMENTS.tsv INDEX.json README.md; do
  cmp -s "$tmp/out/$file" "docs/specs/$file" || stale="$stale $file"
done
test -n "$stale" || exit 0
printf '%s: docs/specs/{%s} are stale against the working-tree specs; unstaged spec edits:\n' "$0" "$(printf '%s' "${stale# }" | tr ' ' ',')" >&2
printf '%s\n' "$unstaged" | sed 's/^/  /' >&2
printf '%s: fix: git add the spec, then rerun make spec-requirements (SRG-V0-800)\n' "$0" >&2
exit 1
