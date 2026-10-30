# SPDX-License-Identifier: Apache-2.0
# AFP-V0-047: package-level outcomes of one go test -json stream.
# Usage: jq -R -n -c --rawfile expected FILE -f outcomes.jq < STREAM
# FILE names one package per go test invocation that should have reported it
# (a package split into slices appears once per slice). Only package-level
# terminal events (Test absent, Action pass|fail|skip) count; a non-JSON line
# is ignored. A package fails on any fail event, is missing when it reported
# fewer terminal events than invocations, else passes on any pass, else skips.
[$expected | split("\n")[] | select(length > 0)] as $want
| [inputs | fromjson? | objects
   | select(.Test == null and (.Package | type) == "string"
            and (.Action == "pass" or .Action == "fail" or .Action == "skip"))
   | {p: .Package, a: .Action}] as $events
| ($want | reduce .[] as $p ({}; .[$p] += 1)) as $count
| ($events | reduce .[] as $e ({}; .[$e.p] += [$e.a])) as $seen
| [($want + ($events | map(.p))) | unique[]] as $packages
| reduce $packages[] as $p ({};
    ($seen[$p] // []) as $a
    | .[$p] = (if any($a[]; . == "fail") then "fail"
               elif ($a | length) < ($count[$p] // 0) then "missing"
               elif any($a[]; . == "pass") then "pass"
               else "skip" end))
