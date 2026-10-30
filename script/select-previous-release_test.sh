#!/usr/bin/env bash
set -euo pipefail
source_root=$(cd "$(dirname "$0")/.." && pwd -P)
selector="$source_root/script/select-previous-release.sh"
scratch=$(mktemp -d "${TMPDIR:-/tmp}/corvint-previous-release.XXXXXX")
trap 'rm -rf "$scratch"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
export GIT_AUTHOR_NAME=test GIT_AUTHOR_EMAIL=test@example.invalid GIT_COMMITTER_NAME=test GIT_COMMITTER_EMAIL=test@example.invalid
real_git=$(command -v git)
mkdir "$scratch/repo" "$scratch/bin"
cd "$scratch/repo"
git init -q -b main
git commit -q --allow-empty -m base
base=$(git rev-parse HEAD)
git tag v0.8.1
git tag -a v1.0.0-rc.1 -m release
annotated=$(git rev-parse refs/tags/v1.0.0-rc.1)
git commit -q --allow-empty -m candidate
candidate=$(git rev-parse HEAD)
tree=$(git rev-parse 'HEAD^{tree}')
disconnected=$(printf 'preserved release\n' | git commit-tree "$tree")
git tag -a v0.9.9 "$disconnected" -m preserved
preserved=$(git rev-parse refs/tags/v0.9.9)
future=$(printf 'future\n' | git commit-tree "$tree" -p "$candidate")
git tag v2.0.0 "$future"
pass() {
  local want=$1; shift
  "$selector" "$@" >"$scratch/out" 2>"$scratch/err"
  test "$(cat "$scratch/out")" = "$want"
  grep -Fq "tag=$want direct=" "$scratch/err"
  grep -Fq "candidate=$candidate" "$scratch/err"
}
fail() {
  local why=$1; shift
  local code=0
  "$selector" "$@" >"$scratch/out" 2>"$scratch/err" || code=$?
  if [ "$code" -ne 2 ]; then echo "exit $code, want refusal 2: $why" >&2; exit 1; fi
  test ! -s "$scratch/out"
  grep -Fq "$why" "$scratch/err"
  if grep -Fq 'previous-release: mode=' "$scratch/err"; then exit 1; fi
}
# SOP-V0-003: explicit predecessor binding changes selection only, never the upgrade contract.
pass v1.0.0-rc.1 "$candidate"
pass v1.0.0-rc.1 "$candidate" v1.0.0-rc.1 "$annotated"
grep -Fq "direct=$annotated commit=$base" "$scratch/err"
pass v0.8.1 "$candidate" v0.8.1 "$base"
grep -Fq "direct=$base commit=$base" "$scratch/err"
pass v0.9.9 "$candidate" v0.9.9 "$preserved"
grep -Fq "direct=$preserved commit=$disconnected" "$scratch/err"
git branch v0.8.1 "$candidate"
pass v0.8.1 "$candidate" v0.8.1 "$base"
git branch v0.7.7 "$base"
fail 'exact tag ref is missing' "$candidate" v0.7.7 "$base"
fail 'supplied together' "$candidate" v0.8.1 ''
fail 'supplied together' "$candidate" '' "$base"
fail 'usage:' "$candidate" v0.8.1
fail 'candidate must be a full' "$candidate"$'\nv1'
fail 'candidate must be a full' "${candidate:0:8}"
fail 'candidate object unavailable' 0000000000000000000000000000000000000000
fail 'candidate must name a commit object directly' "$annotated"
fail 'candidate must name a commit object directly' "$tree"
fail 'direct OID must be full' "$candidate" v0.8.1 "$base"$'\n'
fail 'tag direct OID differs' "$candidate" v0.8.1 "$candidate"
for bad in 'v1;echo bad' 'v1/../../main' 'v1..2' 'v1^{commit}' $'v1\nv2' 'v1 space' '-v1'; do
  fail 'invalid release tag spelling' "$candidate" "$bad" "$base"
