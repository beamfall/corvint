#!/usr/bin/env bash
# Tests for script/check-ai-coauthor-trailers.sh (ASD-V0-560, V1-3227) on a fixture repository:
# AI co-author trailers are refused naming the commit (also in the --batch form batch-precheck runs), human co-authors (one named Claude) pass,
# the printed rebase recovery clears the range, --message ignores comment and scissors lines, and
# --install-hook refuses the trailer before the commit exists without replacing a foreign hook,
# and a Git failure fails closed.
set -uo pipefail
export LC_ALL=C
script=$(cd "$(dirname "$0")" && pwd)
check=$script/check-ai-coauthor-trailers.sh
tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-ai-coauthor-test.XXXXXX") || exit 1
trap '/bin/rm -rf "$tmp"' EXIT
fail() { printf 'check-ai-coauthor-trailers_test: FAIL line %s: %s\n' "${BASH_LINENO[0]}" "$*" >&2; exit 1; }
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.invalid
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1

repo=$tmp/repo
git init -q "$repo" || fail "git init"
cd "$repo" || fail "cd"
commit() { git commit -q --allow-empty -F - || fail "commit"; git rev-parse HEAD; }
base=$(printf 'base\n' | commit)

# 1. A clean range passes, and an unresolvable default base refuses naming the explicit form.
"$check" "$base" HEAD > "$tmp/out" 2>&1 || fail "empty range refused: $(cat "$tmp/out")"
"$check" > "$tmp/out" 2>&1
[[ $? == 2 ]] && grep -q 'origin/main does not name a commit here; pass the batch base explicitly' "$tmp/out" ||
  fail "missing origin/main: $(cat "$tmp/out")"
# The batch-precheck form skips, saying so, only when no base was given and origin/main is absent.
"$check" --batch '' > "$tmp/out" 2>&1 && grep -q 'skipped: no batch base was given and origin/main does not resolve' "$tmp/out" ||
  fail "--batch without origin/main: $(cat "$tmp/out")"
"$check" --batch no-such-base > "$tmp/out" 2>&1
[[ $? == 2 ]] || fail "--batch with an unresolvable explicit base did not refuse: $(cat "$tmp/out")"

# 2. Human co-authors pass, including people named Claude with personal or GitHub no-reply
# addresses whose login merely starts with an agent name, and --strip keeps their credit.
humans='Co-authored-by: Ada Lovelace <ada@example.org>
Co-authored-by: Claude Monet <claude@monet.fr>
Co-authored-by: Claude <claude@example.org>
Co-authored-by: Claude Smith <123+claude-smith@users.noreply.github.com>
Co-authored-by: Claude Haiku <haiku@example.org>
Co-authored-by: Cody Pilot <456+copilotfan@users.noreply.github.com>'
printf 'human\n\n%s\n' "$humans" | commit > /dev/null
[[ $(printf '%s\n' "$humans" | "$check" --strip) == "$humans" ]] || fail "--strip dropped a human co-author"
printf 'mentions the rule in prose: no Co-Authored-By: Claude trailer is allowed\n' | commit > /dev/null
"$check" "$base" HEAD > "$tmp/out" 2>&1 || fail "human co-authors refused: $(cat "$tmp/out")"

# 3. Each AI form is refused, naming the commit and the trailer; exit 1.
forms=(
  'Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>'
  'co-authored-by: Anyone <no-reply@mail.anthropic.com>'
  'CO-AUTHORED-BY: Codex <noreply@openai.com>'
  'Co-authored-by: claude[bot] <209825114+claude[bot]@users.noreply.github.com>'
  'Co-authored-by: Someone <198982749+Copilot@users.noreply.github.com>'
  'Co-authored-by: Claude Code <dev@example.org>'
  'Co-authored-by: Claude Sonnet 5 <dev@example.org>'
  'Co-authored-by: chatgpt-codex-connector[bot] <199175422+chatgpt-codex-connector[bot]@users.noreply.github.com>'
  'Co-authored-by: Claude'
)
for form in "${forms[@]}"; do
  git reset -q --hard "$base"
  sha=$(printf 'agent\n\n%s\n' "$form" | commit)
  git update-ref refs/remotes/origin/main "$base"
  "$check" --batch '' > "$tmp/out" 2>&1
  [[ $? == 1 ]] || fail "--batch against origin/main accepted: $form: $(cat "$tmp/out")"
  git update-ref -d refs/remotes/origin/main
  "$check" "$base" HEAD > "$tmp/out" 2>&1
  [[ $? == 1 ]] || fail "AI trailer accepted: $form: $(cat "$tmp/out")"
  grep -q "^  $sha agent$" "$tmp/out" || fail "commit not named for $form: $(cat "$tmp/out")"
  grep -qF "    $form  (" "$tmp/out" || fail "trailer not named for $form: $(cat "$tmp/out")"
  grep -q '"attribution": {"commit": "", "pr": ""}' "$tmp/out" || fail "settings fix missing: $(cat "$tmp/out")"
done

