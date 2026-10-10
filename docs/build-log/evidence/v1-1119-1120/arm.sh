#!/bin/sh
# arm.sh K LOOPS N : K yes burners + LOOPS concurrent loops of N runs; reaps own children by PID.
# Review amendment after the lane G runs: on a signal the loops are not killed mid-run. $W/stop
# ends each loop after its current HLQ run, so the runner cleans up its own private workspace and
# host processes, and the arm waits for that before it exits.
W=${W:?}; pids=""; lpids=""
cleanup() {
  trap - EXIT INT TERM
  touch "$W/stop"
  for p in $pids; do kill $p 2>/dev/null; done
  for p in $lpids; do wait $p 2>/dev/null; done
}
trap cleanup EXIT INT TERM
k=0; while [ $k -lt $1 ]; do yes >/dev/null & pids="$pids $!"; k=$((k+1)); done
echo "$pids" > $W/burn.pids
j=1; while [ $j -le $2 ]; do sh $(dirname $0)/loop.sh ${ARM:-x}$j $3 & lpids="$lpids $!"; j=$((j+1)); done
wait $lpids
