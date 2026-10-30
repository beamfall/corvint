#!/usr/bin/env bash
# usage: script/check-ai-coauthor-trailers.sh [BASE [HEAD]]
#        script/check-ai-coauthor-trailers.sh --batch [BASE]
#        script/check-ai-coauthor-trailers.sh --message FILE
#        script/check-ai-coauthor-trailers.sh --install-hook
#        script/check-ai-coauthor-trailers.sh --strip < MESSAGE
#
# ASD-V0-560 (V1-3227): no commit may credit an AI agent as a GitHub co-author. The owner rule of
# 2026-10-30 is that Claude must never appear as a contributor, and GitHub attributes a commit to
# every `Co-authored-by:` trailer it carries.
#
#   BASE [HEAD]     check every commit in BASE..HEAD (merges included); BASE defaults to
#                   origin/main and HEAD to HEAD, and one that does not resolve refuses.
#   --message FILE  check one commit message file, as a commit-msg hook does: lines starting
#                   with `#` and everything below a scissors line are ignored, as Git strips them.
#   --batch [BASE]  the BASE..HEAD form that make batch-precheck runs: an empty BASE means
#                   origin/main, and a checkout where that does not resolve skips, saying so.
#   --strip         copy a commit message from stdin to stdout without its AI co-author lines
#                   (the rewrite step the refusal names).
#   --install-hook  write a commit-msg hook (at `git rev-parse --git-path hooks/commit-msg`, so
#                   core.hooksPath is honoured) that runs --message from the committing checkout,
#                   so the trailer is refused before the commit exists. An existing different
#                   hook is never overwritten.
#
# A `Co-authored-by:` line (any case, any position in the message) names an AI agent when its
# address is a no-reply address at anthropic.com or openai.com (or a subdomain), a GitHub no-reply
# address of a Claude, Codex, ChatGPT or Copilot account, or when its name is an unambiguous agent
# name: Claude Code, Claude <model> (Opus, Sonnet, Haiku, Fable), claude[bot], Codex, OpenAI
# Codex, ChatGPT, Copilot, GitHub Copilot, copilot[bot]. A bare "Claude" is a human name too, so
# it is refused only with an AI address or no address at all. Exit 0 clean, 1 refused, 2 usage.
set -uo pipefail
export LC_ALL=C
me=check-ai-coauthor-trailers

