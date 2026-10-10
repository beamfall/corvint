#!/bin/sh
# burn.sh K MAXSEC : K CPU burners until $W/stop exists or MAXSEC elapses; always reaps its own children.
W=${W:?directory holding hlq, corvint and corvint-n1}
pids=""
cleanup() { for p in $pids; do kill $p 2>/dev/null; done; }
trap cleanup EXIT INT TERM
k=0; while [ $k -lt $1 ]; do yes >/dev/null & pids="$pids $!"; k=$((k+1)); done
echo "$pids" > $W/burn.pids
s=0; while [ $s -lt $2 ] && [ ! -e $W/stop ]; do sleep 5; s=$((s+5)); done
