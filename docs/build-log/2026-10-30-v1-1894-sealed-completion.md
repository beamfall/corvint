# V1-1894: completion after the required seal

The owner requested a repair after the security change had completed at its bind, then
sealed its CEM. The old Stop worklist requested new checks on the seal. Those checks passed,
but every verify cleared the prior terminal; finish then attempted OCM preparation against
the deliberately removed shared CEM. The real public-command regression reproduces this
failure after successful canonical binding, report review and strict completion.

The owner's explicit request to fix V1-1894 so completion works after sealing amends
decision 0534 disposition 2 for the bounded LCP-V0-040 case; other cases retain their
existing refusal. It also supersedes DCW-V0-349's former experimental non-goals for
a shared CI gate definition and live closeout evidence, matching the ticket's original
acceptance criteria. CI and local precheck now use the same DOC_GATE_TARGETS list;
the fourteen targets, use-case conformance command and CI environment were compared
against the base and preserved.

LCP-V0-040 separates the current completion target from the evidence target. A narrowly
internal seal resolver requires one parent and precisely the unchanged regular-blob/mode
move to that parent's archive path. It grants no generic historical target override.
The continuation retains the original reviewed report set and observations, validates them
against the archive and bind, and independently requires the latest checks at the actual
seal. A fresh reviewed report set states both identities. The real checker then verifies
the parent-bound CEM/OCM with no cached-pass reuse, records its agreement in a separate
sealed-check artifact, and leaves the parent coordinator/outcome unchanged. Read-only
status revalidates the seal and retained evidence; repeated finish produces no duplicate
outcome. No checkout, commit, shared-map restoration or synthetic test result supplies success.

The bounded design review identified fixed-path/target coupling, stale-pass resurrection,
fake terminal restoration and reader mutation as the four principal risks. The chosen
continuation addresses all four without broadening aggregate-profile completion. Query or
impact abstention/deadline replay remains unsupported because those commands observe HEAD;
continuation refuses such reports rather than reinterpreting parent evidence at the seal.
The actual retained security closeout has PRODUCED query/impact steps and null abstention
digests. Ordinary `dogfood check` still refuses sealed HEAD; this recovery is an internal
entry used only by the enrolled workflow with a separately reviewed continuation.

Focused baseline localcompletion tests passed. The new public-command regression failed
at the observed missing-CEM OCM preparation before the fix and passed after it. Positive-first
controls exercise private report/outcome/current and historical log drift, a later failed
observation, unexpected ignored shared map, actual final-checker failure, repeated finish,
and current/evidence target reporting. Separate immutable-seal controls reject source/map/
name/mode changes, a symlink, multiple parents and a descendant. An initial mode injection
was incorrectly undone by git add; changing the real file mode and asserting the committed
mode reached the test corrected the fixture before acceptance. A direct-check fixture also
exposed an unset default CEM path; initializing it in newCheck restored its existing behavior.

Independent implementation review found and resolved two additional issues. Caller context
now reaches sealed report validation at every call site, with a cancellation witness that
reaches the actual seal resolver. Previously recorded coordinator paths and digests are also
required when present; a positive-first control changes the outcome and report digest together
and verifies that report binding still refuses the substitution. The final real workflow run
passed in 38.374 seconds. Shared CI/precheck wiring passed its parser regression and an
independent comparison of all fourteen ordered targets, the conformance command and environment.
The reviewer found no remaining implementation issue; live closeout acceptance remains separate.

Current local evidence is under `/private/tmp/v1-1894/`. Final frozen checks, independent
implementation review, the one-command live closeout transcript, original-worktree recovery,
CEM sealing, hosted integration and native ticket completion are parent-owned and pending;
this entry does not claim them. No repository-wide gate or complete host matrix was run.
Rollback preserves the old sealed worktree and evidence; an older binary cannot read the
new optional continuation state, so recovery requires a compatible runtime rather than
state deletion. Official-release freshness and installed-tool qualification are separate.
