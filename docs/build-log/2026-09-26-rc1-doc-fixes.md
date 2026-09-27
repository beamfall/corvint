## 2026-09-26 V1-0376, V1-0377, V1-0378: rc1 documentation audit fixes

V1-0376 (docs/INSTALL.md, README.md): fixed both stale
`#try-it-on-this-repository` anchors to `#sixty-seconds-on-this-repository`;
replaced hardcoded `0.5.0a3` with `<version>` placeholders plus a release
inventory link, keeping exact-version checksum/`Expect` steps; corrected
README's agent-facing server count (three to four) and marked
`corvint-corpus-mcp` source-only/experimental in both files. No
documentation link checker exists in Makefile or script/.

V1-0377 (README.md): qualified "Read commands change nothing" with the two
bounded AGENTS.md invariant-4 ledger exceptions (self-observations.jsonl;
unplanned-reads.jsonl only while unplanned-reads.enabled exists); help.go,
docs/AUTOMATION.md, and unplanned-read-events-v0.md already stated this.
Repinned the FRONTIER-DECISION-BRIEF-2026-08-29.md citation the edit shifted
(README.md:188-198 -> 192-202, same content and anchor).

V1-0378 (docs/README.md, internal/console/{server,render}.go): recast the
six docs/agent-memory pages and the console's backlog pane (renamed Backlog
history) as historical redirects, pointing active work at the task store
(.taskman/, corvint-tasks); kept the LAC-V0-020 untrusted-text-renders-inert
guarantee. AGENTS.md, agent-memory/*.md, and local-admin-console-v0.md
already stated this.
Checks: check-line-citations.sh clean; internal/console tests pass.
