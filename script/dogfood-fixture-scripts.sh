#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# ASD-V0-086 / V1-3242: the shared copy step of the dogfood wrapper shell-test fixtures. Source it,
# then call `dogfood_fixture_copy SOURCE_ROOT DEST_SCRIPT_DIR WRAPPER...`: it copies each named
# script and, transitively, every script/*.sh helper the copied scripts source or run, so a new
# helper reaches every fixture without editing each test. Bash 3.2 compatible (no assoc arrays).

# dogfood_fixture_deps FILE prints the script/*.sh names FILE references outside comments.
dogfood_fixture_deps() {
  sed -e 's/^[[:space:]]*#.*//' "$1" | grep -oE 'script/[A-Za-z0-9_.-]+\.sh' | sed 's|^script/||' |
    grep -v '_test\.sh$' | sort -u || true
}

dogfood_fixture_copy() {
  local source_root=$1 dest=$2 queue copied=' ' name dep
  shift 2
  queue=("$@")
  while ((${#queue[@]} > 0)); do
    name=${queue[0]}
    queue=("${queue[@]:1}")
    [[ $copied == *" $name "* ]] && continue
    if [[ ! -f $source_root/script/$name ]]; then
      printf 'dogfood_fixture_copy: REFUSE missing-script script/%s\n' "$name" >&2
      printf 'fix: name an existing script under %s/script, or remove the reference to it\n' "$source_root" >&2
      return 2
    fi
    /bin/cp -f "$source_root/script/$name" "$dest/" || return 2
    copied="$copied$name "
    for dep in $(dogfood_fixture_deps "$source_root/script/$name"); do
      queue+=("$dep")
    done
  done
}
