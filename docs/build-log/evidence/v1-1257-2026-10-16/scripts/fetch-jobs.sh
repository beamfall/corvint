#!/bin/sh
# fetch jobs (all attempts) for run ids on stdin into jobs/<id>.json, skipping done ones; stop after $1 seconds
end=$(( $(date +%s) + $1 ))
while read id; do
  [ -s jobs/$id.json ] && continue
  [ $(date +%s) -ge $end ] && break
  gh api --paginate "repos/beamfall/corvint/actions/runs/$id/jobs?filter=all&per_page=100" --jq '.jobs[] | {status,conclusion,started_at,completed_at,name,run_attempt} | @json' -q '.jobs[] | {status,conclusion,started_at,completed_at,name,run_attempt} | tojson' > jobs/$id.json.tmp 2>/dev/null && /bin/mv -f jobs/$id.json.tmp jobs/$id.json
done
