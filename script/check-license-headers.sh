#!/bin/sh
set -eu

# Decision 0504 (amending decision 0002) moved `examples/**` from the Apache-2.0 interoperability
# layer to the AGPL-3.0-or-later product layer; `LICENSING.md` is the path authority. Over the
# tracked files under `examples/`:
#
#   1. No file carries an Apache-2.0 notice: an `SPDX-License-Identifier` naming Apache, or the
#      "Licensed under the Apache License" line.
#   2. Every non-Markdown file carries `SPDX-License-Identifier: AGPL-3.0-or-later` in its first
#      five lines.
#
# `protocol/**` stays Apache-2.0 and is not checked here. A third-party file placed under
# `examples/` would need its own owner-approved exception recorded in `LICENSING.md` and here.

root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
cd "$root"

apache_pattern='SPDX-License-Identifier:[[:space:]]*Apache|Licensed under the Apache License'
agpl_pattern='SPDX-License-Identifier: AGPL-3\.0-or-later([[:space:]]|$)'

failures=0
checked=0
paths=$(git ls-files -- examples)
set -f
IFS='
'
for path in $paths; do
    checked=$((checked + 1))
    if grep -E -q "$apache_pattern" "$path"; then
        printf 'license-headers: %s carries an Apache-2.0 notice; examples/** is AGPL-3.0-or-later (decision 0504)\n' "$path" >&2
        failures=$((failures + 1))
    fi
    case $path in
        *.md) continue ;;
    esac
    if ! head -n 5 "$path" | grep -E -q "$agpl_pattern"; then
        printf 'license-headers: %s lacks "SPDX-License-Identifier: AGPL-3.0-or-later" in its first five lines\n' "$path" >&2
        failures=$((failures + 1))
    fi
done

if [ "$failures" -ne 0 ]; then
    printf 'license-headers: %d failure(s) over %d tracked examples/ path(s)\n' "$failures" "$checked" >&2
    exit 1
fi
printf 'license-headers: %d tracked examples/ path(s) clean\n' "$checked"
