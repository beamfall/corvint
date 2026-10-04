## 2026-10-04 V1-0712 spike: output-class inventory of host- and model-produced input

Human-owned intent: the owner asked to start V1-0712. The ticket was filed from the claude-mem
finding that a single catch-all path silently accepted every non-conforming model reply. It asks for:

- an inventory of every path where host- or model-produced output enters Corvint evidence,
  evaluation or learning, with the file and function for each;
- whether each path has an explicit output-class policy (accept, abstain, refuse), with a ticket
  for each gap and a cross-reference to V1-0369;
- whether every buffer or history on those paths has a byte or token bound.

### Method and limits

- Source is `origin/main` a2615609. Three read-only surveys covered: host adapters and hooks;
  learning, evaluation and benchmark harnesses; and documentation and evidence admission.
- Every gap claim filed as a ticket was re-read directly at a2615609, and the branch was moved to 431d6b15 without changes to any cited file. None of the surveyed files
  changed between the survey base eeef6276 and a2615609.
- Claims not re-read are marked INFERRED. No runtime reproduction was run, so every gap is
  `runtime NOT_RUN`.
- Terms in the tables:
  - "Explicit" means the path maps each unrecognised or malformed input to a named refusal,
    abstention or degradation code.
  - "Silent" means some class is accepted or dropped with no code and no count.
- Excluded:
  - Corvint's own subprocess output re-read by Corvint (dogfood step outputs, the report, gate
    output the Tasks service runs itself), unless it is first written by an agent.
  - Committed repository content that only becomes authority through project-owned path and status
    conventions (`contextindex` `documentAuthority`, `internal/contextindex/impact.go:497`). Review
    of the commit is the boundary there, not output classification.

### Inventory

Host adapters and hooks:

| # | Path (file, function) | Producer | Policy | Bounds |
|---|---|---|---|---|
| H1 | `cmd/corvint/host_adapter.go` `decodeAdapterJSON`, `normalizeAdapterInput` (Claude Code stdin) | host | Explicit for malformed JSON and refused fields. Unknown fields are ignored. Post-tool turns an out-of-project path into empty `changedPaths` silently, while file-change refuses it (`host_adapter.go:533-551`). Gap: V1-0746 | 8 MiB (`adapterInputLimit`, `io.LimitReader`) |
| H2 | `host_adapter.go` `runCodexAdapter` | host | Closed event map. An unknown `hook_event_name` returns `{}` silently (`host_adapter.go:286`). Gap: V1-0746 | 8 MiB |
| H3 | `host_adapter_compaction.go` `runClaudePostCompact` (`compact_summary`) | host or model | Explicit. A missing pin is `compaction-pin-not-preserved`. An empty summary abstains by documented design (cached replacement compaction) | 8 MiB |
| H4 | `internal/gokernel/harness.go` `HandleEventContext`; `cmd/corvint/native_hook.go` | host | Explicit refusal on closed fields and events. The observation write fails open (`_ = observations.Append`) | 131072 B (`MaxInputBytes`) |
| H5 | `integrations/gemini-cli/hooks/corvint-hook.mjs` `readHookInput`, event mapping | host | Explicit on main for the input bound. A JSON parse failure is generic until #531 lands `malformed-hook-json`. A tool error skips the after-tool event (abstain: no file changed) | `INPUT_LIMIT` |
| H6 | `integrations/opencode/src/index.js` `rememberPath` | host | Silent. A path outside the repository is ignored, and paths past `MAX_TRACKED_PATHS` are dropped with no flag (`index.js:212-219`). Gap: V1-0746 | 256 paths |
| H7 | `integrations/pi*` adapter | host | Explicit refusal on closed fields | adapter limits |
| H8 | `cmd/corvint/pi_tools.go` `runPiTool` `record` (`corvint_record_outcome`) | model | Closed outcome set and field set, so explicit. The stored `trace.Record` has no producer field. Gap: V1-0745 | task ≤2000 runes; trace caps below |
| H9 | `pi_tools.go` `expand` | model | Explicit (digest-checked source view) | `sourcePacketLimit` |

