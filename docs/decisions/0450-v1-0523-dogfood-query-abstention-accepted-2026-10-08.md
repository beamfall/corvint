# Decision 0450 — dogfood query abstention (QAT-V0) accepted

Date: 2026-10-08. Status: accepted by the owner. Authority: owner approval in chat, 2026-10-08
("approve all, and you can self approve these tickets"), covering native ticket V1-0523.

## Context

V1-0523 asked for a contract-consistent closeout path when the unchanged original dogfood task
selects authority-start on a clean tree that already holds a retained local outcome trace. The
proposed answer is `docs/specs/dogfood-query-abstention-v0.md` (`QAT-V0-001..005`), built in
`internal/dogfoodflow` with focused hostile tests
(`docs/build-log/2026-09-30-query-trace-abstention-closeout.md`). AGENTS.md invariant 8 keeps
acceptance human-owned.

## Decision

The owner accepts `QAT-V0-001..005` as written. The spec intent changes from `proposed` to
`accepted (decision 0450; V1-0523)` in the header, digest, `docs/specs/README.md` and `INDEX.json`.

## Limits

This decision settles intent only. Delivery stays `experimental`: the spec's own evidence is
focused synthetic tests, while independent implementation review and live final original-task CLI
qualification are recorded as not yet produced. No query support, query capability or release
promotion is implied, and V1-0523 is not completed by this decision.

## Rollback

Revert this decision and restore `proposed` in the spec header, digest, README row and INDEX
entry, then regenerate `docs/specs/REQUIREMENTS.tsv`.
