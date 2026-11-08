# Refuse lossy text at the runner evidence boundary

V1-5103; RSP-V0-006 and RSP-V0-007. During the final input audit, the record
encoder's standard JSON string replacement exposed a fail-closed gap: an invalid
UTF-8 test identity or filesystem path could serialize as a different, valid
string. Likewise, a lone UTF-16 surrogate in JSON could decode as a replacement
character. Neither transformation may change a dependency key silently.

The record validators now reject invalid UTF-8 and NUL in identity, path, body-map
and reason strings, and strict JSON decoding rejects lone surrogates. Valid Unicode
including an explicitly supplied replacement character still round-trips.
`TestRSPV0006RejectInvalidText` pins both outcomes; no existing assertion, golden or
wire field changed.

The first checked bind completed while this issue was being examined. The charter's
one corrective-rebind allowance is used after this repair, with repeated whole
package tests and affected-input checks. The previous bind remains in history;
no commit is rewritten. The measured initial index size remains applicable because
this repair changes validation, not the record encoding or fixture payload.
