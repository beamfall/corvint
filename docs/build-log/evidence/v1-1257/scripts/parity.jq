# Per-shard parity of the shadow corvint test selection against the driver's executed shard.
# Usage: jq -n -r --arg run RUN --slurpfile sel SELECTION --slurpfile dry DRYRUN --slurpfile out OUTCOMES -f parity.jq
# Driver argv prefix from ci.yml "Go tests" (FULL branch), RUNNER_TEMP=/home/runner/work/_temp.
["go","test","-exec","/home/runner/work/_temp/test-confine","-json","-p","1","-count=1","-race","-timeout","50m"] as $prefix
| $sel[0] as $s | $dry[0] as $d | $out[0] as $o
| ($s.selected) as $S
| ($o.outcomes | keys) as $D
| ($S - $D) as $sOnly
| ($D - $S) as $dOnly
| [$d.invocations[] | select(.adapter == "go") | .argv[0:11] == $prefix] as $argvOK
| [$d.invocations[] | .units[] | select(.slice != "whole") | "\(.unit | ltrimstr("go:github.com/Beamfall/corvint/"))@\(.slice)"] as $slices
| ([$o.outcomes | to_entries[] | select(.value == "fail" or .value == "missing") | .key]) as $F
| ($F - $S) as $missed
| [$run, ($s.pr // "-" | tostring), ($s.shard | tostring), $s.head[0:12], $s.target[0:12], $o.target[0:12], ($s.toolSource[0:12]),
   $s.scope, ($s.exit | tostring), ($S | length | tostring), ($D | length | tostring), ($sOnly | length | tostring),
   ($dOnly | length | tostring), (if all($argvOK[]; .) then "same" else "DIFF" end),
   (($s.incomplete // []) | map(sub(": .*"; "")) | join(",") | if . == "" then "-" else . end),
   ($slices | join(",") | if . == "" then "-" else . end),
   ($o.status | tostring), ($F | length | tostring), ($missed | length | tostring),
   (if $s.target != $o.target or $s.head != $o.head then "TARGET_MISMATCH"
    elif ($sOnly | length) > 0 then "UNEXPLAINED"
    elif all($argvOK[]; .) | not then "UNEXPLAINED"
    elif ($missed | length) > 0 then "MISS"
    else "subset" end),
   ($sOnly | join(" ") | if . == "" then "-" else . end)]
| @tsv
