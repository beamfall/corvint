#!/usr/bin/env bash
# Tests for script/check-ai-coauthor-trailers.sh (ASD-V0-560, V1-3227) on a fixture repository:
# AI co-author trailers are refused naming the commit (also in the --batch form batch-precheck runs), human co-authors (one named Claude) pass,
# the printed rebase recovery clears the range, --message ignores comment and scissors lines, and
# --install-hook refuses the trailer before the commit exists without replacing a foreign hook,
# a Git failure fails closed, and "Generated with" lines and the retired guard hook are handled
# (ASD-V0-831, V1-3337).
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

# 8. ASD-V0-831 (V1-3337): the "Generated with" line the retired check-ai-trailers.sh refused is
# refused here too, and stripped; prose "regenerated with" passes. --install-hook replaces the
# retired guard hook exactly as it was written, and still refuses any other hook.
gbase=$(git rev-parse HEAD)
gen1=$(printf 'g1\n\n\xf0\x9f\xa4\x96 Generated with [Claude Code](https://claude.com/claude-code)\n' | git commit -q --no-verify --allow-empty -F - && git rev-parse HEAD)
gen2=$(printf 'g2\n\nGenerated by OpenAI Codex\n' | git commit -q --no-verify --allow-empty -F - && git rev-parse HEAD)
printf 'g3\n\nGoldens regenerated with -update; Claude reviewed nothing.\n' | git commit -q --no-verify --allow-empty -F - || fail "commit g3"
"$check" "$gbase" HEAD > "$tmp/out" 2>&1
[[ $? == 1 ]] || fail "Generated-with lines passed: $(cat "$tmp/out")"
for c in "$gen1" "$gen2"; do grep -q "^  $c " "$tmp/out" || fail "commit $c not named: $(cat "$tmp/out")"; done
grep -q 'Generated by OpenAI Codex  (an AI "Generated with" attribution line)' "$tmp/out" || fail "line not named: $(cat "$tmp/out")"
grep -q ' g3$' "$tmp/out" && fail "a prose regenerated-with line was refused: $(cat "$tmp/out")"
[[ $(printf 'x\n\nGenerated with Claude Code\nkeep\n' | "$check" --strip) == $'x\n\nkeep' ]] || fail "--strip kept a Generated-with line"
printf 'x\n\nGenerated with Claude Code\n' > "$tmp/msg"
"$check" --message "$tmp/msg" > /dev/null 2>&1
[[ $? == 1 ]] || fail "--message passed a Generated-with line"
hookf=$(git rev-parse --path-format=absolute --git-path hooks/commit-msg)
printf '#!/bin/sh\n# corvint ai-trailer guard (ASD-V0-071)\ns="$(git rev-parse --show-toplevel 2> /dev/null)/script/check-ai-trailers.sh"\n[ -x "$s" ] || s=/old/lane/script/check-ai-trailers.sh\n[ -x "$s" ] || { echo "commit-msg: the ai-trailer guard is gone; message not checked" >&2; exit 0; }\nexec "$s" --commit-msg "$1"\n' > "$hookf"
"$check" --install-hook > "$tmp/out" 2>&1 && grep -q 'replacing the retired check-ai-trailers.sh guard' "$tmp/out" ||
  fail "the retired guard hook was not replaced: $(cat "$tmp/out")"
grep -q 'ASD-V0-560' "$hookf" || fail "the replaced hook is not the kept one"
printf '#!/bin/sh\n# corvint ai-trailer guard (ASD-V0-071)\nexit 0\n' > "$hookf"
"$check" --install-hook > "$tmp/out" 2>&1 && fail "a marked hook with other logic was replaced"
grep -q 'already exists and is not this hook' "$tmp/out" || fail "marked foreign hook: $(cat "$tmp/out")"

# 9. ASD-V0-560 / CIN-V0-006 (V1-5621 AC2): actual identities, CI entry point and pre-push.
# This separate fixture has historical prohibited identities, just as origin/main does.
git init -q "$tmp/identities" || fail "identity repo"
cd "$tmp/identities" || fail "identity cd"
git -c commit.gpgsign=false commit -q --allow-empty -m root || fail "identity root"
iroot=$(git rev-parse HEAD)
GIT_AUTHOR_NAME=Claude GIT_COMMITTER_EMAIL=person@anthropic.com \
  git -c commit.gpgsign=false commit -q --allow-empty -m historical || fail "historical identity"
