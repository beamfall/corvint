# V1-0614: full-corpus structural qualification, evidenced narrowing (label kept)

Date: 2026-10-16. Base: `82d1317e1b6bcd42c37b7aa8b4219b76271af085` (origin/main); the corpus run
started at `b1ee552b3bebeaacf8f54e62f7f67572f231848e`. Ticket V1-0614, as re-scoped by decision
0525: full-corpus structural qualification under `protocol/cem-1.0`, no `protocol/cem-0.3` packet
and no `verify-03` command. The native store still holds the pre-0525 acceptance criteria.

Outcome: `EXPERIMENTAL_TUPLE_PENDING_FULL_CORPUS` is **not lifted**. This entry records the narrowing
under `CEM-V1-050`. No code, schema, packet or spec row changed.

## What "full corpus" means on origin/main

- Committed maps: 751 `.corvint/changes/*.cem.json` at the base (747 when the run started). All
  are `cem/1.0`; 615 carry V1-1277 converter provenance. They hold 26,907 hunks: 26,864
  `supported` and 43 `unknown`. None are `mechanical`, so no committed map ever requires a
  structural proof.
- `protocol/cem-1.0`: 178 tracked files. The stable packets are citation vectors,
  neutral-criterion, receipt authentication, reference precedence, the repository-envelope
  (S0E) packet with 56 full-result cases and the tamper matrix. The sealed tuple packet is
  `structural-tuples/go1.27.2/`.
- Structural tuple: the exact Go release whose `go/parser`, `go/scanner`, `go/ast`, `go/token`
  and `go/format` behaviour the structural predicates depend on. `structuralTuples` admits
  `go1.27.1` and `go1.27.2`. The only qualification host so far is darwin/arm64.
- Toolchains on disk (no downloads): go1.27.2 (Homebrew), plus module-cache go1.26.4/5/6/9,
  go1.27.0, go1.27.1 and go1.27.2. There is no go1.24, which is the `interop/cem01-go` module
  floor, and no Linux or Windows host.

## Lift condition and why it is not met

`protocol/cem-1.0/stable/inherited/ALGORITHMS.md` (structural predicates) admits a tuple as
qualified only after it passes the frozen new corpus, and only after go1.24 and go1.27.1
parsers, scanners and formatters have been compared over that whole corpus plus
version-sensitive syntax, comment, directive and formatting cases. Four gaps block this:

1. **Corpus gap.** No committed map has a `mechanical` hunk, so none requires structural proof.
   Every completed sample result below reports `axes.structuralProof` and
   `runtime.structuralQualification` as `NOT_REQUIRED`; an early refusal would leave the axis
   `NOT_CHECKED` instead. Running the whole corpus cannot qualify structural semantics.
2. **Toolchain gap.** go1.24 is absent and was not downloaded.
3. **Frozen packet.** Lifting would rewrite `structuralQualification` in 54 frozen S0E
   expectations and in the tuple packet. That is not the additive member regeneration that
   `CEM-V1-091` allows, so it amends `CEM-V1-050`/`CEM-V1-091`.
4. **Hosts.** Linux, Windows and CI-hosted go1.27.2 runs remain NOT_RUN (`CEM-V1-050` trace row).

## Corpus run (sample; the full run was infeasible)

The scratch harness was not committed. It ran native `corvint cem verify-stable` and portable
`cem01 verify-stable-canonical` (the S0E route that matches native) over each committed map, with
`--expected-base` set to the map's base. The target was the bind commit when its sidecar equals
the committed bytes. Otherwise it was T': the bind tree with only the sidecar replaced by those
bytes, using fixed identity and dates (the V1-1275 method).

Full corpus at run start: 747 native/portable pairs, so 1,494 verifier executions per tuple. In
the last 512 s window with 8 jobs, 14 executions finished and 8 timed out at 240 s. Over all
windows, 18 go1.27.2 executions finished (11 native, 7 portable) and 8 timed out. Load averages were 54 to 83 throughout. All 7 pairs where both consumers finished under
go1.27.2 are byte-identical:

