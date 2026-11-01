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
#   claude-attribution a lane host whose Claude Code attribution setting was
#                      unset added an AI Co-Authored-By trailer to every commit,
#                      crediting Claude as a GitHub contributor (V1-3227,
#                      ASD-V0-561); script/check-ai-coauthor-trailers.sh refuses
#                      those commits, and this check names the setting.
#   landlock           test-confine -probe reported landlock abi 0 in the Unraid
#                      lane container, so read-scope violations surfaced only in
#                      CI (V1-3741, ASD-V0-643). The row reports the host's ABI
#                      and never fails the preflight; CORVINT_LANDLOCK_PROBE names
#                      a probe command to run instead of building test-confine.
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
# The Claude Code attribution setting, read without jq: the user file, then the checkout's
# project files, the last one that sets attribution winning. Both values must be "".
attribution_fix='set "attribution": {"commit": "", "pr": ""} in'
attr_state() { # prints: off, legacy-off, on, or unset
	flat=$(tr -d ' \t\r\n' < "$1" 2>/dev/null) || { echo unset; return; }
	case $flat in
	*'"attribution":{'*)
		attr=${flat#*'"attribution":{'}
		attr=${attr%%\}*}
		case $attr in
		*'"commit":""'*'"pr":""'* | *'"pr":""'*'"commit":""'*) echo off ;;
		*) echo on ;;
		esac
		;;
	*'"includeCoAuthoredBy":false'*) echo legacy-off ;;
	*) echo unset ;;
	esac
}
claude_dir=${CLAUDE_CONFIG_DIR:-${HOME:-}/.claude}
if [ ! -d "$claude_dir" ]; then
	note "claude-attribution" "no $claude_dir -- not a Claude Code host"
else
	state=unset
	from="$claude_dir/settings.json"
	for f in "$claude_dir/settings.json" "$root/.claude/settings.json" "$root/.claude/settings.local.json"; do
		[ -f "$f" ] || continue
		s=$(attr_state "$f")
		if [ "$s" != unset ]; then state=$s; from=$f; fi
	done
	case $state in
	off) note "claude-attribution" "commit and pr attribution off ($from)" ;;
	legacy-off) note "claude-attribution" "includeCoAuthoredBy is false ($from); the current form is \"attribution\": {\"commit\": \"\", \"pr\": \"\"}" ;;
	on) fail "claude-attribution" "attribution is set but not empty in $from -- Claude Code will add an AI Co-Authored-By trailer; $attribution_fix $from" ;;
	*) fail "claude-attribution" "unset -- Claude Code adds an AI Co-Authored-By trailer to every commit by default; $attribution_fix $claude_dir/settings.json" ;;
	esac
fi
# Landlock ABI for confined test runs (ASD-V0-643): test-confine refuses a declared package below
# ABI 2, so a lane on such a host never sees a read-scope violation before CI.
host=$(hostname 2>/dev/null || uname -n)
probe=${CORVINT_LANDLOCK_PROBE:-}
probe_dir=
if [ -z "$probe" ] && [ "$(uname -s)" != Linux ]; then
	note "landlock" "probe is Linux-only ($(uname -s) on $host) -- confined test runs happen on a Linux host or in CI"
else
	why="cannot build test-confine from .github/testconfine/cmd"
	if [ -z "$probe" ]; then
		# A temporary-directory failure is an UNKNOWN row, not a set -e abort of the preflight.
		if probe_dir=$(mktemp -d 2>/dev/null) && [ -n "$probe_dir" ]; then
			probe=$probe_dir/test-confine
			(cd "$root" && go build -trimpath -buildvcs=false -o "$probe" ./.github/testconfine/cmd >/dev/null 2>&1) || probe=
		else
			probe_dir=
			why="cannot create a temporary directory for the test-confine build (TMPDIR=${TMPDIR:-unset})"
		fi
	fi
	if [ -z "$probe" ]; then
		note "landlock" "UNKNOWN on $host: $why"
	else
		probed=$("$probe" -probe 2>&1) && probe_rc=0 || probe_rc=$?
		abi=${probed#landlock abi }
		case $probed in
		'landlock abi '*) ;;
		*) abi= ;;
		esac
		# test-confine -probe exits 0 for an ABI of 2 or more and 1 below it; any other exit, or one
		# that disagrees with the printed ABI, is not a probe result.
		case $abi in
		'' | *[!0-9]*) ;;
		0 | 1) [ "$probe_rc" = 1 ] || abi= ;;
		*) [ "$probe_rc" = 0 ] || abi= ;;
		esac
		case $abi in
		'' | *[!0-9]*) note "landlock" "UNKNOWN on $host: test-confine -probe exited $probe_rc and printed '$probed'" ;;
		0 | 1) note "landlock" "WARN abi $abi on $host: test-confine refuses confined runs here (exit 2); read-scope violations surface only in CI, so run confined tests in CI or on a host whose probe reports abi 2 or more" ;;
		*) note "landlock" "abi $abi on $host, confined test runs enforce read scopes" ;;
		esac
	fi
	[ -z "$probe_dir" ] || rm -rf "$probe_dir"
fi
printf '\n%s\n' "Build and test only the packages you own; a red ./... here is usually not yours."
exit "$status"
