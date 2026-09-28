# Issues 167 and 175: local qualification observations

These are caller-owned observations, not signed execution attestations. `manifest.json` binds retained bytes and the implementation files used for the final focused checks. Terminal checks are bound separately to the committed change by the CEM and local dogfood reports.

- #167: actual companion CLI plan → approved `execute-receipt` → offline `verify-receipt`; one killed control, with exact hook/native bytes and executable pin. The synthetic helper exercises the transport and classifier. It does not authenticate an external adapter.
- #175: real Beamfall UI pricing checkout and API phase read pass, with two intentional failing controls. Reviewed links, ingested run evidence, map/gaps/navigation and generated PROVEN/STALE/UNPROVEN documentation agree. Compiled CLI and MCP receipts match at the same commit. Inventory export correctly refuses absent adapter anchors; external provider export succeeds.
- Semantic faults: counter +1→+2 and Beamfall phase ID drift each cause the retained selected test to fail. Both selections use full-suite fallback with zero omissions; reduction is not demonstrated.
- `/3`: the full browser matrix plus retry assertion and retry infrastructure cases passes on Darwin arm64, Node22.23.2, Playwright1.63.0 and bundled Chromium headless-shell1243. System Chrome153.0.8010.54 is refused. `/3` admits no system tuple or Playwright1.60.
- Compiled MCP conformance passes on both protocol versions; official 2026-07-28 schema SHA-256 ef70b61f99b6d2e5e3b46863822eab08dff6a45bedc7a08914e0e5b133f40203 was supplied.

`beamfall-qualification.patch` retains the declaration, test and fault changes from the named upstream source. `corvint-qualification.patch` retains the real counter fault. Raw reports retain their original temporary paths; they are not portable artifact locators. Native screenshots and trace archives are not included. The Beamfall application uses its in-process mock backend on loopback; production backend behavior and hosted CI are NOT_OBSERVED. The harness checks process-group cleanup and SIGTERM interruption (`live-cleanup.json`).

Earlier failed qualification attempts remain in the private task checkpoint: obsolete system-browser expectation, missing version-probe working directory, harness checkout/default-argument error and concurrent MCP reply-order assumption. No failed attempt is counted as qualification.
