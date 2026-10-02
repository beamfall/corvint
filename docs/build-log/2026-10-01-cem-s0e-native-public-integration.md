# CEM S0E Native Public Integration

This integration composes the reviewed native S0E overlay into public base
`b234541d6022191d7906917d715664b5cac5cede` without changing the historical
portable packet bytes that already matched that base. The source overlay is
`/private/tmp/cem10-build/stable-next/native-s0e-impl/overlay-final-reviewed.tar`
with SHA-256 `5c2c20753fe72ade40a6cf5dd35abca86c0259c219c368fdc53ee05a85250dc3`;
its allowed member list has SHA-256
`13332ec3874eeadcff94dd9f5d57c2b67519da5599291eaca4942bdc866341b1`.
Every non-AppleDouble archive member matched that allowed list; AppleDouble
members were ignored and no `._*` files were written.

The native owner dependency is retained locally from PR472 commit
`19ed3a76f8a3b20f1f2dfb3d4053a17dc1e0c4eb`. The five
`internal/groupreap/owner*.go` files are byte-identical to that commit,
including the `recordOwnerEvent` helper rename, and the build log
`docs/build-log/2026-10-01-groupreap-owner.md` is retained with the same commit
identity. The stale overlay copy of `internal/groupreap/owner.go` was not used.

The implemented S0E slice adds the Darwin/Linux stable repository admission
boundary, the stable Git budget and owned process lifecycle, checked object
session cleanup, and the `canonical-repository-bounded/1` verifier envelope.
Stable callers on supported Darwin/Linux use the owned runner. Unsupported
stable containment refuses before spawn; legacy non-Stable gitrun callers keep
their existing path and tests. Existing legacy tests were not edited for changed
expectations.

Composition and traceability receipts are retained under
`/private/tmp/cem10-codex-control/`: `NATIVE-COMPOSITION-PLAN.json` freezes the
base, source hashes, intended filenames, API seams and rollback; raw affected
advice is retained in `affected-after-native.txt` with exit metadata in
`affected-after-native.exit.json`. `script/gen-spec-requirements.sh`, run through
a temporary index/object directory for the changed spec, produced no
`docs/specs/REQUIREMENTS.tsv` diff. The existing spec index/digest wording still
matches the unchanged high-level claim; only the bounded S0E envelope details
changed.

Focused local verification:

- `GOCACHE=/private/tmp/cem10-go-cache GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/cem/verify` passed, including the public S0E default cases with all 21 result keys compared without normalization.
- `GOCACHE=/private/tmp/cem10-go-cache GOTOOLCHAIN=local go test -count=1 -timeout 30m ./internal/groupreap ./internal/cem/gitrun ./internal/cem/gitauth ./internal/cem/verify ./internal/cemcandidate` passed.
- `GOCACHE=/private/tmp/cem10-go-cache GOTOOLCHAIN=local go vet ./internal/groupreap ./internal/cem/gitrun ./internal/cem/gitauth ./internal/cem/verify ./internal/cemcandidate` passed.

Repository-wide gates, release gates and complete seam tests are not claimed
here. Linux lifecycle, cross-device topology and escaped-session behavior remain
`NOT_RUN` / excluded. CEM1 S1-S9 remain unimplemented, external conformance is
unavailable, and this is not release promotion. Rollback is to revert the paths
listed in `NATIVE-COMPOSITION-PLAN.json` to the public base and remove the new
integration build log and top-level S0E envelope files, while preserving the
already-merged public packet bytes and historical fixtures.

Independent composition review found a prose regression that classified observed
artifact changes/digest mismatches as unsupported reads. The spec split was
restored: unreadable/unavailable bytes are exit 2, while observed mismatches
remain exit 1 rejections. Verifier code, public schemas and test expectations
were preserved. Earlier scratch findings in the lifecycle repair entry are
historical; the focused composition checks above describe the delivered source.

The required line-citation check found one stale content anchor in DOGFOOD.md
for the unchanged 1,024-operation limit after gitrun gained stable state. The
reference now names only the unchanged operational constants. This repairs the
locator without changing the dogfood contract. The addendum path is covered by
the native reservation’s existing docs namespace.
