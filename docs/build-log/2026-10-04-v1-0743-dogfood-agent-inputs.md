## 2026-10-04 V1-0743: named refusals for empty plans, over-bound inputs and malformed receipts

Human-owned intent: the owner asked to start V1-0743. The ticket covers three defects in
`dogfood change`:

- An empty `DOGFOOD_CITATIONS` file passed `cem-cite` as a zero-citation no-op. Only `cem-status`
  caught the uncited hunks later.
- `DOGFOOD_INTENTS_FILE`, `DOGFOOD_VERIFY_FILE` and the agent pre-change receipts were read whole
  before any bound was checked.
- A malformed agent receipt was reported as `agent-receipt-tree-unknown`, the same note as a
  well-formed receipt with no tree.

### Change

All three fixes are in `internal/dogfoodflow/change.go` and specified as the proposed `DCW-V0-032`.

- **Empty citation plan.**
  - An empty plan on a map that still records an `unknown` hunk outside the `DCW-V0-019` bootstrap
    omission now reports `cem-cite` NOT_PRODUCED `empty-citation-plan`. It runs no `cem cite` and
    has its own `fix:` line, like `empty-ocm-link-plan` under `DCW-V0-018`.
  - The 256-row split-plan exemption does not cover an empty plan, so an empty plan on a map with
    more than 256 unknown hunks is refused too.
- **Bounded reads.** Each input is read through at most its bound plus one sentinel byte:
  - the intents file at 8208 bytes; over that, `ocm-aggregate` is NOT_PRODUCED
    `intent-manifest-over-bound`, and other invalid manifests stay `missing-intent-scope`;
  - the verify file at 64 KiB; over that, `local-outcome` is NOT_PRODUCED
    `verify-file-over-bound`;
  - each agent receipt at 4 MiB.
- **Receipt notes.** A receipt over the bound prints `agent-receipt-over-bound`, one that cannot
  be opened `agent-receipt-unreadable`, and one that does not decode as a receipt object
  `agent-receipt-malformed`. `agent-receipt-tree-unknown` stays for a receipt that decodes but names
  no `context.revision`. The notes
  remain non-blocking under `DCW-V0-031`.

### Review findings applied

- The self-observation ledger (`internal/observations` `validDogfoodReason`) refused the three new
  step reasons, so `dogfood-observe` dropped their rows silently. They are now admitted, along with
  the pre-existing `verify-file-unavailable`, which was refused the same way.
- A plan refused because the map cannot be read stays `citation-plan-map-mismatch`; before, an empty
  plan in that case reported `empty-citation-plan` with an untrue fix line.
- The `docs/DOGFOOD.md` failure-class table names the empty-plan refusal, and `DCW-V0-032` defines
  `agent-receipt-malformed` as the code's "does not decode as a receipt object".
- `DCW-V0-032` binds `dogfood change` only: `script/dogfood-bind-range.sh` keeps its empty-plan no-op.

### Narrowed acceptance criterion 1

- The ticket asks that an empty plan always refuse. The first draft did that, and
  `TestDogfoodFinishRunsFromBinaryInForeignRepository` failed with `dogfood-coordination-failed`.
- `dogfood finish` (`internal/localcompletion/finish.go` `coordinate`) deliberately passes an
  empty plan once strict CEM status has validated the bound citations. `local-completion-policy-v0.md`
  allows exactly that.
- The rule therefore refuses only when the map still owes a hunk, which is the silent case the
  ticket describes. An empty plan on a map that owes no hunk stays the `DCW-V0-019` zero-citation
  no-op. Making every empty plan refuse would need an LCP change first.
- The `empty` citation case of `script/dogfood-change_test.sh` asserted the old pass on a map with
  an unknown hunk. It now expects the refusal and its `fix:` line.

### Evidence and limits

- Tests:
  - New: `TestChangeRefusesEmptyPlanAndOverBoundInputs`, which covers the three refusals, their fix
    lines, that no cite runs, and that an empty plan on a map that owes no hunk is still PRODUCED.
  - Extended: a malformed and over-bound case in `TestChangeNotesAbsentOrStaleAgentReceipts`.
  - Before the fix, both failed on the missing codes; with it, both pass.
- `go test ./internal/dogfoodflow ./internal/specindex`, `go test ./cmd/corvint -run
  'Dogfood|TraceGitignoreFinish'` and `script/dogfood-change_test.sh` pass.
- `go test ./internal/localcompletion` fails `TestAggregateExclusiveLateCancelRemovesStage`,
  `TestAggregateExclusiveLateHoldRetainsStage` and `TestAggregateQualificationActualIOFailures`
  with `local-state-symlink`. They fail identically at base `cd70ba0a` on this host, so they are
  unrelated to this change.
- `corvint affected` selected about 170 units, almost all through the conservative
  `UNBOUNDED_READER` witness. Only the units above were run; the rest are `NOT_RUN`.
- `DCW-V0-032` is proposed, not accepted. It amends the accepted `DCW-V0-019` empty-plan sentence,
  so it needs owner acceptance.