done
blob=$(printf bytes | git hash-object -w --stdin)
git tag v0.0.1 "$blob"
fail 'must peel to a commit' "$candidate" v0.0.1 "$blob"
git tag v1.1.0 "$candidate"
fail 'names the candidate itself' "$candidate" v1.1.0 "$candidate"
fail 'descends from the candidate' "$candidate" v2.0.0 "$future"
# Invalid newest reachable spelling is a refusal, not a downgrade to an older release.
git tag 'v9!invalid' "$base"
fail 'invalid release tag spelling' "$candidate"
git tag -d 'v9!invalid' >/dev/null
# No ancestry is not a Git failure. An isolated candidate has no automatic predecessor: exit 3,
# distinct from every refusal (exit 2), with no tag on stdout.
code=0; "$selector" "$disconnected" >"$scratch/out" 2>"$scratch/err" || code=$?
test "$code" -eq 3; test ! -s "$scratch/out"
grep -Fq "previous-release: none: no reachable previous release tag before $disconnected" "$scratch/err"
if grep -Fq 'previous-release: mode=' "$scratch/err"; then exit 1; fi
# Both real info/grafts and ambient graft files must be ignored.
printf '%s %s\n' "$disconnected" "$candidate" > .git/info/grafts
pass v0.9.9 "$candidate" v0.9.9 "$preserved"
printf '%s %s\n' "$disconnected" "$candidate" > "$scratch/grafts"
GIT_GRAFT_FILE="$scratch/grafts" pass v0.9.9 "$candidate" v0.9.9 "$preserved"
rm .git/info/grafts
git replace "$disconnected" "$future"
pass v0.9.9 "$candidate" v0.9.9 "$preserved"
GIT_NO_REPLACE_OBJECTS=0 pass v0.9.9 "$candidate" v0.9.9 "$preserved"
git replace -d "$disconnected" >/dev/null
# Real Git errors must not be interpreted as disconnected ancestry or empty discovery.
printf '#!/bin/sh\ncase "$*" in *merge-base*) exit 128;; esac\nexec %q "$@"\n' "$real_git" > "$scratch/bin/git"
chmod +x "$scratch/bin/git"
PATH="$scratch/bin:$PATH" fail 'ancestry check failed (exit 128)' "$candidate" v0.8.1 "$base"
printf '#!/bin/sh\ncase "$*" in *"tag --merged"*) exit 128;; esac\nexec %q "$@"\n' "$real_git" > "$scratch/bin/git"
PATH="$scratch/bin:$PATH" fail 'reachable tag enumeration failed' "$candidate"
rm "$scratch/bin/git"
# Execute the actual workflow n1 run block with stubbed network/archive commands.
# Invalid selection must prevent gh; valid selection must retain identity via the real logger.
awk '/^          cat >"\$RUNNER_TEMP\/run-gate" <<.EOF./ {copy=1;next} copy && /^          EOF$/ {exit} copy {sub(/^          /, "");print}' \
  "$source_root/.github/workflows/release-gates.yml" > "$scratch/run-gate"
awk '/^      - name: Fetch the previous release/ {step=1;next} step && /^      - name:/ {exit} step && /^        run: \|/ {copy=1;next} copy {sub(/^          /, "");print}' \
  "$source_root/.github/workflows/release-gates.yml" > "$scratch/n1-step"
test -s "$scratch/run-gate"; test -s "$scratch/n1-step"
chmod +x "$scratch/run-gate"
mkdir -p script
cp "$selector" script/select-previous-release.sh
cp "$source_root/script/core-n1-source.sh" script/core-n1-source.sh
cat > "$scratch/bin/gh" <<SHIM
#!/bin/sh
printf '%s\n' "\$*" >> '$scratch/gh-invoked'
case "\$1 \$2" in
  'release view') printf '%s false\n' "\$3"; exit 0 ;;
  'release list') exit 0 ;;
