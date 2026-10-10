#!/bin/sh
# loop.sh LANE N : HLQ claude-code N times; keep report+stderr when rc!=0.
W=${W:?}; SOURCE=${SOURCE:?}; HLQ=${HLQ:-$W/hlq}; CORV=${CORV:-$W/corvint}
lane=$1; n=$2; out=$W/runs-$lane.tsv; i=1
while [ $i -le $n ] && [ ! -e $W/stop ]; do
  l0=$(sysctl -n vm.loadavg | awk '{print $2}'); t0=$(date -u +%H:%M:%S)
  rep=$W/rep-$lane-$i.tsv
  $HLQ --host claude-code --corvint $CORV --base-corvint $W/corvint-n1 --source $SOURCE --report $rep --failed-reports $W/kept </dev/null >/dev/null 2>$W/err-$lane-$i.txt
  rc=$?
  l1=$(sysctl -n vm.loadavg | awk '{print $2}'); t1=$(date -u +%H:%M:%S)
  fc=$(awk -F'\t' '$1=="case"&&$3!="PASS"{printf "%s;",$2}' $rep 2>/dev/null)
  rt=$(grep -c 'attempt [0-9]' $rep 2>/dev/null)
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$lane" "$i" "$t0" "$t1" "$l0" "$l1" "$rc" "${fc:--}" "${rt:-0}" >> $out
  if [ "$rc" = 0 ] && [ "${rt:-0}" = 0 ]; then /bin/rm -f $rep $W/err-$lane-$i.txt; fi
  i=$((i+1))
done
