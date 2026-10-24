# A real cem/1.0 lifecycle over the V1-2323 change (V1-2322)

Date: 2026-10-24. Tickets: V1-2322 and V1-2323. Spec: `docs/specs/cem-stable-v1.md` CEM-V1-600..605,
still proposed. Base `0949876eba7b706a77988d94944a46c0979690d2` (origin/main). This entry records the
lifecycle of one real change: the V1-2323 release-readiness integration. Its map was sealed, verified
by both consumers, and then reported stale after a follow-up drift commit. Nothing is promoted, and
the native plan, claim, gate and completion are left to the orchestrator.

## Commits

| Role | Commit | Content |
| --- | --- | --- |
| C1, change | `f31b14d600fb8eac1b4b0bbffbb74ecb477f755b` | the V1-2323 code, spec, gate and tests |
| T, target | `b3a82e86695a58d02d81c5eb0e43c1e030b4554e` | runner plans and receipts under `artifacts/` |
| BIND | `fafdef4db349da6aedf6098067801d9a1e978a53` | `.corvint/change.cem.json` |
| SEAL | `adda562c8b8f534c3f7c04ddffaec3811ce3da69` | the map moved to `.corvint/changes/fafdef4db349da6aedf6098067801d9a1e978a53.cem.json` |
| D, drift | `1a90657a697152a742da74e9db2acb71bd3584b7` | one added sentence in `docs/specs/cem-stable-v1.md` |

A first bind and seal (`6fa7c6515`, `ae2724a0e`) were discarded before push and re-cited, because
the cem-verdict gate refused them as below `anchored` on every path.

## Runner receipts

The map's criteria admit three receipts, retained by digest at T under `artifacts/`. Two use the Go
runner profile and one uses the non-Go `node-test` profile. Every receipt profile is
`corvint-test-runner-receipt/0`.

| Index | Runner and target | Plan SHA-256 | Receipt SHA-256 | Result |
| --- | --- | --- | --- | --- |
| 0 | go-test `./internal/companionrelease` | `38db3f2f80a0f856fba8fec821503b34233485ae842fe46ca55dfe4f361566b9` | `f57af630e2fa3d074cbb3b3ab6e50b069bfbbf091ce6705867c2486105d4e134` | 55 PASSED |
| 1 | go-test `./tools/cem-release-suites` | `35559b41a9c4b41bd5e2ed5a36100dce3835c40331a4e007309dedc6e7a5f98b` | `2d0baae5674d8b4ec3b8266c66b35cf7916fa18c660941d1abba9fc1c3e4e011` | 4 PASSED |
| 2 | node-test `tools/cem-release-suites/readiness.test.mjs` | `3b9c9cdcf277f0905815122f9c7d38ed291d1c93fce8cbf047ca5d99401ff346` | `ee1355cdd8c3257b7c5489733ce74e72f14b98f477e4fe0c71bd2687e8fb017d` | 4 PASSED |

The runners used Go 1.27.3 (Homebrew) and Node 22.23.3 on the local darwin/arm64 host.

## Map

- Producer: `corvint-cem-candidate assemble-stable --experimental`, profile
  `cem-stable-plain-assembly/1`, runner admission `corvint-test-runner/0`.
- Map (`stable.json`) SHA-256 `f392f540dce0a8ef1f0fc66791b15cdb38c81100fed250c731f89d3aac5a4abc`.
- Patch SHA-256 `7d0f303a7bae0f41ec0bfa5e34a14f7b5f0621ed3788182fb3c3d58420f142dd`. The map holds
  8 evidence spans.
- Source document `c5a50db600cf134c935a67bf63db253febfa4e2eb04ad182e8eca870680d22f9`; assemble
  request `583c637fbbc1c9bfb21668bf2c606684a186e6e5123a2dced800606b83d47ce4`; assemble output
  `ee3aa36d5c3afcf3fd499d4feca43b69ed127cc1ebad25aca8840b721de6fa54`.
- The bundle's artifact bytes equal the committed `artifacts/` files.

## Both consumers

| Binary | SHA-256 |
| --- | --- |
| corvint built at C1 | `662e3ceaab3583c398a5a91eb6508ebb2dd29e21e44d4381a049aaa75ba4d0cc` |
| cem01-go built at C1 | `a3b854c1770defd912e93ac7b6c93ed57ed4f84da8edea12f0e38b7c16d3cd4b` |
| installed corvint 1.0.0-rc.3 | `b0b004c30cc7d607231ebacbd6800014b412930c1182b72e7a5af0816d5ba648` |
| installed cem01-go | `0aa671fb9febe5f35044556ff8cfd39f68947e278f67ff479125e4bf2cfcf19a` |
| corvint-cem-candidate | `45a7366513f920139ed238b9e8a38f3f5681fac1659a59944381cb49b64010bf` |
| corvint-test-runner | `ad6237562ea3741d61d1547d9f72a5c55062f6dc31c6fb78644dcde472bfbc39` |

- **At T** every one of the four consumers returned `ACCEPT` complete with verdict
  `fcf77e461382802dceb7bc93c32044153dfbf87061181c2e40bf29c8163df44c`. The output files are
  byte-identical, SHA-256 `c8cf3a2d60b84390b66c24a4a7f46055dcd2528b89e8d8f67838d177996a40a3`.
