#!/bin/sh
set -eu

# Decision 0586 relicensed the whole repository under Apache-2.0; `LICENSING.md` is the authority.
# Over the tracked files:
#
#   1. No non-Markdown file outside the historical records carries an `SPDX-License-Identifier`
#      expression naming the former AGPL terms. Historical records are `docs/build-log/**`,
#      `docs/decisions/**`, `.taskman/**` and `.corvint/**`; they quote former terms as evidence.
#      The two preregistered benchmark harnesses `LICENSING.md` names keep their byte-pinned historical
#      line (their sealed preregistrations pin the harness digest), and
#      the frozen public-root-history notice fixtures quote the former terms as historical text, and
#      so does the frozen task-store ticket corpus of the duplicate-check evaluation (TCL-V0-009).
#   2. Every non-Markdown file under `examples/` carries exactly `SPDX-License-Identifier: Apache-2.0`,
#      with no further licence expression, in its first five lines.

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
cd "$root"

# The bracket keeps this script's own text from matching the pattern it enforces.
former_pattern='SPDX-License-Identifier:.*A[G]PL'
apache_pattern='SPDX-License-Identifier: Apache-2\.0[[:space:]]*$'

failures=0
set -f
IFS='
'

former=$(git grep -l -I -E "$former_pattern" -- . ':(exclude)*.md' \
    ':(exclude)docs/build-log/' ':(exclude)docs/decisions/' ':(exclude).taskman/' ':(exclude).corvint/' \
    ':(exclude)benchmarks/daily-loop-v0/harness.py' ':(exclude)benchmarks/untouched-repository-v1/harness.py' \
    ':(exclude)internal/cem/corpusbinding/testdata/public-root-history/notices/' \
    ':(exclude)internal/tasks/similar/testdata/corpus-2026-10-30.jsonl') || true
for path in $former; do
    printf 'license-headers: %s carries a former AGPL SPDX identifier; the repository is Apache-2.0 (decision 0586)\n' "$path" >&2
    failures=$((failures + 1))
done

checked=0
for path in $(git ls-files -- examples); do
    case $path in
        *.md) continue ;;
    esac
    checked=$((checked + 1))
    if ! head -n 5 "$path" | grep -E -q "$apache_pattern"; then
        printf 'license-headers: %s lacks "SPDX-License-Identifier: Apache-2.0" in its first five lines\n' "$path" >&2
        failures=$((failures + 1))
    fi
done

if [ "$failures" -ne 0 ]; then
    printf 'license-headers: %d failure(s); %d examples/ path(s) checked\n' "$failures" "$checked" >&2
    exit 1
fi
printf 'license-headers: tracked tree clean; %d examples/ path(s) carry Apache-2.0\n' "$checked"
