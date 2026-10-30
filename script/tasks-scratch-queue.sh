#!/usr/bin/env bash
# usage: script/tasks-scratch-queue.sh DIR
#
# Creates an execution-ready scratch Corvint Tasks queue in DIR in seconds (TCD-V0-183, V1-2834):
# a fresh Git repository on `main` with one empty commit, and a `.taskman/` store initialized from
# the maintained external-agents templates (internal/tasks/cli/testdata/external-agents) with
#   queueId queue:scratch:main, repositoryAuthorityId repo:scratch, prefix SCR, fixture:true,
#   and the required `verify` gate's argv replaced by ["true"].
# A fixture queue admits claims without an execution cutover, so `ticket create`, `claim`,
# `submit`, `gate run` and `complete` work at once. It is for lane rehearsal and tests only:
# every read reports fixture:true, and a non-fixture (live) queue still refuses claims until the
# owner records `cutover --execution` with a retained CAL-V0-019 qualification (unchanged).
#
# DIR must be absent or empty; anything else refuses (exit 2) and nothing is written. The
# corvint-tasks executable is $CORVINT_TASKS (default: corvint-tasks on PATH) and the init actor is
# $CORVINT_TASKS_ACTOR (default: scratch). On success it prints one line:
#   READY <DIR> queue:scratch:main fixture:true
set -euo pipefail

if [[ $# -ne 1 || -z $1 ]]; then
  echo "usage: script/tasks-scratch-queue.sh DIR" >&2
  exit 2
fi
dir=$1
tasks=${CORVINT_TASKS:-corvint-tasks}
templates="$(cd "$(dirname "$0")/.." && pwd)/internal/tasks/cli/testdata/external-agents"

if [[ -e $dir && ( ! -d $dir || -n $(ls -A "$dir") ) ]]; then
  echo "refused: $dir exists and is not an empty directory" >&2
  exit 2
fi

queue=$(sed -e 's/"fixture":false/"fixture":true/' \
  -e 's/"prefix":"APP"/"prefix":"SCR"/' \
  -e 's/"queueId":"queue:example:main"/"queueId":"queue:scratch:main"/' \
  -e 's/"repositoryAuthorityId":"repo:example"/"repositoryAuthorityId":"repo:scratch"/' \
  "$templates/queue.json")
policy=$(sed -e 's/"argv":\["make","verify"\]/"argv":["true"]/' "$templates/policy.json")
# Fail closed when a template no longer has the shape these edits expect.
for want in '"fixture":true' '"prefix":"SCR"' '"queueId":"queue:scratch:main"' '"repositoryAuthorityId":"repo:scratch"'; do
  [[ $queue == *"$want"* ]] || { echo "refused: queue template lacks $want after editing" >&2; exit 1; }
done
[[ $policy == *'"argv":["true"]'* ]] || { echo "refused: policy template has no make verify gate to replace" >&2; exit 1; }

mkdir -p "$dir/.taskman"
printf '%s\n' "$queue" >"$dir/.taskman/queue.json"
printf '%s\n' "$policy" >"$dir/.taskman/policy.json"
git -C "$dir" init -q -b main
git -C "$dir" -c user.name=scratch -c user.email=scratch@example.invalid commit -q --allow-empty -m "scratch base"
(cd "$dir" && CORVINT_TASKS_ACTOR=${CORVINT_TASKS_ACTOR:-scratch} "$tasks" init --request-id scratch-init >/dev/null)
echo "READY $dir queue:scratch:main fixture:true"
