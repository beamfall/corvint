# Experimental CEM candidate companion

`corvint-cem-candidate` verifies or assembles `cem/1.0-experimental.1` reference bundles. It is optional and explicitly admitted; default Core/OCM/frontier commands continue using their existing protocols. Build with `go build -o /absolute/output/corvint-cem-candidate ./cmd/corvint-cem-candidate`.

Use `verify --experimental --repository REPO --map MAP --expected-base FULL_BASE --target FULL_TARGET --artifacts ARTIFACT_ROOT` with independently supplied immutable revisions. `assemble --experimental --request REQUEST.json --out-dir NEW_ABSOLUTE_DIRECTORY` reads the closed declaration in `internal/cemcandidate/schema.go` and writes a new standalone bundle only. The request declares source-map and native Tasks/runner artifact paths with exact raw SHA256, one native ticket/attempt, a literal source prefix and explicit criterion/hunk/evidence links. It accepts only canonical `cem/0.2` source vocabulary and a target without a conflicting committed CEM sidecar. It refuses `cem/0.3` rather than discarding its witnesses.

Assembly retains all native Tasks capture/verification/claim and runner plan/receipt bytes, including failed or incomplete observations. Each declared source input is compared with its self-hashed immutable target Git blob. `DECLARED_INPUT_BYTES_MATCH_TARGET` proves that declared subset matches target bytes; complete source inventory and execution-at-commit remain unknown. The native serialized plan identity differs from SHA256 of the original raw plan file, and each is checked at its own boundary.

Both operations provide reference integrity only. Neither rechecks native historical semantics/current authority nor satisfies a criterion, gate, completion or release. All eight candidate assurance limits stay `NOT_OBSERVED`; operator-authored criterion associations remain declarations. Actual same-target assembly and native/portable review evidence are traced in `docs/specs/cem-stable-v1.md`.

Rollback removes this optional companion. Historical packets, default commands and native task history stay intact.
