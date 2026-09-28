# Bounded navigation execution candidate

Date: 2026-09-28
Task: V1-0252 / AFU-V1-029
Base: 2e281305c49badc68d1a46035ca4ff966cbaff7c

The original navigation packet carried prose and grants but no executable operation. Its synthetic
agent tests did not prove browser execution; the AFU-V0 scenario observer neither consumed packets
nor applied effect admission. Gate A identified two material risks: hidden/reset traffic could write
before admission, and prose could be misinterpreted as executable instructions. Review then rejected
extending existing closed intent/packet schemas. The chosen proposed technical profile uses a
separate committed execution input and leaves both schemas unchanged.

The companion rederives the packet at one commit, independently checks the caller's maximum effect,
loads committed disposable origins, and refuses unsupported mappings before startup. Browser
requests and both native/programmatic forms are gated before dispatch. There is no implicit reset.
The existing bounded process-group runner and owned browser lifecycle are reused; navigation assets
join only navigation's provider identity, preserving the original observer's identity contract.

The first compiled real-browser proof passed navigate/readiness/fixture fill/granted click/visible
outcome, default-read zero writes, and a declared-read click whose POST was blocked before mutation.
Expanded development qualification passed absent/uncommitted/unlisted origins, initial background
writes, redirect/cross-origin/WebSocket refusal, GET forms/direct submit, precondition order, bounded
recovery, unknown fixture and SIGINT/SIGTERM/timeout cleanup. The timeout case asserts the actual
process timeout diagnostic. Node22.23.3, Playwright1.63.0 and its bundled Chromium153.0.8010.12 were
used; the already-installed bundle was exposed at the default cache path without host Chrome.
Exact final-candidate qualification and independent review remain required before integration.

Failures retained: initial fixture omitted a required outcome value; adding navigation assets to
legacy identity broke its fake-provider cleanup test; both were corrected before qualification.
A missing default-cache bundle and non-exported package metadata subpath initially prevented launch;
the exact previously installed bundle and resolved package metadata corrected them. A duplicate
fixture variation ID initially stopped precondition admission. No failed run was labelled passed.

Corvint context initially selected unrelated decision0086, omitted one ranked result and withheld
16 test candidates; original receipts are retained. Go impact does not establish JS coverage.
Enrollment accepts only the base-present AFU-V1 contract; the new profile is proposed/base-absent,
not retroactively accepted authority. Initial dogfood change retained missing CEM/intent-scope and
outcome inputs as NOT_PRODUCED. Native task completion, full release gate and integration remain
root-owned. No API observer or AFU-V1-014 per-test LOCALLY_OBSERVED claim is made.
