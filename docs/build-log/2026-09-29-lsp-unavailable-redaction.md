# LSP unavailable-reason redaction — V1-0167

Date: 2026-09-29. Base: `29274889ac6cf78a2c2029d64b1a6601f537441d` (`origin/main`). This repair narrows the Go/gopls provider's untrusted diagnostic boundary. It changes only opt-in LSP failure text; Core ranking, default packet bytes, query budgets and provider success records are unchanged.

## Reproduction and decision

The earlier V1-0167 fix replaced the repository root in a gopls error, so a normal in-repository URI became relative. It still copied the rest of the server message. A focused adversarial test made a fake gopls response include the queried URI plus an unrelated `file:///Users/...` URI and a fake token. Before the repair, `TestExpandEveryQueryFailedIsUnavailable` failed because the `unavailable` reason included both untrusted values verbatim. This is a real packet-disclosure path even though the test values are synthetic.

When every query fails, the reason now names the method and the admitted repository-relative query origin. It classifies the recognized `no package metadata for file` diagnostic without copying its text; all other query errors use `server error for file`. The query and failure counts remain available separately. A second test checks that an unknown error cannot smuggle a path or token, and two independent temporary module roots produce the same reason. An independent reviewer found a nearby foreign-server identity path that could also echo an absolute file URI. That reason now says only that the server did not identify as gopls; a test supplies an absolute file-URI identity. The EEP-V0-026 reason contract was updated in the same change.

## Evidence and limits

The new adversarial test failed on the base with the unrelated URI and fake token visible, then passed after the repair. The full `internal/lspprovider`, `internal/lspevidence` and `internal/extevidence` packages passed for the query-error repair. After the foreign-identity change, the full provider package and selected external-evidence and CLI degradation tests passed, as did focused spec/requirements/traceability/citation checks. The independent read-only review found no actionable issue in the original query-error fix and no remaining finding after the foreign-identity repair. It did not execute tests. The two-root test establishes root-independent output locally; CI on another host is still needed before claiming a cross-machine observation. Repository-wide and live external-server qualification remain `NOT_RUN` for this scoped repair.
