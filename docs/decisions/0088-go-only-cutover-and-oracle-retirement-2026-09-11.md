# Decision 0088 — Go-only cutover and retirement of the Python oracle

Date: 2026-09-11. Status: accepted. Authority: repository owner, “we should be fully moved over
to golang and the old python oracle code should have been deleted” and “stop doing that
measurement. the python code needs to be gone”.

The owner cancels decision 0086's unexecuted paired retry and orders removal of the legacy Python
engine/oracle and Python packaging. No formal retry-2 run started. Its preregistration and earlier
receipts remain historical evidence, never qualifications. The implementation contract is
`docs/specs/go-only-cutover-v0.md`.

This supersedes the mandatory live Python cross-check, coexistence/retirement waiting windows,
Python fallback and wheel prerequisites in GPK-V0-002/017/018/019/021/023/024/025/033/036 only as
needed for the Go-only product. Independent frozen/spec-authored expectations, negative and
adversarial tests, read safety, exact-target artifact verification, explicit unsupported surfaces
and honest unmeasured performance remain binding. No current candidate emits its own expectations.

The prior handoff's conditional tag push is not exercised using an unrun Packet 5 result. A Go-only
release must first pass its applicable native gates and report any remaining qualifications
explicitly. No performance or FULL claim is inferred from the language change. The Go engine
remains Go; a Rust rewrite is not authorized by the owner's question about possible future speedups.

Rollback is the prior native Go revision/artifact; no persisted state migration or Python install
is required. Old source and evidence remain recoverable through Git. The owner subsequently explicitly chose “Remove every Python runtime dependency”, including
optional adapters and developer tools. They must be ported or explicitly retired under GOC-V0-008.
Installed protected configuration remains user-owned and no native qualification is inferred.
