# Tasks: `rc4-candidate`

Accepted by the owner on 2026-11-18 with RCF-D1 to RCF-D3 answered (see `design.md`).

## 1. Spec and ticket

- [x] 1.1 File the tracking ticket (milestone `v1-0`) and link it in `proposal.md`.
  Requirements: all. Proof: `corvint-tasks ticket show` output.
- [x] 1.2 Write `docs/specs/release-candidate-4-v1.md` from the accepted spec delta and index it.
  Requirements: RCF-V1-001 to RCF-V1-012. Proof: `make spec-requirements-check`.

## 2. Docs search qualification

- [x] 2.1 Commit the frozen query set and its digest; pin the three repositories.
  Requirements: RCF-V1-007. Proof: committed file and digest.
- [x] 2.2 Add the qualification runner: CLI and MCP per query, passage verification, canonical
  record comparison, bounds and `git status` checks, Beamfall digest-only output.
  Requirements: RCF-V1-008. Proof: focused tests with an injected unverified passage and an
  injected CLI/MCP mismatch, each failing.
- [ ] 2.3 Run the qualification and DCS-V1-404 twice on one commit; record the build-log entry.
  Requirements: RCF-V1-008, RCF-V1-009. Proof: build-log entry with digests and counts.
- [ ] 2.4 Add the rerun trigger to the affected plan for ranking, corpus and promoted surfaces.
  Requirements: RCF-V1-010. Proof: `corvint affected` on a ranking diff selects the runner.

## 3. Promotion

- [ ] 3.1 Contract tests for the three tools' schemas and the corpus format (RCF-D2).
  Requirements: RCF-V1-011. Proof: tests fail on a renamed field.
- [ ] 3.2 One commit: spec delivery status, `README.md`, PROD-DOCS-009 state, compatibility
  table and generated `docs/COMPATIBILITY.md`.
  Requirements: RCF-V1-006, RCF-V1-011. Proof: `make compatibility-check`, `make openspec-check`.
- [ ] 3.3 Revert drill on a scratch branch.
  Requirements: RCF-V1-012. Proof: checks pass on the reverted tree.

## 4. Candidate

- [ ] 4.1 Apply RCF-D1 (keep `corvint-mcp`): mark MTT-V1-010 superseded and keep V1-5473's
  three archives. Requirements: RCF-V1-003. Proof: spec edit and V1-5473 archive list.
- [ ] 4.2 Write the readiness record for the candidate commit.
  Requirements: RCF-V1-003. Proof: every row `PASS` or `WAIVED`.
- [ ] 4.3 Owner tags `v1.0.0-rc.4` and approves publish; record the draw seed.
  Requirements: RCF-V1-001, RCF-V1-002. Proof: release URL and seed record.
- [ ] 4.4 Release notes with promotions, previews and the V1-0019 result.
  Requirements: RCF-V1-004, RCF-V1-005. Proof: `docs/RELEASE-NOTES.md` section.

## 5. Review

- [ ] 5.1 One independent review of the runner and promotion commit; fix P0/P1 findings.
  Requirements: all. Proof: review record.
