#!/usr/bin/env bash
# usage: script/check-ai-coauthor-trailers.sh [BASE [HEAD]]
#        script/check-ai-coauthor-trailers.sh --pre-push REMOTE LOCATION < REF_UPDATES
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
#   --message FILE  check one commit message file, as a commit-msg hook does. The text below a
#                   scissors line is ignored only when Git's own template comment follows it
#                   (`git commit -v`, or scissors cleanup in the editor), since only then does Git
#                   drop it; `git commit -F` and verbatim cleanup keep it, so it is checked.
#   --batch [BASE]  the BASE..HEAD form that make batch-precheck runs: an empty BASE means
#                   origin/main, and a checkout where that does not resolve skips, saying so.
#   --strip         copy a commit message from stdin to stdout without its AI co-author lines
#                   (the rewrite step the refusal names).
#   --install-hook  write a commit-msg hook (at `git rev-parse --git-path hooks/commit-msg`, so
#                   core.hooksPath is honoured) that runs --message from the committing checkout,
#                   so the trailer is refused before the commit exists. An existing different
#                   hook is never overwritten.
#
# A `Co-authored-by:` line (any case, any position in the message) names an AI agent only when it
# is a known agent identity: a no-reply address at anthropic.com or openai.com (or a subdomain);
# the GitHub no-reply address of one of the agent logins claude, claude[bot], copilot,
# copilot[bot] or chatgpt-codex-connector[bot] (exact login, so a human `claude-smith` passes);
# or an agent name: Claude Code, Claude <Opus|Sonnet|Haiku|Fable> <version>, claude[bot],
# copilot[bot], GitHub Copilot, OpenAI Codex, or a bare Claude with no address. A line that
# starts, after any non-alphanumeric prefix such as an emoji, with "Generated with" or "Generated
# by" and names Claude, Anthropic, OpenAI, ChatGPT, Codex or Copilot is refused too (ASD-V0-831).
# This is the one AI-trailer check and hook installer (ASD-V0-831 retired check-ai-trailers.sh).
# ASD-V0-560 (V1-5621 AC2): range and pre-push checks also refuse raw author/committer names
# or emails equal to Claude (any case), or any anthropic.com address (subdomains included).
# --pre-push checks pushed tips minus the destination's branch/tag history and replaced refs.
# Example: script/check-ai-coauthor-trailers.sh --pre-push origin /path/to/remote.git < updates
# Exit 0 clean, 1 refused, 2 usage or a Git failure (fail closed).
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
    if [[ $local_part =~ ^(claude|claude\[bot\]|copilot|copilot\[bot\]|chatgpt-codex-connector\[bot\])$ ]]; then
      printf 'the GitHub no-reply address of an AI agent account <%s>' "$email"
      return
    fi
  fi
  if [[ $lname =~ ^(claude[[:space:]]+code|claude[[:space:]]+(opus|sonnet|haiku|fable)[[:space:]]+[0-9][0-9.]*|claude\[bot\]|copilot\[bot\]|github[[:space:]]+copilot|openai[[:space:]]+codex)$ ]]; then
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
    # ASD-V0-831 (V1-3337): the "Generated with" attribution line the retired check refused.
    if [[ -z $reason && $line =~ ^[^[:alnum:]]*generated[[:space:]]+(with|by)[^[:alnum:]]*.*(claude|anthropic|openai|chatgpt|codex|copilot) ]]; then
      reason='an AI "Generated with" attribution line'
    fi
    shopt -u nocasematch
    if [[ $mode == strip ]]; then
      [[ -n $reason ]] || printf '%s\n' "$line"
    elif [[ -n $reason ]]; then
      printf '%s\t%s\n' "$reason" "$(printf '%s' "$line" | sed 's/^[[:space:]]*//')"
    fi
  done
}

# Identity rules intentionally differ from trailer rules: a personal address does not excuse
# an exact Claude author/committer name. Raw %an/%ae/%cn/%ce cannot be masked by a mailmap.
identity_scan() {
  local value lower field=0 labels=('author name' 'author email' 'committer name' 'committer email')
  while IFS= read -r value; do
    lower=$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]')
    if [[ $lower == claude || $lower =~ ^[^@[:space:]]+@([^@[:space:]]+\.)?anthropic\.com$ ]]; then
      printf '    %s: %s\n' "${labels[$field]}" "$value"
    fi
    field=$((field + 1))
  done
}

