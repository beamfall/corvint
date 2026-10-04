#!/bin/sh
# Local, read-only doctor for contradictions between this repository's agent instructions and the
# host-side files an owner points it at: hook settings, agent memory, global instruction files.
# V1-0721; the decision and its limits are in
# docs/build-log/2026-10-04-v1-0721-instruction-conflicts-doctor.md.
#
# Two contradictions reached agents on 2026-10-04 before anyone saw them: AGENTS.md required
# draft pull requests while a PreToolUse hook refused the draft flag, and AGENTS.md required
# approval to merge while a saved memory and a global instruction file preauthorized merging.
#
# Host files are not Git-pinned evidence, so every finding is local and uncertain, and a conflict
# never decides which side wins: project-owned files govern this repository. Nothing here is a
# gate input, and nothing outside the named files is read; no home or session discovery happens.
# The only write is a private temporary list of inputs, removed on exit.
#
# Claims come from a closed table of three topics matched per sentence or semicolon clause (lines
# of one paragraph are joined first, so wrapped prose still matches). A paraphrase outside the
# table is missed, so NO_CONFLICT_FOUND means only that the table found none.
#
# usage: script/check-instruction-conflicts.sh [--project FILE]... [--host FILE|DIR]...
#   Without --project, reads AGENTS.md (UNKNOWN if absent) and, if present, CLAUDE.md at the
#   repository root (working-tree bytes). --host DIR reads DIR/*.md, not recursively.
# exit: 0 no conflict found and every input read; 1 conflict found; 2 usage error;
#   3 UNKNOWN: no conflict found, but an input was absent, unreadable or not a regular file, or
#   no --host was given.
set -eu
root=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd -P)

usage() {
	printf 'usage: %s [--project FILE]... [--host FILE|DIR]...\n' "$0" >&2
	exit 2
}

inputs=$(mktemp "${TMPDIR:-/tmp}/corvint-instruction-conflicts.XXXXXX")
trap 'rm -f "$inputs"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

# Only a readable regular file reaches awk; anything else is recorded as an unknown input.
add() {
	if [ -f "$2" ] && [ -r "$2" ]; then
		printf '%s\t%s\n' "$1" "$2" >>"$inputs"
	else
		printf 'unknown\t%s\t%s\n' "$1" "$2" >>"$inputs"
	fi
}