historical=$(git rev-parse HEAD)
git -c commit.gpgsign=false commit -q --allow-empty -m clean-base || fail "clean base"
ibase=$(git rev-parse HEAD)
git clone -q --bare . "$tmp/identity-remote.git" || fail "seed historical remote"
git remote add origin "$tmp/identity-remote.git" || fail "identity remote"
git update-ref refs/remotes/origin/main "$ibase" || fail "vet comparison base"
mkdir -p script "$tmp/push-hooks"
/bin/cp -f "$check" "$script/pre-push-windows-vet.sh" script/ || fail "copy push scripts"
printf '#!/bin/sh\nexec script/pre-push-windows-vet.sh "$@"\n' > "$tmp/push-hooks/pre-push"
chmod +x "$tmp/push-hooks/pre-push"
git config core.hooksPath "$tmp/push-hooks"
# Run the actual Makefile target; no build or unrelated target runs in this fixture.
ci_check() {
  MAKEFLAGS= MFLAGS= MAKELEVEL=0 DOGFOOD_BASE= BASE= \
    GITHUB_EVENT_NAME=pull_request GITHUB_EVENT_PATH="$tmp/event.json" BASE_SHA="$ibase" \
    make -s -f "$script/../Makefile" ai-coauthor-trailers-check
}
pr_event() { printf '{"pull_request":{"head":{"sha":"%s"}}}\n' "$1" > "$tmp/event.json"; }
for role in AUTHOR COMMITTER; do
  for field in NAME EMAIL; do
    for value in cLaUdE person@AnThRoPiC.CoM person@mail.anthropic.com; do
      git reset -q --hard "$ibase" || fail "reset identity fixture"
      env "GIT_${role}_${field}=$value" git -c commit.gpgsign=false commit -q --allow-empty -m identity ||
        fail "plant $role $field"
      bad_sha=$(git rev-parse HEAD)
      "$check" "$ibase" "$bad_sha" > "$tmp/out" 2>&1
      [[ $? == 1 ]] && grep -q "$bad_sha has a prohibited identity" "$tmp/out" ||
        fail "$role $field $value range accepted: $(cat "$tmp/out")"
      pr_event "$bad_sha"
      ci_check > "$tmp/out" 2>&1 && fail "CI accepted $role $field $value"
      grep -q "$bad_sha has a prohibited identity" "$tmp/out" || fail "CI did not check identity"
      git push origin HEAD:refs/heads/proposed > "$tmp/out" 2>&1 &&
        fail "pre-push accepted $role $field $value"
      grep -q "$bad_sha has a prohibited identity" "$tmp/out" || fail "push did not check identity"
    done
  done
done

# Signature presentation must not shift the four raw identity fields. Only the committer
# email is prohibited; a real SSH-signed commit exercises Git's log.showSignature setting.
git reset -q --hard "$ibase" || fail "reset signed fixture"
ssh-keygen -q -t ed25519 -N '' -f "$tmp/signing-key" || fail "signing key"
printf 't@example.invalid %s\n' "$(cat "$tmp/signing-key.pub")" > "$tmp/allowed-signers"
git config gpg.format ssh
git config user.signingKey "$tmp/signing-key"
git config gpg.ssh.allowedSignersFile "$tmp/allowed-signers"
git config log.showSignature true
GIT_COMMITTER_EMAIL=person@anthropic.com git commit -q -S --allow-empty -m signed ||
  fail "signed identity commit"
signed_sha=$(git rev-parse HEAD)
git verify-commit "$signed_sha" > "$tmp/signature" 2>&1 || fail "verify signed fixture"
git show -s --format='%an%n%ae%n%cn%n%ce' "$signed_sha" > "$tmp/signed-output" 2>&1
[[ $(wc -l < "$tmp/signed-output") -gt 4 ]] || fail "signature presentation not exercised"
"$check" "$ibase" "$signed_sha" > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q 'committer email: person@anthropic.com' "$tmp/out" ||
  fail "signed committer email bypass: $(cat "$tmp/out")"
git config --unset log.showSignature

# Unknown output shapes fail closed even when every presented identity is otherwise clean.
# The stub represents arbitrary extra presentation and truncation, not known signature text.
cat > "$tmp/stub/git" <<'STUB'
#!/usr/bin/env bash
for arg in "$@"; do
  if [[ $arg == '--format=%an%n%ae%n%cn%n%ce' ]]; then
    printf 'Human\nhuman@example.org\nHuman\n'
    if [[ $IDENTITY_SHAPE == extra ]]; then
      printf 'human@example.org\nunknown presentation\n'
    fi
    exit 0
  fi
