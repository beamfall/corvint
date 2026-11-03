# Spec capacity and early README row checks (V1-4331, V1-4332)

Owner intent: the 2026-11-02 spc-w47 builder request requires lane and batch warnings before
individually valid spec edits exceed the OCM limit, base-attributed overflow diagnostics, and a
cheap duplicate-row check before slow doc gates. Base:
`b486d3978728731c6638b50fa2210dfbb6e365ea`; branch `claude/lane-spc-w47-1102`.

The pre-change index counted 61 SRG, 224 ASD and 251 DCW requirements. SRG owns generation and
capacity (`SRG-V0-740` amended, `SRG-V0-744/745` added); ASD owns batch and lane integration
(`ASD-V0-2010/2011`). After the additions SRG has 63 and ASD 226, so neither needs a companion.
DCW stays unchanged, and no ASD-V0-1990..1992, DRS-V0 or DWC-V0 IDs are used. Requirement
acceptances are recorded as standing (2026-11-02), not owner read-back.

`make spec-capacity-check` reads the Git index, reports every spec within the configurable margin
(default 8), and fails above 256. At exactly 256, it fails only when BASE is supplied and at least
one section ID was added since BASE; unchanged or unattributed full specs warn with the companion
route. It names the companion route: new file, new prefix,
README/INDEX registration under AGENTS.md and SPEC-DRIVEN-DEVELOPMENT.md. The existing generator
still admits OCM's exact 256 and refuses 257 before output; optional BASE resolves once to an
immutable commit and reports sorted IDs added in each reported spec. Missing BASE stays
NOT_OBSERVED. Capacity retains the shared duplicate-definition refusal before reporting success. The bound itself is unchanged. The staged citation gate found two moved references: the
Makefile spec-check target and the generator's ID matcher. Both were read at their new positions
and mechanically re-anchored; the compatibility-runner spec changes only that citation.

`dogfood-closeout` runs capacity and README checks with explicit `BASE=$(BASE)` before its
input/build/bind steps. Batch static checks use BATCH_PRECHECK_BASE with DOGFOOD_BASE as fallback.
Batch-precheck runs both static checks before repins, planning, receipt reuse and slow gates, with
an explicit `--static-check` route. The README checker reads two worktree files using Perl/JSON,
names byte-identical duplicate rows and all line numbers in a fix line, and checks exactly one
unanchored file row per indexed spec. Product/decision rows outside docs/specs are excluded.
Status/claim/header conformance remains in internal/specindex. Revert the checks and call sites
together to roll back; the existing OCM and specindex guards remain.

Focused evidence: `TestSpecCapacityCombinedLanes` passes through script/heavy.sh; two fixtures
with 128 and 129 obligations each pass independently and their 257-obligation union is refused by
both Go OCM and generation. Base attribution excludes the original lane and includes the second
lane's IDs. The fixture also distinguishes exact-256 OCM admission from BASE-attributed capacity refusal,
and proves unchanged 256 with BASE and unattributed 256 both warn and pass.
Generator shell fixtures pass margin/default/invalid-input/multiple-spec/staging/atomic-output
cases. Focused batch case 1d passes duplicate locators, valid/missing/anchored-only rows and a
LANE=1 refusal for a spec filled to 256 since BASE before a repin marker can run, plus an
untouched full-spec warning/pass; both BATCH_PRECHECK_BASE and DOGFOOD_BASE are exercised. Bash compatibility tests pass. Full
spec-requirements-check passes internal/specindex and the OCM enumeration/bound checks; flow-guide
passes (45 spans, 16 mapped codes, 250 refusal sites). Exact commands and final citation/format
results are retained in `spc-w47.handback.md`.

Retained failures and limits: the first duplicate fixture exposed a newline-key lookup error in
the new Perl check; that was repaired and the focused fixture passed. The broad batch shell run
also reported an unrelated Pi adapter Node subtest failure without the expected PWV-V0-011
selector; that broad run is invalid/incomplete because the script was edited while its shell
was still reading it, and later ended at a syntax read error. The final file passes `bash -n`;
only the stable focused static run is counted as passing batch evidence. Orchestrator review
blocked the original unconditional exact-256 refusal because it rejected this baseline's already-full
corvint-tasks-agent-leases-v0.md and documentation-corpus-v1.md (each 256, no added IDs since base).
The owner's fix-round-1 decision makes those warnings and retains refusal when a change fills the
last slot or exceeds 256. No unrelated requirement migration is included.

Dogfood routes used: context (READY, no critical omissions, 11 omitted results), query, impact
(in-memory index fallback), and affected before tests. Evidence outputs are under
`/tmp/spc-w47-evidence`. Learning, mutation, providers and external interoperability are excluded
because this change owns repository gate scripts only. `make dogfood-change` refused at the
read-only worktree Git directory; CEM, reviewer report, frozen retrieval evaluations and local
outcome are NOT_PRODUCED. No claim of self-use qualification, independent review, native ticket
completion or landed integration is made. The builder was forbidden to commit, push, run Tasks
or codex. Independent review and post-commit dogfood binding remain for the integrating operator.

Sandbox staging uses a private index and object directory under /tmp, with the repository's
immutable objects as alternates. Generation and checks passed that snapshot; the actual Git
index is unchanged because its lock path is read-only. Integration must stage the delivered files
in the real index and repeat the staged gates there. No new public binary/command, network
service, ranking, authority or wire format is introduced.

Fix round 1 (2026-11-02): implemented the owner-decided BASE-aware exact-256 rule and explicit
base forwarding. Added-ID attribution now precedes the capacity decision. SRG-V0-744/745,
ASD-V0-2010, trace rows, the authoring contract and regression fixtures agree. The orchestrator
finding is the retained independent review; repeat-run results and remaining integration limits
are recorded under Fix round 1 in `spc-w47.handback.md`.

Round-1 result: all requested focused commands passed, including capacity and batch static checks
against the frozen base. The two full baseline specs warn with no added IDs. Generation reports
no changes; internal/specindex, OCM enumeration/bound checks, the Heavy-wrapped capacity Go
regression, both focused shell suites, Bash 3.2 fixtures, final line citations and gofmt pass.
The first citation run found one unchanged matcher shifted from line 459 to 460 by the header
comment; its meaning and unchanged hash were checked before repinning, then the gate passed.
Real-index staging and post-commit integration remain unverified under the retained sandbox and
owner constraints.
