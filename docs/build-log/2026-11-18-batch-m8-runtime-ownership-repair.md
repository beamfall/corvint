<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-5087: declare the runtime-edge projection reader

Owner: Russell Lewis. Date: 2026-11-18. Requirements: CIN-V1-942, AFP-V0-602, AZU-V0-002.

The required whole live-repository package run found `tools/ci-forward-row/runtime-edges.jq`
unowned in both ownership audits. Its only current caller is the literal `jq -f` invocation
in `TestRuntimeEdgeDegradationRecord_CINV1942`, which belongs to `tools/ci-forward-row`.
Declare that reader in the existing runtime-readers profile. This names the original consumer
without adding a language owner, changing the projection or asserting future CI integration.
The declaration preserves enclosing-package and literal readers. Revert this entry to roll back;
the ownership failures then recur. No JSON schema, frozen golden or test assertion changes.

The first whole `liverepo` run failed in 129.190 s: the two ownership audits named this file,
and AFP-V0-3330 counted twelve narrowed shell entries instead of eleven (repaired with V1-5621).
Its raw log remains in `/home/russell/lanes/batch-m8-evidence/test-liverepo.log`.
SHA-256:
65c673474eae90f74bff65b8a19d18282841b4740b137ef09fbb6945f3253913

Final whole `liverepo` package: PASS (157.746 s), with the original ownership and
eleven-entry audit assertions intact. Final log SHA-256:
`b742b99bed61e276b5493653be08996f14a3eb08255f1c4d02cd1b1873b7a3f4`

Whole `tools/ci-forward-row`: PASS (67.037 s), including CIN-V1-942.
Final reader log SHA-256:
`bf97c13ecd9c29a47b458661c2fb9014ada40a04c546a96dcf79cd8e0a783179`
