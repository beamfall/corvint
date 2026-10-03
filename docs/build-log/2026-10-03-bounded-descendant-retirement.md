# Bounded descendant retention for V1-0689

## Decision and evidence

Owner-requested V1-0689 repair retains the existing observed PID/start cleanup contract and wire
fields. Independent Gate A resolved resident zombie continuity and recommended keeping the
existing 4095 displayed-descendant maximum, 4 MiB ingestion and current identity representation.
The coordinator accepted that smaller scope on 2026-10-03. Resident ownership remains bounded
including visible zombies; displayed witnesses are separate and omissions explicit. Duplicate
PID snapshots cannot establish retirement. Transient recovery semantics remain unchanged.

The bounded diagnostic at public base32aa5a3f2b7f3fe658d208049fb009e2c9e24c13 reproduced cumulative
4096-entry failure with at most three simultaneous rows. Nested three-second off/on observation
retained 2/90 identities; zero named sampler matches leave attribution inconclusive. Both
cancellation cases passed with independent absence readback. Claim230 returned at receipt2326.
Same-PID replacement at the cap is source-confirmed, runtime NOT_RUN at this seed; the original
replacement control was below the cap. It is retained on V1-0689 at receipt2329.

## Implementation and focused evidence

The seed is372d40120017a1c4aa39828a744728f5f8543cad. The old-source
`TestV10689BoundedChurn` compiled and failed specifically on the cumulative4096 bound with
at most three simultaneous rows. Production hashes still matched32aa at that RED. The repair
separates the immutable root, resident descendants and historical witnesses; successful
absence/reuse retires residents before deterministic admission. Visible zombies preserve
ancestry and never become live cleanup targets. Duplicate PID tables are wholly refused.

Focused GREEN covers churn, zombie continuity after root loss, root-PID reuse, both exact-cap
replacement cases, sticky overflow, deterministic bounded witness selection and parser refusals.
`TestV10689BoundedCancellation` acknowledged a separate-session child, joined four short children,
then cancelled and joined the escaped child and root while preserving a sibling sentinel's
PID/start identity. Darwin arm64 passed; independent UTC/C host readback found all acknowledged
fixture identities/groups and supervised jobs absent after their joins. Other OS execution is
NOT_RUN. Existing process-group cancellation tests remain part of the final selected checks.

NEA and freshness codec tests preserve sampled process rows and the omission limitation through
existing fields; survivors/failures do not promote successful cleanup or current freshness.
Legacy canonical freshness bytes roundtrip unchanged. A cancelled-provider Execute fixture
accepts a fitting aggregate and refuses an oversized encoded report via the existing16MiB bound.
This exercises the bound, not a guarantee that all maximum witness/repeat combinations fit.

The private supervisor initially refused an unrelated Darwin `?N` state after an affected
command exited0. The builder launched synthetic GREEN before inspecting that HOLD; the original
HOLD and chronology remain retained. V1-0692 records this temporary harness friction, not a
production observer defect. Independent narrow review accepted `[A-Z?]` as the leading state
class while preserving non-Z as potentially live. Fourteen no-child mocked discriminators passed
before resumed execution; original scripts and raw evidence remain unchanged.

Private raw RED/GREEN, supervisor bindings, process readbacks and enrollment are retained under
`/private/tmp/corvint-0689-repair-proof`. Pre-change dogfood-change retained NOT_PRODUCED for absent
CEM map, intent input and outcome; later binding does not replace that chronology. At this source
commit, independent source review and the seven frozen post-commit checks/final dogfood seal are
pending. No production delivery or native completion is claimed. V1-0691's suspected stderr
resource risk remains separate, runtime NOT_RUN, and is not a prerequisite.

## Limits and rollback

The observer remains bounded sampling, not atomic kernel identity or full process containment.
Witness rows do not promise exhaustive historical identity replay. Aggregate report-bound
refusal remains; no maximum-size fit claim. Original parent state, consumed N32 marker, original
unqualified N32 and provider-validity-incomplete remain unchanged. No new N32 is authorized.
Rollback reverts this isolated repair and clarification; it restores the known cumulative-cap
blocker and does not erase either version's evidence.
