#!/usr/bin/env bash
# usage: script/check-ai-trailers.sh [--range BASE [HEAD]]  check every commit in BASE..HEAD (default origin/main)
#        script/check-ai-trailers.sh --commit-msg FILE check one message (the commit-msg hook form)
#        script/check-ai-trailers.sh --install          install that hook in this repository
#
# ASD-V0-071 (V1-3227): no commit may carry an AI co-author or "Generated with" trailer. On
# 2026-10-30 lane commits with an AI `Co-Authored-By:` trailer reached main, GitHub listed the
# assistant as a contributor, and main had to be rewritten. A message line is refused when it is a
# `Co-Authored-By:` trailer (any case) naming an AI assistant or its vendor (claude, anthropic,
# openai, chatgpt, gpt, codex, copilot, gemini, cursor, devin, aider), or when it starts, after
# any non-alphanumeric prefix such as an emoji, with "Generated with" and names one of them.
# A refusal prints `check-ai-trailers: REFUSE ai-trailer COMMIT: LINE` per line, with the fix, and
# exits 1; a usage error or an unresolvable base exits 2.
#
# --install writes `$(git rev-parse --git-path hooks)/commit-msg`, shared by every linked worktree
# of the clone. The hook runs the committing worktree's own script/check-ai-trailers.sh, else the
# installing copy, so removing one lane checkout never breaks the others; with neither present it
# warns and lets the commit through. It refuses to replace a commit-msg hook it did not write. A configured core.hooksPath is named instead, since Git would not run the hook.
set -uo pipefail
export LC_ALL=C

AI='(claude|anthropic|openai|chatgpt|gpt|codex|copilot|gemini|cursor|devin|aider)'
MARK='# corvint ai-trailer guard (ASD-V0-071)'

# offending reads a message on stdin and prints each refused line; it exits 0 when it found one,
# 1 when it found none and 2 or more on a read error, as grep does.
offending() {
  grep -Ei -e "^[[:space:]]*co-authored-by:.*$AI" -e "^[^[:alnum:]]*generated (with|by)[^[:alnum:]]*.*$AI"
}

# hook_text INSTALLER prints the managed commit-msg hook for that installer path.
hook_text() {
  printf '#!/bin/sh\n%s\ns="$(git rev-parse --show-toplevel 2> /dev/null)/script/check-ai-trailers.sh"\n[ -x "$s" ] || s=%q\n[ -x "$s" ] || { echo "commit-msg: the ai-trailer guard is gone; message not checked" >&2; exit 0; }\nexec "$s" --commit-msg "$1"\n' "$MARK" "$1"
}

# managed_hook FILE succeeds when FILE is exactly a hook --install wrote, from any installer path:
# only then may a reinstall replace it, so a marked hook with added logic is never overwritten.
managed_hook() {
  local fallback
  fallback=$(sed -n '4s/^\[ -x "\$s" \] || s=//p' "$1") && [[ -n $fallback ]] || return 1
  [[ $(cat "$1"; printf x) == "$(hook_text "$fallback"; printf x)" ]]
}

fix='remove the trailer (git commit --amend, or git rebase -i BASE with reword), then rerun'

case ${1:-} in
  --commit-msg)
    [[ $# == 2 && -r $2 ]] || { printf 'usage: check-ai-trailers.sh --commit-msg FILE\n' >&2; exit 2; }
    # Every line up to a `git commit -v` scissors line counts, comment lines included: with -m, -F
    # or --cleanup=verbatim Git keeps a `#` line in the commit. Below the scissors is the diff.
    lines=$(awk '/^# -+ >8 -+$/ { exit } { print }' "$2" | offending)
    rc=$?
    ((rc <= 1)) || { printf 'check-ai-trailers: could not read %s\n' "$2" >&2; exit 2; }
    ((rc == 1)) && exit 0
    while IFS= read -r line; do printf 'check-ai-trailers: REFUSE ai-trailer in this commit message: %s\n' "$line" >&2; done <<< "$lines"
    printf 'check-ai-trailers: remove that line and commit again\n' >&2
    exit 1
    ;;
  --install)
    [[ $# == 1 ]] || { printf 'usage: check-ai-trailers.sh --install\n' >&2; exit 2; }
    self=$(cd "$(dirname "$0")" && pwd)/${0##*/}
    hooks_path=$(git config --get core.hooksPath 2> /dev/null) && [[ -n $hooks_path ]] && {
      printf 'check-ai-trailers: core.hooksPath is %s; add `%q --commit-msg "$1"` to its commit-msg hook instead\n' "$hooks_path" "$self" >&2
      exit 2
    }
    hook=$(git rev-parse --path-format=absolute --git-path hooks/commit-msg) || exit 2
    if [[ -e $hook ]] && ! managed_hook "$hook"; then
      printf 'check-ai-trailers: %s exists and is not exactly this guard (another hook, or this one with edits); add `%q --commit-msg "$1"` to it by hand\n' "$hook" "$self" >&2
      exit 2
    fi
    mkdir -p "$(dirname "$hook")" || exit 2
    hook_text "$self" > "$hook.tmp" && chmod 755 "$hook.tmp" && /bin/mv -f "$hook.tmp" "$hook" || exit 2
    printf 'check-ai-trailers: installed %s\n' "$hook"
    exit 0
    ;;
  --range) [[ ($# == 2 || $# == 3) && -n $2 ]] || { printf 'usage: check-ai-trailers.sh [--range BASE [HEAD]]\n' >&2; exit 2; }; base=$2 head=${3:-HEAD} ;;
  '') base=origin/main head=HEAD ;;
  *) printf 'usage: check-ai-trailers.sh [--range BASE] | --commit-msg FILE | --install\n' >&2; exit 2 ;;
esac

sha=$(git rev-parse --verify -q "$base^{commit}") || { printf 'check-ai-trailers: base %s is not a commit\n' "$base" >&2; exit 2; }
tip=$(git rev-parse --verify -q "$head^{commit}") || { printf 'check-ai-trailers: head %s is not a commit\n' "$head" >&2; exit 2; }
bad=0 n=0
# Every Git read is checked: a failed rev-list or log is an error (exit 2), never a clean range.
commits=$(git rev-list "$sha..$tip") || { printf 'check-ai-trailers: git rev-list %s..%s failed\n' "$sha" "$tip" >&2; exit 2; }
for commit in $commits; do
  n=$((n + 1))
  message=$(git log -1 --format=%B "$commit") || { printf 'check-ai-trailers: git log %s failed\n' "$commit" >&2; exit 2; }
  lines=$(offending <<< "$message")
  rc=$?
  ((rc <= 1)) || { printf 'check-ai-trailers: could not scan the message of %s\n' "$commit" >&2; exit 2; }
  ((rc == 1)) && continue
  bad=1
  while IFS= read -r line; do printf 'check-ai-trailers: REFUSE ai-trailer %s: %s\n' "$commit" "$line"; done <<< "$lines"
done
if ((bad)); then
  printf 'check-ai-trailers: %s\n' "${fix/BASE/$sha}"
  exit 1
fi
printf 'check-ai-trailers: no AI co-author or "Generated with" trailer in %d commit(s) of %s..%s\n' "$n" "$sha" "$tip"
