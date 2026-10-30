---
name: corvint-openspec
description: Read-only Corvint context before an OpenSpec change is applied and Corvint review after it. Use when the user runs or finishes "openspec apply" / "opsx apply" in a project that has the corvint binary. Advisory only; never blocks, edits or commits.
allowed-tools: Bash(corvint context:*), Bash(corvint affected:*), Bash(corvint review:*), Bash(git merge-base:*), Bash(git rev-parse:*)
license: Apache-2.0
compatibility: Requires the corvint binary on PATH. OpenSpec versions listed in interop/spec-toolchains/versions.json.
metadata:
  author: beamfall
  version: "0.1.1"
---

<!-- SPDX-License-Identifier: Apache-2.0 -->

# Corvint around OpenSpec apply

Two read-only steps around `openspec apply` (STA-V0-009). Neither step edits, stages or commits
anything, runs tests, or decides whether the change is done. Report Corvint's output as printed and
keep every `unknown`, `UNKNOWN`, `NOT_PRODUCED`, refusal and non-zero exit visible. If `corvint` is
missing or a step fails, say so and let the OpenSpec workflow continue.

## Step 1: before apply, read context

Take `<id>` from the change being applied and a one-line summary from its `proposal.md`.

```bash
corvint context --task "<change summary>" --subject openspec/changes/<id>/proposal.md
```

Report the packet `state`, each result path with its evidence line, and every unknown. Suggest
reading those files before implementing.

## Step 2: after apply, read review advice

Resolve the base as a full commit ID, for example `git merge-base HEAD <default branch>`, then run:

```bash
corvint affected --base "<full base commit>"
corvint review --base "<full base commit>"
```

`corvint affected` covers the working diff, including uncommitted changes. `corvint review` reads a
clean HEAD only; with uncommitted changes it refuses. Report that refusal as printed; never commit
to make it run. Report the selected tests with witnesses, the excluded summary, the review advice
and every unknown. List tests the user may run; do not run them.

For other Corvint capabilities, `corvint help` lists every command, and the catalog at
https://github.com/beamfall/corvint/blob/main/docs/CAPABILITIES.md gives each command, binary and MCP tool with
its maturity and refusals.
