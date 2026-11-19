# V1-5694 source validity and retained qualification

RMV-V1-008 cuts the report assembly at the command boundary. The package tests still
exercise the retired report, its selection and environment projections, and its seed-rule
evaluator as Go test fixtures. Production still needs the shared JS/TS and RSpec findings.
The example-join command calls both example readers, which call ValidityFindings.
UnselectedFiles reads runner configuration directly. Teardown owns its 256-file bound.
This split preserves the existing rule assertions and the kept commands' reports.
Reverting this repair restores the earlier package boundary.

For RMV-V1-009, TestQualificationReadsSuccessiveRetentions passed using two provider
emissions in the same fresh directory. The shared reader selected the exact stdout of
each emission, including after the producer created the evidence directory's ignore file.
The Node harness also checks the invocation kind, every required axis, an unknown hygiene
state, and an unrecognized housekeeping filename. Its two browser cleanup cases require
the opt-in Playwright installation.

The two launcher regressions execute Node child processes and the editor comparison loads
its module through a VM. Runtime edges identify these dependencies. A test owner covers
the fixture tree copied by the core launcher. The Make recipe names both harness tests
literally. Neither portable launcher regression qualifies an installed editor or browser.

AFU-V1-950 retains the provider and flow-ingest decoders. Their shared attribution test
ran in the complete appflows package. The complete testsource and JS provider package
tests also passed.
