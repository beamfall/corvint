# Dangling symlink dependency repair (V1-5106)

Date: 2026-11-07. Intent: RSP-V0-007, RSP-V0-012(e), RSP-V0-021, RSP-V0-022.

Independent review found that a missing expanded symlink target was discarded;
resolve's lexical fallback retained only the link. Adding missing/data behind
alias -> missing/data could change the observed result without selecting the test.
This repairs the missing expanded dependency class, preserving the result schema,
existing diagnostic reasons, goldens, and all existing assertions.

## Decision and closed forms

Continue component-wise traversal of absolute ordinary filesystem paths, directories,
regular files and textual relative/absolute links (at most 40 hops). On ENOENT,
certify only a suffix composed entirely of ordinary nonempty entry names: each
slash-separated component must be neither dot nor dot-dot. No suffix is also
admitted. Retain that expanded target and every repository ancestor as lookups,
with identical lexical/resolved repository identities; retain traversed links as
reads. This covers direct, absolute and chained dangling links without inferring
which future addition will make the operation succeed.

Every suffix outside that grammar is unproven and uses the existing
symlink-resolution-failed incompleteness reason. In particular, missing/../data
cannot be cleaned to data: the missing component prevents the kernel from reaching
it. Other resolution errors, nonregular objects, procfs, relative inputs and cycles
retain their existing fail-closed behavior. Missing suffix certification does not
broaden the existing supported filesystem model or qualify a runner.

Rollback: revert this repair; no migration is needed. That restores the reviewed
soundness gap and must not be treated as qualified selection evidence.

## Evidence and limits

TestRSPV0007DanglingSymlink exercises relative, absolute and chained targets,
requires present reads/lookups and exact expanded target/ancestor identities,
then proves that adding missing/data alone changes the failed open to success.
TestRSPV0007UnknownMissingSuffix proves fail-closed handling of missing/../data
and that creating the missing directory changes the open outcome. Existing
syscall, rename and chained-link assertions remain unchanged.

The entire internal/proctrace package passed through script/heavy.sh with no
-run filter (2.743s package time); focused vet also passed. Test log SHA-256:
d1cc6539de8915aa5b8d7120947029572e0f1d11d2d61e2925768c1e74c6685e
Raw evidence is external under /home/russell/lanes/rspt4-evidence/review-tests.log.
These are correctness observations on a shared host, not performance claims.

The required affected plan used base 8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734
and selected internal/proctrace. Plan scope remains UNKNOWN; other selected units
and qualification belong to host verification under the focused repair contract.
Pre-change evidence is reused; final CEM/report binding follows this repair.
Whole-repository checks, model-backed review, Tasks writes, publication and sealing
are NOT_RUN by owner instruction. V1-5106 acceptance still awaits host qualification
and integration; the isolated trace service is not a qualified runner.
