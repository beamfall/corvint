#!/bin/sh
# SPDX-License-Identifier: Apache-2.0
# Reference installer for pinned Corvint binaries on a CI host (PCH-V0-006).
# Usage: install-pinned.sh COMMAND...
# Requires CORVINT_SOURCE_COMMIT (full 40-hex commit) and CORVINT_PINS (a
# sha256sum-format file listing every COMMAND by bare name). Optional
# CORVINT_SOURCE_URL overrides the source repository. It builds each COMMAND
# from the pinned source, refuses any binary whose digest is not pinned, and
# only then adds the directory to GITHUB_PATH. Rebuilding the pin file is an
# operator step on a trusted machine with the same Go toolchain and platform.
set -eu

: "${CORVINT_SOURCE_COMMIT:?CORVINT_SOURCE_COMMIT is required}"
: "${CORVINT_PINS:?CORVINT_PINS is required}"
url=${CORVINT_SOURCE_URL:-https://github.com/beamfall/corvint.git}
if ! printf '%s' "$CORVINT_SOURCE_COMMIT" | grep -Eqx '[0-9a-f]{40}'; then
  echo "install-pinned: CORVINT_SOURCE_COMMIT must be a full 40-hex commit" >&2
  exit 1
fi
if [ "$#" -eq 0 ]; then
  echo "install-pinned: name at least one command" >&2
  exit 1
fi
pins=$(cd "$(dirname "$CORVINT_PINS")" && pwd)/$(basename "$CORVINT_PINS")
base=${RUNNER_TEMP:-${TMPDIR:-/tmp}}
src="$base/corvint-src"
bin="$base/corvint-bin"
rm -rf "$src" "$bin"
mkdir -p "$src" "$bin"
git -C "$src" init -q
git -C "$src" fetch -q --depth 1 "$url" "$CORVINT_SOURCE_COMMIT"
git -C "$src" -c advice.detachedHead=false checkout -q FETCH_HEAD
if [ "$(git -C "$src" rev-parse HEAD)" != "$CORVINT_SOURCE_COMMIT" ]; then
  echo "install-pinned: fetched source does not match the pinned commit" >&2
  exit 1
fi
for name in "$@"; do
  case "$name" in
    corvint | corvint-[a-z]*) ;;
    *) echo "install-pinned: $name is not a Corvint command" >&2; exit 1 ;;
  esac
  (cd "$src" && CGO_ENABLED=0 GOTOOLCHAIN=local go build -trimpath -buildvcs=false -o "$bin/$name" "./cmd/$name")
  if ! grep -Eq "^[0-9a-f]{64}  $name\$" "$pins"; then
    echo "install-pinned: $name has no pin" >&2
    exit 1
  fi
done
(cd "$bin" && for name in "$@"; do grep -E "^[0-9a-f]{64}  $name\$" "$pins"; done | sha256sum -c -)
if [ -n "${GITHUB_PATH:-}" ]; then
  echo "$bin" >> "$GITHUB_PATH"
else
  echo "install-pinned: add $bin to PATH"
fi