- **At D** every one of the four returned `REJECT` (exit 1) at stage `verification` with issue code
  `patch-digest-mismatch` and an empty `drift[]`, so the map is reported stale. Verdict
  `e68c3fa39d45a27742cfa41e8fe215c85f3fe3f277a8e06fed9c22f22cb2d021`; output SHA-256
  `37c2468705c153cf714f13f74829cb47fe051d3ff0286c32ec04f7ce824cf506`.

## cem-verdict gate

The `touched-anchored` predicate was evaluated locally at SEAL.

- **Over every touched path** the gate refuses with `tier-below-anchored` (output SHA-256
  `98d4cba04b84d083d790ce7b25619a68618764aa1aae80d2c709d0730d90aa6a`). Thirteen paths cannot reach
  `anchored`:
  - the six `artifacts/runner-{plan,receipt}-{0,1,2}.json` files;
  - `internal/companionrelease/core_stable_readiness.go` and its test;
  - `internal/companionrelease/core_stable_result_test.go`;
  - `tools/cem-release-suites/READINESS.json`, `main.go`, `main_test.go` and `readiness.test.mjs`.

  They are new or JSON files, or files whose added bytes define no requirement ID. No BASE span can
  name a new path, and those bytes cannot carry an ID.
- **Over the anchored subset as `effects.verdictPaths`**, the gate grants with exit 0, a portable
  `ACCEPT`, 12 hunks anchored and 24 cited. Verdict
  `5058b4ccfe2d6d7b1371e708ba8ed853b69e1c0f4b3dab20356a31ab8da9f605`; output SHA-256
  `a2adca57b208c525f113db153d5a986a0be8080ae2be67e14b9775a9e58923a3`. The subset is
  `.corvint/test-read-scopes.json`, `Makefile`, `docs/specs/REQUIREMENTS.tsv`,
  `docs/specs/cem-stable-v1.md`, `internal/companionrelease/core_stable_result.go`,
  `protocol/cem-1.0/stable/MIGRATION.json` and `protocol/cem-1.0/stable/manifest.json`.

These BASE spans anchor the subset:

| Spans | Anchors |
| --- | --- |
| `docs/specs/cem-stable-v1.md` 212:250 and 251:268, `protocol/cem-1.0/stable/MIGRATION.json` 412:418 | spec, MIGRATION and manifest hunks, by path and ID |
| `docs/build-log/2026-10-23-cem-release-readiness-review-repair.md` 37:39 | `Makefile` |
| `docs/specs/cem-stable-v1.md` 4596:4596 | `core_stable_result.go` |
| `docs/specs/cem-stable-v1.md` 1966:1967 | `MIGRATION.json` and `manifest.json` |
| `docs/SHELL-TEST-REGISTRATION.md` 15:18 | `.corvint/test-read-scopes.json` |
| `docs/SPEC-DRIVEN-DEVELOPMENT.md` 145:147 | `REQUIREMENTS.tsv` |

The wider span 4596:4599 was refused as `cite-span-not-stable`, because the change rewrites those
rollback lines.

## Release gate

`make cem-release-gate` was run at C1. Make exited 2 and the tool exited 3:
`REFUSED (28 accepted stable results read): release-runner-incomplete`. The output SHA-256 is
`00cf64b2de4bab68799ba02d76bfe4ba2e9e1d8fb52a2035bd6524ce20e003d4`.

- The node row passed 4 of 4, and `darwin/arm64` was PASSED.
- The binary-level `corvint:repository-envelope` and `cem01-go:repository-envelope` rows were
  NOT_RUN.
- `linux/amd64` and `independent-external-consumer` were NOT_OBSERVED.

This refusal is the expected result, not a pass.

## Recorded limits

- The runner receipts were produced on darwin/arm64 only. linux/amd64 (the WSL2 box) was not run
  for this lifecycle.
- One release-gate run observes one platform. `linux/amd64` stays NOT_OBSERVED, and nothing
  aggregates readiness across platforms.
- The binary-level repository-envelope rows are NOT_RUN.
- External interoperability is NOT_OBSERVED.
- Receipt authority and execution at the commit are NOT_OBSERVED. Execution authority is
  CALLER_OBSERVED.
- The cem-verdict grant covers only the seven anchored paths (see above).
- The one review round found a P2: `encoding/json` merged a case-aliased member such as `Runner`
  into `runner`. The fix is commit `37c20d339`, which requires the exact member names, with three
  new refusal cases. It lands after SEAL and changes bound source. The sealed map describes target
  T, not the PR head, so completion binds to SEAL.
- `make dogfood-change` was tried twice, which is the lane's limit. Both runs were NOT_PRODUCED.
  - At the clean base it exited 2: `git-diff-failed`, `cem-map-not-produced`,
    `missing-intent-scope`, `map-unavailable` and `outcome-input-not-provided`.
  - At SEAL it exited 2 with `citation-plan-not-provided`, `missing-intent-scope`, cem-status
    `not-ready` and `outcome-input-not-provided`. It reported `sealed-cem-merged
    bind=fafdef4db…`. The report SHA-256 is
    `6edb5e006cffe11f1d282e17404b4a9e20752bb5338a8135517bcd95e2379e21`.

## Rollback

Revert the lane's commits. The sealed map, the `artifacts/` receipts and this entry are evidence
only. No reader takes them as input to ranking, authority or promotion.
