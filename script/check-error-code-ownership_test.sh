#!/bin/sh
set -eu

# Eight cases, on a fixture repository: an emitted set that equals owned plus allowlisted
# passes; a new unowned code, found through a discovered `code`-parameter closure, fails and
# is named with its site; an allowlisted code a spec now names fails as stale; test files and
# untracked files are outside the scan; a `git grep` failure is refused loudly instead of
# being read as an empty emitted set; and a conversion to a named `...Error` string type, a
# later `code` parameter (matched only in its declaring directory) and a multi-target `reason`
# assignment are each extracted.

source_root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/corvint-error-code-ownership-test.XXXXXX")
fakebin=$(mktemp -d "${TMPDIR:-/tmp}/corvint-error-code-ownership-fakebin.XXXXXX")
cleanup() { rm -rf "$test_root" "$fakebin"; }
trap cleanup EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

mkdir -p "$test_root/script" "$test_root/docs/specs" "$test_root/internal/pkg"
cp "$source_root/script/check-error-code-ownership.sh" "$test_root/script/"

git -C "$test_root" init -q
git -C "$test_root" config user.email fixture@example.test
git -C "$test_root" config user.name Fixture

fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

run() {
    status=0
    output=$(cd "$test_root" && script/check-error-code-ownership.sh 2>&1) || status=$?
    printf '%s' "$output"
    return $status
}

cat > "$test_root/internal/pkg/pkg.go" <<'EOF'
package pkg

import "errors"

type Error struct{ Code string }

var errOwned = errors.New("owned-code")

func unowned() Error { return Error{Code: "unowned-code"} }
EOF
# shellcheck disable=SC2016
printf '# Spec\n\nThe `owned-code` refusal.\n' > "$test_root/docs/specs/spec.md"
printf 'unowned-code\n' > "$test_root/script/error-code-ownership.allowlist"
git -C "$test_root" add -A

# 1. Emitted equals owned plus allowlisted: pass, silently.
output=$(run) || fail "a matching allowlist did not pass: $output"
test -z "$output" || fail "a passing run printed output: $output"

# 2. A new unowned code reached through a discovered closure fails, naming code and site.
cat >> "$test_root/internal/pkg/pkg.go" <<'EOF'

func closure() error {
	refuse := func(code string) error { return errors.New(code) }
	return refuse("fresh-code")
}
EOF
git -C "$test_root" add -A
if output=$(run); then fail "a new unowned code passed: $output"; fi
case $output in
    *"fresh-code	internal/pkg/pkg.go:13"*) ;;
    *) fail "the new code or its site was not named: $output" ;;
esac
# shellcheck disable=SC2016
printf 'The `fresh-code` refusal.\n' >> "$test_root/docs/specs/spec.md"
git -C "$test_root" add -A
output=$(run) || fail "owning the new code did not restore a pass: $output"

# 3. An allowlisted code that a spec now names is stale and fails.
# shellcheck disable=SC2016
printf 'The `unowned-code` refusal.\n' >> "$test_root/docs/specs/spec.md"
git -C "$test_root" add -A
if output=$(run); then fail "a stale allowlist entry passed: $output"; fi
case $output in
    *"now owned or no longer emitted"*unowned-code*) ;;
    *) fail "the stale entry was not named: $output" ;;
esac
: > "$test_root/script/error-code-ownership.allowlist"
git -C "$test_root" add -A
output=$(run) || fail "removing the stale entry did not restore a pass: $output"

# 4. Test files and untracked files are outside the scan.
printf 'package pkg\n\nvar _ = Error{Code: "test-only-code"}\n' > "$test_root/internal/pkg/pkg_test.go"
git -C "$test_root" add internal/pkg/pkg_test.go
printf 'package pkg\n\nvar _ = Error{Code: "untracked-code"}\n' > "$test_root/internal/pkg/scratch.go"
output=$(run) || fail "a test-file or untracked code failed the gate: $output"

# 5. A git grep failure is refused, never read as an empty emitted set.
cat > "$fakebin/git" <<'EOF'
#!/bin/sh
if [ "$1" = "grep" ]; then
    echo "fatal: fake git grep failure" >&2
    exit 128
fi
exec /usr/bin/git "$@"
EOF
chmod +x "$fakebin/git"
status=0
output=$(cd "$test_root" && PATH="$fakebin:$PATH" script/check-error-code-ownership.sh 2>&1) || status=$?
test "$status" -ne 0 || fail "a git grep failure passed the gate: $output"
case $output in
    *"git grep failed"*) ;;
    *) fail "the failure did not name git grep: $output" ;;
esac

# Cases 6-8 each stage one new file (case 4's untracked file stays untracked), expect its
# unowned code named with its site, then own it.
expect_named() {
    git -C "$test_root" add "${3%:*}" internal/other
    if output=$(run); then fail "an unowned $1 code passed: $output"; fi
    case $output in
        *"$2	$3"*) ;;
        *) fail "the $1 code or its site was not named: $output" ;;
    esac
    # shellcheck disable=SC2016
    printf 'The `%s` refusal.\n' "$2" >> "$test_root/docs/specs/spec.md"
    git -C "$test_root" add docs/specs/spec.md
    output=$(run) || fail "owning the $1 code did not restore a pass: $output"
}

# 6. A conversion to a discovered string type whose name ends in Error.
mkdir -p "$test_root/internal/other"
cat > "$test_root/internal/pkg/conversion.go" <<'EOF'
package pkg

type inventoryError string

func conversion() error { return inventoryError("converted-code") }
EOF
expect_named conversion converted-code internal/pkg/conversion.go:5

# 7. The argument at a later `code` parameter's index, including after a nested call; a
#    same-named helper without that parameter in another directory is not read as one.
cat > "$test_root/internal/pkg/receipt.go" <<'EOF'
package pkg

import "fmt"

func invalid(activation string, id *string, code string, limit int) map[string]any { return nil }

var _ = invalid(fmt.Sprint(1), nil, "third-argument-code", 1)
EOF
cat > "$test_root/internal/other/other.go" <<'EOF'
package other

func invalid(activation string, id *string, label string, limit int) {}

func unrelated() { invalid("", nil, "not-a-code", 1) }
EOF
expect_named parameter third-argument-code internal/pkg/receipt.go:7

# 8. The value at a `reason` target's index in a multi-target assignment.
cat > "$test_root/internal/pkg/classify.go" <<'EOF'
package pkg

func classify() (string, string) {
	class, reason := "INCLUDED", "none"
	class, reason = "UNSUPPORTED", "assigned-code"
	return class, reason
}
EOF
expect_named assignment assigned-code internal/pkg/classify.go:5

printf 'check-error-code-ownership_test: 8 cases passed\n'
