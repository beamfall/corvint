#!/usr/bin/env bash
# Temp-fixture test for script/pre-push-windows-vet.sh (V1-1725, ASD-V0-023): a real `git push`
# through the hook on a tiny Go module, where a test file references a helper whose build tag
# excludes windows.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
hook="$here/pre-push-windows-vet.sh"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/pre-push-vet-test.XXXXXX")
tmp=$(cd "$tmp" && pwd -P)
trap '/bin/rm -rf "$tmp"' EXIT INT TERM
fail() { printf 'pre-push-windows-vet_test: FAIL %s\n' "$*" >&2; exit 1; }
export GIT_CONFIG_NOSYSTEM=1 HOME="$tmp/home" GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid \
  GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.invalid
export GOCACHE=${CORVINT_GOCACHE:-${GOCACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/corvint-go-build-cache}} GOTOOLCHAIN=local GOFLAGS= GOWORK=off
mkdir -p "$HOME"
git config --global init.defaultBranch main
q() { git "$@" > /dev/null 2>&1; }

q init --bare "$tmp/origin.git"
work="$tmp/work"
q clone "$tmp/origin.git" "$work"
mkdir -p "$work/script"
/bin/cp -f "$hook" "$work/script/pre-push-windows-vet.sh"
# The documented install: a wrapper in the hooks directory that runs the checkout's own copy.
hooks=$(git -C "$work" rev-parse --path-format=absolute --git-path hooks)
mkdir -p "$hooks"
printf '#!/bin/sh\nexec script/pre-push-windows-vet.sh "$@"\n' > "$hooks/pre-push"
chmod +x "$hooks/pre-push"
cat > "$work/Makefile" << 'EOF'
windows-vet:
	@echo vet-ran >> vet.log
	@GOENV=off CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go vet ./...
EOF
printf 'vet.log\nscript/\n' > "$work/.gitignore"
printf 'module example.com/fixture\n\ngo 1.21\n' > "$work/go.mod"
printf 'package fixture\n\nfunc Add(a, b int) int { return a + b }\n' > "$work/add.go"
q -C "$work" add -A && q -C "$work" commit -m base && q -C "$work" push origin main || fail "base push refused"
[[ $(grep -c vet-ran "$work/vet.log") == 1 ]] || fail "base push did not run the vet"

# 1. A test that references a unix-only helper is refused before push.
printf '//go:build !windows\n\npackage fixture\n\nfunc helper() int { return 1 }\n' > "$work/helper_other.go"
printf 'package fixture\n\nimport "testing"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(helper(), 1) != 2 {\n\t\tt.Fatal("add")\n\t}\n}\n' > "$work/add_test.go"
q -C "$work" add -A && q -C "$work" commit -m unix-only
if git -C "$work" push origin main > "$tmp/out1" 2>&1; then cat "$tmp/out1" >&2; fail "unix-only helper push was accepted"; fi
grep -q 'REFUSE windows-vet-failed' "$tmp/out1" || { cat "$tmp/out1" >&2; fail "no windows-vet-failed refusal"; }
grep -q 'undefined: helper' "$tmp/out1" || { cat "$tmp/out1" >&2; fail "vet error not shown"; }
[[ $(git -C "$tmp/origin.git" rev-parse main) != $(git -C "$work" rev-parse HEAD) ]] || fail "refused commit reached the remote"

# 2. Making the helper portable lets the push through.
printf 'package fixture\n\nfunc helper() int { return 1 }\n' > "$work/helper_other.go"
q -C "$work" commit -am portable
git -C "$work" push origin main > "$tmp/out2" 2>&1 || { cat "$tmp/out2" >&2; fail "portable push refused"; }

# 3. A docs-only push does not run the vet.
runs=$(grep -c vet-ran "$work/vet.log")
echo docs > "$work/README.md" && q -C "$work" add README.md && q -C "$work" commit -m docs
git -C "$work" push origin main > "$tmp/out3" 2>&1 || { cat "$tmp/out3" >&2; fail "docs push refused"; }
[[ $(grep -c vet-ran "$work/vet.log") == "$runs" ]] || fail "docs-only push ran the vet"

# 4. A Go-changing branch that is not checked out is refused without vetting the wrong tree.
q -C "$work" branch side && q -C "$work" checkout side
echo '// side' >> "$work/add.go" && q -C "$work" commit -am side
q -C "$work" checkout main
if git -C "$work" push origin side > "$tmp/out4" 2>&1; then fail "push of a non-HEAD Go branch was accepted"; fi
grep -q 'REFUSE windows-vet-not-run' "$tmp/out4" || { cat "$tmp/out4" >&2; fail "no windows-vet-not-run refusal"; }

# 5. An annotated tag on the checked-out Go-changing HEAD is peeled and vetted, not refused.
echo '// tagged' >> "$work/add.go" && q -C "$work" commit -am tagged
q -C "$work" tag -a v1 -m v1
runs=$(grep -c vet-ran "$work/vet.log")
git -C "$work" push origin v1 > "$tmp/out5" 2>&1 || { cat "$tmp/out5" >&2; fail "annotated tag push refused"; }
[[ $(grep -c vet-ran "$work/vet.log") == $((runs + 1)) ]] || fail "annotated tag push did not run the vet"

# 6. An untracked Go file (the vet would read it, the push would not carry it) is refused.
printf 'package fixture\n' > "$work/stray.go"
if git -C "$work" push origin main > "$tmp/out6" 2>&1; then fail "push with an untracked Go file was accepted"; fi
grep -q 'REFUSE windows-vet-not-run untracked Go files' "$tmp/out6" || { cat "$tmp/out6" >&2; fail "no untracked-Go refusal"; }

echo "pre-push-windows-vet_test: ok"
