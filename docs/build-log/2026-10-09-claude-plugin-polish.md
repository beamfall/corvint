# Claude Code plugin polish and 2.1.293 requalification (V1-1069..V1-1072)

Date: 2026-10-09

## Decision

An audit of the Claude Code integration, covering both the terminal and the desktop Code tab (which share one plugin),
found four gaps. These were addressed together in plugin version 0.3.0, then 0.3.1 after review, under `AHI-051`:

- V1-1069: the context skills told the model "never invoke the legacy `corvint` executable" while also
  telling it to run `corvint`. Both host skills now name the installed `corvint` on `PATH` and drop
  the contradiction.
- V1-1070: the plugin and marketplace manifests now carry a display name, author, homepage,
  repository, license and keywords. The description no longer says "preview", and each hook has a
  plain-language `statusMessage`. `SessionEnd` previously had none.
- V1-1071: added `/corvint:impact`, `/corvint:affected` and `/corvint:review`, which the model may also
  invoke. Added `/corvint:status` and `/corvint:index`, which only the user can invoke. Each skill's
  `allowed-tools` is limited to its own `corvint` verb plus read-only `git` and `ls`.
- V1-1072: requalified the Claude Code tuple on host 2.1.293 with adapter 0.3.1.

Per-prompt `systemMessage` summaries were considered and rejected: decision 0161 forbids routine
user-visible hook messages. `/corvint:status` is the on-demand view instead, and persistent UI stays
with V1-1054.

## Evidence

- `claude plugin validate` passes for both the plugin and the marketplace (Claude Code 2.1.293).
- `script/check-host-package-versions.sh` exits 0.
- `TestHostAdapterJavaScriptHosts` passes, including the new `AHI-051` test. A mutation check
  confirmed the test fails when a `statusMessage` contains "frontier".
- `host-lifecycle-qualification-v1`, Claude Code 2.1.293 / adapter 0.3.1, darwin/arm64:
  - With the published v1.0.0-rc.2 binary (build 360) and N-1 rc.1 (build 163): PASS 9/9. The report
    is `conformance/host-lifecycle-v1/results/1.0.0-rc.2-darwin-arm64/claude-code.txt`, and the
    published matrix row now cites it.
  - A preceding run with a branch build (`Corvint 1.0.0-rc.2 (build 0)`) and N-1 local build 404 also
    passed 9/9. That run is diagnostic only and its report was not retained.
- A live compaction cycle is `NOT_RUN`: the nested `claude -p` session failed with "OAuth session
  expired". The shipped `compatibility.json` therefore keeps `maximumTestedVersion` and the
  compaction `verifiedHostVersion` at 2.1.267, and the installed-lifecycle evidence lives only in the
  matrix.

## Independent review

A Codex review (gpt-6-astra, read-only) of `1120e565..78812d33` made three findings, all confirmed:

- A shipped `compatibility.json` edit landed after the 0.3.0 bump, so `script/check-host-package-versions.sh`
  failed. Fixed by moving the package to 0.3.1 and requalifying.
- The status skill's command `corvint --root ROOT dogfood status` did not match its
  `Bash(corvint dogfood status:*)` pre-approval. The skill now runs it from inside the root.
- The README called every command read-only, although `/corvint:context` keeps its enrolled-workflow
  writes. The README and `AHI-051` now name which skills write.

## Found in passing

- V1-1078: after a core binary update changes the index engine key, every hook degrades with
  `dogfood-event-index-snapshot-stale` until someone runs `corvint index --if-stale` by hand.
