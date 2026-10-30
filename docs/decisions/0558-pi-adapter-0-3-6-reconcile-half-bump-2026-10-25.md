# Decision 0558 — Pi adapter 0.3.6 reconciles the 0.3.5 half bump

Date: 2026-10-25. Status: accepted by the orchestrator lane (V1-2670, beamfall/corvint#1766). It
applies the existing `AHI-020`, `AHI-024` and `AHI-071` rules and adds no new requirement.

## Context

Commit `2ac3d2255` moved only the Pi package manifests (`package.json` version and
`corvintIntegration.adapterVersion`, both compatibility manifests) to 0.3.5 for shipped content
from #1713. The emitters (`runtime.js` `PI_ADAPTER_VERSION`, the `workflow.js` tuple and
`dogfood event` argv), native `adapter pi` and `dogfood event` admission, the specs and the README
stayed at 0.3.4. The installed pair therefore ran as 0.3.4 under a package labelled 0.3.5, and the
PWV-V0-011 inventory test failed in every scheduled full gate.

Aligning the emitters to 0.3.5 would change shipped files (`runtime.js`, `workflow.js`, README)
under an unchanged manifest version, which `AHI-020` (`make host-package-versions-check`) refuses:
a host that caches by version would keep the old 0.3.5 content.

## Decision

The Pi adapter moves to 0.3.6 in every location at once: the package, both compatibility
manifests, native `adapter pi` and `dogfood event` admission, the runtime and workflow guards,
AHI-024, AHI-071, LCP-V0-008/009, the PWV-V0 inventory, the README and the tests. Adapter 0.3.4
and the never-emitted 0.3.5 refuse visibly. Host version stays 0.99.1 and support stays FALLBACK.
Recorded evidence fields (`lastConformance`, `hostVersionEvidenceAdapterVersion`) keep naming
0.3.3. A new node test fails when any emitting or admitting literal differs from `package.json`,
so a later half bump fails at PR time.

## Rollback

Revert the change. Restoring 0.3.4 requires replacing the Pi package and the Corvint binary as a
pair, and reverting the #1713 shipped-content commits so `AHI-020` holds again.
