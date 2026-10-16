# 2026-10-16 V1-0414: accepted governing-authority screen and invisible operators

Ticket V1-0414 (security: hidden or bidirectional Unicode in governing documents and governing
files changed in the same diff) was re-admitted after its acceptance revision 2 (2026-10-15). The
behavior had shipped in PR #467 and the `cem cite` refusal in PR #902
(`2026-10-14-v1-0414-closeout-evidence.md`). This entry audits the three criteria again at BASE
`08195a48314d9cf0a8bdeeeb92ba8ab932b52c9a` and closes what was still missing: the contract was not
accepted, part of it lived in a retired spec, and the zero-width set omitted the invisible operators.

## Gaps found at BASE

- `TCP-V0-055..058` were `experimental, not owner-accepted` (REQUIREMENTS.tsv acceptance
  `unrecorded`). `AGENTS.md` invariant 8 and `SPEC-DRIVEN-DEVELOPMENT.md` do not let an
  experimental contract be called delivered.
- The `cem status` report (`selfModifiedAuthority`) was specified only as `CEM-CB-026`, an
  experimental requirement of `cem-0.2-canonical-binding.md`, which decision 0525 retired
  (`CEM-V1-225`). The `cem/1.0` status read still emits it, so the 1.0 contract had no owning
  requirement for criterion 2's `cem status` half.
- The zero-width class held U+200B..U+200D, U+2060 and a non-leading U+FEFF, but not
  U+2061..U+2064 (FUNCTION APPLICATION, INVISIBLE TIMES, INVISIBLE SEPARATOR, INVISIBLE PLUS). They
  are default-ignorable and render as nothing, so they hide text the same way;
  `internal/repoenvelope` already treats U+2060..U+2064 as one range. The class test listed U+2061 as
  visible.
- The threat and both references were recorded only inside the packet spec, not in the operator
  security note.

## Decisions

- Widen `TCP-V0-055`'s zero-width set to U+2060..U+2064 before accepting it. The requirement was
  unaccepted, so this is not an amendment of an accepted requirement. No new warning code, class,
  member or enum value: the code points report as `zero-width`, so the frozen `governance_refused`
  registration (CCF-V1-007 (d), decision 0468) and `core-compatibility-freeze-v1.md` are unchanged.
- Accept `TCP-V0-055..058` and the new `CEM-V1-270` under the owner's standing auto-accept of lane
  requirements (2026-10-11). `CEM-V1-270` restates `CEM-CB-026` for `cem/1.0` with no behavior
  change; `CEM-CB-026` keeps its ID in the retired spec for traceability. `CEM-V1-114` (accepted)
  is not amended; only a stale failure-mode sentence beside it, which said the dogfood fix hint and
  ledger admission were missing (both delivered by V1-1385), is corrected.
- Record the threat in `docs/SECURITY.md`, citing Pillar Security's "Rules File Backdoor" and
  Backslash Security's `AGENTS.md` injection in OpenAI Codex CLI by name, and pointing to the
  exact sets and non-goals in the specs. The references were not re-fetched.
- Fixtures carry hidden code points only as Go escape sequences (`\u2062`); no tracked document
  contains one.

## Acceptance audit

1. Hidden-Unicode governing row warned and downgraded, with fixtures: met at BASE for the
   zero-width, bidi-control and tag sets; widened here. `TestHiddenUnicodeClassesAreExactlyTheSpecSets`
   now walks U+2060..U+2064 under `zero-width` and keeps U+205F and U+2065 outside;
   `TestTaskContextDowngradesAGoverningFileHidingAnInvisibleOperator` downgrades a governing
   `AGENTS.md` whose only hidden code point is U+2062 end to end (warning `hidden-unicode`, class
   `zero-width`, count 1, `first_code_point` `U+2062`, governance `unresolved`).
2. Self-modified governing file: packet (`TCP-V0-057`), `cem status` (`CEM-V1-270`, formerly only
   `CEM-CB-026`) and the `cem cite` refusal (`CEM-V1-114`) all met; the status half now has an
   accepted 1.0 requirement.
