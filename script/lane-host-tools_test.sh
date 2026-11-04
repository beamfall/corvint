#!/usr/bin/env bash
# Tests for the lane-host tools (docs/LANE-ORCHESTRATION.md sections 6, 12 and 14):
# lane-scratch.sh (ASD-V0-765), lane-host-heavy.sh (ASD-V0-766), lane-host-check.sh (ASD-V0-767, 704),
# lane-remote-edit.sh (ASD-V0-768), ssh-heredoc-check.sh (ASD-V0-769),
# lane-inflight-branches.sh (ASD-V0-770), capability-screen.sh (ASD-V0-771) and
# lane-base-check.sh (ASD-V0-772). Fixtures are temporary Git repositories; ssh is a local stub and
# no network or Go build runs.
set -uo pipefail
export LC_ALL=C
unset $(git rev-parse --local-env-vars)
script=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$script/.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/corvint-lane-host-tools-test.XXXXXX") || exit 1
trap 'chmod -R u+rwx "$tmp" 2> /dev/null; /bin/rm -rf "$tmp"' EXIT
export HOME=$tmp/home GIT_CONFIG_NOSYSTEM=1 GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.invalid GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.invalid
mkdir -p "$HOME"
fails=0
fail() { printf 'lane-host-tools_test: FAIL %s\n' "$*" >&2; fails=$((fails + 1)); }
expect() { # expect RC WANT-SUBSTRING -- command...
  local want_rc=$1 want=$2 out rc; shift 3
  out=$("$@" 2>&1); rc=$?
  [[ $rc == "$want_rc" ]] || fail "rc $rc != $want_rc: $* -> $out"
  [[ -z $want || $out == *"$want"* ]] || fail "missing '$want': $* -> $out"
}
mkrepo() { git init -q -b main "$1" && git -C "$1" commit -q --allow-empty -m base; }

# ASD-V0-765: two lanes get two directories; a foreign marker is refused.
mkrepo "$tmp/lanes/a" && mkrepo "$tmp/lanes/b"
da=$(cd "$tmp/lanes/a" && "$script/lane-scratch.sh" --root "$tmp/s") || fail 'lane-scratch a'
db=$(cd "$tmp/lanes/b" && "$script/lane-scratch.sh" --root "$tmp/s") || fail 'lane-scratch b'
[[ $da == "$tmp/s/a" && $db == "$tmp/s/b" ]] || fail "lane-scratch dirs $da $db"
[[ $(stat -c %a "$da" 2> /dev/null || stat -f %Lp "$da") == 700 ]] || fail 'lane-scratch mode'
expect 2 'foreign-scratch' -- bash -c "cd '$tmp/lanes/b' && '$script/lane-scratch.sh' --root '$tmp/s' --lane a"
mkrepo "$tmp/other/a"
expect 2 'belongs to worktree' -- bash -c "cd '$tmp/other/a' && '$script/lane-scratch.sh' --root '$tmp/s'"
expect 2 'bad-lane' -- "$script/lane-scratch.sh" --root "$tmp/s" --lane ../x
expect 0 "$tmp/s/a" -- bash -c "cd '$tmp/lanes/a' && GIT_CONFIG_GLOBAL='$tmp' '$script/lane-scratch.sh' --root '$tmp/s'"
# A mode that cannot be enforced is refused, not handed out (chmod stubbed to fail).
mkdir -p "$tmp/nochmod" && printf '#!/bin/sh\nexit 1\n' > "$tmp/nochmod/chmod" && chmod +x "$tmp/nochmod/chmod"
expect 2 'chmod-failed' -- bash -c "cd '$tmp/lanes/a' && PATH='$tmp/nochmod':\$PATH '$script/lane-scratch.sh' --root '$tmp/s'"
# Two worktrees with one lane name start at once, first on a fresh directory, then on a marker the
# orchestrator staged outside any worktree: exactly one of each pair acquires the directory.
mkrepo "$tmp/race1/r" && mkrepo "$tmp/race2/r"
race() { # race ROOT: print how many of the two callers acquired ROOT/r
  local ok=0 p1 p2
  (cd "$tmp/race1/r" && "$script/lane-scratch.sh" --root "$1" > /dev/null 2>&1) & p1=$!
  (cd "$tmp/race2/r" && "$script/lane-scratch.sh" --root "$1" > /dev/null 2>&1) & p2=$!
  wait "$p1" && ok=$((ok + 1))
  wait "$p2" && ok=$((ok + 1))
  printf '%s\n' "$ok"
}
for k in 1 2 3 4 5 6 7 8; do
  [[ $(race "$tmp/rc$k") == 1 ]] || fail "lane-scratch concurrent creation round $k"
  (cd "$tmp" && "$script/lane-scratch.sh" --root "$tmp/ra$k" --lane r > /dev/null) || fail "lane-scratch staged marker $k"
  [[ $(race "$tmp/ra$k") == 1 ]] || fail "lane-scratch concurrent adoption round $k"
  [[ $(grep -c '^worktree=' "$tmp/ra$k/r/.lane") == 1 ]] || fail "lane-scratch adopted marker $k"
