#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Usage: lane-handback.sh template [PREVIOUS] | lint FILE [PREVIOUS]
# DWC-V0-047: print a closeout template or lint it; no repository or ticket writes.
# Example: script/lane-handback.sh template old.md > round2.md
set -uo pipefail
field() { sed -n "s/^$1: //p" "$2"; }
refuse() { printf 'lane-handback: REFUSE %s\n' "$*" >&2; exit 2; }
case ${1-} in
  -h|--help) sed -n '3,5s/^# //p' "$0"; exit 0 ;;
  template)
    [[ $# == 1 || $# == 2 ]] || refuse 'usage: template [PREVIOUS]'
    delivered='NOT_PRODUCED list all delivered ticket IDs'
    superseded=none
    if [[ $# == 2 ]]; then
      [[ -r $2 ]] || refuse "unreadable previous handback: $2"
      delivered=$(field 'Delivered tickets' "$2")
      superseded=$(field 'Superseded tickets' "$2")
    fi
    cat <<TEMPLATE
Base: FULL_SHA
Fix head: FULL_SHA
Bind head: NOT_PRODUCED binding pending
Sealed head: NOT_PRODUCED seal pending
Delivered tickets: $delivered
Superseded tickets: $superseded
GitHub issues closed: none

Per-ticket evidence (requirement IDs and focused tests):

Commands and results; NOT_RUN/NOT_PRODUCED with reasons:

Numbered friction list with reproduction commands:
TEMPLATE
    ;;
  lint)
    [[ $# == 2 || $# == 3 ]] || refuse 'usage: lint FILE [PREVIOUS]'
    [[ -f $2 && -r $2 ]] || refuse "unreadable handback: $2"
    for name in 'Base' 'Fix head' 'Bind head' 'Sealed head' 'Delivered tickets' 'Superseded tickets' 'GitHub issues closed'; do
      value=$(field "$name" "$2")
      [[ -n $value && $value != *$'\n'* ]] || refuse "missing or duplicate $name"
      case $name in
        Base|'Fix head') [[ $value =~ ^[0-9a-f]{40}$ ]] || refuse "invalid $name" ;;
        'Bind head'|'Sealed head') [[ $value =~ ^[0-9a-f]{40}$ || $value == 'NOT_PRODUCED '?* ]] || refuse "invalid $name: full SHA or NOT_PRODUCED plus reason required" ;;
        'Delivered tickets'|'Superseded tickets')
          [[ $value == none || $value =~ ^V1-[0-9]+(\ V1-[0-9]+)*$ ]] || refuse "invalid $name: space-separated ticket IDs or none required" ;;
      esac
    done
    if [[ $# == 3 ]]; then
      [[ -f $3 && -r $3 ]] || refuse "unreadable previous handback: $3"
      current=" $(field 'Delivered tickets' "$2") $(field 'Superseded tickets' "$2") "
      for name in 'Delivered tickets' 'Superseded tickets'; do
        prior=$(field "$name" "$3")
        [[ -n $prior ]] || refuse "previous handback missing $name"
        for ticket in $prior; do
          [[ $ticket == none || $current == *" $ticket "* ]] || refuse "cumulative ticket lost: $ticket"
        done
      done
    fi
    printf 'lane-handback: PASS %s\n' "$2"
    ;;
  *) refuse 'usage: lane-handback.sh template [PREVIOUS] | lint FILE [PREVIOUS]' ;;
esac
