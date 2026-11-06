# Darwin shard foreign-caller registration (V1-4732)

Verifier repair 1 addresses the host's failed
`TestRepositoryShellForeignCallersComplete_TSP_V0_005` at
`65f3fc33b92a1ff8e6c9b51511614433d3f5ecc8`. The new Darwin runner
and its test both name `heavy.sh` but were absent from the declaration.

Under TSP-V0-005 and CIN-V0-4625, register `.github/macos/shard.py` as
a caller: it executes the heavy wrapper once per module. Register
`.github/macos/shard_test.py` as a mention: it checks the constructed
command through a stub runner without invoking the wrapper. Keep both
lists sorted and explain the entries in the existing reason. No schema,
command output, golden or assertion changes are needed.

The first focused rerun also exposed the stale `.github/workflows/ci.yml`
caller left by removing the old Darwin job: it no longer names the wrapper.
Remove that entry as part of the same exact-completeness repair. The initial
rerun failed with the plugin's generic invalid-declaration frontier after
the missing-file diagnostics disappeared; the retained log records that
failed evaluation without changing its assertion or diagnostic.

The existing repository completeness test is the acceptance witness;
focused execution and whole previously touched Go packages are recorded
in the external lane handback. The host owns broad gates and independent
review. Reuse the pre-change receipts and rebind the final CEM once;
hosted Darwin qualification remains NOT_OBSERVED. Rollback removes the
two entries and their reason alongside reverting the Darwin sharder.
