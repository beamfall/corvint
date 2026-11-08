# .NET lane self-review repair

Ticket V1-5113 (absorbs V1-4370), with the requested inherited-descriptor
repair in the shared T8 collector (RSP-V0-007, RSP-V0-021, RSP-V0-022).
The .NET repairs implement RSP-V0-005. No wire fields, error vocabulary,
exit status, stderr text, existing assertion, golden or freeze artifact changes.

## Decisions and behavior

The Linux collector preserves the lexical names of inherited file-backed stdio
before execution. These are the only descriptors inherited by its exec.Cmd;
recording them before dup/close conservatively retains even unused dependencies.
The supplied os.File name must resolve to the procfs target. Missing or stale
provenance retains the existing unresolved-descriptor-path incomplete reason;
parent traversal uses the existing parent-traversal reason. Anonymous descriptors
retain their existing observation-time peer uncertainty. This avoids inventing
lexical names from procfs or adding a descriptor-lifetime state machine.
TestRSPV0021InheritedStdinSymlinkRetarget reopens link after changing a to b,
checks the resulting size-dependent exit statuses, checks both lexical and
resolved dependencies, and checks an unnamed inherited handle widens.

Coverlet VSTest collection first declares results.trx. Core then calls the
in-process DotnetVSTestArtifacts resolver on captured bytes and captures its
run-directory-relative coverage paths before passing handles to Evidence.
The resolver bounds XML bytes, nodes, depth and attachment count; rejects
absolute paths, traversal, duplicate handles, URI encodings and malformed XML;
and normalizes Windows separators. It opens no files. Core remains responsible
for rejecting symlink escapes when capturing these paths, as with other artifact
capture. This is a two-step capture contract for the configured factory; no
production runner integration or qualification is promoted by this repair.
The native-layout regression captures results.trx and a GUID-subdirectory
coverage.cobertura.xml, resolves the attachment and decodes executed evidence.
The MTP artifact path remains its explicitly configured output path.

DotnetBodyMutants now applies the same CR-only normalization as CSharpBodies
before indexing its body lines. LF, CRLF and CR-only fixtures produce identical
mutation candidates and line numbers; literal and unsupported-body refusals stay.

## Verification and retained limits

Whole-package registry tests passed (2.096 s), including the existing tests and
new native-layout, unsafe-attachment and CR-only regressions. The macOS proctrace
package passed (0.848 s); Linux/amd64 proctrace test compilation and vet passed.
Linux ptrace runtime execution is NOT_RUN on this Mac. Live .NET execution and
qualification remain NOT_RUN; every runner remains FULL unqualified. Independent
review belongs to the orchestrator; no model-backed reviewer was launched.

The requested affected base stays 8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734.
Its receipt selects registry, contextindex and proctrace directly, but retains
UNKNOWN scope, 545 selected units and 95 unknowns (74 language frontier,
1 no selectable test, 20 unowned dirty paths), plus self-binary-stale.
The range includes inherited integration work. CEM binding uses the original
lane parent b33282354e0e28705502554bfee2da359e4ae851, already identified as this
lane's BASE in the original handback, so its evidence covers this lane and the
requested repair rather than claiming unrelated integration changes.
The pre-change capture is retained; make dogfood-change is not rerun and no seal
is authorized. Binding/check results are retained in the updated handback.

make changed-checks against the requested affected base fails on inherited
unbounded-reader and test-read-scope declarations, including runnerselection,
benchmarks/selfuse-batch and script_plugin_test.go. Those are outside the three
findings and remain unmodified. Citation repinning found no moved citations.
Core/Tasks are build 128. Official update checks fail process quiescence under
sandbox policy; available release versions and account usage are UNKNOWN.
No failed verification was bypassed. Tasks writes and publication are prohibited.

Raw evidence remains in /Users/russelllewis/lanes/rspt11dn-evidence/.

| File | SHA-256 |
|---|---|
| repair-affected.json | 561a13df5d7e40f1da81b7e2b7189c3bfcf078c430c7388fc02358ec3e4941d9 |
| repair-test-registry.log | 1e7371166d4b9c4ea3811caf202d014141cd8a706f97ee56a4ec95e2fedd572f |
| repair-test-proctrace.log | e2ab574b07658513a32491716764fafda45d6d0958a26048761f015a8a35b047 |
| repair-linux-compile.log | a58efeca1036e0c35512ac4cf5e475cd82709fce082fd004f2449014d9d5458c |
| repair-linux-vet.log | 09154c4c2cffc2c95611f60f3a9926cc7c9930f67e2f17c1c7018522540ca2b0 |
| repair-registry-vet.log | 881d21a8a1db5cbdaa5a7adb439493ba78804da570458865ae7131021fbd22b4 |
| repair-changed-checks.log | 63a6a6d6218c8b4a707550e86b6927e658245f34b360d2b3403262472e1cd629 |

Rollback: revert this repair's source changes and restore the previous command
artifact declaration; no schema or stored-state migration is involved.
