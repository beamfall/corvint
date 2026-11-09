package main

// worktreeOverlayFlag is the explicit opt-in on `corvint context` that reads
// tracked dirty paths from the worktree (DIRTY-CACHE-940, V1-0418). Absent, the
// verb is byte-for-byte unchanged.
const worktreeOverlayFlag = "--worktree-overlay"
