# unbrd: close prefix-only self-reference proof (2026-11-07)

V1-4283, AFP-V0-3149, preserving AFP-V0-3147 and TSP-V0-960. The owner's independent
review identified `x=check; x="$PWD/script/$x"; "$x"` in `use/wrap_test.sh`: the unrolling
proof suppressed the cut, while the constructed-target model dropped the wildcard-prefixed
extensionless target. An edit of `config/settings.env`, read through `. "$ENVFILE"` in
`script/check`, then omitted the wrapper. Root `check` and `use/check` are absent in the regression.

## Decision and closed class

An unknown prefix is insufficient evidence of path-preserving normalization. Restrict prefix
certification to exactly `$PWD/` and `${PWD}/` before a plain self-reference, with no suffix;
the prefix's complete bounded expansion must be exactly one unknown part followed by `/`.
This also rejects an in-file PWD value that adds a directory. Every other prefix fails closed,
including opaque command results. This narrows the proof rather than resolving new path forms.
The existing allowlist's other entries remain plain identities and standalone `-`, `:-`, `=`,
`:=` defaults with literal operands containing no dollar or backquote. Non-self values and
allowed default operands remain seeds. Argument references inside opaque substitutions remain
nonrecursive. Unsupported self-reference forms retain `shell:dynamic-source`.

This amendment to AFP-V0-3149 follows the owner's repair instruction and standing lane acceptance.
No wire shape, error, diagnostic, golden, core-freeze output or existing test assertion changes.
No unrelated implementation changes. Rollback: disable the unrolling shortcut and retain the
conservative cut; never restore arbitrary-prefix certification.

## Evidence and limits

`TestComposedSelfReferencesKeepSelection_AFPV03149` adds the reported prefix-directory case,
an unknown command-result prefix (`$(printf '%s' "$PWD/script")/`), an in-file PWD-directory
case, and a `${PWD}/${x}` control. It asserts dynamic-source protection and wrapper selection
for both configuration and target edits, with the control preserving its literal execution edge.
The reported and opaque-prefix cases failed before the fix (four assertions, including both
configuration-selection omissions). The full shell package then passed, 1.235s, with every
existing assertion intact. Raw evidence is external under `/home/russell/lanes/unbrd-evidence/`:

| Evidence | SHA-256 | Observation |
| --- | --- | --- |
| `repair10-before.log` | `4519ab76598bd79f6c4ddbe645abdcc2e1bb89648d51a27ac9b34066d1ef50b8` | Focused counterfactual fails, exit 1 |
| `repair10-shell.log` | `e8771183ca41101e785679a570f77849abe8269c69e492d2a5c97c21e9d6c777` | Full shell package passes, exit 0 |
| `repair10-affected-tests.log` | `7e60f89a0fc3202a62d773ce9113ac4df1cff50e8761817811247fce59f7cfc5` | Full parent affected package passes, 127.061s, exit 0 |
| `repair10-affected.json` | `66e10c80c7166ece63672aec39a42c1fb3a5802b47c6e5f8f400297ba2a5c8a0` | 497 selected, 71 unknown, scope UNKNOWN |

The affected route was used against `ae20b398c260933a152e6285de9cfc385a2580c7`, selecting
shell and its parent graph package. Its installed binary reported `self-binary-stale` (four
changed Go source paths); no current-source selection equivalence is claimed. Pre-change
capture is reused by explicit instruction. Final package, spec/citation and CEM results are
retained in `/home/russell/lanes/unbrd.handback.md`. Independent review and broad host verification
belong to the orchestrator. Provider, learning, retrieval and publication routes are not
applicable; no Tasks write, push, PR or seal is authorized in this repair.
