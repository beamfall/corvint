# Merge-forward unbrd onto 2026-11-07 mainline

Base: `ae20b398c260933a152e6285de9cfc385a2580c7`. Lane input: `2ac93bd5d1e71365d89d10e02b8dc8500ad2d176`.

The whole resulting range will be rebound to the merged mainline. This round stops after dogfood-check; the lane was not sealed before the round. Host verification and batch CI own broader validation.

## Conflict resolutions

- `.corvint/shell-foreign-callers.json`: Kept mainline caller and mention unions, including new merge-driver and verify-coverage references; lane references were already present.
- `.corvint/unbounded-readers.json`: Kept lane execution-edge reader additions and their reasons plus mainline shell caller reasons.
- `docs/specs/INDEX.json`: Used merged mainline version pending post-merge regeneration by make spec-requirements (the same target owns INDEX.json).
- `docs/specs/README.md`: Kept lane acceptance and review references in the shared row; mainline entries outside that row remain; generated columns will be refreshed.
- `docs/specs/REQUIREMENTS.tsv`: Used merged mainline version pending post-merge regeneration by make spec-requirements.
- `docs/specs/affected-plan-selection-v0.md`: Kept mainline requirements, trace rows, rollback and single digest; retained lane initializer, buffered-listing and certified self-reference refinements and their tests.
- `docs/specs/falsifiable-packet-v0.md`: Kept mainline symbol-based NormalizePaths citation, preserving the lane normalization claim without a stale line anchor.
- `internal/gateselect/locator_parity_test.go`: Kept safe empty initializer control and unknown-initializer parity regressions.
- `internal/gateselect/locatorproof.go`: Kept lane initializer and imported-Getwd certification; retained all nonconflicting mainline code.
- `internal/gateselect/readers.go`: Retained mainline non-source declaration indexing alongside lane execution-runner maps.
- `internal/liveverify/affected/affprec_repo_test.go`: Kept mainline measured integration bound and explanation; execution-edge floor accounting is retained.
- `internal/liveverify/affected/driver_shell.go`: Kept lane go-list output allowlist in addition to mainline feed tracking.
- `internal/liveverify/affected/driver_shell_test.go`: Kept output-format regressions alongside all mainline tests.
- `internal/liveverify/affected/golang/golang.go`: Kept mainline three-result locator API and component suppression alongside lane joined-path tokens.
- `internal/liveverify/affected/golang/locator_selection_test.go`: Kept safe empty initializer control, both lane selection regressions and mainline symlink selection regression.
- `internal/liveverify/affected/golang/locatorproof.go`: Combined mainline leading-component suppression and symlink-safe last-component retention with lane initializer and imported-Getwd certification.
- `internal/liveverify/affected/golang/locatorproof_test.go`: Kept mainline three-result API and updated the added lane initializer test to match.
- `internal/liveverify/affected/readers.go`: Kept mainline testless, untested-owner and hidden-path semantics and comments, including execution-runner traversal.
- `internal/liveverify/affected/shell/computed.go`: Kept lane closed self-reference allowlist, default preservation and opaque substitution handling; mainline shared bounds remain.
- `internal/liveverify/affected/shell/computed_bounds_test.go`: Kept lane composition regressions and existing mainline bounds tests.
- `internal/liveverify/affected/shell/shell_test.go`: Kept mainline disabled temp-root/canonicalization regression cases plus lane environment-source and reassignment cases.

## Requirement identities and verification scope

No unrelated requirement-ID collision was found. Mainline already contains the lane's AFP-V0-2950..2962, 2990..2995 and 3140..3149 definitions for the same capabilities; the remaining lane refinements amend those definitions. No IDs are renumbered.

Run the four conflict-owning packages separately through script/heavy.sh, plus spec-requirements-check and line-citations-check. No doc-gates, corvint test, gate-affected, whole selection or publication is authorized in this round. Fresh dogfood NOT_PRODUCED reasons remain in the private report; no prior evidence is removed.
