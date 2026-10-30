#!/bin/sh
set -eu

# On a fixture repository: Apache-headed examples, an unheaded README and an unheaded source file
# outside examples/ pass; a former AGPL SPDX identifier anywhere outside the historical records and
# the four byte-pinned frozen-evidence files fails and names the file, as do a missing or compound
# Apache identifier under examples/; historical records and the pinned files are exempt.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-license-headers-test.XXXXXX")
cleanup() { rm -rf "$test_root"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir -p "$test_root/script" "$test_root/examples/demo" "$test_root/protocol" "$test_root/internal/x" \
    "$test_root/docs/build-log" "$test_root/docs/decisions" "$test_root/.taskman" "$test_root/.github/workflows" \
    "$test_root/benchmarks/daily-loop-v0" "$test_root/internal/cem/corpusbinding/testdata/public-root-history/notices"
cp "$source_root/script/check-license-headers.sh" "$test_root/script/"
git -C "$test_root" init -q

# Built from pieces so this test's own text carries no former identifier.
former="SPDX-License-Identifier: AG""PL-3.0-or-later"

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

# 1. Apache-headed examples, an unheaded README, an unheaded Go file, Apache protocol material,
#    historical records quoting the former identifier and a pinned frozen file pass.
printf '#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\necho ok\n' >"$test_root/examples/demo/run.sh"
printf '// SPDX-License-Identifier: Apache-2.0\npackage demo\n' >"$test_root/examples/demo/demo.go"
printf '# Demo\n\nThe repository license is in LICENSING.md.\n' >"$test_root/examples/demo/README.md"
printf '// SPDX-License-Identifier: Apache-2.0\n' >"$test_root/protocol/schema.go"
printf 'package x\n' >"$test_root/internal/x/x.go"
printf '# Entry\n\n%s\n' "$former" >"$test_root/docs/build-log/2026-10-10-entry.md"
printf '# %s\n' "$former" >"$test_root/docs/decisions/evidence.py"
printf '{"body":"%s"}\n' "$former" >"$test_root/.taskman/ticket.json"
printf '# %s\n' "$former" >"$test_root/benchmarks/daily-loop-v0/harness.py"
printf 'policy: %s\n' "$former" >"$test_root/internal/cem/corpusbinding/testdata/public-root-history/notices/aaaa.txt"
git -C "$test_root" add -A
output=$(run) || fail "a clean tree did not pass: $output"
case $output in
    *'2 examples/ path(s) carry Apache-2.0'*) ;;
    *) fail "a passing run did not report the examples count: $output" ;;
esac

# 2. A former identifier outside examples/ fails, including under .github/.
printf '// %s\npackage x\n' "$former" >"$test_root/internal/x/x.go"
expect_failure "a former identifier in internal/" "internal/x/x.go carries a former AGPL SPDX identifier"
printf 'package x\n' >"$test_root/internal/x/x.go"
printf '# %s\nname: ci\n' "$former" >"$test_root/.github/workflows/ci.yml"
expect_failure "a former identifier in .github/" ".github/workflows/ci.yml carries a former AGPL SPDX identifier"
rm "$test_root/.github/workflows/ci.yml"

# 2b. The exemption is the frozen notices directory only: a real former header beside it in the
#     corpusbinding tree still fails.
printf '// %s\npackage x\n' "$former" >"$test_root/internal/cem/corpusbinding/other.go"
expect_failure "a former identifier next to the notice fixtures" "internal/cem/corpusbinding/other.go carries a former AGPL SPDX identifier"
rm "$test_root/internal/cem/corpusbinding/other.go"

# 3. A former identifier under examples/ fails, alone or in a compound expression.
printf '#!/bin/sh\n# %s\necho ok\n' "$former" >"$test_root/examples/demo/run.sh"
expect_failure "a former identifier in examples/" "examples/demo/run.sh carries a former AGPL SPDX identifier"
printf '#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0 OR AG''PL-3.0-or-later\necho ok\n' >"$test_root/examples/demo/run.sh"
expect_failure "an Apache OR former expression" "examples/demo/run.sh lacks"
printf '#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0 OR MIT\necho ok\n' >"$test_root/examples/demo/run.sh"
expect_failure "an Apache OR MIT expression" "examples/demo/run.sh lacks"
printf '#!/bin/sh\n# SPDX-License-Identifier: Apache-2.0\necho ok\n' >"$test_root/examples/demo/run.sh"

# 4. A non-Markdown example without the Apache identifier fails.
printf 'name: demo\n' >"$test_root/examples/demo/workflow.yml"
expect_failure "a missing Apache header" "examples/demo/workflow.yml lacks"
rm "$test_root/examples/demo/workflow.yml"

git -C "$test_root" add -A
output=$(run) || fail "the restored tree did not pass: $output"
printf 'license-headers-test: ok\n'