done

# ASD-V0-766: the host wrapper delegates even with GIT_CONFIG_GLOBAL naming a directory, and
# refuses outside a worktree whose heavy.sh answers --print-lock-dir.
mkrepo "$tmp/w" && mkdir -p "$tmp/w/script"
printf '#!/usr/bin/env bash\n[[ $1 == --print-lock-dir ]] && { echo /x; exit 0; }\necho ran "$@"\n' > "$tmp/w/script/heavy.sh"
chmod +x "$tmp/w/script/heavy.sh"
expect 0 'ran true' -- bash -c "cd '$tmp/w' && GIT_CONFIG_GLOBAL='$tmp' '$script/lane-host-heavy.sh' true"
expect 2 'REFUSED no-delegate' -- bash -c "cd '$tmp' && '$script/lane-host-heavy.sh' true"
printf '#!/usr/bin/env bash\nexit 1\n' > "$tmp/w/script/heavy.sh"
expect 2 'REFUSED no-delegate' -- bash -c "cd '$tmp/w' && '$script/lane-host-heavy.sh' true"

# ASD-V0-767: findings per check, each naming the owner step; OK when present.
mkdir -p "$HOME/lanes" "$HOME/.ssh" "$tmp/bin"
cp "$repo/script/lane-host-heavy.sh" "$HOME/lanes/heavy.sh"
printf '#!/bin/sh\nexit 0\n' > "$tmp/bin/okssh"; printf '#!/bin/sh\nexit 255\n' > "$tmp/bin/badssh"; chmod +x "$tmp/bin/"*
expect 1 'MISSING toolchain no-such-tool-xyz; owner:' -- env LANE_HOST_TOOLS='git no-such-tool-xyz' "$script/lane-host-check.sh" --only toolchain
expect 0 'OK toolchain git' -- env LANE_HOST_TOOLS=git "$script/lane-host-check.sh" --only toolchain
# ASD-V0-704: a tail that rejects the obsolete -N form warns without failing; one that fails -n N too
# is a finding; a working tail prints neither.
out=$(env LANE_HOST_TOOLS=git "$script/lane-host-check.sh" --only toolchain 2>&1)
[[ $out != *tail-* ]] || fail "working tail warned: $out"
mkdir -p "$tmp/tailbin"
real_tail=$(command -v tail)
printf '#!/bin/sh\ncase $1 in -n) exec %s "$@" ;; -*) echo "tail: invalid option" >&2; exit 1 ;; esac\nexec %s "$@"\n' "$real_tail" "$real_tail" > "$tmp/tailbin/tail"
chmod +x "$tmp/tailbin/tail"
expect 0 'WARN toolchain tail-count-unsupported' -- env PATH="$tmp/tailbin:$PATH" LANE_HOST_TOOLS=git "$script/lane-host-check.sh" --only toolchain
printf '#!/bin/sh\nexit 1\n' > "$tmp/tailbin/tail"
expect 1 'WARN toolchain tail-n-unsupported' -- env PATH="$tmp/tailbin:$PATH" LANE_HOST_TOOLS=git "$script/lane-host-check.sh" --only toolchain
expect 0 'matches script/lane-host-heavy.sh' -- bash -c "cd '$repo' && '$script/lane-host-check.sh' --only wrapper"
printf '\n# local edit\n' >> "$HOME/lanes/heavy.sh"
expect 1 'DRIFT wrapper' -- bash -c "cd '$repo' && '$script/lane-host-check.sh' --only wrapper"
expect 1 'MISSING ssh identity' -- "$script/lane-host-check.sh" --only ssh
: > "$HOME/.ssh/id_ed25519"
expect 0 'OK ssh identity' -- /bin/bash "$script/lane-host-check.sh" --only ssh
expect 0 'OK ssh peer p' -- env LANE_HOST_SSH="$tmp/bin/okssh" "$script/lane-host-check.sh" --only ssh --peer p
expect 1 'MISSING ssh peer p (BatchMode); owner:' -- env LANE_HOST_SSH="$tmp/bin/badssh" "$script/lane-host-check.sh" --only ssh --peer p
expect 1 'MISSING adopter VanillaTwisted' -- env CORVINT_ADOPTER_VANILLATWISTED="$tmp/none" "$script/lane-host-check.sh" --only adopter
expect 0 'OK adopter VanillaTwisted' -- env CORVINT_ADOPTER_VANILLATWISTED="$tmp/w" "$script/lane-host-check.sh" --only adopter
expect 0 'OK hygiene' -- bash -c "cd '$tmp/w' && '$script/lane-host-check.sh' --only hygiene"
expect 1 'git maintenance run' -- bash -c "cd '$tmp/w' && LANE_HOST_REFS_MAX=0 '$script/lane-host-check.sh' --only hygiene"
expect 2 'usage' -- "$script/lane-host-check.sh" --only nope