3. Threat recorded with the two external references: met in `task-context-packet-v0.md`, and now
   also in `docs/SECURITY.md`.

## Failure modes and limits

- A governing file that legitimately uses an invisible operator (MathML-derived text) is now
  downgraded; the warning names the code point and line.
- Still not screened, as before: homoglyphs, U+00AD, variation selectors (including the
  supplementary U+E0100..U+E01EF used for emoji smuggling), U+180E, Hangul fillers, ordinary rows,
  `query`/`impact` wires, and a renamed governing file's old path. Absence of a warning is not
  evidence that a file is benign.
- The packet's self-modified screen covers the working tree only; a committed range is reported by
  `cem status`, which is advisory and never changes the verdict.

## Acceptance evidence

Host load averages during the runs were 40 to 60 on 12 cores.

- `internal/contextindex` authority-screen tests (`TestHiddenUnicode*`, `TestScreen*`,
  `TestTaskContext{Downgrades,Keeps,Screens,Orders,Reports}*`, `TestCEMGoverningInstructionRuleMatchesThePacket`,
  `TestAnalyzerSchemaInputs`): 21 PASS, 0 FAIL.
- Mutation (equal to BASE behavior): narrowing the zero-width case back to `r == 0x2060` fails
  `TestHiddenUnicodeClassesAreExactlyTheSpecSets` (`U+2061 = "", want "zero-width"`) and
  `TestTaskContextDowngradesAGoverningFileHidingAnInvisibleOperator`; restored, both pass.
- Packages `internal/cem/workflow` (717 s), `internal/observations` and `internal/specindex`: ok.
- Clean-repository output: `cmd/corvint` `TestCoreVerbsEmitTheFrozenProfiles` passes with the
  `context_task`, `context_reserved_rows` and `context_downgraded_governing_row` goldens unchanged.
  That test compares decoded JSON with value placeholders, so it shows structural compatibility
  only. Raw bytes were compared directly: `-trimpath` builds of BASE and of this change each
  indexed a copy of one clean five-file Git fixture (`AGENTS.md`, one spec, one Go package) and
  compiled `context --task "change Greet per docs/specs/greet.md" --subject pkg/greet/greet.go`.
  The two 3694-byte packets are byte-identical (`cmp`).
- BASE against this change on the same fixture with a committed U+2062 in `AGENTS.md`, generated
  at run time: BASE reports `governance` `reserved` and an empty `governance_refused`, so the file
  passed as authority. This change reports `governance_refused` `[{path: AGENTS.md, trust:
  repository-content, warnings: [hidden-unicode]}]`.
- The edit to `authority_screen.go` is an analyzer input, so `TestAnalyzerSchemaInputs`
  (IDX-SNAP-V0-017) required bumping `analyzerSchemaID` from `corvint-analyzer/119` to
  `corvint-analyzer/120` and re-pinning its audited SHA-256. Opt-in analyzer packs built under
  `/119` are invalidated and rebuilt, as for any toolchain change; no wire field changes.
- Local doc-gates (the CI list), `gofmt -l`, and cross-OS `go vet` (windows/amd64, linux/arm64,
  CGO off) of the two changed packages: pass.
- Not run: `make gate` and the repository-wide `go test ./...` (the ticket's `full-gate` runs at
  closeout).

## Review

One Codex read-only round over BASE..HEAD found no correctness defect in the set, the tests or the
spec restatement, and it resolved all 248 requirement locators. One low finding is fixed above: the
golden test was claimed as byte identity, and raw bytes are now compared directly. One
informational finding is out of scope: `integrations/testfixture/main.go:192` (U+2028, U+202E,
U+200B) and `internal/tasks/ticket/ticket_test.go:100` (U+200E) carry literal invisible code points
in Go string literals. Both predate this change, and neither is a governing file.

## Rollback

Revert the change commit: the zero-width set returns to U+200B..U+200D, U+2060 and non-leading
U+FEFF, the four TCP requirements and `CEM-V1-270` return to their unaccepted or absent state, and
the `docs/SECURITY.md` paragraph goes away, and the analyzer schema returns to `/119`. No
state, wire or golden changes.