| Bind | Target | Outcome | Sidecar | structuralProof / qualification | Result SHA-256 (native = portable) |
| --- | --- | --- | --- | --- | --- |
| `01a55f046ac0` | bind | ACCEPT | EXACT | NOT_REQUIRED / NOT_REQUIRED | `f60fae684b1cdcc717ab72afd6f1951df7882bad10e598473d5c58ab38c824e3` |
| `01e2f0a3b44e` | bind | ACCEPT | EXACT | NOT_REQUIRED / NOT_REQUIRED | `fd0519b0d3cb705995b5d34688171216ddbf660eb027d9c95f32bbac7a2bee1a` |
| `02676771e3f3` | bind | ACCEPT | EXACT | NOT_REQUIRED / NOT_REQUIRED | `917eed42b2dfd8dbc370f114ffed941af440aa61feddee5eaae36194a73988ea` |
| `02ed58f34022` | T' `c1b34c2bc2e1` | ACCEPT | EXACT | NOT_REQUIRED / NOT_REQUIRED | `432a477f44e3e5dba8c16b2f8151a5e14b636d50e35901f375bf4e031da791ac` |
| `03539676bf82` | T' `c0b4a2ee79b1` | ACCEPT | EXACT | NOT_REQUIRED / NOT_REQUIRED | `165061c22deaf597938dd53350f8091e2ab91428641f08df8d3d19c8e2d28c19` |
| `070fcd632953` | bind | ACCEPT | EXACT | NOT_REQUIRED / NOT_REQUIRED | `d82b086697b3685eef6713b78119af1a522a126a4acef9eae34cd3b24ac367a7` |
| `19dac6bc8326` | T' `e6d9e21cf6a2` | ACCEPT | EXACT | NOT_REQUIRED / NOT_REQUIRED | `337c178d1ea914615dcde8190261bd21b512777f1f01e5b98b9365a8b86f1d8b` |

Under go1.27.1, `070fcd632953` gave native = portable at
`b2f3aef247bec2a06c06760678e6e285f220c061db6895c8294c8b856269ada7`. It differs from go1.27.2 only
in `runtime.goVersion`.

Every execution, including the 8 timeouts (`NOT_RUN`), is retained one row per execution in
`docs/build-log/evidence/v1-0614/runs.jsonl` (SHA-256
`073a2235cf94da241a26e04a397a22ebba548bb5d73eb750ed17e2a14a0e68b8`): the target commit, exit,
seconds, 1-minute load, result digest and the result's outcome, sidecar, structural axes, Go version
and map digest. The T' commits and full result bytes stay in the scratch clone and are not
retained, so the digests are author-reported observations.

Four further maps finished natively only. Witness, identity and adversarial semantics have no
per-map instances in this corpus; they live in the packets below. The portable legacy
`verify-stable` route refused `070fcd632953` with UNSUPPORTED `unsupported-resource-limit` at
stage `repository`: its snapshot envelope allows 4096 tree entries and 1024 Git operations.
Earlier full parity (V1-1275 and V1-1277 evidence) was 614/615 native = portable under go1.27.2,
with f0fda542 the known resource-limit divergence.

## Packet tests run (go1.27.2, darwin/arm64, load 54 to 68)

PASS, `interop/cem01-go`: `Test03IndependentStructuralPredicates`, `TestCEMV1050StructuralTupleAdmission`,
`TestStableSidecarFailureStates`, `TestCEMV1111CitationVectorsPortable`,
`TestCEMV1083NeutralCriterionPublicCase`, `TestCEMV1093PortableStableMemberVectors`,
`TestCEMV1084PortableReceiptAuthenticationPublicCase`, `TestCEMV1177ReferencePrecedencePortable`,
`TestCEMV1226TamperMatrixPortable`; `go vet ./...` clean. PASS, native:
`TestCEMV1050StructuralTupleAdmission` (`internal/cem/verify`).

NOT_RUN: `TestStableCanonicalFullResults` (S0E) timed out at 590 s, and the full
`interop/cem01-go` `go test ./...` timed out at 580 s, both under host load. The full corpus, go1.24,
Linux, Windows and CI runs are also NOT_RUN.

## Owner decision needed (recorded, not applied)

Lifting the label needs an owner amendment of `CEM-V1-050`/`CEM-V1-091`. The amendment has to
define a structural Go-source corpus (the committed maps cannot serve). It has to either supply a
go1.24 comparison or drop it for cem/1.0. It also has to authorize a non-additive change to frozen
S0E `structuralQualification` values.

## Rollback

Delete this entry and `docs/build-log/evidence/v1-0614/`.

## Independent review (Codex, read-only, one round)

Two P3 findings, both repaired here. First, the run size was mislabelled: the corpus has 747 pairs
per tuple, which is 1,494 executions. Second, "every run reports `NOT_REQUIRED`" overstated the
claim, because an early refusal leaves the axis `NOT_CHECKED`; the statement now covers only the
completed sample. The reviewer confirmed the corpus, packet, tuple and go.mod-floor counts, and
agreed that the label must stay and that an amendment is needed. It could not confirm the run
digests from committed evidence, so the run rows above are now retained.
