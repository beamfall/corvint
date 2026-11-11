#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# OSC-V1-012 (V1-5399): script/openspec-validate.sh with no command, a command at another version
# and a fake command at the pin, each outside and inside CI. The fake checks that it runs from
# docs/specs with OPENSPEC_TELEMETRY=0, and fails `validate --all` when FAKE_INVALID names a folder
# whose requirement has no scenario, as the real command does. When the real command is installed
# at the pin, a fixture root whose change has a requirement with no scenario must fail it;
# otherwise that case prints NOT_RUN.
set -eu
source_root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
work=$(mktemp -d "${TMPDIR:-/tmp}/corvint-openspec-validate-test.XXXXXX")
trap 'rm -rf "$work"' EXIT INT TERM
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }
validate() { env -u CI "$@" "$source_root/script/openspec-validate.sh" 1.17.0; }

cat > "$work/openspec" <<'FAKE'
#!/bin/sh
test "${OPENSPEC_TELEMETRY:-}" = 0 || { echo "telemetry on" >&2; exit 9; }
case $1 in
  --version) echo "${FAKE_VERSION:-1.17.0}"; exit 0 ;;
  validate) ;;
  *) echo "unexpected $*" >&2; exit 9 ;;
esac
case $(pwd) in */docs/specs) ;; *) echo "not run from docs/specs: $(pwd)" >&2; exit 9 ;; esac
if test -n "${FAKE_INVALID:-}"; then
  echo "✗ change/$FAKE_INVALID: Requirement must have at least one scenario"
  exit 1
fi
if test "${3:-}" = --strict; then
  echo "⚠ change/openspec-check: WARNING Requirement text is over 500 characters"
  exit 1
fi
echo "Totals: 46 passed, 0 failed (46 items)"
FAKE
chmod 0755 "$work/openspec"

out=$(validate OPENSPEC="$work/missing/openspec") || fail "absent command outside CI: want exit 0"
case $out in *"openspec-validate: NOT_RUN the OpenSpec command ($work/missing/openspec) is not installed"*"@fission-ai/openspec@1.17.0"*) ;;
  *) fail "absent command outside CI printed: $out" ;; esac
if out=$(validate CI=true OPENSPEC="$work/missing/openspec" 2>&1); then fail "absent command in CI: want a failure"; fi
case $out in *"openspec-validate: FAIL the OpenSpec command"*"in CI the pinned command is required"*) ;;
  *) fail "absent command in CI printed: $out" ;; esac

out=$(validate OPENSPEC="$work/openspec" FAKE_VERSION=1.16.0) || fail "other version outside CI: want exit 0"
case $out in *"NOT_RUN $work/openspec is version 1.16.0, not the pinned 1.17.0"*) ;; *) fail "other version printed: $out" ;; esac
if validate CI=true OPENSPEC="$work/openspec" FAKE_VERSION=1.16.0 > /dev/null 2>&1; then fail "other version in CI: want a failure"; fi

out=$(validate OPENSPEC="$work/openspec") || fail "valid folders: want exit 0, got: $out"
case $out in *"Totals: 46 passed"*"openspec-validate: strict: ⚠ change/openspec-check: WARNING"*"PASS openspec 1.17.0 validate --all in docs/specs; --strict printed 1 warning line(s)"*) ;;
  *) fail "valid folders printed: $out" ;; esac
validate CI=true OPENSPEC="$work/openspec" > /dev/null || fail "valid folders in CI: want exit 0"

if out=$(validate OPENSPEC="$work/openspec" FAKE_INVALID=no-scenario 2>&1); then fail "invalid folder: want a failure"; fi
case $out in *"change/no-scenario: Requirement must have at least one scenario"*"openspec-validate: FAIL openspec 1.17.0 validate --all exited 1"*) ;;
  *) fail "invalid folder printed: $out" ;; esac

"$source_root/script/openspec-validate.sh" > /dev/null 2>&1 && fail "no pin: want a usage error"

# The real command, when installed at the pin, refuses a requirement with no scenario.
real=$(command -v openspec 2> /dev/null || true)
if test -n "$real" && test "$(OPENSPEC_TELEMETRY=0 "$real" --version 2> /dev/null | tail -n 1 | tr -d '[:space:]')" = 1.17.0; then
  fixture=$work/real/docs/specs/openspec
  mkdir -p "$fixture/changes/no-scenario/specs/thing"
  cp "$source_root/docs/specs/openspec/config.yaml" "$fixture/config.yaml"
  cat > "$fixture/changes/no-scenario/proposal.md" <<'MD'
# Change proposal: no-scenario

## Why

This fixture change exists only to prove that the pinned OpenSpec command refuses a requirement
that has no scenario block, as OSC-V1-012 requires of make openspec-validate.

## What Changes

- One added requirement, deliberately written without a scenario.
MD
  printf '# Tasks\n\n- [ ] 1 Build it. Requirements: NOS-V1-001. Proof: test.\n' > "$fixture/changes/no-scenario/tasks.md"
  cat > "$fixture/changes/no-scenario/specs/thing/spec.md" <<'MD'
## ADDED Requirements

### Requirement: NOS-V1-001 A requirement without a scenario

The fixture tool MUST do one thing, and this requirement has no scenario block.
MD
  if out=$(env -u CI OPENSPEC_VALIDATE_ROOT="$work/real" "$source_root/script/openspec-validate.sh" 1.17.0 2>&1); then
    fail "the real command passed a requirement with no scenario: $out"
  fi
  case $out in *[Ss]cenario*) ;; *) fail "the real command's refusal names no scenario: $out" ;; esac
  printf 'real-command fixture: PASS (openspec 1.17.0 refused the requirement with no scenario)\n'
else
  printf 'real-command fixture: NOT_RUN (openspec 1.17.0 is not installed)\n'
fi
printf 'ok\n'