Learning ledgers and learning:

| # | Path (file, function) | Producer | Policy | Bounds |
|---|---|---|---|---|
| L1 | `internal/unplannedread/unplannedread.go` `HookPostToolSession` (writer) | host and model tool input | Explicit. Closed tool set, literal-operand Bash rules, `projectpath` containment, closed error reasons | row 2048 B, file 128 KiB (`unplannedread.go:33-34`) |
| L2 | `unplannedread.go` `Read`, `foldRow` (reader) | ledger file, writable by any owner process | Silent. Non-JSON rows are dropped uncounted, `{}` and escaping paths count as reads, and an over-cap file is cut with no flag (`unplannedread.go:773-833`). Gap: V1-0740 | `io.LimitReader(maxFileBytes+1)` |
| L3 | `internal/observations/observations.go` `Append`, `validateWriterContract` (writer) | host hook paths | Explicit. Closed kinds, codes, hosts and events; `validObservationPath`; `observation-prohibited-content` | row 2048 B, file 128 KiB |
| L4 | `observations.go` `Read`, `readRow` (reader) | ledger file | Silent. Writer checks are re-run only for proof and adapter-degradation rows, so an escaping `touchedPaths` entry becomes a Miss. Rejected rows are uncounted. Gap: V1-0740 | 128 KiB (truncation unflagged, INFERRED same pattern as L2) |
| L5 | `internal/slotlearn/slotlearn.go` `ReadLabels`, `Learn`; `cmd/corvint/eval_slot_weights.go` | L2 and L4 | Labels are not re-validated, and `ServingSlot` defaults unknown paths to `definition`. The held-out gate (`internal/evalrepo/slot_weights.go`) still decides admission. Gap: V1-0740 | `MaxLabelPaths` 256 (reported as `Truncated`), `MaxProposals` 4, admitted file 4096 B strict |
| L6 | `internal/trace/record.go` `validOutcome`; `internal/outcomecal` `Observe` via `cmd/corvint/calibrate.go` | caller-reported outcomes | Explicit closed outcome set. Stance is always Unknown; `MinimumSample` 20 | `MaxTraces` 1000, row 256 KiB, store 16 MiB, 200 paths, 50 verification commands |
| L7 | `internal/evalrepo/fixture.go` `loadTraceFixture` | agent trace fixture | Explicit refusal (strict decode, contamination checks) | Unbounded `os.ReadFile` (`fixture.go:44`). Gap: V1-0747 |
| L8 | `cmd/corvint/prove_observe.go` `readProofDocument` | possibly agent-relayed | Explicit refusal on recount mismatch | 8 MiB |

Evaluation, benchmark and trial harnesses:

| # | Path (file, function) | Producer | Policy | Bounds |
|---|---|---|---|---|
| E1 | `benchmarks/daily-loop-v0/harness.py` `agent_cost` | agent token and outcome rows | Silent. No schema; a prose line crashes; negative or unmatched rows can yield `threshold met`. Semantics are V1-0369; read bound and malformed class are V1-0742 | Unbounded |
| E2 | `benchmarks/daily-loop-v1/harness.py` and `untouched-repository-v1/harness.py` `agent_cost`, `cost_row_problem` | same | Mostly explicit, through closed problem codes that force a no-claim result. A prose line still crashes; invalid rows are copied verbatim into the sealed receipt; fractional or huge tokens are accepted. Gap: V1-0742 | Unbounded |
| E3 | `benchmarks/dogfood-workers/main.go` `parseClaude`, `parseCodex`, `number` | host transcripts | Mostly explicit (NOT_OBSERVED on bad numbers, `unparsedLines` counted). Codex `usage` is summed from any record type (INFERRED). Gap: V1-0742 | 64 MiB via `boundedFile`, text 4096 |
| E4 | `tools/cw-trial/main.go` `codexAgent.run`, `parseCodexEvents`, `runCommand`, `extractClaims`; `tools/cem-trial/main.go` equivalents | model reply and codex events | Partly explicit (closed claim kinds and confidences, ABSENT, MALFORMED, PRESENT). cw collapses ABSENT, MALFORMED and empty into `abstained`. Stream truncation is discarded (`runCommand` never reads `StdoutOverflow`, `main.go:1455-1479`). Negative tokens are accepted. Gap: V1-0742 | Reply: unbounded read, then 1 MiB truncation (`main.go:1332`, `:1056`); events 8 MiB truncated silently |

