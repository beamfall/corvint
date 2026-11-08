# Runner dependency self-review repair

Tickets: V1-5106 (RSP-V0-021, RSP-V0-022), V1-5103 (RSP-V0-009,
RSP-V0-049, RSP-V0-054); consumer V1-5115 (RSP-V0-050).

The review identified three missing dependency edges. Linux/amd64 `lseek` now
enters the existing descriptor-read collector instead of bypassing it through
seccomp. All seek forms conservatively retain the descriptor dependency; existing
unresolved-descriptor and peer handling remain fail-closed. A static, libc-free
assembly fixture seeks inherited stdin to EOF and exits with its size, proving
that a size change affects the outcome without another open, read or stat.
`TestRSPV0021InheritedStdinSeek` requires the file edge at both sizes.

Freshness applies the existing sibling-pin-drift result to listings, lookups and
writes as well as blob-bearing components. `TestRSPV0049SiblingProbePin` covers
lookup-only, listing-only and write-only records, unchanged pins and unrelated
pin changes. The first whole-package run reproduced all three stale-pin failures.

Modeled build attribution now supplements the trace with BuildLoaded, preserving
all other components for every test in the build. The strengthened
`TestRSPV0009Attribution` compares each entire attributed record, including reads,
listings, failed lookups, executable dependencies, writes, contacts and an
incomplete read state. Its first run reproduced the dependency loss. This
corrects the earlier build-log statement that modeled activity belongs only in
loaded; RSP-V0-009 and RSP-V0-054 require both kinds of evidence.

No wire shape, field name, error code, exit code, stderr text, committed golden or
core-freeze artifact changes. The old attribution expectation is corrected to
require retention, as explicitly requested by the review; other assertions remain.
Rollback is a normal revert, with no persisted-state migration. No design fork
requiring a broader product contract was needed.

## Verification and limits

Go 1.27.3, local Mac, one whole package per script/heavy.sh command, no -run:
internal/testevidence passed (0.562 s), internal/proctrace passed (0.594 s), and
internal/testrunner/registry passed (2.087 s). Linux/amd64 go vet of proctrace and
cross-assembly of the syscall-only ELF fixture passed. Live Linux tracing is
NOT_RUN on this Mac and remains host qualification, including the new seek test.
Independent review belongs to the orchestrator and was not run here.

Citation repin found no moved content; line-citations-check passed. The broad-base
changed-checks failed on preexisting stale reader inventories and the prior
script runner test's undeclared .github/cishards/test-commands.json read scope.
They are retained for the orchestrator, outside this three-finding repair scope.
No check assertion or scope inventory was loosened to suppress those failures.

Corvint context and affected routes were used; affected against the required
8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734 selects all three packages and retains
scope UNKNOWN, 511 selected and 83 unknowns plus self-binary-stale. Context
returned e0e335d3 rather than the lane revision; original source/spec reads govern.
The CEM uses the already recorded charter integration baseline
80274abee8da1afdf4e193751b6f1f71231e4581, the parent before the lane's first
implementation commit; the requested 8b80e602 base remains the affected baseline.
No pre-change capture was repeated. Final binding/check evidence is recorded in
the external handback; seal, Tasks writes and publication are prohibited.

Core and Tasks remain build 127. Update checks cannot prove process quiescence
under host restrictions; the official release inventory returned an empty list.
Available compatible versions remain UNKNOWN; no failed verification was bypassed.
Account usage and live runtime-control telemetry remain NOT_OBSERVED through the
available tools; no additional model session or delegation was launched.

## Retained evidence

Raw files live outside the repository at
/Users/russelllewis/lanes/rspt13-evidence/. SHA-256 identities:

| File | SHA-256 |
|---|---|
| `repair-test-baseline.log` | `cdfefb8f6aad38be216e4bb617e487855cedf4d4b908c13186852429bdd3f4e0` |
| `repair-testevidence.log` | `81dce1601f209c93caf742be97e0510d9c8ae536c7a17242f93ce668e5665cdd` |
| `repair-proctrace.log` | `fc198ed56efb530279f4b3f921c19c5b6be44d25be84cff6b181f4a4423db470` |
| `repair-registry.log` | `a2cfd50eb8a48c8b816b81781bceedf1d68e4ac170e6ac9a819c7b6e0989d392` |
| `repair-linux-vet.log` | `031bf527788652732d0c5d9fd4990ae65fffe58b286da0e6320ac994f41b2fd4` |
| `repair-fixture-compile.log` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `repair-changed-checks.log` | `4dc58d9ed2e616858fc05644b5de1bdaa3fe79366ffca48b8665e788442793f7` |
| `repair-line-citations.log` | `855a4551b199f407e4e7cec705511746022a39aa0f70383fc13aac9acc748bba` |
| `repair-context.json` | `06e0f3f02b141022c648daddfb308684c02d687edfd2e0ea06b0c112675dc6ce` |
| `repair-affected-tests.json` | `df35f672b3387483bcff1547abd723d2e27940592ccc29d6a84c50ebf3f14421` |
