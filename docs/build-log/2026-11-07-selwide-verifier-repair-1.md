# Selwide verifier repair 1: record newly observed hidden readers

The host verifier at `5bb9f905d9ed1de0ea0f0f773d60ad731b30acb6` found
`.github/cishards` and `.github/testconfine/cmd` missing from the unbounded-reader
record after V1-5018's hidden-package discovery (AFP-V0-9012).

## Decision and requirement evidence

Record both directories in `.corvint/unbounded-readers.json`, in ascending order,
with the reasons required by AFP-V0-025 and AFP-V0-033. The former package's
`isolated_copies_test.go` reaches the checkout with a parent-directory literal
and reads workflow and helper sources. The latter package's `main.go` calls
`os.Getwd` to locate the wrapped package before loading scopes and resolving
runtime paths. AFP-V0-9012 preserves the frozen read-scope profile's rejection
of hidden-directory declarations in both readers: `matchReadScopes` requires
`walkedDirectory`, which rejects hidden components. Declaring these packages
would therefore invalidate scopes rather than confine their reads. Keep their
conservative selection and record the floor. Confined qualification is
`NOT_PRODUCED`; no narrower-read or performance saving is claimed.

This repairs V1-5018's integration record without changing selection code,
declaration shapes, diagnostics, goldens or test assertions. V1-4941 / AFP-V0-9013
and V1-4710 / AFP-V0-9014 need no further implementation change for this finding.
Rollback removes the two directories and reasons, which restores the observed
ratchet failure rather than hiding either reader.

## Focused verification

`make -s unbounded-readers-check` passed: 37 recorded directories, formerly 35.
`script/check-line-citations.sh --repin` found no moved citation;
`make -s line-citations-check` passed. The current-tree
`corvint affected --base cb403735903b1e54a8469373489960c0b2b89d99` plan retained
`UNKNOWN`, selecting 133 Go packages with 70 unknown entries. No broad selection
of tests was executed. The installed-binary probe warned that its build was stale;
the retained current-tree plan came from `GOTOOLCHAIN=local go run ./cmd/corvint`.

Whole-package checks through `script/heavy.sh`, without identity exports or
test-name filters, passed for `internal/liveverify/affected` (125.123 s) and
`internal/liveverify/affected/golang` (9.834 s). Whole `cmd/corvint` failed after
415.063 s with the previously recorded sandbox fixture boundaries, including
parent Git-root discovery and `CORVINT_WORK_RUN_PARENT` ancestor ownership.
No fixture, assertion or frozen output was changed to conceal this failure.
The local outcome remains `blocked` pending host validation; the repair
handback retains the final CEM check. Raw evidence stays under
`/home/russell/lanes/selwide-evidence/`; these SHA-256 digests support the numbers:

| Artifact | Bytes | SHA-256 |
|---|---:|---|
| `repair1-affected-current.json` | 174133 | `1838500e0ac82e7fd6b81e8762d7888c45158b33f18864026d4a41a43add7292` |
| `repair1-whole-affected.log` | 840 | `ecc81f9c72c9980a6bc944020118c034b13650bcd460ce336efc62b305976ae7` |
| `repair1-whole-golang.log` | 651 | `7bc85b6e9bd044c785ee6090a6697aaeac96bb62f818c1777d1e6988c3212170` |
| `repair1-whole-cmd.log` | 64225 | `fade9bff3b3a49bb65926bee9488b655fcf0e904e15bd4fa773e68acac9c600d` |

Repository-wide gates, baseline selection and independent review remain
host-owned. No pre-change recapture, Tasks write, publication or seal is part
of this repair.
