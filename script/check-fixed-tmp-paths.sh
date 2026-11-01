#!/usr/bin/env bash
# usage: script/check-fixed-tmp-paths.sh
#
# ASD-V0-720 (V1-3847): a test that writes under the fixed /tmp, or hands a child TMPDIR=/tmp,
# ignores the TMPDIR the lane or CI gave it; on a host whose /tmp is a full tmpfs the test fails
# with "no space left on device" while its own TMPDIR has room. For every indexed *_test.go line
# that calls MkdirTemp, CreateTemp, WriteFile, Mkdir, MkdirAll, Create or OpenFile on a "/tmp"
# literal, joins onto "/tmp", or sets TMPDIR to "/tmp" in an environment entry or Setenv, it
# prints `check-fixed-tmp-paths: FAIL PATH:LINE: ...` with the fix, unless the line carries
# `// fixed-tmp: REASON` or the line before it does. It exits 1 on any failure, 2 when the
# index cannot be read, and 0 otherwise.
set -uo pipefail
export LC_ALL=C
cd "$(git rev-parse --show-toplevel)" || { printf 'check-fixed-tmp-paths: not in a Git worktree\n' >&2; exit 2; }

pattern='(MkdirTemp|CreateTemp|WriteFile|MkdirAll|Mkdir|Create|OpenFile)\("/tmp[/"]|filepath\.Join\("/tmp"|"TMPDIR=/tmp[/"]|Setenv\("TMPDIR", "/tmp[/"]'
hits=$(git grep -n --cached -E "$pattern" -- '*_test.go')
status=$?
((status <= 1)) || { printf 'check-fixed-tmp-paths: cannot read the index\n' >&2; exit 2; }
failed=0
while IFS= read -r hit; do
  [[ -n $hit && $hit != *'// fixed-tmp:'* ]] || continue
  location=$(cut -d: -f1,2 <<< "$hit")
  path=${location%:*} line=${location##*:}
  ((line > 1)) && git show ":$path" | sed -n "$((line - 1))p" | grep -qF '// fixed-tmp:' && continue
  printf 'check-fixed-tmp-paths: FAIL %s: writes under or exports the fixed /tmp; use t.TempDir() or os.TempDir(), or mark the line // fixed-tmp: REASON\n' "$location"
  failed=1
done <<< "$hits"
exit "$failed"
