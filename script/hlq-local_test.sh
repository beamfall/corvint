#!/usr/bin/env bash
# hlq-local.sh: the runner's exit code reaches the caller unchanged, a dirty --source is refused
# without --snapshot-dirty and run as a committed private snapshot with it (HLQ-V1-056).
set -euo pipefail
source_root=$(cd "$(dirname "$0")/.." && pwd -P)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/corvint-hlq-local-test.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
export GIT_AUTHOR_NAME=test GIT_AUTHOR_EMAIL=test@example.invalid GIT_COMMITTER_NAME=test GIT_COMMITTER_EMAIL=test@example.invalid
# Stub runner: refuses a dirty --source with exit 2 as the real one does, otherwise records the
# source and its tracked content and exits with STUB_EXIT.
cat >"$scratch/runner" <<'STUB'
#!/bin/sh
source=
while [ "$#" -gt 0 ]; do
  case $1 in --source) source=$2; shift 2 ;; *) printf '%s\n' "$1" >>"$STUB_LOG.args"; shift ;; esac
done
if [ -n "$(git -C "$source" status --porcelain --untracked-files=all)" ]; then
  echo "host-lifecycle-v1: --source $source is not clean" >&2; exit 2
fi
printf '%s\n' "$source" >"$STUB_LOG.source"
git -C "$source" show HEAD:edited.txt >"$STUB_LOG.edited"
git -C "$source" show HEAD:new/untracked.txt >"$STUB_LOG.untracked" 2>/dev/null || :
exit "${STUB_EXIT:-0}"
STUB
chmod +x "$scratch/runner"
export HLQ_RUNNER="$scratch/runner" STUB_LOG="$scratch/log"
mkdir "$scratch/repo"
git -C "$scratch/repo" init -q -b main
printf 'base\n' >"$scratch/repo/edited.txt"
printf 'ignored\n' >"$scratch/repo/.gitignore"
git -C "$scratch/repo" add . && git -C "$scratch/repo" commit -q -m base

# Exit codes 0, 1 and 2 pass through unchanged (go run would collapse 2 to 1).
for want in 0 1 2; do
  code=0
  STUB_EXIT=$want "$source_root/script/hlq-local.sh" --source "$scratch/repo" --host cli --corvint x 2>/dev/null || code=$?
  [ "$code" -eq "$want" ] || { echo "exit $code, want $want" >&2; exit 1; }
done
grep -Fqx -- --host "$scratch/log.args"
# A dirty source is refused by the runner without --snapshot-dirty.
printf 'dirty\n' >"$scratch/repo/edited.txt"
mkdir "$scratch/repo/new"
printf 'fresh\n' >"$scratch/repo/new/untracked.txt"
printf 'skip\n' >"$scratch/repo/ignored"
code=0
"$source_root/script/hlq-local.sh" --source "$scratch/repo" 2>"$scratch/err" || code=$?
[ "$code" -eq 2 ]
grep -Fq 'is not clean' "$scratch/err"
# With --snapshot-dirty the runner sees a clean private clone holding the edits, and the source
# checkout is left untouched.
rm -f "$scratch/log.source"
code=0
STUB_EXIT=1 "$source_root/script/hlq-local.sh" --snapshot-dirty --source "$scratch/repo" 2>"$scratch/err" || code=$?
[ "$code" -eq 1 ]
grep -Fq 'running against snapshot' "$scratch/err"
[ "$(cat "$scratch/log.source")" != "$scratch/repo" ]
[ "$(cat "$scratch/log.edited")" = dirty ]
[ "$(cat "$scratch/log.untracked")" = fresh ]
[ "$(git -C "$scratch/repo" status --porcelain | wc -l | tr -d ' ')" -eq 2 ]
[ ! -e "$(cat "$scratch/log.source")" ]
# A snapshot that cannot be made is a setup error (exit 2) and the runner never starts.
rm -f "$scratch/log.source"
code=0
chmod 000 "$scratch/repo/new/untracked.txt"
"$source_root/script/hlq-local.sh" --snapshot-dirty --source "$scratch/repo" 2>"$scratch/err" || code=$?
chmod 644 "$scratch/repo/new/untracked.txt"
[ "$code" -eq 2 ]
grep -Fq 'setup failed before the runner started' "$scratch/err"
[ ! -e "$scratch/log.source" ]
printf 'hlq-local: PASS\n'
