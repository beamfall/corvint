# 2026-10-22 lane corpus-gh-1022w: claim text search (V1-2435), stat-stamped corpus MCP freshness (V1-2434), status and intent listings (V1-2436)

## Problem

- V1-2435 (GitHub #1555): claim text was not indexed and `docs_claims` took only a path, so an
  evaluation case with `expect.claims` could reach claim recall only by path.
- V1-2434 (GitHub #1554): every warm corpus MCP call ran `git rev-parse` plus `git status
  --porcelain=v1 -z --untracked-files=all` in every observed checkout and re-verified every pinned
  revision. On four additional repositories (13,806 / 3,111 / 450 / 274 tracked files, 9,170 pinned
  inputs) the reporter measured median `freshness_ms` 1,042, mean `verify_ms` 339 and p95 total
  1,945 ms. The DCP-V1-281 fixture had no checkout above 10,000 tracked files.
- V1-2436 (GitHub #1556): supplied status claims could only be counted (`docs_coverage`) or read one
  unit at a time, and `docs_intent` required an `id`, so no operation listed intent comparisons.

## Decisions

All requirements below are accepted under the owner's standing auto-accept (2026-10-11).

- DCP-V1-313: the query index keeps, per claim, the evidence terms of its ID and text under the
  existing DCP-V1-193 bounds. `claims` takes exactly one of a path, paths batch or `query`; a query
  ranks claims by distinct query terms held, an exact trimmed case-insensitive text match first,
  ties by claim ID. The receipt is an ordinary claims receipt, so eval scoring is unchanged.
- DCP-V1-314..316: a checkout stamp is a parallel lstat walk (name, mode, size, inode, ctime, mtime)
  over each checkout's worktree and repository directory, captured before the pin check and the
  observation it pairs with. An equal later stamp reuses the memoized overlay with no Git process
  and skips pin re-verification. The stamp fails closed: it is unavailable (always re-observes)
  for gitfile worktrees, nested repositories, alternates/commondir, config includes or worktree
  settings, more than 250,000 entries, any entry changed within 2 s of capture, and every platform
  other than Darwin and Linux. LFS storage is not walked. Loose objects are walked, so an object
  truncated or rewritten in place changes the stamp and forces pin re-verification (review P1).
  The repository config is read through a bounded reader that opens only a regular file, with
  O_NOFOLLOW and O_NONBLOCK, and refuses one over 1 MiB before reading; a FIFO, symlink or
  oversized config makes the stamp unavailable (review P1).
- DCP-V1-317: the DCP-V1-281 fixture now includes a 12,000-file additional checkout.
- DCP-V1-318: `claims` takes a `state` with optional `qualifier` and `run` as a third exclusive
  form, instead of a new tool. Without a qualifier only unqualified claims are listed, matching the
  DCP-V1-301 coverage rows. A run keeps claims whose subject is an end of a `migration_mapping` of
  that run.
- DCP-V1-319: `intent` without an id lists every intent comparison subject, paged.
- The companion `corpusindex.ParseRequest` refuses an explicitly empty `state`, `qualifier` or
  `run` member, as the CLI and MCP do, instead of treating it as absent (review P2). An empty
  `id` stays absent there, because `doccorpus.Request` always encodes `id`, `path` and `query`
  and the parity encoder sends them empty; the CLI and MCP still refuse an explicit empty id.
- `docs/CAPABILITIES.md` rows for `docs_claims` and `docs_intent` describe the new forms; no tool
  or command was added.

## Evidence

- Fail-before: with the stamp forced unavailable, `TestDCPV1315WarmCallsSkipGitWhileStampUnchanged`
  failed ("observed 4 verified 4"); with the status and intent branches disabled,
  `TestDCPV1318StatusClaimListing` and `TestDCPV1319IntentListing` returned no results; before
  DCP-V1-313, `docs_claims` refused `query` (invalid-arguments) so the eval case could not run.
- Review fixes, fail-before against the unfixed code: the loose-object truncation case left the
  stamp equal; `TestDCPV1316WarmCallRefusesCorruptedPinnedObject` returned evidence with 0 pin
  verifications; the symlinked-config case produced a stamp; `TestDCPV1314StampRefusesFIFOConfig`
  blocked; the companion `{"operation":"claims","state":"unknown","qualifier":""}` parsed. The
  oversized-config case already failed closed, but only after reading the whole file.
- Scale (old code, synthetic fixture with a 12,000-file checkout): warm `freshness_ms` 36-61 ms,
  p95 total 200 ms. The old code already met the bound at fixture size (the reporter's cost comes
  from real Git object stores and status work), so the Git-call counter in DCP-V1-315 is the
  regression proof. New code ([4000,12000,1000]), three runs: warm `freshness_ms` 0 on 19 of 20
  calls each, p95 total 185, 193 and 206 ms.
- On the lane checkout (12,895 tracked files) a stamp took 26-58 ms against 85-124 ms for one
  `contextindex.Observe`.
- Focused packages: `./internal/mcp/corpusbridge/` (full), `./internal/doccorpus/` (full),
  `./internal/corpusindex/`, `./internal/corpusserve/`, `./cmd/corvint/ -run 'Corpus|DCPV1|Docs'`.
  `go vet` on touched packages, `make windows-vet`, `gofmt -l` clean; capabilities-check passed.

## NOT_RUN

- `make gate` (owner standing preference: focused tests only).
- The reporter's four-repository corpus: not available to this lane; the acceptance fixture is
  the synthetic DCP-V1-281/317 fixture.
- Network filesystems and coarse (1 s) timestamp filesystems: the 2 s racy window is the only
  guard; not measured.

## Rollback

Revert the lane's merge commit. The stamp is runtime-only memo state (no artifact, wire or
persisted change); removing it restores per-call Git observation and pin verification. The new
request fields are optional and omitted when empty, so existing receipts and requests keep their
bytes.