done
exec "$REAL_GIT" "$@"
STUB
chmod +x "$tmp/stub/git"
for shape in extra truncated; do
  REAL_GIT="$real_git" IDENTITY_SHAPE="$shape" PATH="$tmp/stub:$PATH" \
    "$check" "$ibase" "$signed_sha" > "$tmp/out" 2>&1
  [[ $? == 2 ]] && grep -q "git show could not read the identity of $signed_sha" "$tmp/out" ||
    fail "$shape identity output accepted: $(cat "$tmp/out")"
  grep -q 'no AI co-author trailer' "$tmp/out" && fail "$shape output reported clean"
done

# A clean commit passes on top of historical violations, including lookalike domains/names.
git reset -q --hard "$ibase" || fail "reset clean"
GIT_AUTHOR_NAME='Claude Monet' GIT_AUTHOR_EMAIL=claude@example.org \
  GIT_COMMITTER_NAME=claude-smith GIT_COMMITTER_EMAIL=person@notanthropic.com \
  git -c commit.gpgsign=false commit -q --allow-empty -m clean || fail "plant clean"
clean_sha=$(git rev-parse HEAD)
"$check" "$ibase" HEAD > "$tmp/out" 2>&1 || fail "clean range: $(cat "$tmp/out")"
pr_event "$clean_sha"
ci_check > "$tmp/out" 2>&1 || fail "clean CI: $(cat "$tmp/out")"
git push origin HEAD:refs/heads/proposed > "$tmp/out" 2>&1 || fail "clean new ref: $(cat "$tmp/out")"
git -c commit.gpgsign=false commit -q --allow-empty -m next-clean || fail "next clean"
git push origin HEAD:refs/heads/proposed > "$tmp/out" 2>&1 || fail "clean update: $(cat "$tmp/out")"
git tag -a -m tagged clean-tag || fail "tag clean"
git push origin clean-tag > "$tmp/out" 2>&1 || fail "clean tag: $(cat "$tmp/out")"
git push origin :refs/heads/proposed > "$tmp/out" 2>&1 || fail "delete ref: $(cat "$tmp/out")"
# Neither the destination's name nor stale local tracking refs can exempt unpublished commits.
git update-ref refs/remotes/origin/stale "$bad_sha" || fail "stale ref"
git push origin "$bad_sha:refs/heads/proposed" > "$tmp/out" 2>&1 && fail "stale ref bypass"
grep -q "$bad_sha has a prohibited identity" "$tmp/out" || fail "stale ref refusal missing"
# Another empty destination must check all ancestry, including the historical identity.
git init -q --bare "$tmp/empty.git" || fail "empty destination"
git push "$tmp/empty.git" HEAD:refs/heads/proposed > "$tmp/out" 2>&1 && fail "empty remote bypass"
grep -q "$historical has a prohibited identity" "$tmp/out" || fail "wrong remote excluded history"

# Both ref updates must be examined; a clean first ref cannot hide a bad second ref.
zero=0000000000000000000000000000000000000000
printf 'refs/heads/a %s refs/heads/a %s\nrefs/heads/b %s refs/heads/b %s\n' \
  "$clean_sha" "$zero" "$bad_sha" "$zero" |
  "$check" --pre-push origin "$tmp/identity-remote.git" > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q "$bad_sha has a prohibited identity" "$tmp/out" || fail "second ref bypass"
# Mailmap presentation never changes the raw identities being checked.
printf 'Human <human@example.org> <person@mail.anthropic.com>\n' > .mailmap
"$check" "$ibase" "$bad_sha" > "$tmp/out" 2>&1
[[ $? == 1 ]] || fail "mailmap masked identity"
rm .mailmap

# A synthetic merge with an offending committer is outside the PR's exact base..head range.
git checkout -q -b synthetic "$ibase" || fail "synthetic checkout"
git -c commit.gpgsign=false commit -q --allow-empty -m base-advance || fail "advance base"
GIT_COMMITTER_NAME=Claude git -c commit.gpgsign=false merge -q --no-ff -m synthetic "$clean_sha" ||
  fail "synthetic merge"