Documentation, CEM and dogfood admission:

| # | Path (file, function) | Producer | Policy | Bounds |
|---|---|---|---|---|
| D1 | `internal/dogfoodflow/change.go` `validateCitationPlan`, `citeStep` (`DOGFOOD_CITATIONS`) | agent | Explicit for malformed rows. An empty plan returns ok, so cem-cite reports PRODUCED (`change.go:527`). Gap: V1-0743 | 4 MiB `readPrefix`, 256 rows |
| D2 | `change.go` `validateIntentManifest` (`DOGFOOD_INTENTS_FILE`) | agent | Explicit closed format | Read in full before the 16×513 B check (`change.go:326`). Gap: V1-0743 |
| D3 | `change.go` `localOutcome`, `verifyArguments`; `flow.go` `readFile` (`DOGFOOD_OUTCOME`, `DOGFOOD_VERIFY_FILE`) | agent | Explicit closed outcome set; the recorder enforces command limits | Verify file unbounded (`flow.go:299`). Gap: V1-0743 |
| D4 | `change.go` `loadOCMLinkPlan` | agent | Explicit (empty is refused) | 4 MiB, 256 rows |
| D5 | `change.go` `noteAgentReceipt` (prechange receipts) | agent host | Silent. The unmarshal error is ignored, so malformed input reads as `agent-receipt-tree-unknown` (`change.go:1010-1029`). Gap: V1-0743; related V1-0316 | Unbounded |
| D6 | `internal/cem/workflow/commands.go` `Session.Cite` | agent | Explicit closed relation set; the evidence must be a pinned regular blob with a stable span. No provenance class (committed content, see exclusions) | span 1 MiB |
| D7 | `internal/dogfoodocm/aggregate.go` `verifyScope` | Corvint OCM over agent links | Explicit (state, drift and limit codes) | map 1 MiB, 16 scopes, 512 claims, 256 obligations |
| D8 | `internal/doccompiler/admission.go` `AdmitClauses`, `verifyAnchor` | model-proposed clauses | Explicit for malformed clauses and stale anchors. The declared anchor authority is never checked against the anchored path's class, against HDCV0-024 (`admission.go:214-257`). Gap: V1-0741 | 2000 clauses, 10000 anchors, span 8 KiB, text 1 MiB |
| D9 | `internal/doccompiler/draft.go` `ConsumeDraft`; `admittedplan.go` `VerifyAdmittedPlan` | generated drafts and plans | Explicit (byte-equal re-derivation, strict decode, tagged GENERATED) | draft 64 KiB; plan 2000 ops, 128 MiB source |
| D10 | `internal/doccorpus/providers.go` `normalizedRecord`, `checkEvidence`; `compile.go` `loadInput` | external providers | Explicit (verified trust is refused, reviewed is downgraded, generated reviewed is refused) | 64 MiB, 4096 records, but `loadInput` reads the whole blob before checking. Gap: V1-0747 |
| D11 | `internal/opencodequalification/native.go` `readRows`, `record.go` `readObject` | host evidence stream | Unknown fields are accepted (INFERRED impact) | 16 MiB checked after a full read; `readObject` unbounded. Gap: V1-0747 |
| D12 | `internal/localcompletion` lifecycle | agent prechange receipts | Copied verbatim as evidence attachments | 4 MiB |
| D13 | `tools/gate-ledger/main.go` `ledger.lookup` | ledger file, writable by owner processes | Silent. Lenient decode; schema and key alone produce a HIT (`main.go:776-786`). Suspected; question V1-0744 | Unbounded `os.ReadFile` |
| D14 | `internal/appflows` `Decode`, `ReadFile`, `IngestRunEvidence` | caller-reported web-flow evidence | Explicit (secret screen, strict decode, `CALLER_REPORTED` authority, closed outcomes; importers propose with `Proposed: true`) | `MaxBytes`; over bound becomes Incomplete |
| D15 | `internal/tasks` gate results (`store/lease_gate.go`, `transaction/lease_gate.go`) | gate the service runs itself | Explicit; no agent-supplied result is accepted | `MaxGateOutputBytes`, `MaxGateRecordBytes` |
| D16 | `tools/cem-interop-runner`, `tools/beamfall-shadow` | external implementations | Explicit (strict object, closed states). beamfall-shadow receipts carry authority NONE | 64 KiB, 128 KiB; 1 MiB, depth 8 |

