# Retain JavaScript runner failures independently of diagnostic detail

Requirement: `RSP-V0-003` in `docs/specs/runner-selection-plugins-v0.md`.

The self-review found two ways that a valid terminal failure could disappear before the core
reconciler saw it: Playwright global errors short-circuited test traversal, and Jest file-unit
failure depended on decoding every assertion. Missing identities then triggered a FULL retry,
whose passing results could incorrectly make the invocation succeed.

The repair separates independent failure evidence from ancillary decoding. Playwright retains
validated results before checking global diagnostics. Only absent, null or empty-array diagnostics
certify a clean report; nonempty or unknown forms retain the existing report error. Jest accepts
only recognized failed file status or a validated failed assertion as sufficient failure evidence,
regardless of malformed assertion details. A pass still requires a recognized passed file status
and a decoded assertion array using the existing status allowlist: passed, failed, pending, todo,
skipped, disabled. Unknown forms retain uncertainty and do not certify passing results. No wire
shape, field, error code, stderr contract, golden or existing assertion changes.

`TestRSPV0003PlaywrightGlobalErrorRetainsFailureAcrossRetry` covers a teardown error array and an
unknown diagnostic object. `TestRSPV0003JestFileFailureRetainedAcrossRetry` covers an assertion
status object, an unknown status string, an unknown assertion container and null assertions, with
negative controls against certifying passed files. Both drive `RunInventoried` through an actual
missing-result FULL retry that passes, while the invocation remains failed and unqualified.

Validation: whole `internal/browsercov` and `internal/jslinecov` suites passed through
`script/heavy.sh`, each with `GOTOOLCHAIN=local go test -count=1 -timeout 30m` and no test filter.
The affected plan at `9761ea1f459c0d3b3c963953e24a76eaeda16370` selected both packages, but its
overall scope was UNKNOWN (135 unknowns) and the installed binary reported self-binary-stale.
No broader coverage is inferred. Independent review and live qualification remain NOT_RUN here;
the orchestrator owns independent review and host verification. No task-store writes or publishing.

Binding uses the existing merged-forward repair base
`fce5b85b53306df6a0b276245ac04ae3c6097add`, per DOGFOOD section 4, rather than attesting the
272-file integration range from the original lane base. Original pre-change evidence is reused;
no new capture is claimed. Final check evidence and local limitations are in the external handback.
Rollback is the inverse of this parser repair and its new tests; no persisted format migration.
