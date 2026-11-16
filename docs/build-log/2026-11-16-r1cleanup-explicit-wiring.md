# Explicit request and registry wiring (V1-4818)

Owner charter: r1cleanup, base df315e35056d15c7933b5ff97c605cedf5ef4919.
The reusable regwire branch bd91272fc supplied design/spec work only. Its proposed
Swift exception is rejected by this charter: Swift and Kotlin remain in 1.0.

Decision (LGP-V0-023, LGP-V0-025, LGP-V0-8600, MTV-V0-950, NGI-V0-988):
construct private dialect registries from explicit Ruby, shader and web hook values;
construct the flows table and plugin manifests completely in their initializers.
No init registration remains in these areas. Registry construction retains the
existing invalid-hook panic text and copies maps/probe slices. Order and isolation
regressions exercise two independently constructed registries and returned values.

Swift's provider-free types move to `langplugin/dialect`, allowing the adapter to
avoid importing contextindex. CLI/MCP startup supplies the adapter to request-local
index views; an unwired view retains its old uncertainty. This keeps the planner out
of the contextindex production closure and avoids a process-global setter. The three
command blank imports are removed. The withheld test-validity server never serves
path impact, so its inert registration disappears without changing its tool surface.

The HTTP transport is passed in E2EConfig and through readiness/preflight calls.
The client package imports no parent provider, so fixtures can supply the same real
client explicitly. The transport is excluded from worker-job JSON; the subprocess
helper constructs its client before decoding the unchanged job. A serialization
regression proves that supplying the client changes no JSON bytes. Missing-client
error text and readiness behavior remain unchanged.

Evidence: whole-package tests PASS for contextindex, langplugin, dialect, Ruby,
shader, Swift, flows, jstestprovider, its external fixtures, the JS provider command,
and MCP bridge; standalone MCP and loopbackhttp compile with no test files.
Changed-package vet PASS. The new AST guard rejects init/global registrations and
blank wiring imports. Existing help/golden files were not edited.

Overall acceptance remains PARTIAL for host verification: the unfiltered core CLI,
MCP CLI and withheld test-validity MCP suites report outside-checkout/profile-fixture
failures because `/home/russell/lanes/.git` is an artificial read-only sandbox path.
No failed assertion or golden was changed. Live browser qualification, non-Linux
execution, whole-repository validation and independent review are NOT_RUN here.

Raw logs stay in `/home/russell/lanes/r1cleanup-evidence/`:

| Log | Bytes | SHA-256 |
|---|---:|---|
| test-contextindex.log | 850 | 5a5dc35a3d604330ce3cbad58dd554310b3f04eeccd6e357d81a7698868a4c50 |
| test-jstestprovider-final.log | 866 | 66e81eadfcb71498897204239ab14582f0864a9004fdc9db1396c85b8e6957fa |
| test-corvint.log | 6856 | 87018b156360a2c2094290e8dab417b069bbc2900f78a132d466ffdccecebd5c |

Rollback: revert the wiring and its spec amendment together. No JSON/wire fields,
help text, error codes, exit codes, stderr strings or committed goldens change.
