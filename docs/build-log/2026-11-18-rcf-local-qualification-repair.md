# RCF qualification stays on its input host

Ticket: V1-5692. Requirements: RCF-V1-010/013. Repair of the qualification subprocess adapter.

The heavy wrapper can offload tests, but does not transfer the qualification configuration,
private corpora or external report. The adapter now sets `HEAVY_REMOTE=0` and a host-local input
and report reason on its child environment. The parent environment and heavy lock settings stay
unchanged. All live runs through this adapter remain on the candidate host, including the two
ranking evaluations. No wire shape, output message, golden or existing assertion changed.

Class: execution placement of host-bound evidence, not an analyzer soundness gap. The permitted
execution location is the local candidate host for every adapter call, independent of configured
remote names. No remote-host allowlist, syntax classifier or unknown-form exception was added.
`TestLiveTest` configures offloading to an arbitrary remote and an empty local reason, then checks
that the child overrides offloading and supplies a reason. Existing command and event assertions
still cover uncached execution and rejection of skip/fail results.

Evidence retained outside the checkout at `/home/russell/lanes/rcf-enforce-evidence/`:

- `self-review-regression-before.log`: full package FAIL in `TestLiveTest/pass` before the fix,
  1.359 seconds; SHA-256
  `a61975efd682690a117c0e0644c98bb8004845963499bf2cf1f40c23d3308f3e`.
- `self-review-package-test.log`: full package PASS after the fix, 1.379 seconds; SHA-256
  `627c05e6f17f0181c01ee0ba7d58f0b860be21dbfb12fa3d1016f3aa23a27cb9`.
- `self-review-vet.log`: package vet PASS; SHA-256
  `5b51677986e62719685f2ae12a27aebac37ac1feff49b99a7f03179c88586833`.

Tests used `script/heavy.sh env GOTOOLCHAIN=local go test -count=1 -timeout 30m
./tools/rcf-currency`; vet used the same wrapper and package. No identity variables were exported
for checks. The preceding affected plan selected this package and retained UNKNOWN scope with
11 unknowns. Broader verification belongs to the host under this repair's focused-check rule.

Private live qualification remains NOT_RUN without its inputs. V1-5692 remains PARTIAL for the
hosted integration and readiness citation documented in the original build log. The CEM workflow
is removed under decision 0594; bind/check/seal evidence is NOT_PRODUCED. Rollback is to revert
this repair, which restores the known inability to qualify with remote offloading configured.
