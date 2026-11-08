<!-- SPDX-License-Identifier: Apache-2.0 -->
# Reject obsolete environment-audit completeness claims

Independent review of V1-4364 found that the collector-variable repair did not invalidate
previously collected indexes. Their `EnvBound=true` claim omitted GOFLAGS and GOCOVERDIR reads;
unchanged runner and format identities allowed an assertion skipped during collection to be
excluded during an ordinary run. This entry addresses GLS-V0-017 and GLS-V0-018. The earlier
round's statement that old bindings remain usable is superseded by this admission boundary.

Decision: version the completeness claim in the existing compact header's environment table.
Its final, unreferenced row has the reserved name NUL + `linecov-env-audit` and value `2`.
NUL cannot name a process environment variable. This uses the existing `EnvBinding` shape;
there are no new fields, format identifiers, error codes, exit codes or diagnostic text.
The reader strips this metadata before resolving test bindings. The closed allowlist is exactly
one supported marker in the final position plus `EnvBound=true`. Missing, unknown, duplicate
and misplaced markers, and older gob encodings, clear the completeness claim. The existing
`environment-unbound` fallback then retains the static selection. Both eager and lazy readers
enforce the same boundary. Merge and re-encode preserve rejection; only recollection restores
admission. This closes the class of unversioned or unsupported audit claims, rather than checking
whether a particular omitted variable appears in an already incomplete record.

The writer stamps a current in-memory completeness claim; an index loaded from obsolete bytes
has already lost that claim and cannot acquire the stamp by being rewritten. Existing collection
and Build remain the producers of complete in-memory records. Rollback means disabling line
narrowing and recollecting with a corrected audit, never restoring trust in obsolete indexes.
Old executables do not gain this admission check; operators must use the repaired reader.

`TestEnvAuditVersionAdmission` collects the original `cov.TestDir` GOCOVERDIR fixture and removes
the bindings the old collector omitted. The control demonstrates that trusting that claim omits
the test while its mutant fails under the ordinary runner. Valid compact bytes with absent,
unknown, duplicate and misplaced markers are tested through Decode and Open, Plan and Narrow,
and after merge/re-encoding. Only the supported marker admits; legacy gob formats also stay
unbound. Existing assertions and committed goldens were not changed.

Verification on Linux, go1.27.3, with TMPDIR at `/home/russell/lanes/lineidx.tmp`:

- `corvint affected --base b75053a5b23cb0a4737a593d1fc1912aa3b79436 --format json` selected
  internal/linecov and tools/linecov. The installed binary reported stale source; its retained
  plan includes language-frontier and unowned-path unknowns. No exhaustive coverage is claimed.
- Through `script/heavy.sh`, one package per command, `GOTOOLCHAIN=local go test -count=1
  -timeout 30m ./internal/linecov`: PASS, 29.938 seconds, all existing tests and new regression.
- The same whole-package command for `./tools/linecov`: PASS, 17.019 seconds, including the
  wrapper qualification and miss-confirmation regressions from independent review.
- `script/gen-spec-requirements.sh --write`: no generated changes.
  `script/check-line-citations.sh --repin`: no moved citations.
  `make -s spec-requirements-check line-citations-check`: PASS.
- Independent review of this repair remains the orchestrator's responsibility. No extra reviewer
  was launched. Whole-repository gates, publication, Tasks writes and sealing are NOT_RUN by
  the repair-round instructions. The existing qualification record is stale against these source
  blobs; renewed mutation qualification is NOT_PRODUCED and CI must remain SHADOW until refreshed.

Raw evidence is outside the repository in `~/lanes/lineidx-evidence/repair-audit-version/`:

| Artifact | SHA-256 |
| --- | --- |
| affected.json | 5777f4287d4679a432fe559592cf8cd2aaffb006ccb83a0a0fea7b482b0a46ea |
| linecov-test-whole.log | a1379ed1ee551f41a5664e5ca5d8475769ec45f8b8bf088152535dcbf0819f0e |
| tools-linecov-test.log | c8c43d7f6a8825de19733f97a43583e24c8acf4d72413dae4524f0335f415666 |
| doc-checks.log | 1728ab0c025b841f1ff8143190e5aa28d018d83e1aca8c83c6d94127e869235c |

Friction: a whole-package invocation without HEAVY_WHOLE_REASON refused before running tests.
The rerun supplied the explicit owner requirement and affected selection as its reason. The
configured shared lock directory worked without overrides. The original stale-binary warning
and the refusal are retained, not recast as successful checks.
