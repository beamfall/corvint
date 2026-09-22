# Decision 0291 — Explicit foreground documentation watch

Date: 2026-09-13
Status: proposed
Owner acceptance: pending

The owner-authorized release implementation exposed a gap: `docs maintain --apply` ran one
compile/apply cycle, while PUB-V0-009 asks for automatic maintenance during an explicitly
enabled local session. This proposal records the bounded experimental implementation for
owner review; it does not assert formal acceptance.

Add explicit `--watch` foreground maintenance, retaining one-shot and preview compatibility.
SDD-V0-007..010 and their proposed watch extension own the concrete profile. Reuse the native
compiler, cited-source digest, admitted Git reader and atomic page replacement. Preserve
expected whole-page identity across idle ticks and stop on human edits. Bind previews to
commit/tree and check HEAD before/after writes; unavoidable post-write drift is visibly
superseded/incomplete, never justification to roll back human bytes.

Keep a one-second poll and explicit write/time/cycle/page/output bounds. Reuse existing
contained Git child cleanup; no daemon, network, new Git parser, MCP writer or runtime.
A real source-built watch→source commit→automatic apply→MCP server-discover/tools-list/draft/exact consume
proof passed. Installed-artifact qualification is NOT_RUN in this leaf and remains a
separate release gate. Source binding is not human usefulness or accepted authority.

Rollback removes `--watch` use; default one-shot commands remain available and no durable
watcher state or service remains. Formal owner acceptance is pending.
