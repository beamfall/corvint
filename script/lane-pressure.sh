#!/usr/bin/env bash
# usage: script/lane-pressure.sh [--level CURRENT]
#
# ASD-V0-936 (V1-1627): classify one host-pressure sample by the lane admission table of
# docs/LANE-ORCHESTRATION.md section 1 (ASD-V0-013, the dispatcher signal of CAL-V0-068) and print
#   level=L load_per_cpu=X memory=M swap=S admit=N
# where admit is the number of Go-test-heavy lanes that level admits (0: 4, 1: 3, 2: 1). Inputs: the
# 1-minute load average per online CPU, plus the memory-pressure level on Darwin
# (kern.memorystatus_vm_pressure_level: 1 normal, 2 warn, 4 critical) or used/total swap on Linux; the
# signal a host lacks prints as `-`. The sample enters the highest level whose entry rule holds; from
# CURRENT (default 0) a lower level needs that level's release rule, so a sample between thresholds
# keeps CURRENT. The orchestrator still requires two agreeing samples (ticksToChange 2) before it
# changes the cap, and pressure holds only new heavy launches.
#   --level CURRENT     the level in force now (0, 1 or 2)
#   LANE_PRESSURE_PROC  the Linux procfs root (default /proc)
# Example: script/lane-pressure.sh --level 1
# Exit: 0 classified; 2 usage; 3 unreadable sample (prints level=CURRENT ... unreadable).
set -uo pipefail

cur=0
case ${1-} in
  -h | --help)
    sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
    exit 0
    ;;
  --level)
    [[ $# == 2 && $2 =~ ^[012]$ ]] || { printf 'lane-pressure: usage: lane-pressure.sh [--level 0|1|2]\n' >&2; exit 2; }
    cur=$2
    ;;
  '') ;;
  *) printf 'lane-pressure: usage: lane-pressure.sh [--level 0|1|2]\n' >&2; exit 2 ;;
esac

load='' cpus='' mem=- swap=-
if [[ $(uname -s 2> /dev/null) == Darwin ]]; then
  load=$(sysctl -n vm.loadavg 2> /dev/null | awk '{ print $2 }')
  cpus=$(sysctl -n hw.ncpu 2> /dev/null)
  mem=$(sysctl -n kern.memorystatus_vm_pressure_level 2> /dev/null)
  [[ $mem =~ ^[124]$ ]] || mem=-
else
  proc=${LANE_PRESSURE_PROC:-/proc}
  load=$(awk '{ print $1 }' "$proc/loadavg" 2> /dev/null)
  cpus=$(getconf _NPROCESSORS_ONLN 2> /dev/null)
  swap=$(awk '/^SwapTotal:/ { t = $2 } /^SwapFree:/ { f = $2 } END { if (t == "") exit 1; printf "%.2f", (t > 0 ? (t - f) / t : 0) }' "$proc/meminfo" 2> /dev/null) || swap=-
fi

if ! [[ $load =~ ^[0-9]+(\.[0-9]+)?$ && $cpus =~ ^[1-9][0-9]*$ ]]; then
  admit=(4 3 1)
  printf 'level=%s load_per_cpu=unknown memory=%s swap=%s admit=%s unreadable\n' "$cur" "$mem" "$swap" "${admit[cur]}"
  exit 3
fi

awk -v l="$load" -v c="$cpus" -v m="$mem" -v s="$swap" -v cur="$cur" 'BEGIN {
  lpc = l / c
  mm = (m == "-") ? 1 : m + 0
  ss = (s == "-") ? 0 : s + 0
  entered = (lpc >= 3.0 || mm == 4 || ss >= 0.85) ? 2 : (lpc >= 1.5 || mm >= 2 || ss >= 0.7) ? 1 : 0
  calm = lpc < 1.0 && mm <= 1 && ss < 0.5
  level = entered
  if (entered < cur) {
    if (cur == 2 && !(lpc < 1.5 && mm != 4 && ss < 0.85)) level = 2
    else level = calm ? 0 : 1
  }
  admit = level == 0 ? 4 : level == 1 ? 3 : 1
  printf "level=%d load_per_cpu=%.2f memory=%s swap=%s admit=%d\n", level, lpc, m, s, admit
}'
