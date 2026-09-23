#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-or-later
# Clean-checkout authoring proof for evidence-provider kit 0.2.0 (EEP-V0-022).
# Usage: authoring-proof.sh REPOSITORY COMMIT CORVINT
# Checks out only examples/evidence-provider/v0 of REPOSITORY at COMMIT into a
# fresh scratch clone, builds the conformance runner there offline, and runs it
# against the CORVINT binary. Needs Git, local Go 1.27.1 and Darwin or Linux.
set -eu
if [ "$#" -ne 3 ]; then
	echo "usage: authoring-proof.sh REPOSITORY COMMIT CORVINT" >&2
	exit 2
fi
repository=$1
commit=$2
corvint=$3
work=$(mktemp -d "${TMPDIR:-/tmp}/corvint-kit-proof.XXXXXX")
trap 'rm -rf "$work"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
git clone -q --no-checkout "$repository" "$work/clone"
git -C "$work/clone" sparse-checkout set --no-cone /examples/evidence-provider/v0/
git -C "$work/clone" checkout -q --detach "$commit"
outside=$(cd "$work/clone" && find . -path ./.git -prune -o -type f ! -path './examples/evidence-provider/v0/*' -print)
if [ -n "$outside" ]; then
	echo "checkout is not the kit directory alone: $outside" >&2
	exit 1
fi
kit="$work/clone/examples/evidence-provider/v0"
(cd "$kit" && env GOTOOLCHAIN=local GO111MODULE=off GOENV=off GOWORK=off GOFLAGS= GOPROXY=off GOSUMDB=off \
	GOCACHE="$work/gocache" go build -trimpath -o "$work/conformance" ./conformance)
"$work/conformance" -kit "$kit" -corvint "$corvint"
