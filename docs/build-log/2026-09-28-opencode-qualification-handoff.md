# OpenCode qualification report handoff

Issue #323 exposed a missing product path in PR #318: native campaign PASS used boolean checks,
while `corvint_status` required a separate conformance record. The earlier local installation used
a scratch assembler, so another checkout could not reproduce qualification through the supplied
command. Reproduction retained native PASS followed by `qualification-tuple-or-gate-mismatch`.

The first-party command now owns the focused suite and native campaign, freezes all relevant
identities across both, and writes the exact consumer record at the documented sibling path.
The native-only collector remains a separate implementation helper. It also verifies the host image
observed inside the actual native process; shell launchers are refused by the public command.
Previous evidence is retained, while active qualification becomes INCOMPLETE before either gate.
Publication uses an atomic rename; a failed or interrupted run cannot retain stale FULL.

The regression feeds a native-schema fixture through the same builder and writer used by the
command, then invokes the actual status consumer. It covers missing/failed/skipped evidence,
identity drift, atomic interruption and prior-record invalidation. The fixture preserves the shape
and measurements of the earlier real campaign; runtime identity fields are rebound to the isolated
test package. This fixture establishes the handoff contract, not native host qualification.

The independent plan review required ownership of both gates, complete identity binding, explicit
failure state and atomic publication. Frozen live qualification and independent delta review remain
required before merge. Authority NONE, Frontier UNAVAILABLE and legacy FALLBACK are unchanged.
The repository's required PR CI remains the merge gate; no duplicate local full-suite gate is run.

Independent delta review found Python/Node architecture names disagree on Linux (x86_64 versus
x64, and aarch64 versus arm64). Producers now normalize those names, and native discovery also
checks the platform/architecture reported inside OpenCode. The regression uses the producer's
architecture rather than substituting Node's value, with explicit Linux alias coverage.

The targeted repair review found no remaining blocking findings; the final clean first-party
qualification command remains the promotion gate for this source revision.
