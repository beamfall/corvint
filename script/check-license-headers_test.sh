#!/bin/sh
set -eu

# On a fixture repository: AGPL-headed examples and an unheaded README pass; an Apache-2.0 SPDX
# header, an Apache licence line and a missing AGPL header under examples/ each fail and name the
# file; an Apache-2.0 header under protocol/ is not this check's concern and stays allowed.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-license-headers-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir -p "$test_root/script" "$test_root/examples/demo" "$test_root/protocol"
cp "$source_root/script/check-license-headers.sh" "$test_root/script/"
git -C "$test_root" init -q

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

run() {
    status=0
    output=$(cd "$test_root" && script/check-license-headers.sh 2>&1) || status=$?
    printf '%s' "$output"
    return $status
}

expect_failure() {
    git -C "$test_root" add -A
    if output=$(run); then
        fail "$1 passed the check"
    fi
    case $output in
        *"$2"*) ;;
        *) fail "$1 did not name $2: $output" ;;
    esac
}

# 1. AGPL-headed sources, an unheaded Markdown README and an Apache protocol file pass.
printf '#!/bin/sh\n# SPDX-License-Identifier: AGPL-3.0-or-later\necho ok\n' >"$test_root/examples/demo/run.sh"
printf '// SPDX-License-Identifier: AGPL-3.0-or-later\npackage demo\n' >"$test_root/examples/demo/demo.go"
printf '# Demo\n\nThe enumerated Apache-2.0 boundary is in LICENSING.md.\n' >"$test_root/examples/demo/README.md"
printf '// SPDX-License-Identifier: Apache-2.0\n' >"$test_root/protocol/schema.go"
git -C "$test_root" add -A
output=$(run) || fail "a clean examples tree did not pass: $output"
case $output in
    *'4 tracked examples/'*) fail "the check counted a protocol/ path: $output" ;;
    *'3 tracked examples/ path(s) clean'*) ;;
    *) fail "a passing run did not report the examples count: $output" ;;
esac

# 2. An Apache-2.0 SPDX header under examples/ fails.
printf '# SPDX-License-Identifier: Apache-2.0\nprint(1)\n' >"$test_root/examples/demo/tool.py"
expect_failure "an Apache-2.0 SPDX header" "examples/demo/tool.py carries an Apache-2.0 notice"
rm "$test_root/examples/demo/tool.py"

# 3. An Apache licence line fails even beside an AGPL identifier, and in Markdown.
printf '// SPDX-License-Identifier: AGPL-3.0-or-later\n// Licensed under the Apache License, Version 2.0.\npackage demo\n' >"$test_root/examples/demo/demo.go"
expect_failure "an Apache licence line" "examples/demo/demo.go carries an Apache-2.0 notice"
printf '// SPDX-License-Identifier: AGPL-3.0-or-later\npackage demo\n' >"$test_root/examples/demo/demo.go"
printf '# Demo\n\nSPDX-License-Identifier: Apache-2.0\n' >"$test_root/examples/demo/README.md"
expect_failure "an Apache identifier in Markdown" "examples/demo/README.md carries an Apache-2.0 notice"
printf '# Demo\n' >"$test_root/examples/demo/README.md"

# 4. A non-Markdown file without the AGPL identifier fails.
printf 'name: demo\n' >"$test_root/examples/demo/workflow.yml"
expect_failure "a missing AGPL header" "examples/demo/workflow.yml lacks"
rm "$test_root/examples/demo/workflow.yml"

git -C "$test_root" add -A
output=$(run) || fail "the restored tree did not pass: $output"
printf 'license-headers-test: ok\n'