# Prints the reason a trailer value names an AI agent, or nothing.
ai_reason() {
  local value=$1 name email lname lemail local_part
  name=${value%%<*}
  email=
  [[ $value == *"<"*">"* ]] && { email=${value#*<}; email=${email%%>*}; }
  name=$(printf '%s' "$name" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')
  lname=$(printf '%s' "$name" | tr '[:upper:]' '[:lower:]')
  lemail=$(printf '%s' "$email" | tr '[:upper:]' '[:lower:]' | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')
  if [[ $lemail =~ ^[^@]*no-?reply[^@]*@([^@]+\.)?(anthropic|openai)\.com$ ]]; then
    printf 'an AI vendor no-reply address <%s>' "$email"
    return
  fi
  if [[ $lemail == *@users.noreply.github.com ]]; then
    local_part=${lemail%@*}
    local_part=${local_part#*+}
    if [[ $local_part =~ ^(claude|codex|chatgpt|copilot|openai)([-[]|$) ]]; then
      printf 'the GitHub no-reply address of an AI agent account <%s>' "$email"
      return
    fi
  fi
  if [[ $lname =~ ^(claude[[:space:]]+code|claude[[:space:]]+(opus|sonnet|haiku|fable)([[:space:]].*)?|claude\[bot\]|(openai[[:space:]]+)?codex([[:space:]]+cli)?|chatgpt|(github[[:space:]]+)?copilot|copilot\[bot\])$ ]]; then
    printf 'the AI agent name "%s"' "$name"
    return
  fi
  if [[ $lname == claude && -z $lemail ]]; then
    printf 'the AI agent name "%s" with no address' "$name"
  fi
}

# Reads a message on stdin; prints "reason<TAB>trailer line" for each AI co-author trailer, or
# with "strip" prints the message without those lines.
scan() {
  local mode=${1-} line value reason
  while IFS= read -r line || [[ -n $line ]]; do
    reason=
    shopt -s nocasematch
    if [[ $line =~ ^[[:space:]]*co-authored-by:[[:space:]]*(.*)$ ]]; then
      value=${BASH_REMATCH[1]}
      shopt -u nocasematch
      reason=$(ai_reason "$value")
    fi
    shopt -u nocasematch
    if [[ $mode == strip ]]; then
      [[ -n $reason ]] || printf '%s\n' "$line"
    elif [[ -n $reason ]]; then
      printf '%s\t%s\n' "$reason" "$(printf '%s' "$line" | sed 's/^[[:space:]]*//')"
    fi
  done
}

settings_fix='set the Claude Code attribution setting so no future commit gets one: "attribution": {"commit": "", "pr": ""} in ${CLAUDE_CONFIG_DIR:-~/.claude}/settings.json (script/agent-preflight.sh reports it)'

case ${1-} in
  -h|--help)
    sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'
    exit 0
    ;;
  --message)
    file=${2-}
    [[ -n $file && -r $file ]] || { printf '%s: refused: --message needs a readable commit message file\n' "$me" >&2; exit 2; }
    found=$(sed -e '/^# -\{1,\} >8 -\{1,\}$/,$d' -e '/^#/d' "$file" | scan)
    [[ -z $found ]] && exit 0
    {
      printf '%s: refused (ASD-V0-560): this commit message credits an AI agent as a GitHub co-author:\n' "$me"
      printf '%s\n' "$found" | while IFS=$'\t' read -r reason trailer; do printf '  %s  (%s)\n' "$trailer" "$reason"; done
      printf 'fix: delete that Co-authored-by line and commit again; then %s\n' "$settings_fix"
    } >&2
    exit 1
    ;;
  --strip)
    scan strip
    exit 0
    ;;
  --install-hook)
    hook=$(git rev-parse --path-format=absolute --git-path hooks/commit-msg 2>/dev/null) ||
      { printf '%s: refused: not inside a Git checkout\n' "$me" >&2; exit 2; }
    body='#!/bin/sh
# Installed by script/check-ai-coauthor-trailers.sh --install-hook (ASD-V0-560, V1-3227).
check="$(git rev-parse --show-toplevel)/script/check-ai-coauthor-trailers.sh"
if [ -x "$check" ]; then exec "$check" --message "$1"; fi
echo "commit-msg: $check is absent in this checkout; AI co-author trailers are not checked here" >&2'
    if [[ -e $hook ]]; then
      if [[ $(cat "$hook") == "$body" ]]; then
        printf '%s: commit-msg hook already installed at %s\n' "$me" "$hook"
        exit 0
      fi
      printf '%s: refused: %s already exists and is not this hook; add this line to it instead:\n  "$(git rev-parse --show-toplevel)/script/check-ai-coauthor-trailers.sh" --message "$1" || exit 1\n' "$me" "$hook" >&2
      exit 1
    fi
    mkdir -p "$(dirname "$hook")" && printf '%s\n' "$body" > "$hook" && chmod +x "$hook" ||
      { printf '%s: refused: cannot write %s\n' "$me" "$hook" >&2; exit 1; }
    printf '%s: commit-msg hook installed at %s\n' "$me" "$hook"
    exit 0
    ;;
  --batch)
    # make batch-precheck (ASD-V0-560): an empty BASE is the batch default, origin/main; a
    # checkout without it (a test fixture, a clone with no origin) skips, naming the explicit form.
    shift
    if [[ -z ${1-} ]] && ! git rev-parse --verify --quiet origin/main^{commit} > /dev/null; then
      printf '%s: skipped: no batch base was given and origin/main does not resolve here; run make batch-precheck DOGFOOD_BASE=<sha> to check the batch range\n' "$me"
      exit 0
    fi
    ;;
  -*)
    printf '%s: refused: unknown option %s (see --help)\n' "$me" "$1" >&2
    exit 2
    ;;
esac

base=${1:-origin/main}
head=${2:-HEAD}
for rev in "$base" "$head"; do
  git rev-parse --verify --quiet --end-of-options "$rev^{commit}" > /dev/null ||
    { printf '%s: refused: %s does not name a commit here; pass the batch base explicitly (make batch-precheck DOGFOOD_BASE=<sha>, or %s <base-sha> [head])\n' "$me" "$rev" "$0" >&2; exit 2; }
done
bad=0
while IFS= read -r sha; do
  found=$(git show -s --format=%B "$sha" | scan)
  [[ -z $found ]] && continue
  ((bad == 0)) && printf '%s: refused (ASD-V0-560): %s..%s has commits crediting an AI agent as a GitHub co-author:\n' "$me" "$base" "$head"
  bad=$((bad + 1))
  printf '  %s %s\n' "$sha" "$(git show -s --format=%s "$sha")"
  printf '%s\n' "$found" | while IFS=$'\t' read -r reason trailer; do printf '    %s  (%s)\n' "$trailer" "$reason"; done
done < <(git rev-list --reverse "$base..$head")
((bad == 0)) && { printf '%s: no AI co-author trailer in %s..%s\n' "$me" "$base" "$head"; exit 0; }
printf 'fix: rewrite those %d commit(s) without the trailer before pushing, then rerun this check:\n  git rebase --rebase-merges %s --exec '"'"'git show -s --format=%%B HEAD | script/check-ai-coauthor-trailers.sh --strip | git commit --amend --allow-empty -q -F -'"'"'\nthen %s; script/check-ai-coauthor-trailers.sh --install-hook refuses the trailer before a commit is made\n' "$bad" "$base" "$settings_fix"
exit 1
