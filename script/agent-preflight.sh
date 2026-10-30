#!/bin/sh
# Preflight for an agent about to work in this checkout, and for the operator
# before believing a parity result.
#
# Every check here fired for real during the 2026-08-29 session, and each one
# cost more to diagnose from its symptom than it costs to run:
#
#   installed binary   two agents produced full "not mine" exonerations for a
#                      parity failure that was a day-old ~/.local/bin/corvint.
#                      manifest.json sets candidateCommand: corvint, so a bare
#                      replay tests whatever is installed, never this tree.
#   staged residue     a shared index across concurrent agents absorbed one
#                      agent's files into another's commit twice.
#   oracle residue     src/ was the frozen conformance oracle, and this check
#                      refused a candidate that edited it. Superseded 2026-09-12
#                      by decision 0088: the Python oracle is retired and src/ is
#                      deleted, so the dirty-src/ check could never fire again.
#                      It now guards the retirement instead -- src/ must hold no
#                      tracked path, and one reappearing means it didn't stick.
#   own-package build  `go build ./...` in a live tree reports other agents'
#                      half-written files, which reads as your own breakage.
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
status=0

note() { printf '%-18s %s\n' "$1" "$2"; }
fail() { note "$1" "$2"; status=1; }

installed=$(command -v corvint 2>/dev/null || true)
if [ -z "$installed" ]; then
	note "installed-binary" "absent - the harness hook will fall back"
else
	head_sha=$(git -C "$root" rev-parse --short HEAD)
	built=$(mktemp -u)
	if (cd "$root" && go build -o "$built" ./cmd/corvint 2>/dev/null); then
		if cmp -s "$built" "$installed"; then
			note "installed-binary" "current with $head_sha"
		else
			fail "installed-binary" "STALE vs $head_sha: $installed -- refresh before trusting a replay or a hook receipt"
		fi
		rm -f "$built"
	else
		fail "installed-binary" "cannot build candidate to compare"
	fi
fi

staged=$(git -C "$root" diff --cached --name-only | wc -l | tr -d ' ')
if [ "$staged" = 0 ]; then
	note "staged-residue" "index clean"
else
	fail "staged-residue" "$staged file(s) already staged by another agent -- commit ONLY with an explicit pathspec"
fi

if oracle_paths=$(git -C "$root" ls-files src/); then
	if [ -z "$oracle_paths" ]; then
		oracle_src=0
	else
		oracle_src=$(printf '%s\n' "$oracle_paths" | wc -l | tr -d ' ')
	fi
	if [ "$oracle_src" = 0 ]; then
		note "oracle-retired" "src/ has no tracked path"
	else
		fail "oracle-retired" "src/ has $oracle_src tracked path(s) -- decision 0088 retired the Python oracle, it must not come back"
	fi
else
	fail "oracle-retired" "git ls-files failed -- cannot verify that src/ has no tracked path"
fi

# Refuses only when the caller names the paths it owns (as arguments). With many
# sessions sharing one checkout, "some tracked file was modified by someone" is the
# normal state, and a refusal built on that signal alone would fire on every run and
# get turned off. Given an owned-path list, a modified path OUTSIDE it is exactly what
# a broad `git add -u`/`git add -A` would silently absorb, so failing then is actionable.
concurrent=$(git -C "$root" status --short | grep '^ [MDT]' | cut -c4- || true)
if [ -z "$concurrent" ]; then
	note "concurrent-writers" "no tracked file modified by anyone else"
elif [ "$#" -eq 0 ]; then
	note "concurrent-writers" "$(printf '%s\n' "$concurrent" | wc -l | tr -d ' ') tracked file(s) modified by someone -- pass the paths you own as arguments to enable a hard refusal; until then, stage ONLY the paths you wrote"
	printf '%s\n' "$concurrent" | sed 's/^/                   /'
else
	owned_file=$(mktemp)
	printf '%s\n' "$@" > "$owned_file"
	foreign=$(printf '%s\n' "$concurrent" | grep -Fxvf "$owned_file" || true)
	rm -f "$owned_file"
	if [ -z "$foreign" ]; then
		note "concurrent-writers" "$(printf '%s\n' "$concurrent" | wc -l | tr -d ' ') tracked file(s) modified by someone, all within the $# path(s) you named -- safe to stage exactly those paths"
		printf '%s\n' "$concurrent" | sed 's/^/                   /'
	else
		fail "concurrent-writers" "$(printf '%s\n' "$foreign" | wc -l | tr -d ' ') tracked file(s) modified by someone else, outside the $# path(s) you named -- stage ONLY your named paths with an explicit pathspec, never git add -u or git add -A"
		printf '%s\n' "$foreign" | sed 's/^/                   /'
	fi
fi
printf '\n%s\n' "Build and test only the packages you own; a red ./... here is usually not yours."
exit "$status"