projects=0
hosts=0
while [ $# -gt 0 ]; do
	case $1 in
	--project)
		[ $# -ge 2 ] || usage
		add project "$2"
		projects=$((projects + 1))
		shift 2
		;;
	--host)
		[ $# -ge 2 ] || usage
		if [ -d "$2" ]; then
			found=0
			for f in "$2"/*.md; do
				[ -e "$f" ] || continue
				add host "$f"
				found=1
			done
			# An empty directory is an input that yielded nothing to read, not a pass.
			[ "$found" = 1 ] || printf 'unknown\thost\t%s\n' "$2/*.md" >>"$inputs"
		else
			add host "$2"
		fi
		hosts=$((hosts + 1))
		shift 2
		;;
	*) usage ;;
	esac
done
if [ "$projects" = 0 ]; then
	# AGENTS.md is the project contract, so its absence is unknown; CLAUDE.md is optional.
	add project "$root/AGENTS.md"
	if [ -e "$root/CLAUDE.md" ]; then add project "$root/CLAUDE.md"; fi
fi

# Paths go through ENVIRON, because awk -v would interpret backslashes in them.
CORVINT_IC_ROOT="$root/" CORVINT_IC_HOME="${HOME:-}" awk -F '\t' -v hosts="$hosts" '
BEGIN {
	root = ENVIRON["CORVINT_IC_ROOT"]
	home = ENVIRON["CORVINT_IC_HOME"]
}
function display(path) {
	if (index(path, root) == 1) return substr(path, length(root) + 1)
	if (home != "" && index(path, home "/") == 1) return "~" substr(path, length(home) + 1)
	return path
}
function lineat(pos,   k, best) {
	best = lnum[1]
	for (k = 1; k <= nl; k++) if (off[k] <= pos) best = lnum[k]
	return best
}
function claim(topic, polarity, ln,   key, ref) {
	key = topic SUBSEP polarity
	ref = layer " " name ":" ln
	if ((key SUBSEP ref) in seen) return
	seen[key SUBSEP ref] = 1
	refs[key] = refs[key] (refs[key] == "" ? "" : ", ") ref
}
function classify(sentence, ln,   t, m, draftneg, negated, amneeds, mneg) {
	t = tolower(sentence)
	# Manual-merge rules must not see the word merge inside auto-merge.
	m = t
	gsub(/auto-merge/, "auto_m_", m)

	# A negation or refusal ahead of "draft" ("never open draft pull requests", "a hook refuses
	# draft pull requests"), or a prohibition after it, turns a draft mention into a refusal.
	draftneg = (t ~ /(^|[^a-z])(no|not|never|non|refuses?|refused|den(y|ies|ied)|forbids?|blocks?|blocked)[- ]([^.]* )?draft/ || t ~ /draft (pull requests?|prs?)[^.]*(not allowed|disabled|refused|forbidden|prohibited|blocked)/)
	if (t ~ /(^|[^-a-z])draft pull requests?/ && !draftneg)
		claim("pr-draft", "draft-required", ln)
	if ((draftneg && t ~ /draft (pull requests?|prs?)/) || t ~ /non-draft|ready pull requests?|(deny|refuse|refuses|refused)[^.]*--draft|--draft[^.]*(deny|refuse)/)
		claim("pr-draft", "draft-refused", ln)

	# A negated sentence ("do not arm auto-merge on a stacked PR") is usually a scoped limit, so it
	# only withholds the allowed claim instead of asserting that auto-merge needs approval. A
	# sentence that makes arming need approval is not also an allowance.
	negated = (t ~ /(do not|don.t|never|must not)[^.]*(^|[^a-z])(arm|enable|turn on)[^.]*auto-merge/)
	amneeds = (t ~ /auto-merge[^.]*(requires?|needs?) (explicit|separate) (approval|confirmation)/)
	if (!negated && !amneeds && t ~ /(^|[^a-z])(arm|enable|turn on)[^.]*auto-merge|auto-merge[^.]*(by default|without (asking|offering))/)
		claim("pr-auto-merge", "allowed", ln)
	if (amneeds)
		claim("pr-auto-merge", "needs-approval", ln)

	if (m ~ /merg(e|es|ing)[^.]*(require|requires|need|needs) (explicit|separate|their own) (approval|confirmation|authori[sz]ation)/) {
		claim("pr-manual-merge", "needs-approval", ln)
		# A rule about merging in general also covers arming auto-merge; one about merging by
		# hand does not.
		if (m !~ /by hand|manual/) claim("pr-auto-merge", "needs-approval", ln)
	}
	# "Do not merge without asking" forbids, so a negation withholds the without-asking match.
	mneg = (m ~ /(do not|don.t|never|must not)[^.]*(^|[^a-z_])merge/)
	if (m ~ /merges? (are|is) (pre-?authori[sz]ed|pre-?approved)/ || (!mneg && m ~ /(^|[^a-z_])merge[^.]*without asking/))
		claim("pr-manual-merge", "preauthorized", ln)
}
function flush(   rest, pos, i, len, sentence) {
	if (para == "") return
	rest = para
	pos = 1
	while (rest != "") {
		# A semicolon also ends a clause, so a negation in one clause cannot flip the next.
		i = match(rest, /[.;]( |$)/)
		if (i == 0) {
			sentence = rest
			len = length(rest)
		} else {
			sentence = substr(rest, 1, i)
			len = i + RLENGTH - 1
		}
		rest = substr(rest, len + 1)
		classify(sentence, lineat(pos))
		pos += len
	}
	para = ""
	nl = 0
}
$1 == "unknown" {
	unknown[++unknowns] = $2 " " display($3) " absent or unreadable"
	next
}
{
	layer = $1
	path = $2
	name = display(path)
	para = ""
	nl = 0
	n = 0
	while ((status = (getline text < path)) > 0) {
		n++
		if (text ~ /^[ \t]*$/) {
			flush()
			continue
		}
		sub(/^[ \t]+/, "", text)
		if (para != "") para = para " "
		off[++nl] = length(para) + 1
		lnum[nl] = n
		para = para text
	}
	flush()
	if (status < 0) unknown[++unknowns] = layer " " name " absent or unreadable"
	close(path)
}
END {
	print "instruction-conflicts: local, uncertain and non-authoritative; host files are not Git-pinned, and a conflict does not decide precedence"
	topics = "pr-draft pr-auto-merge pr-manual-merge"
	pa["pr-draft"] = "draft-required"; pb["pr-draft"] = "draft-refused"
	pa["pr-auto-merge"] = "allowed"; pb["pr-auto-merge"] = "needs-approval"
	pa["pr-manual-merge"] = "needs-approval"; pb["pr-manual-merge"] = "preauthorized"
	split(topics, order, " ")
	conflicts = 0
	for (k = 1; k <= 3; k++) {
		tp = order[k]
		a = refs[tp SUBSEP pa[tp]]
		b = refs[tp SUBSEP pb[tp]]
		if (a != "" && b != "") {
			conflicts++
			printf "%-17s %-15s %s: %s | %s: %s\n", "CONFLICT", tp, pa[tp], a, pb[tp], b
		} else if (a != "") {
			printf "%-17s %-15s %s: %s\n", "NO_CONFLICT_FOUND", tp, pa[tp], a
		} else if (b != "") {
			printf "%-17s %-15s %s: %s\n", "NO_CONFLICT_FOUND", tp, pb[tp], b
		} else {
			printf "%-17s %s\n", "NO_CLAIM", tp
		}
	}
	for (k = 1; k <= unknowns; k++) printf "%-17s %s\n", "UNKNOWN", unknown[k]
	if (hosts == 0) printf "%-17s %s\n", "UNKNOWN", "host layer not observed: no --host given"
	if (conflicts > 0) { printf "result: CONFLICT (%d)\n", conflicts; exit 1 }
	if (unknowns > 0 || hosts == 0) { print "result: UNKNOWN"; exit 3 }
	print "result: NO_CONFLICT_FOUND"
}
' "$inputs"
