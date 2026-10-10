#!/bin/sh
# loop.sh LANE N : run the HLQ claude-code tuple N times; one summary row per run.
# Row: lane i startUTC endUTC load1Start load1End rc upgradeStatus upgradeTimeBoundRetries otherFailedCases
W=${W:?directory holding hlq, corvint and corvint-n1}
export TMPDIR=${TMPDIR:-/tmp}
lane=$1; n=$2; out=$W/runs-$lane.tsv
i=1
while [ $i -le $n ] && [ ! -e $W/stop ]; do
  l0=$(sysctl -n vm.loadavg | awk '{print $2}'); t0=$(date -u +%H:%M:%S)
  rep=$W/rep-$lane-$i.tsv
  $W/hlq --host claude-code --corvint $W/corvint --base-corvint $W/corvint-n1 --source ${SOURCE:?clean checkout of the source commit} --report $rep </dev/null >/dev/null 2>$W/err-$lane-$i.txt
  rc=$?
  l1=$(sysctl -n vm.loadavg | awk '{print $2}'); t1=$(date -u +%H:%M:%S)
  up=$(awk -F'\t' '$1=="case"&&$2=="upgrade"{print $3}' $rep 2>/dev/null)
  ur=$(awk -F'\t' '$1=="case"&&$2=="upgrade"{n=gsub(/SessionStart attempt [0-9]/,"");print n}' $rep 2>/dev/null)
  of=$(awk -F'\t' '$1=="case"&&$2!="upgrade"&&$3!="PASS"{printf "%s;",$2}' $rep 2>/dev/null)
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$lane" "$i" "$t0" "$t1" "$l0" "$l1" "$rc" "${up:-NONE}" "${ur:-0}" "${of:--}" >> $out
  if [ "${up}" = PASS ] && [ "${ur:-0}" = 0 ]; then /bin/rm -f $rep $W/err-$lane-$i.txt; fi
  i=$((i+1))
done
