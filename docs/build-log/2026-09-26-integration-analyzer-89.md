## 2026-09-26 Integration: the rc.1 fixes land together at `corvint-analyzer/89`

This integration merges the rc.1 Git runner hardening (`/88`), the authority parse fixes (`/89`),
the evaluation observation fixes and V1-0388. The last two re-pinned the audited-input digest
without a schema bump. Together their `contextindex`, `gitstatus` and slot-weight changes match none
of the branch pins. Main therefore moves from `corvint-analyzer/87` straight to `/89`, and the
combined digest is `ba439f2725ffb6abe2f410982b2e4a4cc8d367ee24379dc57e143543a455d801`
(`IDX-SNAP-V0-017`). `/88` never reaches main. Snapshots rebuild once.

The trace-table conflicts were resolved by keeping both sides:
- the DCW-V0-013..015 row names both V1-0354's remediation test and V1-0362's ambient-configuration
  test;
- the SRR-V1-011 row keeps `runSourceGit`, and the SRR-V1-012 row keeps the implementation;
- the IDX-SNAP-V0-007 amendment keeps decision 0424's acceptance and V1-0361's temporary sweep.
