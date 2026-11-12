# Lane dmf: independent review findings on docs MCP hardening (V1-5409)

The independent review of lane dmf (`docs/build-log/2026-11-11-docs-mcp-hardening.md`, head
`0e1ff64c2`) raised two P2 and four P3 findings against `docs/specs/docs-mcp-hardening-v1.md`. Each is
fixed with a test that fails on `0e1ff64c2`, except P3-4, which is a specification record.

## Findings and dispositions

- P2-1, DMF-V1-011 stale reload: a call that read rebuilt bytes and then waited for the corpus load
  slot could install them over a newer artifact another call had verified meanwhile. The reload now
  uses the caller's bytes only while the artifact file keeps the stamp captured before that read and
  otherwise reads the file again under the slot. `TestDMFV1011StaleReloadNeverReplacesNewerArtifact`
  orders the race with the load slot held by the test, then runs eight parallel callers across an
  A to B swap under `-race`.
- P3-3, DMF-V1-011 header used for arguments: the tool-support and `offset` checks now use the header
  in force after the artifact check. `TestDMFV1011ArgumentsJudgedByReloadedHeader` rebuilds a `/1`
  artifact as `/2` and calls with `offset` and `docs_inventory`.
- P2-2, DMF-V1-002 exit status: a flows check that errors exits 2 unless the error is a flow-docs
  refusal (`flowdocs.Refused`, now a typed error with unchanged text), which alone stays
  `NOT_CHECKED`. The report status is `NOT_CHECKED` when any part is and none is `STALE`. Exit 0 is
  kept for that case because the requirement says "0 otherwise" and its nothing-to-check scenario
  requires exit 0. `TestDMFV1002DocsCheckFlowsFailureIsNotNotChecked`.
- P3-1, DMF-V1-004 open fence: a fence still open at the end of a document is an `unclosed-fence`
  failure naming the document and the opening line. `TestDMFV1004UnclosedFenceFails`. This
  repository has none: 942 failures, as in round 1.
- P3-2, DMF-V1-005 flows worktree: of the two options, the smaller change was chosen. The
  `docs check` flows part now carries a `worktree` member observing each manifest's scope and
  recorded output directory, instead of moving the comparison to the committed output at HEAD.
  `TestDMFV1005DocsCheckFlowsWorktree`. The spec no longer claims that no check reads worktree bytes.
- P3-4, DMF-V1-010 busy object: recorded as a server-level object outside the tool-group profiles,
  with exactly `code`, `message`, `retryable` and `next`. The server answers it before routing, so it
  cannot name a group's profile, and giving it one would mislabel frozen code-tool errors.
  `TestDMFV1010ServerBusy` now pins the members. That assertion also holds on `0e1ff64c2`, because
  behaviour did not change.
