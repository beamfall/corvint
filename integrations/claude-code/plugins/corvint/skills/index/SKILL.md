---
name: index
description: Build or refresh Corvint's index snapshot for the current commit so impact and affected answer quickly.
disable-model-invocation: true
allowed-tools: Bash(corvint index:*)
---

# Corvint index

Run `corvint index --if-stale` in the repository root and report whether the snapshot was already
fresh or was rebuilt, with how long it took. The snapshot lives under the Git common directory, is
shared by every linked worktree, and changes no repository file. If the command fails, show its
error.

Other Corvint capabilities: `corvint help` lists every command, and the catalog at
https://github.com/beamfall/corvint/blob/main/docs/CAPABILITIES.md gives each command and binary with
its maturity and refusals.
