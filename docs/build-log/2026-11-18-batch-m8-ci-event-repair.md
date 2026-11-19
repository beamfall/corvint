<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-5621: isolate the PR event fixture

Owner: Russell Lewis. Date: 2026-11-18. Repair input: `811ceae0ac9fb3a55a97c995909976622f7324c7`.
Requirements: CIN-V0-006; existing identity boundary ASD-V0-560 remains unchanged.

The workflow test inherited the hosted runner's `GITHUB_EVENT_PATH`, so its no-event case
received the real pull request head instead of HEAD. Each invocation now explicitly sets
that variable to empty or its own private event file. The enclosing test sets an unavailable
inherited path to prove isolation and fixes its stub exit status independently of the host.

A valid event carries exactly 40 hexadecimal characters and must reach the checker as its
head argument. Malformed JSON, a missing head and a non-SHA head must fail before the checker
runs. Existing skip, missing-base and checker-refusal assertions remain. The Makefile and
production checker are unchanged. Rollback reverts only this test repair.

Whole `tools/ci-reuse-plan` package: PASS, 30.209 s, Go 1.27.4, through `script/heavy.sh` with
`-count=1 -timeout 30m`. Focused package vet and Windows vet pass on the final test input.
Two preliminary runs failed because their handwritten event SHA had 42 characters; the
final fixture uses `strings.Repeat("a", 40)`. Those failures are retained, not test exclusions.
Raw logs remain in `/home/russell/lanes/batch-m8-evidence/`.

The required live-repository run also found that this ticket introduced a twelfth audited
`localReads` entry while AFP-V0-3330 pins eleven. Remove only that optional local narrowing;
the identity test retains its declared Makefile read and conservative default `script/` scope.
This broadens test selection, preserves the audit assertion, and changes no checker behavior.

Retained log digests:

- `test-ci-reuse-plan.log`:
  `649c27432047bda6803c6518bb21ec95d15aa3ebd792d14d408a19668ad0c2c5`
- `test-ci-reuse-plan-final.log`:
  `981bd904079923e89a75f07642d81d2f22083cd428b05225048022f487d35f32`
- `test-ci-reuse-plan-final2.log`:
  `d416deceaede85035cec02235b0631c0126cb479988d60416eba7e890732905a`

Final whole `liverepo` package: PASS (157.746 s), with the original ownership and
eleven-entry audit assertions intact. Final log SHA-256:
`b742b99bed61e276b5493653be08996f14a3eb08255f1c4d02cd1b1873b7a3f4`
