# Native release archive staging acceptance repair

PR #359 CI shard 2 exposed one remaining pre-CAL-V0-027 witness: the archive
reader expected `UNSUPPORTED` when a non-fixture queue contained an unpublished
staging temporary file. The exact test failed locally before this repair with a
nil error, matching CI run 36600837212. The accepted CAL-V0-027 shared observation
contract permits that read while forbidding cleanup or execution authority.

The test now requires a verified nonempty archive, excludes staging payloads from
its manifest, and checks both intent and state trees remain unchanged. No
production admission, binding, recovery, or export behavior changes.

Focused archive tests passed (13.689s); requirements, definitions, test traceability,
decision numbering and line citation checks passed. Corvint query and affected
receipts are retained under `/tmp/corvint-native-release-357/archive-*`; unsupported
or omitted context remains explicit. Other self-development routes are not
applicable to this test-only change. Final bound evidence and independent review
are required before publication; repository-wide CI remains the landing check.
