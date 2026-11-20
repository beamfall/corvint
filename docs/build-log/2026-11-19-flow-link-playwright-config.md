# Playwright exclusion and execution bounds

The round 3 amendment to FLP-V1-010 requires the closure and the admitting project's resolved
testDir to cover every changed file. Checkout can fetch an application file over HTTP even when
only search imports it. The closure still supports exclusion of search's own spec under testDir.

For FLP-V1-007, discovery and canonical inventory both contribute config identities to the
in-process execution plan. The session executor identifies Playwright invocations by runner and
root. A declared provider likewise selects files by path. Two configs at the same root can
therefore select distinct projects that the executor cannot address independently.

The CLI uses the authorized refusal for these selections. It reports
`UNKNOWN playwright-config-unrepresentable` with the spec path and sorted config paths before
building or executing the manifest. This also covers a single selected config absent from the
executor's owning instances. The existing receipt and manifest formats stay frozen.
