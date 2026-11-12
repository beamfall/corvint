# Tasks: appmap-usable-ui-map

Each task names its requirement IDs and its proof. One commit per ticket.

- [x] 1 Add AMAP-V0-970 to 977 to `docs/specs/application-map-v0.md` with traceability rows, move
   AMAP-V0-215 to `appmap-builder/4`, add the acceptance-log row, the README index row and the
   ticket-vocabulary entry. Requirements: AMAP-V0-970 to 977. Proof: `openspec validate`.
- [x] 2 Shared goto resolver, loop gotos in build and infer, helper attribution for specs
   (V1-5468). Requirements: AMAP-V0-972, 973, 974. Proof: `TestAMAPV0972*`, `TestAMAPV0973*`,
   `TestAMAPV0974*` in `internal/appmap`.
- [x] 3 React `.map` expansion, `key` ignored, builder `appmap-builder/4` (V1-5467).
   Requirements: AMAP-V0-970, 971. Proof: `TestAMAPV0970*`, `TestAMAPV0971*`.
- [ ] 4 `flows appmap <sub>` help (V1-5471). Requirements: AMAP-V0-975. Proof:
   `TestAMAPV0975*` in `cmd/corvint-docs`.
- [ ] 5 `flows appmap build --worktree` and the refusal hint (V1-5472). Requirements:
   AMAP-V0-976, 977. Proof: `TestAMAPV0976*`, `TestAMAPV0977*`; `make capability-catalog`.
- [ ] 6 Per-package tests, vet, windows-vet, doc gates, the lane binary on the reproduction
   fixture, and archive. Requirements: AMAP-V0-970 to 977. Proof: commands and exit codes in the
   handback.
