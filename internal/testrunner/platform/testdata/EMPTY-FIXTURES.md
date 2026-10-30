# Native report fixture packaging

`native-report-fixtures.json` stores each named original report as base64 bytes
and SHA-256. Test loaders reconstruct only declared names and verify the digest;
missing entries or changed bytes fail. Native outcome assertions are unchanged.
The platform bundle also retains its two explicitly empty Swift stdout captures.

This lossless packaging reduces the changed-path count without raising the trace
admission limit and avoids metadata-only empty-file additions unsupported by the current
canonical CEM 0.2 patch reader. It changes no production parser, wire or limit.
Non-report build projects, source programs, runtime provenance and original
README files stay separate. Original captures remain in the expanded build
checkout and private raw evidence identified by the existing provenance.