Checked and found not to be ingestion points: `tools/native-hook-observer`, the
`local_completion_event` and `dogfood_handoff` writers, host transcripts outside
`benchmarks/dogfood-workers` (no production reader), `prompt_bound` and `source_handoff` (not
evidence), `tools/retrieval-bench`, `tools/compat-trial`, `tools/heading-nav-bench`,
`benchmarks/runner`, `benchmarks/dogfood-measure`, `tools/local-authority`, `interop/cem01-go`,
`internal/docviews` (no non-test caller), and PR or review comment ingestion (none exists;
`internal/postmergehost/audit.go` only refuses those triggers).

### Result

- Most paths have an explicit policy. Model-produced content reaches ranking, authority or a sealed
  receipt only through closed vocabularies, digest checks, or the held-out gate.
- Eight gaps were filed, deduplicated against the queue:

  | Ticket | Kind | Priority | Gap |
  |---|---|---|---|
  | V1-0740 | BUG | P2 | Ledger readers L2/L4 admit rows the writers refuse, and slotlearn L5 turns them into labels |
  | V1-0741 | BUG | P2 | Doc-compiler anchor authority D8 is self-declared, against HDCV0-024 |
  | V1-0742 | BUG | P3 | Harness and trial readers E1–E4: read bounds, malformed classes, silent stream truncation; complements V1-0369 |
  | V1-0743 | BUG | P3 | dogfood change D1/D2/D3/D5: empty plan, unbounded agent inputs, malformed receipt misclassified; related V1-0316 |
  | V1-0744 | SPIKE | P3 | Question: gate-ledger D13 record admission (suspected, not a confirmed contract violation) |
  | V1-0745 | FEATURE | P3 | Idea: trace records H8/L6 carry no producer provenance |
  | V1-0746 | FEATURE | P3 | Idea: host adapter silent abstentions H1, H2, H6 get degradation codes |
  | V1-0747 | BUG | P3 | Read-then-check bounds L7, D10, D11; same class as V1-0691 |

- V1-0369 (OPEN, P2) remains the owner of the daily-loop-v0 savings semantics. V1-0742 covers only
  the read bound and malformed-row classes it does not.

### Bounds

- Byte or count bounds that apply before allocation: H1–H9, L1–L6, L8, E3, D1, D4, D6–D9, D12,
  D14–D16.
- Unbounded, or bounded only after a full read:
  - E1, E2: harness observations, no bound.
  - E4: reply read in full, then truncated.
  - L7: trace fixture.
  - D2, D3, D5: intents, verify file and receipts.
  - D10: provider blob.
  - D11: OpenCode evidence.
  - D13: gate-ledger record.
- Token bounds: none of these paths has a token budget, and none buffers a model conversation
  history. The only token-shaped inputs (E1–E4) are counts, not buffers.
- Silent truncation: three bounded readers truncate without reporting it.
  - L2 and L4 cut an over-cap ledger file with no flag.
  - E4 drops the codex event stream past 8 MiB.
  - These are covered by V1-0740 and V1-0742.

Rollback: documentation and ticket exports only; revert this entry and the eight
`.taskman/tickets/V1-074[0-7].json` exports with the queue serial.