# Git supplies the destination URL/path as $2 even when $1 is a remote name with a pushurl.
# Read published branches/tags, not server-internal PR refs a normal fetch cannot supply.
# Cached --remotes could exempt an unpublished commit via a stale ref.
push_commits() {
  local location=${2-} local_ref local_sha remote_ref remote_sha advertised sha ref tip
  local tips=() exclusions=()
  [[ -n $location ]] || { printf '%s: refused: --pre-push needs REMOTE LOCATION\n' "$me" >&2; return 2; }
  while read -r local_ref local_sha remote_ref remote_sha; do
    [[ -n ${local_sha-} && ! $local_sha =~ ^0+$ ]] || continue
    if ! git cat-file -e "$local_sha"; then
      printf '%s: refused: cannot read pushed object %s\n' "$me" "$local_sha" >&2
      return 2
    fi
    # Tags to trees/blobs introduce no commits.
    tip=$(git rev-parse --verify --quiet "$local_sha^{commit}") || continue
    tips+=("$tip")
    # Include the replaced ref even if its namespace is not heads/ or tags/.
    if [[ -n ${remote_sha-} && ! $remote_sha =~ ^0+$ ]]; then
      if ! git cat-file -e "$remote_sha" 2>/dev/null; then
        printf '%s: refused: remote object %s is missing locally; fetch the destination history and retry\n' "$me" "$remote_sha" >&2
        return 2
      fi
      tip=$(git rev-parse --verify --quiet "$remote_sha^{commit}") && exclusions+=("^$tip")
    fi
  done
  ((${#tips[@]})) || return 0
  if [[ $(git rev-parse --is-shallow-repository) != false ]]; then
    printf '%s: refused: shallow or unreadable history; fetch complete destination history and retry\n' "$me" >&2
    return 2
  fi
  if ! advertised=$(git ls-remote --heads --tags -- "$location"); then
    printf '%s: refused: git ls-remote failed; pushed commits were not checked\n' "$me" >&2
    return 2
  fi
  while read -r sha ref; do
    [[ -n $sha ]] || continue
    if ! git cat-file -e "$sha" 2>/dev/null; then
      printf '%s: refused: remote object %s is missing locally; fetch the destination history and retry\n' "$me" "$sha" >&2
      return 2
    fi
    tip=$(git rev-parse --verify --quiet "$sha^{commit}") || continue
    exclusions+=("^$tip")
  done <<< "$advertised"
  git rev-list --reverse "${tips[@]}" ${exclusions[@]+"${exclusions[@]}"} -- || {
    printf '%s: refused: git rev-list failed; pushed commits were not checked\n' "$me" >&2
    return 2
  }
}

# retired_guard FILE succeeds when FILE is exactly the commit-msg hook check-ai-trailers.sh
# --install wrote (ASD-V0-071, retired by ASD-V0-831), with any fallback path on line 4.
retired_guard() {
  local fallback
  fallback=$(sed -n '4s/^\[ -x "\$s" \] || s=//p' "$1") && [[ -n $fallback ]] || return 1
  [[ $(cat "$1"; printf x) == "$(printf '#!/bin/sh\n# corvint ai-trailer guard (ASD-V0-071)\ns="$(git rev-parse --show-toplevel 2> /dev/null)/script/check-ai-trailers.sh"\n[ -x "$s" ] || s=%s\n[ -x "$s" ] || { echo "commit-msg: the ai-trailer guard is gone; message not checked" >&2; exit 0; }\nexec "$s" --commit-msg "$1"\n' "$fallback"; printf x)" ]]
}

settings_fix='set the Claude Code attribution setting so no future commit gets one: "attribution": {"commit": "", "pr": ""} in ${CLAUDE_CONFIG_DIR:-~/.claude}/settings.json (script/agent-preflight.sh reports it)'

case ${1-} in
  -h|--help)
    awk 'NR == 1 { next } /^#/ { sub(/^# ?/, ""); print; next } { exit }' "$0"
    exit 0
    ;;
  --message)
    file=${2-}
    [[ -n $file && -r $file ]] || { printf '%s: refused: --message needs a readable commit message file\n' "$me" >&2; exit 2; }
    # Git's verbose template is the scissors line followed by its own comment; only then does
    # Git drop what follows. A line starting with `#` cannot be a trailer, so it needs no filter.
    message=$(awk '
      held != "" { if ($0 == "# Do not modify or remove the line above.") exit; print held; held = "" }
      /^# -+ >8 -+$/ { held = $0; next }
      { print }
      END { if (held != "") print held }' "$file") ||
      { printf '%s: refused: cannot read %s\n' "$me" "$file" >&2; exit 2; }
    found=$(printf '%s\n' "$message" | scan)
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
    # ASD-V0-831: the retired ASD-V0-071 guard, exactly as its --install wrote it (any fallback
    # path), is replaced so a clone set up before the retirement moves to this hook.
    if [[ -e $hook ]] && retired_guard "$hook"; then
      printf '%s: replacing the retired check-ai-trailers.sh guard at %s\n' "$me" "$hook"
      rm -f "$hook"
    fi
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
  --pre-push)
    shas=$(push_commits "${2-}" "${3-}") || exit 2
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

if [[ ${1-} == --pre-push ]]; then
  base="remote ${2-}"
  head="pushed tips"
else
  base=${1:-origin/main}
  head=${2:-HEAD}
  for rev in "$base" "$head"; do
    git rev-parse --verify --quiet --end-of-options "$rev^{commit}" > /dev/null ||
      { printf '%s: refused: %s does not name a commit here; pass the batch base explicitly (make batch-precheck DOGFOOD_BASE=<sha>, or %s <base-sha> [head])\n' "$me" "$rev" "$0" >&2; exit 2; }
  done
  if [[ $(git rev-parse --is-shallow-repository) != false ]]; then
    printf '%s: refused: shallow or unreadable history; fetch complete destination history and retry\n' "$me" >&2
    exit 2
  fi
  # Fail closed: a range Git cannot enumerate, or a message it cannot read, is never "clean".
  if ! shas=$(git rev-list --reverse "$base..$head"); then
    printf '%s: refused: git rev-list %s..%s failed, so the range was not checked; fetch the missing history (git fetch --unshallow, or fetch the base) and rerun\n' "$me" "$base" "$head" >&2
    exit 2
  fi
fi
bad=0
identity_bad=0
for sha in $shas; do
  if ! message=$(git show -s --format=%B "$sha"); then
    printf '%s: refused: git show could not read the message of %s, so the range was not checked\n' "$me" "$sha" >&2
    exit 2
  fi
  if ! identities=$(git show -s --format='%an%n%ae%n%cn%n%ce' "$sha"); then
    printf '%s: refused: git show could not read the identity of %s\n' "$me" "$sha" >&2
    exit 2
  fi
  identity_found=$(printf '%s\n' "$identities" | identity_scan)
  if [[ -n $identity_found ]]; then
    printf '%s: refused (ASD-V0-560): commit %s has a prohibited identity:\n%s\n' "$me" "$sha" "$identity_found" >&2
    identity_bad=$((identity_bad + 1))
  fi
  found=$(printf '%s\n' "$message" | scan)
  [[ -z $found ]] && continue
  ((bad == 0)) && printf '%s: refused (ASD-V0-560): %s..%s has commits crediting an AI agent as a GitHub co-author:\n' "$me" "$base" "$head"
  bad=$((bad + 1))
  printf '  %s %s\n' "$sha" "$(git show -s --format=%s "$sha")"
  printf '%s\n' "$found" | while IFS=$'\t' read -r reason trailer; do printf '    %s  (%s)\n' "$trailer" "$reason"; done
done
((bad == 0 && identity_bad > 0)) && exit 1
((bad == 0)) && { printf '%s: no AI co-author trailer in %s..%s\n' "$me" "$base" "$head"; exit 0; }
if [[ ${1-} == --pre-push ]]; then
  printf 'fix: remove the listed trailers from the unpublished commits and retry the push\n'
  exit 1
fi
# The rewrite runs a copy of this script taken outside the checkout: the commits being rewritten
# may predate it, so the checkout's own copy can be absent while the rebase runs.
self=$(cd "$(dirname "$0")" && pwd)/$(basename "$0")
printf 'fix: rewrite those %d commit(s) without the trailer before pushing, then rerun this check:\n  strip=$(mktemp) && cp %q "$strip" && git rebase --rebase-merges %q --exec "git show -s --format=%%B HEAD | bash \047$strip\047 --strip | git commit --amend --allow-empty -q -F -"\nthen %s; script/check-ai-coauthor-trailers.sh --install-hook refuses the trailer before a commit is made\n' "$bad" "$self" "$base" "$settings_fix"
exit 1
