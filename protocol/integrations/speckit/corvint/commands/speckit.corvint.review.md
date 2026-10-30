---
description: "Report Corvint affected tests and review advice for the working diff (read-only, advisory)"
---

<!-- SPDX-License-Identifier: Apache-2.0 -->

# Corvint review of the working diff

Read Corvint's affected-test plan and review advice for the change and report them. This command is
advisory and read-only (STA-V0-008). It never blocks the workflow, never edits a file and never
commits.

## User Input

```text
$ARGUMENTS
```

If the user input names a base commit, use it as `BASE`. Otherwise use
`git merge-base HEAD <default branch>`, resolved to the full 40-hex commit ID. If no base can be
resolved, say so and report the affected plan without `--base`.

## Rules

- Run only the commands below. Do not run tests, other `corvint` subcommands, or anything that edits,
  stages or commits.
- Report Corvint's output as it is. Keep every `unknown`, `UNKNOWN`, `NOT_PRODUCED`, refusal and
  non-zero exit visible; never restate one as success or fill it in.
- If `corvint` is missing or a command fails, say so and continue the calling workflow. This command
  never stops `/speckit.implement` and never decides whether the change is done.

## Steps

1. Affected tests for the working diff, including uncommitted changes:

   ```bash
   corvint affected --base "$BASE"
   ```

2. Review advice for the committed range:

   ```bash
   corvint review --base "$BASE"
   ```

   `corvint review` reads a clean HEAD only. With uncommitted changes it refuses; report that refusal
   as Corvint printed it and note that the review covers committed work only. Do not commit to make
   it run.

3. Report the selected tests with their witnesses, the excluded summary, the review advice and
   branch-overlap hints, and every unknown verbatim. List the tests the user may choose to run. Do
   not run them.

For other Corvint capabilities, `corvint help` lists every command, and the catalog at
https://github.com/beamfall/corvint/blob/main/docs/CAPABILITIES.md gives each command, binary and
MCP tool with its maturity and refusals.
