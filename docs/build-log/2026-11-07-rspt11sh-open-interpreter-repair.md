# Open and implicit interpreter dependency repair

Owning tracing slice: V1-5106, RSP-V0-007, RSP-V0-021, RSP-V0-022.
Consumer lane: rspt11sh / V1-5113. This repair addresses only the two supplied
self-review findings in the inherited tracing implementation.

Successful ordinary opens retain a read dependency even for O_WRONLY: their
success can depend on an existing path without any later read syscall. Exclusive
creation and previously proven creation retain their existing suppression rules.
TestRSPV0007WriteOnlyOpenDependency opens and closes an existing file without
reading it, requires its dependency, then removes it and requires exit 91.

Outside executable classification now filters only the executable digest entry;
it does not skip ELF interpreter inspection. TestRSPV0021OutsideExecutableInterpreter
links an executable under a declared toolchain root against a copied,
repository-owned ELF loader, executes it, and requires the loader read dependency
while retaining no outside executable entry. Unsupported executable formats still
use the existing incomplete reason. No wire shapes, names, reason codes, exit
codes, stderr text, goldens, freeze outputs or existing assertions change.

Decision: inspect outside ELF images using the existing interpreter path instead
of unconditionally marking outside executions incomplete. This retains completeness
when the implicit dependency is observable. Rollback is a normal revert; there is
no stored-state migration. The source branches establish the reported causes;
Linux runtime reproduction is NOT_RUN on the available Mac.

## Verification and retained limits

Corvint affected against 8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734 selects
internal/proctrace with BASE_UNREADABLE fallback. The broad inherited range has
scope UNKNOWN, 547 selected units and 96 unknowns; self-binary-stale remains.
Whole-package proctrace tests through script/heavy.sh passed before and after the
repair on the local Mac (0.563 s and 0.603 s). Linux/amd64 go vet of the whole
package passed, including the new tests. Live Linux tracing and the two new
runtime regressions remain NOT_RUN pending host verification. Independent review
is reserved for the orchestrator.

Citation repin found no moved citation content; line-citations-check passed.
The required broad-base changed-checks failed on inherited unbounded-reader and
test-read-scope inventories, including the earlier script plugin's undeclared
.github/cishards/test-commands.json read. These are outside the two-finding repair
scope; no checks or inventories were weakened. Final CEM and dogfood-check results
are retained in the external handback. No pre-change capture is rerun or seal made.

Raw evidence lives at /Users/russelllewis/lanes/rspt11sh-evidence/.
repair-proctrace.log SHA-256:
b84fa5f92d06a5e3ffaf0c9bb5fdc69e96941ecf625a57fe031c0a02e1d6ef81.
repair-linux-vet.log SHA-256:
678f6f5e8699177349b37a51ae13a62b7d3692ce7733a8b69ee768dccd4344f9.

Core and Tasks remain 1.0.0-rc.3.dev build 128. Official update checks refused
because process group quiescence cannot be established under host restrictions;
available versions and freshness remain UNKNOWN. No failed verification was
bypassed. Usage and live model-control telemetry remain NOT_OBSERVED. No model
reviewer, Tasks write, background job, publication or tool installation was run.
