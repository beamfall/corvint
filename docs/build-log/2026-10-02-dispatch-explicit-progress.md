# Explicit dispatcher progress: experimental source qualification

Issue 468 supplies the human-owned intent: programs can declare progress independently of work-state
role matching. The selected command token keeps the existing state contract and durable-attempt
fingerprint. CAL-V0-064/S14 remains a proposed technical contract with experimental implementation;
source qualification does not claim human acceptance, installed release qualification or completion.
S12 CAL059..061 and reserved S13 CAL062..063 remain separate. The reservation proof is integrate456
commit 14eaf5e251691976b6cc81421c9d07a44c701dbe, CAL spec blob
74aff5bd11116bab050dfb7731b5fdbb232fd0fb.

The implementation stages bounded SHA-256 replay history and token-dependent worker/backoff
accounting in a cloned ledger. A checked atomic write precedes publication and effects. First tokens
seed baselines without credit; duplicates and observed A-to-B-to-A replay keep the accepted digest.
The lifetime limits include seeds and deleted keys: 256 digests per key and 8,192 per program,
allocated in full-ticket-ID byte order. Pending active-worker credit survives UNKNOWN state until a
healthy observation grants once. Tokens remain producer assertions, not proof of referenced work;
previously unseen old assertions cannot be identified as stale. Ordinary roles still match state.

Independent review of a86efafbc3871c79a77c0fe526a3e09e112b29f8 found one HIGH: Go's case-insensitive
struct field matching let aliased JSON members erase or regress replay history. Actual red tests
reproduced 13 accepted alias cases. Repair fbc80a5e1de6261ea1ce5290a4aa6451fd0c0b2f, tree
cdc41201eb3d8dc874e88bb67eeda63a5d66277f, detects progress aliases before decoding and validates
canonical static fields and decoded duplicate keys while preserving dynamic case-sensitive keys.
The same independent reviewer passed this bounded repair with no remaining HIGH/MED findings;
unchanged source retains the earlier review. Private review receipts are
`/private/tmp/dispatch-468-independent-source-review.md` and
`/private/tmp/dispatch-468-repair1-independent-review.md`.

Focused source evidence on macOS, Go 1.27.1:

- Full dispatch suite: PASS, 5.850s (`/private/tmp/dispatch-468-repair1-dispatch-host.log`).
- Actual CLI file/token, park/relaunch, restart and stale-replay consumer: PASS, 1.444s
  (`/private/tmp/dispatch-468-repair1-cli-host.log`); disposable native fixture bytes stay unchanged.
- Vet of dispatch and CLI, formatting and diff checks: PASS.
- Tests retain actual failed admission writes, later save failure without resurrection, first-seed
  controls, pending UNKNOWN accounting, pre/post-commit cancellation, sorted capacity allocation,
  lifetime boundaries, strict history/baseline loading, case-sensitive keys and byte-preserving reads.

Host process-table access is part of these supervision witnesses. The sandbox run failed because
its process observations were unsupported; its log remains separate at
`/private/tmp/dispatch-468-final-dispatch-suite.log`. An earlier CLI fixture assumed `/bin/true`, which
is absent on this host; resolving the executable from PATH repaired that fixture. Both failed logs
remain retained. No Linux runtime, Windows, installed Tasks release or broad gate qualification is
inferred from these macOS observations. Reader lifecycle issue 464 remains separate.

Provenance and bindings remain explicit. At public/native origin
1fda1b94984245d0cd0ac0a6d17cc72572ce6619, controls show that the old reader rejects object output
and ordinary file changes alone leave its fingerprint unchanged. These controlled observations do
not requalify the reported historical INV-1 sessions. The genuine initial dogfood pass preceded
tracked edits and refused empty BASE..HEAD: CEM prepare reported git-diff-failed; CEM/OCM were
NOT_PRODUCED and local outcome no-source-paths. Original prechange agent-receipt-absent notes and
actual independently captured receipts remain retained. Preparation commit
0b6bbe062f7f0274941a38ad003f9820c8112774 established the proposed numbered clause before the
original source enrollment; public1f-to-preparation provenance remains part of the whole review.

Original enrollment key 5c37eb83d2e8d8bbf633b520371c9f852d80ca9cefb03db3d2a28acb79c75ca8 and
base 0b6bbe062f7f0274941a38ad003f9820c8112774 remain unchanged. Manual source checks are distinct
from the four frozen enrolled checks: dispatch, dispatch-cli, dispatch-vet and focused-docs.
At the source checkpoint these were unverified and CEM/OCM reports and the final checker were unmet.
Source generation 142 was released COVERAGE_UNKNOWN in receipt 2021; readback CANCELLED and the
receipt audit CONSISTENT/AGREES retain semantic coverage UNKNOWN. This releases ownership, not the
ticket's acceptance or completion.

Optional OCM obligations stay unassessed, meaning not assessed by this change. Existing Go function
names do not contain the exact hyphenated CAL-V0-064 token required by explicit claim linkage;
manual traceability names the actual tests without claiming unsupported OCM links. This known
selector/anchor workflow friction is retained in V1-0275 and related guidance V1-0555. No source
case was added solely to manufacture an ID, and no placeholder selector or manual mark is used.

Remaining delivery evidence is the exact clean-target frozen checks, CEM/OCM and report-set inspection,
keyed outcome/final checker, clean postcommit binding/check/seal, public integration and supported
native completion. Metadata and binding inspection covers these new changes without repeating the
passed unchanged source review. No token/cost savings are claimed. Rollback preserves current ledger
history and later worker/accounting facts; an older reader's refusal is a fail-closed downgrade,
not permission to strip progress fields or restore an old snapshot.