# 4. The printed recovery rewrites the range (merges included) and keeps human co-authors.
git reset -q --hard "$base"
git checkout -q -b side
printf 'side\n\nCo-authored-by: Ada Lovelace <ada@example.org>\nCo-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>\n' | commit > /dev/null
git checkout -q -
git merge -q --no-ff -m 'merge side

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>' side || fail "merge"
"$check" "$base" HEAD > "$tmp/out" 2>&1 && fail "AI trailers in a merged range accepted"
[[ $(grep -c '^  [0-9a-f]\{40\} ' "$tmp/out") == 2 ]] || fail "both commits not named: $(cat "$tmp/out")"
# The fixture never tracks the script, so the recovery must not need it in the rewritten commits.
[[ ! -e script ]] || fail "the fixture has a script directory"
recovery=$(sed -n 's/^  \(strip=.*git rebase .*\)$/\1/p' "$tmp/out")
[[ -n $recovery ]] || fail "no recovery command printed: $(cat "$tmp/out")"
bash -c "$recovery" > "$tmp/rebase" 2>&1 || fail "recovery failed: $(cat "$tmp/rebase")"
"$check" "$base" HEAD > "$tmp/out" 2>&1 || fail "range still refused after the recovery: $(cat "$tmp/out")"
[[ $(git rev-list --count --merges "$base..HEAD") == 1 ]] || fail "the recovery dropped the merge"
git log --format=%B "$base..HEAD" | grep -qx 'Co-authored-by: Ada Lovelace <ada@example.org>' || fail "human co-author dropped"

# 5. --message: a comment line is no trailer; below a scissors line Git keeps the text (commit -F,
# verbatim cleanup) unless its own verbose template follows, so only that case is skipped.
scissors='# ------------------------ >8 ------------------------'
printf 'subject\n# Co-Authored-By: Claude <noreply@anthropic.com>\n' > "$tmp/msg"
"$check" --message "$tmp/msg" > "$tmp/out" 2>&1 || fail "comment line checked: $(cat "$tmp/out")"
printf 'subject\n%s\nCo-Authored-By: Claude <noreply@anthropic.com>\n' "$scissors" > "$tmp/msg"
"$check" --message "$tmp/msg" > "$tmp/out" 2>&1
[[ $? == 1 ]] || fail "a trailer below a bare scissors line (kept by commit -F) was accepted: $(cat "$tmp/out")"
printf 'subject\n%s\n# Do not modify or remove the line above.\n# Everything below it will be ignored.\n Co-Authored-By: Claude <noreply@anthropic.com>\n' "$scissors" > "$tmp/msg"
"$check" --message "$tmp/msg" > "$tmp/out" 2>&1 || fail "Git's verbose diff below its template was checked: $(cat "$tmp/out")"
printf 'subject\n\nCo-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>\n' > "$tmp/msg"
"$check" --message "$tmp/msg" > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q 'delete that Co-authored-by line' "$tmp/out" || fail "--message: $(cat "$tmp/out")"

# 6. --install-hook refuses the trailer before the commit exists, is idempotent, and never
# replaces a foreign hook.
mkdir -p script && /bin/cp -f "$check" script/
"$check" --install-hook > "$tmp/out" 2>&1 || fail "install: $(cat "$tmp/out")"
"$check" --install-hook > "$tmp/out" 2>&1 && grep -q 'already installed' "$tmp/out" || fail "reinstall: $(cat "$tmp/out")"
before=$(git rev-parse HEAD)
printf 'x\n\nCo-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>\n' | git commit -q --allow-empty -F - > "$tmp/out" 2>&1 &&
  fail "the hook let an AI trailer commit"
[[ $(git rev-parse HEAD) == "$before" ]] || fail "a commit was made"
grep -q 'refused (ASD-V0-560)' "$tmp/out" || fail "hook refusal: $(cat "$tmp/out")"
printf 'x\n%s\nCo-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>\n' "$scissors" > "$tmp/msg"
git commit -q --allow-empty -F "$tmp/msg" > "$tmp/out" 2>&1 && fail "the hook let commit -F keep a trailer below a scissors line"
[[ $(git rev-parse HEAD) == "$before" ]] || fail "a commit was made through the scissors line"
# git commit -v: a diff context line that looks like a trailer is below Git's template; not refused.
printf 'a\nCo-Authored-By: Claude <noreply@anthropic.com>\nb\n' > notes.txt && git add notes.txt && git commit -q -m notes
printf 'A\nCo-Authored-By: Claude <noreply@anthropic.com>\nb\n' > notes.txt && git add notes.txt
GIT_EDITOR=true git commit -q -v -e -m 'edit notes' > "$tmp/out" 2>&1 || fail "the hook refused a verbose diff context line: $(cat "$tmp/out")"
printf 'x\n\nCo-authored-by: Ada Lovelace <ada@example.org>\n' | git commit -q --allow-empty -F - > "$tmp/out" 2>&1 || fail "hook refused a human: $(cat "$tmp/out")"
printf '#!/bin/sh\nexit 0\n' > "$(git rev-parse --git-path hooks/commit-msg)"
"$check" --install-hook > "$tmp/out" 2>&1 && fail "a foreign hook was replaced"
grep -q 'already exists and is not this hook' "$tmp/out" || fail "foreign hook: $(cat "$tmp/out")"

# 7. Fail closed: a Git failure while enumerating the range or reading a message exits 2 naming
# the cause, never "no AI co-author trailer".
real_git=$(command -v git)
mkdir -p "$tmp/stub"
for verb in rev-list show; do
  printf '#!/bin/sh\n[ "$1" = %s ] && { echo "fatal: stub failure" >&2; exit 128; }\nexec %q "$@"\n' "$verb" "$real_git" > "$tmp/stub/git"
  chmod +x "$tmp/stub/git"
  PATH="$tmp/stub:$PATH" "$check" "$base" HEAD > "$tmp/out" 2>&1
  [[ $? == 2 ]] || fail "a failing git $verb did not fail closed: $(cat "$tmp/out")"
  grep -q "refused: git $verb" "$tmp/out" || fail "the git $verb failure was not named: $(cat "$tmp/out")"
  grep -q 'no AI co-author trailer' "$tmp/out" && fail "a failing git $verb reported a clean range"
done

printf 'check-ai-coauthor-trailers_test: ok\n'
