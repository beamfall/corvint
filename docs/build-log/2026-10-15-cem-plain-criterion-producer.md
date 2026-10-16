# 2026-10-15 V1-1290 and V1-1267: plain-criterion producer and the rc.3 decision 0523 notes

V1-1290 is decision 0523 P1 adoption work (kill criterion K4). Before this change no producer emitted
the plain criterion form that `CEM-V1-083` allows. `corvint-cem-candidate assemble-stable
--experimental` accepted only the Tasks-bound `cem-stable-assembly/1` request. The public case
`protocol/cem-1.0/stable/neutral-criterion/stable.json` was assembled by hand in
`docs/build-log/2026-10-12-cem-neutral-criterion-walkthrough.md` (step 8).

V1-1267 carries decision 0523's recorded limits, its §6 MUST NOT list and its §5 kill criteria into
the rc.3 release notes and into `docs/CHANGE-EVIDENCE-MAP.md`.

The new requirement is `CEM-V1-240` in `docs/specs/cem-stable-v1.md`. It is accepted under the
owner's standing auto-accept (2026-10-11). The text of `CEM-V1-083` is unchanged; only its trace
cell now names the producer.

## Decisions

- **A new request profile on the existing experimental command, not a new verb.**
  - `cem-stable-plain-assembly/1` is selected by its `profile` member. It has its own closed
    decoder. Every other profile keeps the Tasks-bound path and its closed decoder, so a plain
    request sent under the old profile is still refused (`unknown field "criteria"`).
  - The `corvint cem` frozen surface, the MCP tools, the wire and the verifier are untouched.
- **The producer completes a `cem prepare` / `cem cite` map; it does not cite.**
  - The source must have no criterion binding, receipt, link, artifact or provenance. An assembled
    map is refused, so the producer never re-wraps a map it did not start from.
  - Criterion `i` is the cited evidence record itself: `criterionSha256` is its `spanSha256`, and
    the ID is computed over the three non-ID members, as `CEM-V1-083` defines.
- **Runner files are opaque.** The plan and receipt are checked only by SHA-256, as the verifier
  checks them. The result reports `receiptAuthority` and `executionAtCommit` as `NOT_OBSERVED`, and
  no producer field raises a result axis.
- **Write order.** The map is written last, only after the native stable verifier ACCEPTs the
  bundle and every pin still matches. The bundle directory rules (new, absolute, outside the
  repository, no symlink alias in the parent) are those of the Tasks-bound path.
- **Release notes extend PR #944's section.** #944 merged first and added
  "1.0.0-rc.3 (unreleased): recorded limits" for the V1-1266 hunk-revert work. This change adds
  three subsections under it rather than a second rc.3 heading: the decision 0523 recorded limits,
  the §6 list and the kill criteria. The hunk-revert and K1 bullets point back to #944's text.

## Walkthrough (replaces the hand-assembly step)

This is the walkthrough the pilot kit names. It needs only Git and the three binaries
`corvint`, `corvint-cem-candidate` and `cem01-go`. The repository has no `.taskman/`.

1. Build the recipe repository of `protocol/cem-1.0/stable/neutral-criterion/` from
   `repository.recipe.json` (base `467cbb73…`, target `1f181cd0…`, as in its `EXPECTED.json`).
2. `corvint --root REPO cem prepare --base BASE --target TARGET`.
3. For hunks 1 and 2: `corvint --root REPO cem cite --map .corvint/change.cem.json --hunk N
   --evidence-path SPEC.md --lines 6:6 --relation specification`.
4. Copy `.corvint/change.cem.json` and the case's `artifacts/runner-plan.json` and
   `artifacts/runner-receipt.json` to an input directory outside the repository.
5. Write a request with `profile` `cem-stable-plain-assembly/1`, `repository`, `expectedBase`,
   `target`, the three `{path, sha256}` pins, and one criterion. The criterion's
   `criterionEvidenceId` is the one evidence record, its `hunkIds` are both hunk IDs, and its
   `evidenceIds` is that evidence ID.
6. `corvint-cem-candidate assemble-stable --experimental --request REQUEST --out-dir NEW_DIR`.
7. `corvint cem verify-stable` and `cem01-go verify-stable-canonical`, each with `--repository`,
   `--map NEW_DIR/stable.json`, `--expected-base`, `--target` and `--artifacts NEW_DIR`.

Timed run on 2026-10-15 at 22:16 (load 33.06 45.32 45.06). It used binaries built from this branch.

| Step | Result |
| --- | --- |
| recipe repository, prepare, cite 1, cite 2, assemble, both verifiers | every step exit 0 |
| wall time | 74 s for all steps; stranger time stays `NOT_OBSERVED` |
| assemble result | `ACCEPT`, `receiptAuthority` and `executionAtCommit` `NOT_OBSERVED`, `stableSha256` `0c445ac2cec0623621d15ca6483c1d5e9500348f7ede43852f5ae5448221e8c0` |
| `stable.json` and both artifacts | byte-identical to the public case |
| native and portable verdicts | both `ACCEPT`; equal `verdictSha256`; equal non-runtime results |
| repository contents | `.git` and `.corvint` only; no `.taskman/` |

## Evidence

- Tests. `TestCEMV1240*` does not exist at the base, which has 0 such tests. A plain request is
  refused there by the closed decoder with `unknown field "criteria"`.
  - On this branch there are 3 top-level tests and 12 refusal subtests, all PASS (load 61.20 56.61
    52.71).
  - The reproduction test builds the recipe repository with no `.taskman/`. It runs the workflow
    prepare and cite, then the producer, and compares `stable.json` and both artifacts with the case
    byte for byte.
  - Each refusal subtest asserts that no `stable.json` is written.
- Focused packages pass (load 54 to 64): `internal/cemcandidate`, `cmd/corvint-cem-candidate`,
  `tools/cem-stable-regen`, `tools/test-read-scopes-check`, `internal/doccompiler`,
  `internal/cem/wire`, `internal/cem/cli` and `internal/specindex`. In `internal/cem/verify`, the
  neutral, manifest and arguments tests pass. `interop/cem01-go` runs through its own `go test` and
  `go vet`.
- Mutations, one per fix. Each was restored after the run.
  - Producer: `criterionIndex` set to `i + 1`. The verifier still ACCEPTs that self-consistent map,
    and the reproduction test fails on the byte comparison of `stable.json`.
  - Dispatch: the plain-profile branch was never taken.
    `TestCEMV1240PlainProfileSelectsPlainDecoder` fails with `unknown field "criteria"`.
- `protocol/cem-1.0/stable/manifest.json`: only the `neutral-criterion/README.md` entry's digest
  and size change. The `publicAmendmentManifestSHA256` binds only the packet manifest and is
  unchanged. `tools/cem-stable-regen` passes.

## Limits and residuals

- K4 is not met by this change. The producer removes the hand-assembly step, but stranger time on a
  non-Corvint repository is `NOT_OBSERVED`.
- One runner receipt per map. No producer runs the runner or signs the receipt (`FPK-V0-036`).
- Open PR #941 edits the `CEM-V1-080`..`082` trace rows next to the `CEM-V1-083` row changed here,
  and it regenerates `REQUIREMENTS.tsv`. Whichever lands second needs a `git merge origin/main`
  and a fresh `make spec-requirements`.
- V1-1267 ticket notes are the orchestrator's to write. The tickets named by decision 0523 §3 are
  V1-0592, V1-0593, V1-0594, V1-0612, V1-0620, V1-0622, V1-0632, V1-0596 and V1-0595.