# ASD-V0-768: a patch with apostrophes, double quotes and dollars applies through the ssh stub.
printf '#!/usr/bin/env bash\nshift; exec bash -c "$*"\n' > "$tmp/bin/stubssh"; chmod +x "$tmp/bin/stubssh"
mkrepo "$tmp/e" && printf 'one\n' > "$tmp/e/f.txt" && git -C "$tmp/e" add f.txt && git -C "$tmp/e" commit -q -m f
printf "it's \"quoted\" \$HOME\n" > "$tmp/e/f.txt"; git -C "$tmp/e" diff > "$tmp/p.diff"; git -C "$tmp/e" checkout -q f.txt
expect 0 'f.txt' -- bash -c "LANE_REMOTE_SSH='$tmp/bin/stubssh' '$script/lane-remote-edit.sh' --check h '$tmp/e' < '$tmp/p.diff'"
[[ $(cat "$tmp/e/f.txt") == one ]] || fail 'lane-remote-edit --check wrote'
expect 0 'f.txt' -- bash -c "LANE_REMOTE_SSH='$tmp/bin/stubssh' '$script/lane-remote-edit.sh' h '$tmp/e' < '$tmp/p.diff'"
[[ $(cat "$tmp/e/f.txt") == "it's \"quoted\" \$HOME" ]] || fail "lane-remote-edit content $(cat "$tmp/e/f.txt")"
expect 1 'FAIL git apply refused' -- bash -c "LANE_REMOTE_SSH='$tmp/bin/stubssh' '$script/lane-remote-edit.sh' h '$tmp/e' < '$tmp/p.diff'"
expect 2 'bad-worktree' -- bash -c "'$script/lane-remote-edit.sh' h 'x;rm' < '$tmp/p.diff'"
expect 2 'empty-patch' -- bash -c "'$script/lane-remote-edit.sh' h /x < /dev/null"

# ASD-V0-769: a heredoc inside a quoted ssh argument is refused; stdin heredocs are allowed.
expect 2 "ssh HOST 'bash -s'" -- "$script/ssh-heredoc-check.sh" "ssh h 'cat > f <<EOF
it's
EOF'"
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" 'ssh -o X=1 h "cat <<X"'
expect 0 '' -- "$script/ssh-heredoc-check.sh" "ssh h 'bash -s' <<'EOF'
it's
EOF"
expect 0 '' -- "$script/ssh-heredoc-check.sh" "cat <<'EOF' | ssh h 'x'"
expect 0 '' -- "$script/ssh-heredoc-check.sh" 'echo "<<"; ssh h ls'
# The scan resumes after a stdin heredoc body, so a later quoted heredoc is still refused.
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "ssh h true <<EOF
x
EOF
ssh h 'cat <<INNER'"
expect 0 '' -- "$script/ssh-heredoc-check.sh" "ssh h 'bash -s' <<-'EOF'
	it's 'a <<B'
	EOF
echo done"
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "ssh h cat <<< x
ssh h 'cat <<E'"
# Leading assignments and env do not hide the ssh command name.
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "X=1 ssh h 'cat <<EOF'"
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "env X='a b' ssh h 'cat <<EOF'"
# A comment outside quotes is skipped to its newline: a quote or a << in it neither opens a quote
# nor starts a heredoc body, and the newline still ends the command.
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "# don't nest scripts
ssh h 'cat <<EOF'"
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" '# a " quote
ssh h "cat <<X"'
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "# feed it with <<EOF
ssh h 'cat <<X'
EOF"
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "ssh h ls # don't
ssh h 'cat <<EOF'"
# A # inside a word is not a comment.
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "echo a#'b'; ssh h 'cat <<X'"
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "# \$(date
ssh h 'cat <<EOF'"
# A backslash-newline joins lines, so a # after it can still start a comment.
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "echo a \\
# don't
ssh h 'cat <<EOF'"
# ( and ) bound commands: subshells, \$(...), case arms, and a comment right after (.
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "(# don't
ssh h 'cat <<EOF')"
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "(ssh h 'cat <<EOF')"
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" 'x=$(ssh h "cat <<EOF")'
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "case a in a) ssh h 'cat <<EOF';; esac"
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "f() { ssh h 'cat <<EOF'; }"
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "echo \`ssh h 'cat <<EOF'\`"
# \$'...' honours backslash escapes, so its \' does not end the quote.
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "echo \$'it\\'s'; ssh h 'cat <<EOF'"
# >&, <& and &> are redirections, not command separators.
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "ssh h 2>&1 'cat <<EOF'"
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "ssh h &>/dev/null 'cat <<EOF'"
expect 0 '' -- "$script/ssh-heredoc-check.sh" "ssh h 'bash -s' <<'EOF' 2>&1"
# The command name is compared dequoted.
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "\\ssh h 'cat <<EOF'"
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "'ssh' h 'cat <<EOF'"
# The SCRIPT of SHELL [-opts] -c SCRIPT is scanned as its own command line.
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "bash -c \"ssh h 'cat <<EOF'\""
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "sh -ec \"ssh h 'cat <<EOF'\""
expect 2 'REFUSE' -- "$script/ssh-heredoc-check.sh" "bash --norc -c \"ssh h 'cat <<EOF'\""
expect 0 '' -- "$script/ssh-heredoc-check.sh" "bash -c 'ssh h ls' <<EOF"
expect 0 '' -- "$script/ssh-heredoc-check.sh" "bash script.sh 'cat <<EOF'"
if command -v jq > /dev/null 2>&1; then
  expect 2 'REFUSE' -- bash -c "jq -n '{tool_input:{command:\"ssh h \\u0027a <<E\\u0027\"}}' | '$script/ssh-heredoc-check.sh'"
