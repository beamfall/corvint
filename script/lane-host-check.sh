#!/usr/bin/env bash
# lane-host-check.sh: report what a lane host can and cannot run before a lane is dispatched to it
# (ASD-V0-767; V1-3653, V1-3785, V1-3953, V1-4102, V1-4057). Lanes found missing toolchains, a missing
# ssh identity, a missing adopter checkout, a drifted ~/lanes/heavy.sh and a slow, unmaintained
# repository only after they had started. This prints one line per finding instead; it never
# installs, repacks or creates keys (those are owner steps, and each line names the step).
#
#   script/lane-host-check.sh [--only toolchain|wrapper|ssh|adopter|hygiene]... [--peer HOST]...
#
# Lines: `OK <check> <what>`, `MISSING <check> <what>; <owner fix>`, `DRIFT wrapper ...` and
# `WARN hygiene ...`, `WARN toolchain tail-...`. Exit 0 when every line is OK or the only WARN is the
# informational `tail-count-unsupported`, 1 when any finding is reported, 2 on a usage error.
# Overrides for tests and other hosts: LANE_HOST_TOOLS (space-separated tool names, default below),
# LANE_HOST_WRAPPER (default ~/lanes/heavy.sh), LANE_HOST_SSH (default ssh),
# CORVINT_ADOPTER_VANILLATWISTED (default ~/adopters/VanillaTwisted), LANE_HOST_LOOSE_MAX (default
# 2000 loose objects) and LANE_HOST_REFS_MAX (default 400 refs).
set -uo pipefail
export LC_ALL=C
usage() { printf 'usage: script/lane-host-check.sh [--only toolchain|wrapper|ssh|adopter|hygiene]... [--peer HOST]...\n' >&2; exit 2; }
only=() peers=()
while (($#)); do
  case $1 in
    --only) (($# >= 2)) || usage
      case $2 in toolchain | wrapper | ssh | adopter | hygiene) only+=("$2") ;; *) usage ;; esac
      shift 2 ;;
    --peer) { (($# >= 2)) && [[ -n $2 ]]; } || usage; peers+=("$2"); shift 2 ;;
    *) usage ;;
  esac
done
((${#only[@]})) || only=(toolchain wrapper ssh adopter hygiene)
top=$(env -u GIT_CONFIG_GLOBAL git rev-parse --show-toplevel 2> /dev/null) || top=''
findings=0
ok() { printf 'OK %s\n' "$*"; }
miss() { printf 'MISSING %s\n' "$*"; findings=$((findings + 1)); }

check_toolchain() {
  # The toolchains acceptance criteria name (V1-3653): Go and Git for every lane, the rest for the
  # browser, C++, SQL and image-build criteria.
  local t tools=${LANE_HOST_TOOLS:-go git jq node gh perl playwright gtest-config sqllogictest postgres docker podman}
  for t in $tools; do
    if command -v "$t" > /dev/null 2>&1; then
      ok "toolchain $t $(command -v "$t")"
    else
      miss "toolchain $t; owner: install $t on $(hostname 2> /dev/null || echo this host) or route the ticket to a host that has it"
    fi
  done
  # The obsolete `tail -N` count form fails on some hosts (ASD-V0-704, V1-4125): lane recipes write
  # `tail -n N`. A host where `tail -n N` fails too is a finding.
  local want tp
  want=$(printf 'b\nc\n')
  tp=$(command -v tail 2> /dev/null || printf none)
  if [[ $(printf 'a\nb\nc\n' | tail -n 2 2> /dev/null) != "$want" ]]; then
    printf 'WARN toolchain tail-n-unsupported %s; owner: put a POSIX tail (/usr/bin/tail) first on PATH\n' "$tp"
    findings=$((findings + 1))
  elif [[ $(printf 'a\nb\nc\n' | tail -2 2> /dev/null) != "$want" ]]; then
    printf 'WARN toolchain tail-count-unsupported %s; write tail -n N, never tail -N\n' "$tp"
  fi
}

check_wrapper() {
  local w=${LANE_HOST_WRAPPER:-${HOME:-}/lanes/heavy.sh} src=${top:+$top/script/lane-host-heavy.sh}
  if [[ -z $src || ! -f $src ]]; then
    miss "wrapper source script/lane-host-heavy.sh; run from a Corvint worktree at origin/main"
  elif [[ ! -e $w ]]; then
    miss "wrapper $w; owner: install -m 0755 script/lane-host-heavy.sh $w"
  elif cmp -s "$w" "$src"; then
    ok "wrapper $w matches script/lane-host-heavy.sh"
  else
    printf 'DRIFT wrapper %s differs from script/lane-host-heavy.sh; owner: install -m 0755 script/lane-host-heavy.sh %s\n' "$w" "$w"
    findings=$((findings + 1))
  fi
}

check_ssh() {
  local k p have=''
  for k in "${HOME:-}"/.ssh/id_*; do
    [[ -f $k && $k != *.pub ]] && have=$k && break
  done
  if [[ -n $have ]]; then ok "ssh identity $have"; else
    miss "ssh identity ~/.ssh/id_*; owner: create a key on this host and authorize it on each peer"
  fi
  for p in ${peers[@]+"${peers[@]}"}; do # guarded: Bash 3.2 (macOS) aborts on an empty array under set -u
    if ${LANE_HOST_SSH:-ssh} -o BatchMode=yes -o ConnectTimeout=5 "$p" true < /dev/null > /dev/null 2>&1; then
      ok "ssh peer $p"
    else
      miss "ssh peer $p (BatchMode); owner: authorize this host's key on $p"
    fi
  done
}

check_adopter() {
  local a=${CORVINT_ADOPTER_VANILLATWISTED:-${HOME:-}/adopters/VanillaTwisted} rev
  if rev=$(env -u GIT_CONFIG_GLOBAL git -C "$a" rev-parse --verify -q HEAD 2> /dev/null); then
    ok "adopter VanillaTwisted $a at $rev"
  else
    miss "adopter VanillaTwisted $a; owner: clone it at the pinned revision or set CORVINT_ADOPTER_VANILLATWISTED"
  fi
}

check_hygiene() {
  [[ -n $top ]] || { miss "hygiene: not inside a worktree; run from the lane worktree"; return; }
  local loose refs packs
  loose=$(env -u GIT_CONFIG_GLOBAL git -C "$top" count-objects -v | sed -n 's/^count: //p')
  packs=$(env -u GIT_CONFIG_GLOBAL git -C "$top" count-objects -v | sed -n 's/^packs: //p')
  refs=$(env -u GIT_CONFIG_GLOBAL git -C "$top" for-each-ref --format=x | wc -l | tr -d ' ')
  if ((loose > ${LANE_HOST_LOOSE_MAX:-2000} || refs > ${LANE_HOST_REFS_MAX:-400})); then
    printf 'WARN hygiene loose=%s packs=%s refs=%s; screen with `git log origin/main -S`, not --all; owner: git maintenance run --task=loose-objects --task=incremental-repack\n' "$loose" "$packs" "$refs"
    findings=$((findings + 1))
  else
    ok "hygiene loose=$loose packs=$packs refs=$refs"
  fi
}

for c in ${only[@]+"${only[@]}"}; do "check_$c"; done
((findings == 0))
