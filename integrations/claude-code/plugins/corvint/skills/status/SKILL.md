---
name: status
description: Show Corvint's state for this session and repository - versions, index freshness, workflow enrollment and recent adapter problems - as a short readable report.
disable-model-invocation: true
allowed-tools: Bash(corvint --version), Bash(corvint observations:*), Bash(corvint dogfood status:*), Bash(git rev-parse:*), Bash(ls:*)
---

# Corvint status

Build a short status report for the user. Every command below is read-only.

1. `corvint --version`, and the plugin version from this plugin's manifest.
2. Index: list `$(git rev-parse --git-common-dir)/corvint/index/` and look for a snapshot whose
   name contains `git rev-parse HEAD^{tree}`. Report fresh when one matches, otherwise stale or
   missing, and suggest `/corvint:index`. Do not build the index here. A snapshot written by a
   different Corvint binary is also unused; say so if the names do not let you tell.
3. Workflow: if trusted Corvint hook guidance in this session supplied a root and session key, run
   `corvint --root ROOT dogfood status --session-key KEY` and report the lifecycle, whether it is
   satisfied, and the next actions. Without that guidance, say the session is not enrolled or its
   identity is unavailable. Never invent a key.
4. Recent problems: `corvint observations --limit 20`, summarized as counts of adapter degradations
   by event and code, plus any falsification rate it reports.

Write the report as at most eight short lines, one per item, with plain words for each state. Name
the command that fixes each problem you report.
