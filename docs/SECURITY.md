# Operational security and support

The repository's [security policy](../SECURITY.md) owns vulnerability reporting and the existing
alpha support statement. It currently specifies private GitHub vulnerability reporting, security
fixes for the latest published alpha, and acknowledgment within seven days. This operational note
adds no support promise. Confirm the reporting channel works before a future release; no live
report was sent or reporting-channel availability qualified by this change.

A stable support duration, supported upgrade/downgrade range, end-of-support notice period,
maintainer escalation/contact fallback and stable response commitments remain **draft owner
decisions** for V1-0017. Do not infer them from the alpha policy, a ticket's OPEN state, a fixture
pass or a release gate. The [changelog](RELEASE-NOTES.md) remains the release history.

The default Core is one native Go binary with no account, permanent daemon or external database.
Installation uses the existing verified candidate and version/platform store. It neither installs
services nor changes credentials, global trust, shell selectors or another installation. Candidate
checksums are integrity evidence; the alpha's unsigned artifacts are not authenticated by them.
Execute only a candidate whose provenance you trust. A successful version probe is not a sandbox
for an arbitrary malicious executable.

The candidate verifier admits 15 files, five directories including the root, at most 512 MiB per
file and 1 GiB total input, using bounded directory reads. Decompression and copies use additional
memory; this is not a 1 GiB RSS cap or a hard whole-verifier time limit. Both version probes have
30-second execution limits and one-second shutdown allowances, retain at most 4 KiB per stream,
and suppress their output in error messages. Owned process-group cleanup is required. Deliberate
process-group escape, concurrent adversarial filesystem replacement and power-loss recovery remain
unqualified. The installation store requires exclusive trusted-local ownership.

Before sharing diagnostics, inspect them for repository paths, source contents and secrets. The
existing secret-screening regressions cover traces and product evidence; suppression of candidate
probe output adds a narrower boundary, not a guarantee that arbitrary diagnostics are secret-free.

V1-0017 stays open: V1-0008 and V1-0015 predecessor evidence, exact future immutable release artifacts,
per-platform native lifecycle runs, hostile-repository/security qualification, full gate and accepted
stable support policy are separate requirements. No artifact is promoted or published by this work.