pr_event "$clean_sha"
ci_check > "$tmp/out" 2>&1 || fail "CI scanned synthetic merge: $(cat "$tmp/out")"
printf '{"pull_request":{"head":{"sha":"invalid"}}}\n' > "$tmp/event.json"
ci_check > "$tmp/out" 2>&1 && fail "invalid event passed"
grep -q 'cannot read the pull request head SHA' "$tmp/out" || fail "invalid event not named"
rm "$tmp/event.json"
ci_check > "$tmp/out" 2>&1 && fail "missing event passed"

# Guard failures are fail closed before vet. Direct stdin exercises deletion-only and failures.
zero=0000000000000000000000000000000000000000
printf 'refs/heads/x %s refs/heads/x %s\n' "$zero" "$ibase" |
  "$check" --pre-push origin "$tmp/absent.git" > "$tmp/out" 2>&1 || fail "deletion read remote"
printf 'refs/heads/x %s refs/heads/x %s\n' "$clean_sha" "$zero" |
  "$check" --pre-push origin "$tmp/absent.git" > "$tmp/out" 2>&1
[[ $? == 2 ]] || fail "remote enumeration failed open"
# A remote commit not present locally requires a fetch, not a guessed comparison base.
remote_new=$(printf 'remote-only\n' | git -C "$tmp/identity-remote.git" commit-tree "$ibase^{tree}" -p "$ibase")
git -C "$tmp/identity-remote.git" update-ref refs/heads/unfetched "$remote_new"
printf 'refs/heads/x %s refs/heads/x %s\n' "$clean_sha" "$zero" |
  "$check" --pre-push origin "$tmp/identity-remote.git" > "$tmp/out" 2>&1
[[ $? == 2 ]] && grep -q 'fetch the destination history' "$tmp/out" || fail "missing remote object accepted"
# Internal PR refs are not published branch/tag history and a normal fetch need not have them.
git -C "$tmp/identity-remote.git" update-ref -d refs/heads/unfetched
git -C "$tmp/identity-remote.git" update-ref refs/pull/1/head "$remote_new"
printf 'refs/heads/x %s refs/heads/x %s\n' "$clean_sha" "$zero" |
  "$check" --pre-push origin "$tmp/identity-remote.git" > "$tmp/out" 2>&1 || fail "internal PR ref blocked push"
# A replaced ref outside heads/tags still supplies an exclusion (other destination has none).
git init -q --bare "$tmp/custom.git" || fail "custom remote"
git -C "$tmp/custom.git" fetch -q "$tmp/identities" "$ibase" || fail "seed custom object"
git -C "$tmp/custom.git" update-ref refs/custom/x "$ibase" || fail "seed custom ref"
printf 'refs/heads/x %s refs/custom/x %s\n' "$clean_sha" "$ibase" |
  "$check" --pre-push custom "$tmp/custom.git" > "$tmp/out" 2>&1 || fail "custom old ref ignored"
# Trailer behavior is shared by the push path too; the original trailer tests above stay intact.
printf 'trailer\n\nCo-Authored-By: Claude <noreply@anthropic.com>\n' |
  git -c commit.gpgsign=false commit -q --allow-empty -F - || fail "plant trailer"
trailer_sha=$(git rev-parse HEAD)
printf 'refs/heads/x %s refs/heads/x %s\n' "$trailer_sha" "$zero" |
  "$check" --pre-push origin "$tmp/identity-remote.git" > "$tmp/out" 2>&1
[[ $? == 1 ]] && grep -q 'credits\|crediting' "$tmp/out" || fail "push trailer accepted"

# A shallow clone cannot establish which ancestral commits a range introduces.
git clone -q --depth 1 "file://$tmp/identity-remote.git" "$tmp/shallow" || fail "shallow clone"
(cd "$tmp/shallow" && "$check" HEAD HEAD) > "$tmp/out" 2>&1
[[ $? == 2 ]] && grep -q 'shallow or unreadable history' "$tmp/out" || fail "shallow range accepted"
(cd "$tmp/shallow" && printf 'refs/heads/x %s refs/heads/x %s\n' "$ibase" "$zero" |
  "$check" --pre-push origin "$tmp/identity-remote.git") > "$tmp/out" 2>&1
[[ $? == 2 ]] || fail "shallow push accepted"
(cd "$tmp/shallow" && printf 'refs/heads/x %s refs/heads/x %s\n' "$zero" "$ibase" |
  "$check" --pre-push origin "$tmp/absent.git") > "$tmp/out" 2>&1 || fail "shallow deletion refused"

printf 'check-ai-coauthor-trailers_test: ok\n'
