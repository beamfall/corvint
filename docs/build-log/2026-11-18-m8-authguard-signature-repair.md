# V1-5621 AC2: signature presentation repair

Requirements: `ASD-V0-560`, `CIN-V0-006`. Base:
`84a977dc5721a840159c369f1ab4fce91ce2a2d1`; pre-repair head:
`24f63a448f1acd9af247e2e5641391baceb8dd77`.

The review found that `log.showSignature=true` prefixes signed-commit output and shifts
raw identity fields beyond the scanner's labels. An ignored scanner failure then accepts
a prohibited committer email. This is a framing and error-propagation class, not another
prohibited identity spelling.

Machine-readable `git show` calls now explicitly disable signature presentation. The scanner
accepts only exactly four ordered raw fields: author name, author email, committer name,
committer email. Additional or missing fields refuse, and its caller checks failure using
the existing identity-read diagnostic and exit 2. Known prohibited identities still exit 1.
No wire, diagnostic, golden or existing assertion changes. Rollback: revert this repair;
doing so restores the observed bypass.

The full `script/check-ai-coauthor-trailers_test.sh` suite passes. Its new real SSH-signed
commit has a clean author and only a prohibited committer email. Signature verification
and extra presentation are asserted before expecting refusal. Two unknown-shape cases
inject otherwise clean three- and five-field output; both refuse with exit 2 and never
report a clean range. The same suite against the pre-repair guard fails at the signed
commit: unbound label followed by a clean-range report. Existing assertions remain intact.

`corvint affected --base 84a977dc5721a840159c369f1ab4fce91ce2a2d1 --format json`
reports UNKNOWN, 330 selected units, four unknowns and 25 unresolved packages. Full
selection is not executed under the lane charter. An initial concurrent edit invalidated
the first plan (`unsupported-affected-drift`); a stable rerun succeeded before the final
test run. No Go package source changed in this repair. Independent review and broad
verification belong to the host. CEM binding remains unavailable under decision 0594.

Raw evidence remains outside the repository in `~/lanes/m8-authguard-evidence/`:

| Artifact | SHA-256 |
|---|---|
| self-review-before.log | dcb9fd1bd6bbd09dc39c941da79b888bfbbc158e1b39eb7a25051b55acb97894 |
| self-review-guard-test.log | b7635351cb03a43a7946683b7c94ea9bda407a5b74a41ddc5b7eaba6ea3e2b7c |
| self-review-affected.json | bebed85fc06bf4ed0854d970f0a71861f3189fe4c4da196b159302b146cb4e77 |
