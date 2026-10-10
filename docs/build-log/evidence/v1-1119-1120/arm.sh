#!/bin/sh
# arm.sh K LOOPS N : K yes burners + LOOPS concurrent loops of N runs; reaps own children by PID.
W=${W:?}; pids=""; lpids=""
cleanup() { for p in $pids $lpids; do kill $p 2>/dev/null; done; }
trap cleanup EXIT INT TERM
k=0; while [ $k -lt $1 ]; do yes >/dev/null & pids="$pids $!"; k=$((k+1)); done
echo "$pids" > $W/burn.pids
j=1; while [ $j -le $2 ]; do sh $(dirname $0)/loop.sh ${ARM:-x}$j $3 & lpids="$lpids $!"; j=$((j+1)); done
wait $lpids
