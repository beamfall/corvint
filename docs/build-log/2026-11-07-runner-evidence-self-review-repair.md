# Runner evidence self-review repair

V1-5103; RSP-V0-007, RSP-V0-008 and RSP-V0-049. The independent findings
identified a dropped lexical dependency for repository links to outside targets,
and rejection of valid native Go coverage blocks with equal endpoints.

Count-only outside reads now require both lexical and resolved paths outside every
declared repository and a class in the closed set toolchain, cache, temporary.
Repository links to any outside class mark reads incomplete with the existing
cross-repository-link reason. Unknown classes remain incomplete. The frozen path
schema cannot represent an outside target, so this repair chooses abstention over
inventing a repository-relative target. TestRSPV0007OutsideLinks covers all three
known classes, other, and an unknown future class, with and without a repository
lexical path. Counts are retained without publishing outside paths.

The legacy coverage adapter admits nonempty ranges under the existing component
validator. Only native Go (language go or the legacy empty language) with positive,
equal endpoint coordinates receives zero-width normalization: retain the file blob,
omit the unrepresentable range and mark executed incomplete using the existing
legacy-record-incomplete reason. This deliberately gives up narrowing for those
records; expanding an unexecuted point could incorrectly claim unexecuted bytes.
Malformed ranges and unknown zero-width language semantics still refuse adaptation.
No wire fields, range validation, errors, goldens or existing assertions change.

TestRSPV0008NativeZeroWidth generates an actual Go cover profile containing empty
functions and switch/select clauses, requires executed and unexecuted zero-width
blocks, builds and decodes the immutable index, and checks adaptation for both Go
language representations. Nonempty ranges and their classification stay exact.
Unknown language and malformed-coordinate cases pin the fail-closed boundary.

Validation used Go 1.27.3 and script/heavy.sh, one whole touched package per command,
without exported commit identity. internal/testevidence passed in 0.127 s and
internal/linecov in 22.104 s. The required spec-requirements and line-citations
checks passed. Citation repin found no moved unchanged citations. The affected
receipt against 8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734 selected both packages and
retains 80 unknowns; its installed-binary-stale notice is not hidden.

Raw logs are under /home/russell/lanes/rspt1-evidence/. SHA-256:

- test-testevidence-self-review.log: 4c08b9c7a8cc5c42605fb429a983110d16f499bd15db5bb868793f56d3c47016
- test-linecov-self-review.log: f08f9f7476a2566a1ac4e14bcb38494a3ced916ef149bdbb7bafbd9f7fe8915b
- spec-citations-self-review.log: 4725f44722bd47044324e693e6cee9a364fd863299966855f91af8a03c3f2edd
- affected-self-review.json: db13ff39e01569032d3438adc9765a7ebf01edb472be494df8f0290735070671

Independent review and host-wide gates belong to the orchestrator. No Tasks write,
push, PR or seal is authorized in this round. Rollback is a revert of this repair;
it restores the two reported gaps, and does not migrate any persisted evidence.
