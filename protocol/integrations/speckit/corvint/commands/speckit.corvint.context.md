---
description: "Report the Corvint path brief for the current feature's spec.md and plan.md (read-only, advisory)"
---

<!-- SPDX-License-Identifier: Apache-2.0 -->

# Corvint context for the current feature

Read Corvint's path brief for this feature's `spec.md` and `plan.md` and report it. This
command is advisory and read-only (STA-V0-008). It never blocks the workflow, never edits a file and
never commits.

## User Input

```text
$ARGUMENTS
```

Treat any user input as more repository paths to brief, one `--path` each. When it is empty, brief
only the feature paths.

## Rules

- Run only the commands below. Do not run any other `corvint` subcommand, and do not edit, stage or
  commit anything.
- Report Corvint's output as it is. Keep every `unknown`, `UNKNOWN`, `NOT_PRODUCED`, refusal and
  non-zero exit visible; never restate one as success or fill it in.
- If `corvint` is missing or a command fails, say so and continue the calling workflow. This command
  never stops `/speckit.implement`.

## Steps

1. Resolve the feature paths without side effects:

   ```bash
   .specify/scripts/bash/check-prerequisites.sh --json --paths-only
   ```

   On PowerShell use `.specify/scripts/powershell/check-prerequisites.ps1 -Json -PathsOnly`. Take
   `REPO_ROOT`, `FEATURE_SPEC` and `IMPL_PLAN` from the JSON. Make both paths relative to
   `REPO_ROOT`. Skip a path that does not exist and say so.

2. Run Corvint context once, from `REPO_ROOT`, with one `--path` for each existing path:

   ```bash
   corvint --root "$REPO_ROOT" context --path "$FEATURE_SPEC_RELATIVE" --path "$IMPL_PLAN_RELATIVE"
   ```

   `corvint context` writes nothing. A path Corvint does not track is reported as Corvint reports it.

3. Report, per path: each section with its `state`, its items and every missing source or refusal
   verbatim. Suggest reading the listed instructions, tests and requirements before implementing. Do
   not act on them.

For other Corvint capabilities, `corvint help` lists every command, and the catalog at
https://github.com/beamfall/corvint/blob/main/docs/CAPABILITIES.md gives each command, binary and
MCP tool with its maturity and refusals.
