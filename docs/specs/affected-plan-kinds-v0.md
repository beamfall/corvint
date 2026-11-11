# Affected Plan Kinds V0

Owner: Russell Lewis
Date: 2026-11-09
Requirement prefix: `APK-V0`
Intent status: proposed (2026-11-09; V1-5251; from the owner's testing product target and OpenSpec change `affected-plan-kinds`)
Delivery status: experimental
Authoritative inputs: `docs/specs/PRODUCT-PLAN-1.0.md` (PROD-TEST-003, PROD-TEST-004),
`docs/specs/affected-plan-v0.md` (the plan this spec adds members to; AFP-V0-930 one planning
tail), `docs/specs/application-flow-understanding-v1.md` (AFU-V1-017 flow impact, AFU-V1-019 to
AFU-V1-024 e2e-safe selection, AFU-V1-140 to AFU-V1-145 derived inventory, AFU-V1-1651 to
AFU-V1-1654 flow-bound consumers), `docs/specs/typescript-javascript-affected-adapter-v0.md`
(TJAA-V0-019 discovery receipt), `AGENTS.md` invariants 1, 2 and 4.

## Agent digest
- Claim: `corvint affected` returns one plan whose selections carry a test kind and whose E2E entries name the flow, variation and changed path that selected them.
- Status: proposed (2026-11-09; V1-5251; from the owner's testing product target and OpenSpec change `affected-plan-kinds`) / experimental
- Exists: `internal/appflows/flowplan.go` (the `.corvint/flows.json` reader and the flow join), the `plan.e2e` and `testKind` members of `affected-plan/0` and `affected-plan/1`, the `e2e` lines of `--format text`; fixture tests `TestAPKV0*` in `internal/appflows/flowplan_test.go` and `cmd/corvint/affected_flowplan_test.go`.
- Blocked on: owner acceptance of the intent; a real adopter repository measured end to end.
- Read next: Requirements; Wire; Failure modes; Rollback.

## User and measurable job

A developer saves a change and asks which tests it requires. Today `corvint affected` answers for
unit-style tests the import graph reaches; E2E tests arrive only through
`--provider FILE --selection-profile e2e-safe`, in a separate `advice.test_selection` member whose
entries carry reason codes, not the flow or variation that selected them; the flow join is the
separate command `corvint flows impact`; and no selection says whether it is a unit, integration
or E2E test. The measurable job: on a repository that declares its flows directory once, the plain
`corvint affected --base FULL_SHA` returns unit, integration and E2E selections in one receipt,
every selection carries its kind, every flow-selected E2E entry names its flow, variation and the
changed path, and missing or stale flow evidence widens the E2E answer or marks it `UNKNOWN`,
never dropping a test.

## Verified current state

Read on the layering integration branch at `741ce4df2` (2026-11-09): `corvint affected` plans
static units through `planAffectedGraph` (`cmd/corvint/affected.go`); `appflows.BindE2EFlows`
(`internal/appflows/e2eflows.go`) binds declared binary consumers to flows through
`.corvint/e2e-flows.json`, which requires at least one consumer; `appflows.SelectE2E`
(`internal/appflows/selection.go`) is the e2e-safe profile over a hand-written provider;
`appflows.FlowImpact` (`internal/appflows/impact.go`) is the `flows impact` join, with the
variation and test-key projection `impactFlow`. A selection's `kind` member is the AZU-V0-001
driver kind (`driver-target`, `ci-job`, `DECLARED_CONSUMER`), so the test kind needs its own
member.

## Definitions

- Flows declaration: the tracked file `.corvint/flows.json`, the closed object
  `{"profile": "corvint-flows/0", "flows": DIR, "kinds": [RULE...], "playwright": {"config": PATH,
  "discovery": PATH}}`. `flows` is required and names the AFU-V1-001 intent directory, a canonical
  repository-relative directory other than `.`. `kinds` is optional, 0 to 256 rules, each
  `{"path": PREFIX, "kind": "unit"|"integration"}` where `PREFIX` is a canonical repository-relative
  path or `.` for the whole repository; a prefix may appear once. `playwright` is optional
  (AFU-V1-140). The file is a regular file of at most 64 KiB; no other member is admitted at any
  level.
- Test kind: one of `unit`, `integration`, `e2e`, `UNKNOWN`.
- Kind rule match: a rule matches a path when its prefix is `.`, equals the path, or is a
  directory prefix of it; the longest matching prefix wins.
- E2E inventory: the set of E2E test files, each a (project, path) pair. With a `playwright`
  member the inventory is the derived one of AFU-V1-140; otherwise it is every test link target
  path of every flow in the declared directory at HEAD, with an empty project.
- Flow hit: a flow link whose target path the change reaches over the plan's graph (AFU-V1-017
  `reach`); its changed path is the first element of the reach path.
- Selecting variation: for a hit from a variation, that variation; for a hit from a step or outcome,
  every variation of the flow that lists the step or outcome.
- Global path: a changed path whose base name is in the AFU-V1 global set (`go.mod`,
  `package.json`, `Makefile` and the rest of `builtinGlobalNames`), or the flows declaration, the
  declared Playwright config or any path under the flows directory.

## Requirements

- `APK-V0-001`: (proposed 2026-11-09; V1-5251; delivery experimental) One plan for every test kind.
  When the repository declares a flows directory, `corvint affected --base FULL_SHA`, and every
  other plan form that runs the one planning tail over the repository worktree (`corvint test`, a
  review's worktree plan), MUST return unit, integration and E2E selections in one receipt without
  any extra option: the E2E selections are the plan member `e2e` beside `selected`. A repository
  with no declaration MUST get a plan byte-identical to the plan the same command returned before
  this change. A plan over a materialized snapshot, which has no HEAD of its own, carries no `e2e`
  member.
- `APK-V0-002`: (proposed 2026-11-09; V1-5251; delivery experimental) Every selection carries its
  kind. When the declaration exists, every entry of `plan.selected` MUST carry `testKind`, taken
  only from the declaration's `kinds` rules applied to the selection's test files (or, for a
  selection without test files, to its declared consumer file), and every entry of `plan.e2e.
  selected` carries `testKind` `e2e`. A selection whose test files match no rule, or whose files
  match rules of different kinds, carries `UNKNOWN` and stays selected; a kind is never guessed from
  a file or directory name. Without the declaration no selection carries the member.
- `APK-V0-003`: (proposed 2026-11-09; V1-5251; delivery experimental) E2E selections name flow and
  variation. Every `plan.e2e.selected` entry a flow hit produced MUST carry `flows`, one row
  `{flow, variation, changedPath, target}` per (flow, selecting variation, changed path) that
  selected it, sorted by flow, variation and changed path, and `testKeys`, the test keys of the
  flow links that attach the file to those variations. A test file two variations attach appears
  once with both rows. The reason `flow-reach` names the selection.
- `APK-V0-004`: (proposed 2026-11-09; V1-5251; delivery experimental) Flow evidence fails closed.
  Links are evaluated at the measured commit, the resolved `--base` else HEAD, which is the state
  before the change, as the e2e-safe profile evaluates them (AFU-V1-020); the intents are read at
  HEAD. The E2E member's `scope` is `BOUNDED` only when every link of every flow evaluates
  `reviewed` and the plan's own unknowns leave the obligation closure bounded (the e2e-safe
  `unbounded` predicate). Otherwise the plan MUST select every E2E test that the missing evidence
  could have excluded and name why: a flow with a link that is `stale` or `link-not-at-anchor`
  selects every file its links attach (reason `stale-link`) and adds the unknown
  `{reason: "E2E_FLOW_STALE", detail: FLOW_ID, hint}`; a flow with a link in any other state than
  `reviewed` (inferred, unreviewed, an anchor that is unknown, not an ancestor or not an intent
  change) selects the same files (reason `unreviewed-link`) with the unknown
  `{reason: "E2E_FLOW_UNREVIEWED", detail: FLOW_ID, hint}`; a global path, an unbounded plan
  unknown, more than 4096 changed paths or more than 16384 inventory files selects every inventory
  file (reasons `global-path-changed`, `plan-unbounded`, `bound-exceeded`) with the matching
  unknown (`E2E_GLOBAL_PATH`, `E2E_PLAN_UNBOUNDED`, `E2E_BOUND_EXCEEDED`); an inventory file whose
  own path changed or that the graph reaches is selected (`test-file-changed`, `static-reach`); an
  inventory file no flow attaches is selected (`unlinked`, AFU-V1-141). Every such answer has
  `scope` `FULL` when every inventory file is selected and `UNKNOWN` otherwise, and lists the
  unknowns under `plan.e2e.unknown`. A present declaration that is unreadable, oversized, invalid in
  any member, or names a directory that does not load or evaluate at HEAD yields `scope` `UNKNOWN`,
  the unknown `{reason: "FLOWS_DECLARATION_INVALID", detail: ".corvint/flows.json", hint}` and no
  selection. No inventory file is ever absent from `selected` without a flow link in a non-stale
  review state explaining it; `plan.scope`, `plan.selected` and `plan.unknown` are not changed by
  this member.
- `APK-V0-005`: (proposed 2026-11-09; V1-5251; delivery experimental) The new members are
  additive. `plan.e2e` and `testKind` are optional members of `affected-plan/0` and
  `affected-plan/1`; no existing member of either profile changes name, type, order or value, so a
  reader that ignores the new members reads the same units, scope and unknown reasons as before.
  The compact plan carries both members unchanged, `--format text` prints one `e2e` line per entry
  and one `e2e-unknown` line per E2E unknown after the `selected` lines, and the `affected-summary/1`
  verdict gains only the reduced member of `APK-V0-9970`.
- `APK-V0-006`: (proposed 2026-11-09; V1-5251; delivery experimental) Deterministic, bounded and
  read-only. The joined plan MUST be byte-identical for identical inputs (entries sorted by project
  then path, rows and reasons sorted), MUST apply the existing flow read bounds (AFU-V1-037, the
  4096 changed-path and 16384 inventory bounds of APK-V0-004) and MUST write nothing to the
  repository, to `.corvint/` or to Git state; the join reads the declaration as a file, the intents
  at HEAD and the link review states at the measured commit through Git objects, and the discovery
  receipt as a file.
- `APK-V0-007`: (proposed 2026-11-09; V1-5251; delivery experimental) Agreement with the flows
  impact answer. For the same base, head, graph and flows directory, every `test_keys` entry of
  every flow in the `corvint flows impact` report (AFU-V1-017) MUST be a `testKeys` entry of some
  `plan.e2e.selected` row, and every `variations` entry of that flow a `variation` of one of its
  `flows` rows.
- `APK-V0-9970`: (proposed 2026-11-11; V1-5254; delivery experimental; amends APK-V0-005 and
  AFP-V0-872) The verdict names the E2E selections. When the compact plan carries `plan.e2e`, the
  `affected-summary/1` verdict of `corvint affected --summary` MUST carry the member `e2e`:
  `scope` (the plan's E2E scope), `inventory` (the inventory count), `selected` (one
  `{path, project}` per `plan.e2e.selected` entry in plan order, `project` omitted when the entry
  has none), `variations` (the distinct `{flow, variation}` pairs of every entry's `flows` rows,
  sorted by flow then variation, empty when no flow hit selected a file) and `unknown` (the
  distinct reasons of `plan.e2e.unknown`, sorted, omitted when none). `--summary --format text`
  MUST print after the tier lines `e2e-scope SCOPE inventory=N selected=M`, one
  `e2e-variation FLOW VARIATION` line per pair, one `e2e PATH` line (ending ` project P` when the
  entry has one) per selection and one `e2e-unknown REASON` line per reason. A plan without
  `plan.e2e` MUST give a verdict byte-identical to the verdict before this requirement. Test keys,
  changed paths, targets, reasons and unknown details stay in the plan, which `detail` names. The
  `corvint.affected` MCP tool answers the verdict bytes (`MMR-V0-1118`), so its answer carries the
  same member and still equals the CLI's. Non-goals: test keys or changed paths in the verdict.
  Failure mode: a repository with thousands of selected E2E files gets a verdict of that size; the
  MCP answer size bound of `MMR-V0-1118` still applies. Rollback: drop the member and its text
  lines.

## Wire

The members below are additive to `affected-plan/0` (`--full`) and `affected-plan/1` (compact);
every other member is as `docs/specs/affected-plan-v0.md` states.

```json
{
  "plan": {
    "selected": [{"unitId": "go:example.test/m/lib", "testKind": "unit", "...": "unchanged"}],
    "e2e": {
      "flows": "flows",
      "scope": "BOUNDED",
      "inventory": {"source": "discovery", "state": "MATCHED", "count": 3},
      "selected": [
        {"path": "e2e/checkout.spec.ts", "project": "chromium", "testKind": "e2e",
         "testKeys": ["checkout.spec.ts > pays"], "reasons": ["flow-reach"],
         "flows": [{"flow": "checkout", "variation": "checkout.happy",
                    "changedPath": "src/checkout.ts", "target": "src/checkout.ts"}]}
      ],
      "unknown": [{"reason": "E2E_FLOW_STALE", "detail": "profile", "hint": "..."}]
    }
  }
}
```

`inventory.source` is `discovery` (AFU-V1-140), `flows` (no `playwright` member) or `provider`
(AFU-V1-145, with `providerOmits`); `inventory.state` is the discovery receipt state (`MATCHED`,
`MISSING`, `MALFORMED`, `BINDING_MISMATCH`) or `n/a`. `unknown` and `flows` are omitted when
empty; `project` is omitted when the inventory has no project.

## Non-goals and simpler baseline

- Deriving the inventory from a runner other than Playwright, running any test, or running the
  Playwright lister from the plan read (AFU-V1-144).
- Changing the e2e-safe profile, its proofs or its wire (AFU-V1-019 to AFU-V1-024); it keeps
  running when asked for and its answer wins (AFU-V1-145).
- Corvint's own E2E suites (V1-5195, V1-5215; AFU-V1-1651 to AFU-V1-1655).
- Guessing a test kind from a file name. The simpler baseline is today's plan plus
  `corvint flows impact` run by hand; this spec exists because the owner asked for one answer.

## Trust boundary, limits, and failure modes

The declaration and the intents are project-owned authority read at HEAD; the discovery receipt is
an untracked artifact bound to HEAD, the config bytes and the source digest, and is trusted only
when it binds (AFU-V1-143). Repository-authored text (flow IDs, test keys, paths) is data, never an
instruction. Bounds: the declaration 64 KiB and 256 rules; the flow read bounds of AFU-V1-037; 4096
changed paths and 16384 inventory files, above which the E2E answer is `FULL` with
`E2E_BOUND_EXCEEDED`. Every failure widens or marks `UNKNOWN` (APK-V0-004); none narrows.

## Deterministic acceptance and testing matrix

A fixture repository with a Go module (a `lib` unit package, an `integration` package declared
`integration`), three Playwright spec files, flows with reviewed links for two of them, a flows
declaration and a bound discovery receipt. One Go test per requirement, named for V1-5251:

| Requirement | Test |
|---|---|
| APK-V0-001 | `TestAPKV0001OnePlanForEveryKind_V15251` (both scenarios: declared, and no declaration byte-identical) |
| APK-V0-002 | `TestAPKV0002EverySelectionCarriesItsKind_V15251` |
| APK-V0-003 | `TestAPKV0003E2ESelectionsNameFlowAndVariation_V15251` |
| APK-V0-004 | `TestAPKV0004FlowEvidenceFailsClosed_V15251` (stale link, invalid declaration, global path) |
| APK-V0-005 | `TestAPKV0005MembersAreAdditive_V15251` |
| APK-V0-006 | `TestAPKV0006DeterministicAndReadOnly_V15251` |
| APK-V0-007 | `TestAPKV0007AgreesWithFlowsImpact_V15251` |
| APK-V0-9970 | `TestAPKV09970SummaryNamesE2ESelections_V15254`, `TestV15254MCPAffectedCarriesFlowSelections` |

## Rollout, rollback, and compatibility

Rollout: experimental; a repository opts in by committing `.corvint/flows.json`. Rollback: remove
the file to return to today's plan; to withdraw the change, revert its commits and retire
`APK-V0-001` to `APK-V0-007` and `APK-V0-9970` without reuse. Compatibility: additive members under the unchanged
profile names (APK-V0-005).

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| APK-V0-001 to APK-V0-004, APK-V0-006, APK-V0-007 | `internal/appflows/flowplan.go`; `planAffectedGraph` in `cmd/corvint/affected.go` | tests above |
| APK-V0-005 | `affected.Plan.E2E`, `affected.Selection.TestKind`, `cmd/corvint/affected_compact.go`, `cmd/corvint/affected_text.go` | `TestAPKV0005MembersAreAdditive_V15251` |
| APK-V0-9970 | `affectedVerdictE2EOf` and `renderAffectedVerdictText` in `cmd/corvint/affected_verdict.go` | `TestAPKV09970SummaryNamesE2ESelections_V15254` (verdict member and text lines; the twin without a declaration has no member), `TestV15254MCPAffectedCarriesFlowSelections` (the MCP answer equals `--summary` on the flows fixture and names its flows and variations) |

## Unresolved decisions and promotion or kill criteria

Owner acceptance of the intent; the reduced `affected-summary/1` member of `APK-V0-9970`, which
V1-5254 needed so the `corvint.affected` MCP answer names the flow and variation selections; whether a snapshot plan should read a declaration from the
snapshot's own revision. Promote when an adopter repository runs on the member for one release
with no E2E test dropped; kill if the member is unused by every caller after 1.0.