esac
printf 'corvint_linux_amd64.tar.gz\n' > "\$RUNNER_TEMP/n1/SHA256SUMS"
printf '#!/bin/sh\nexit 0\n' > "\$RUNNER_TEMP/previous/corvint"
chmod +x "\$RUNNER_TEMP/previous/corvint"
SHIM
for command in tar sha256sum; do printf '#!/bin/sh\nexit 0\n' > "$scratch/bin/$command"; done
chmod +x "$scratch/bin/gh" "$scratch/bin/tar" "$scratch/bin/sha256sum"
export SHA="$candidate" RELEASE=fixture RUNNER_LABEL=local ImageVersion=fixture
export GITHUB_SERVER_URL=https://example.invalid GITHUB_REPOSITORY=beamfall/corvint GITHUB_RUN_ID=1 GITHUB_RUN_ATTEMPT=1
export PREVIOUS_RELEASE_TAG=v0.9.9 PREVIOUS_RELEASE_OID="$candidate"
export RUNNER_TEMP="$scratch/refused" GITHUB_OUTPUT="$scratch/refused-output" GITHUB_STEP_SUMMARY="$scratch/summary"
mkdir -p "$RUNNER_TEMP/evidence"; cp "$scratch/run-gate" "$RUNNER_TEMP/run-gate"
if PATH="$scratch/bin:$PATH" bash -e "$scratch/n1-step" >"$scratch/step-out" 2>&1; then exit 1; fi
test ! -e "$scratch/gh-invoked"
test ! -s "$GITHUB_OUTPUT"
grep -Fq 'tag direct OID differs' "$RUNNER_TEMP/evidence/n1-archive-fixture-local.log"
export PREVIOUS_RELEASE_OID="$preserved" RUNNER_TEMP="$scratch/accepted" GITHUB_OUTPUT="$scratch/accepted-output"
mkdir -p "$RUNNER_TEMP/evidence"; cp "$scratch/run-gate" "$RUNNER_TEMP/run-gate"
PATH="$scratch/bin:$PATH" bash -e "$scratch/n1-step" >"$scratch/step-out" 2>&1
grep -Fqx 'release view v0.9.9 --repo beamfall/corvint --json tagName,isDraft --jq "\(.tagName) \(.isDraft)"' "$scratch/gh-invoked"
grep -Fq 'release download v0.9.9 --repo beamfall/corvint' "$scratch/gh-invoked"
test "$(cat "$GITHUB_OUTPUT")" = first_release=false
grep -Fq "tag=v0.9.9 direct=$preserved commit=$disconnected" "$RUNNER_TEMP/evidence/n1-archive-fixture-local.log"
test -s "$RUNNER_TEMP/evidence/n1-archive-fixture-local.log.sha256"
# FIRST_RELEASE (V1-3177): no reachable tag and no published release is a visible, logged
# not-applicable outcome with first_release=true, and nothing is downloaded.
rm "$scratch/gh-invoked"
export SHA="$disconnected" PREVIOUS_RELEASE_TAG='' PREVIOUS_RELEASE_OID='' RUNNER_TEMP="$scratch/first" GITHUB_OUTPUT="$scratch/first-output"
mkdir -p "$RUNNER_TEMP/evidence"; cp "$scratch/run-gate" "$RUNNER_TEMP/run-gate"
PATH="$scratch/bin:$PATH" bash -e "$scratch/n1-step" >"$scratch/step-out" 2>&1
test "$(cat "$GITHUB_OUTPUT")" = first_release=true
grep -Fq 'release list --repo beamfall/corvint --exclude-drafts' "$scratch/gh-invoked"
if grep -Fq 'release download' "$scratch/gh-invoked"; then exit 1; fi
grep -Fq "core-n1-source: FIRST_RELEASE: no published release of beamfall/corvint precedes $disconnected" "$RUNNER_TEMP/evidence/n1-archive-fixture-local.log"
grep -Fq '::notice title=N-1::FIRST_RELEASE' "$scratch/step-out"
grep -Fq "N-1 FIRST_RELEASE: no published release of beamfall/corvint precedes $disconnected" "$GITHUB_STEP_SUMMARY"
grep -Fqx 'n1-archive EXIT 0' "$RUNNER_TEMP/evidence/n1-archive-fixture-local.log"
printf 'previous release selector and workflow boundary: PASS\n'
