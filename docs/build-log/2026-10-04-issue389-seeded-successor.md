# Issue 389 seeded successor: immutable delta intent seed

Human-owned intent: GitHub issue 389, native V1-0536. The owner approved the seeded-successor
decision packet `OWNER-DECISION.proposed.md` (SHA-256
211f112fb92b232e5cba02fb1043aff25e3fcc51926c277532f7b439d95433cd) on 2026-10-04. This entry
records that decision and the seed's provenance once; it is not a delivery or qualification claim.

The original #389 integration enrollment stays a NON_SUCCESS: original BASE
79bd635d91f0f3944c781882ae6f6d4bb968d58b, session key
61668a08d33c7865786bc2f8fb8cf982997d7072c78fae7ad8a81d9f92c261cd, plan
94aa3939437eed96da6955440485e861e8fe7489a9804b937ddef7742d026fea and its ten checks. Its last
report set was produced at 469969beaed20f72228525bead0b57ab9766ee85 and the retained candidate is
4be1a767a5390f6fd2bb14f08c7d4dd2a877689b. Its live evaluation is satisfied=false. The explicit
`dogfood cancel` belongs to its own checkout and is not recorded by this entry. Those are
historical evidence only. The successor neither resets nor requalifies that enrollment, and it does
not revoke any genuine historical check.

The seed adopts the reviewed `immutable-delta-v0.md` intent from 79bd onto public main
4b10a02144faed4a77b36eba4b0d1cbc315706d3. The spec bytes at 79bd and 4be are identical (SHA-256
16f2646987bb7d0bf0dfd87bd0874d434fc5d785a19f6bc7b15e6d86698afb9b). The seed adds four paths: the
spec, its `INDEX.json` record, its README row and its ten generated `REQUIREMENTS.tsv` rows. The
record and the row are the exact 79bd bytes, inserted at their 79bd neighbours, and every other
catalog entry keeps its main bytes. Intent status stays proposed and delivery stays experimental.
No numbered requirement and no original human acceptance criterion changed.

The only difference from the 79bd spec is a `(PLANNED)` marker after each of the 30 named
traceability tests that main does not define yet. The `## Requirements` block and the line count
are byte-identical. Each marker comes off only when the test it names arrives with the merged
implementation; no stand-in tests are added. Statements in the spec that some source exists
describe the retained 79bd/4be branch, not delivery on current main.

At the seed, source, tests and qualification of the delta capability are NOT_PRODUCED. The seed is
bound under its own documentation enrollment, which uses the bootstrap profile for the new spec
and checks only the documentation. Its checked seal becomes the successor's evidence BASE. The
successor enrolls a new key and reruns all ten original checks, unchanged and fresh. The original
four-class human acceptance is still PARTIAL, and the native docs-only class stays open under
V1-0579. This decision completes no ticket.

Rollback: revert the seed and seal commits. Nothing else depends on them until the successor merge.

## Successor enrollment

The seed's documentation enrollment (key
3e09dda7295876cf702c1349358155b6f95805d3986bb42c2aa54d2c89ee6fbe) finished satisfied at its bound
target 541b6462468b5cf0d5db23d3aa070e980b24ac73, and `dogfood-check` and `dogfood-seal` passed.
The seal c58325c4b128d69828640952b421d660a5aebc7b made that key stale, so it was cancelled as the
decision requires. Its pre-seal result stands and is not a failed seed check.

The successor key 02e7d9fcf8283d9beb6669260414a80502d96a43f7bf7412c5619a374b815c81
was enrolled at c58325c4 with plan digest
3ee04d546ffbf10134bab4fceb340e098140424e99826a7450c175c86f6c1849: the three original intents and
all ten original checks, unchanged. The retained candidate 4be1a767 was merged ordinarily. The three
catalogs merged to the seed bytes, the DLT spec took the candidate bytes so its `(PLANNED)`
markers came off with the arriving tests, and main's archives 0ab7b241 and 1722ce93 keep their
BASE bytes. The candidate's historical shared CEM (SHA-256
01f1d27210f86ab1a816dbeae7fd2b5e8ea5dda10c985c5f17f4ee3d6d4abd76) is not carried, and a fresh map
is bound against c58325c4. The candidate's `2026-10-04-immutable-delta-current-main-preservation.md`
entry describes the candidate branch's own archive disposition, not this successor's.