fi

# ASD-V0-770: an in-flight branch published to a host repo becomes visible there.
mkrepo "$tmp/orch" && git -C "$tmp/orch" checkout -q -b claude/lane-x && git -C "$tmp/orch" commit -q --allow-empty -m 'V1-9999 work'
git init -q --bare "$tmp/host.git" && git -C "$tmp/host.git" fetch -q "$tmp/orch" main:refs/heads/main
expect 1 'MISSING claude/lane-x; run on the orchestrator' -- "$script/lane-inflight-branches.sh" check --repo "$tmp/host.git" claude/lane-x
expect 0 'PUBLISHED claude/lane-x' -- bash -c "cd '$tmp/orch' && '$script/lane-inflight-branches.sh' publish --remote '$tmp/host.git' claude/lane-x"
expect 0 'VISIBLE claude/lane-x refs/remotes/inflight/claude/lane-x' -- "$script/lane-inflight-branches.sh" check --repo "$tmp/host.git" claude/lane-x
git -C "$tmp/host.git" log --all --grep=V1-9999 --format=%s | grep -q V1-9999 || fail 'inflight not found by log --all --grep'
expect 2 'usage' -- "$script/lane-inflight-branches.sh" check

# ASD-V0-771: the V1-3649 text finds the delivered `corvint test` row and its --summary flag.
expect 0 'corvint test flags=--dry-run row-flags=--dry-run,--summary' -- bash -c "cd '$repo' && '$script/capability-screen.sh' 'corvint test --dry-run prints 77 KB just to justify HEAVY_WHOLE_REASON'"
expect 1 'NONE' -- bash -c "cd '$repo' && '$script/capability-screen.sh' 'no command named here'"
expect 2 'cannot read' -- "$script/capability-screen.sh" --catalog "$tmp/none.md" 'corvint test'

# ASD-V0-772: a worktree cut from a stale host HEAD is refused; one cut from BASE passes.
mkrepo "$tmp/h" && old=$(git -C "$tmp/h" rev-parse HEAD)
git -C "$tmp/h" commit -q --allow-empty -m newer && base=$(git -C "$tmp/h" rev-parse HEAD)
git -C "$tmp/h" worktree add -q "$tmp/good" -b lane-good "$base"
git -C "$tmp/h" worktree add -q "$tmp/bad" -b lane-bad "$old"
expect 0 "PASS branch=lane-good base=$base" -- "$script/lane-base-check.sh" "$base" "$tmp/good"
expect 1 'REFUSE wrong-base' -- "$script/lane-base-check.sh" "$base" "$tmp/bad"
expect 1 'REFUSE shared-checkout' -- "$script/lane-base-check.sh" "$base" "$tmp/h"
expect 2 'is not a commit' -- "$script/lane-base-check.sh" nope "$tmp/good"
# With reflogs off, a branch cut from a HEAD that contains BASE plus other work has no creation
# evidence and is refused rather than passed on ancestry.
git -C "$tmp/h" commit -q --allow-empty -m unrelated && ahead=$(git -C "$tmp/h" rev-parse HEAD)
git -C "$tmp/h" -c core.logAllRefUpdates=false worktree add -q "$tmp/noref" -b lane-noref "$ahead"
expect 1 'REFUSE no-reflog' -- "$script/lane-base-check.sh" "$base" "$tmp/noref"

if ((fails)); then printf 'lane-host-tools_test: %d failures\n' "$fails" >&2; exit 1; fi
printf 'lane-host-tools_test: PASS\n'
