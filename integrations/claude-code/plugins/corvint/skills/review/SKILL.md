---
name: review
description: Advisory Corvint review of the commits since a base, showing affected areas and overlap with other local branches. Use when preparing or reviewing a branch for merge.
argument-hint: "[BASE_COMMIT]"
allowed-tools: Bash(corvint review:*), Bash(git rev-parse:*), Bash(git merge-base:*), Bash(git status:*)
---

# Corvint review

Run `corvint review --base FULL_COMMIT_ID` on a clean `HEAD`. Use the commit in `$ARGUMENTS`, or by
default the merge base of `HEAD` and `origin/main`, resolved to a full commit ID. If the worktree is
dirty, say that the review covers only committed changes and ask whether to continue.

Present the advisory affected areas and any overlap with other local branches, each with its
evidence. State that the output is inferred guidance: change evidence, test obligations and review
by a person remain open. Show missing sources and index gaps as unknowns.
