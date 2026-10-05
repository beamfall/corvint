## 2026-10-05 V1-0788: prior generations record their stage and pool member

Human-owned intent: owner request [issue 586](https://github.com/beamfall/corvint/issues/586) part 1,
ticket V1-0788. A retried attempt kept only `generation`, `quiescence` and `provedSeq` per ended
generation, so which member and stage ran a failed generation was recoverable only from receipts,
and neither claim results nor the dispatcher `claim` event named the member.

Requirement: `CAL-V0-079` in `docs/specs/corvint-tasks-agent-leases-v0.md` (V1-0788 amendment).
CAL-V0-074..078 were skipped because open lanes (V1-0755, V1-0756, V1-0780, V1-0781) already use
them; the first ID free on main and every known open branch was taken.

### Change

- Wire: a `taskman-attempt/0` `priorGenerations[]` entry gains optional `stage`, `poolId` and
  `memberId`, present together or absent together, each nullable, with `poolId`/`memberId` null
  together and `stage` limited to the stage roles. The closed decoder refuses partial sets,
  unpaired nulls, unknown stages and other keys. An entry without them decodes with no history and
  re-encodes byte-identical; the test pins the N-1 bytes produced by origin/main `ac818777`.
- Writer: CLAIM/CLAIM_NEXT admission of the next generation copies the ended generation's `stage`
  and `poolAllocation` into the new entry, for `external-agent` attempts only. A supervised
  generation spans stages and releases its allocation per stage, so it records nothing rather than
  a guess.
- Reads: `attempt show` adds `history` (`RECORDED` or `NOT_OBSERVED`) to each entry and renders a
  legacy entry's three keys as `NOT_OBSERVED`. The discriminator exists because member labels are
  free-form. Criterion captures (observations off) are unchanged. Claim results always carry
  `poolAllocation` (`null` without one); the dispatcher `claim` event detail carries `pool` and
  `member` (empty without one) and the message names `pool/member`.

### Decisions and limits

- Recovering a legacy entry's member from audited receipt POST state was optional and is not done.
- Downgrade: older closed decoders refuse an attempt whose prior entries carry the new keys
  (`MALFORMED`). Reverting is safe only before such a record exists; the spec rollback names the
  check. No rewrite tool is provided.
- The claim event reports the member current at the dispatcher's observation.

### Evidence

- Focused tests: `TestCALV0079_*` in `internal/tasks/{snapshot,transaction,store,cli,dispatch}`,
  plus `go test ./internal/tasks/...` (see the handoff for the result), `go vet` of touched
  packages, gofmt, `GOOS=windows` cross-build, the CI doc gates and use-case receipt checks.
- NOT_RUN: `make gate`, the repository-wide suite, interop, live dispatcher qualification and the
  dogfood bind/seal (owned by the coordinator after review).
