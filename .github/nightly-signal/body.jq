# SPDX-License-Identifier: Apache-2.0
# SOP-V0-013: the Markdown body of the nightly-triage issue or comment, from one
# run attempt's `gh run view --json jobs`. Arguments: --arg url (the attempt's
# URL) and --arg sha (the commit the run gated). A job or step is failing when its
# conclusion is one of `bad`; the signal job itself is still running, so its empty
# conclusion never lists it.
def bad: . as $c | ["failure", "cancelled", "timed_out", "startup_failure", "action_required"] | index($c) != null;
([.jobs[] | select(.conclusion | bad)] | sort_by(.name)) as $failed
| "Scheduled `release-gates` run on `main` at \($sha) failed.\n\n"
+ "Run: \($url)\n\n"
+ "Failing jobs and steps:\n"
+ (if ($failed | length) == 0 then "- no job of this attempt reported a failing conclusion; read the run\n"
   else $failed | map("- \(.name) (\(.conclusion)): "
       + ([.steps[]? | select(.conclusion | bad) | .name] | if length == 0 then "no failing step reported" else join(", ") end)
       + " (\(.url))\n") | join("")
   end)
+ "\nTriage: the release owner reads the logs, files or links the fix, and closes this issue once a "
+ "scheduled run passes. Until then each failed nightly run comments here instead of opening another issue.\n"
