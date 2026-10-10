---
name: index
description: Build or refresh Corvint's index snapshot for the current commit so hooks and context queries answer quickly.
disable-model-invocation: true
allowed-tools: Bash(corvint index:*)
---

# Corvint index

Run `corvint index --if-stale` in the repository root and report whether the snapshot was already
fresh or was rebuilt, with how long it took. The snapshot lives under the Git common directory, is
shared by every linked worktree, and changes no repository file. If the command fails, show its
error and leave the hooks running on their bounded fallback.
