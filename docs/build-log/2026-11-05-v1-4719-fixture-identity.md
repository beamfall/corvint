# V1-4719: isolate Git fixture identity

Owner intent: the fixtureident lane charter requires Git fixtures to ignore inherited author and
committer names, emails and dates while keeping the existing frozen documents unchanged.
Requirement: CCF-V1-9800, refining the fixture-backed compatibility evidence of CCF-V1-002.

Decision: filter only the six identity/date entries, case-insensitively, before appending existing
fixture overrides. A shared testsupport filter serves the cmd/corvint command constructors and
small Go fixture helpers. Preserve existing config isolation and fixture clocks/identities. The
existing IsolateGit package bootstrap also removes caller dates; its four fixed identity fields
already override caller identity. This is test infrastructure, not a product environment policy.
The source guard checks direct Git constructors and raw inherited Env assignments; it is not an
arbitrary interprocedural dataflow proof.

The Core Compatibility Freeze spec owns the affected frozen-profile evidence. An initial attempt
to index the clause in the CSD companion exposed SRG-V0-790's requirement that a four-digit prefix
also define a three-digit requirement. That companion has no such definition, so its existing
four-digit clauses and the attempted clause were silently absent from REQUIREMENTS.tsv. This lane
leaves that generator behavior unchanged and defines CCF-V1-9800 under the established CCF prefix.
No CSD file changes remain.

Self-use: make dogfood-change and dogfood-prechange retained context, query and impact receipts at
cdeed7ee6396a8d6bf6225eb5b2b4cb4d601b419 before editing. The context packet retained no-specific-terms
and two critical_missing subjects; original helper source and the charter supplied the necessary
evidence. The affected plan selected every touched package, with scope UNKNOWN from
LANGUAGE_FRONTIER go:build-constraint-variants and shell:foreign-invocation-unmodelled. The installed
binary warned self-binary-stale; repeating the plan with the lane-built base binary preserved the
173 selected / 471 excluded / 2 unknown result. Core command sources are unchanged by this lane.
No ranking, learning, optional LSP/provider, mutation campaign or live external qualification was
applicable to this test-only defect. Independent review is left to the orchestrator as instructed.

Repository-wide audit: the initial *_test.go scan retained 257 os.Environ() matches; a broader
Go/helper scan retained 328 matches. AST inspection identified 97 direct Git Env assignments:
11 use the filter, 40 are protected by the updated existing IsolateGit bootstrap, 21 already
supply all six fields, 7 only initialize/clone/read content, and 18 remain explicitly reported
(Tasks' independent import boundary, external interop module, and one Git-version-gated JS
advice fixture). The independently inlined analyzernativebridge bootstrap still inherits dates;
allowlisted fixture producers and shell setup candidates are separately enumerated. This is an
audit/disposition record, not a whole-repository hermeticity claim.

Verification results are recorded below before commit; final binding remains post-commit. Raw receipts, logs and
the full per-path audit stay outside the repository in ~/lanes/fixtureident-evidence/.

| Retained raw evidence | Bytes | SHA-256 |
| --- | ---: | --- |
| `environ-search.txt` | 37684 | `219370990ae106c9d5912733eed45a0739669f9e63617db0fd70b5bead5b389c` |
| `environ-all-go.txt` | 46799 | `131fafaf12776bf30ea8698fb62126ed7a8b123172669b15a0683bb7f082592f` |
| `git-env-audit-final.json` | 37124 | `0bff164ff27639b034eb48e7f910c2d3e625adc7d2f9118d45c2724d5a5a2e8d` |
| `audit.md` | 27439 | `b32185385254d37d39faf6ee71594cf503f7967ca786ae59a136167b266e809b` |
| `foreign-identity.log` | 1148 | `fbdb0da2fa6bfdcf44155b528fb9a0702213099c42f8f42c1de8361d7d675044` |

All eleven additional touched-package suites passed as whole packages through script/heavy.sh
(one package per command, count=1, timeout=30m). The three named frozen-profile tests passed
both without inherited identity and with all six foreign identity/date fields exported. The new
SHA/golden regression and source guard passed. No golden was regenerated. windows-vet passed
for both modules; spec generation, requirement indexing, gofmt and line-citation checks passed.

The required whole cmd/corvint suite ran all existing tests and returned rc=1 after 614.186 seconds
(47 top-level failures). The mandated TMPDIR sits beneath /home/russell/lanes, which is itself a
Git repository and has mode 0775. Negative repository tests therefore discover that ancestor;
profile writers refuse paths within its worktree; work materialization refuses its group-writable
ancestor as SOURCE_UNQUALIFIED. The frozen expectations remain untouched. Remaining failures
include adapter timing/trace diagnostics under host load; their complete output is retained,
without claiming all 47 were independently reproduced at BASE. This is PARTIAL due to external
host qualification, not a passing whole-package gate. The orchestrator must rerun the whole
cmd/corvint package with a suitable private temp ancestry outside any Git repository. No shared
ancestor permissions, TMPDIR contract, product refusal or existing assertion was loosened.

BASE comparison: an archive of the unchanged base ran through the same heavy wrapper and mandated
TMPDIR. TestConvertedRefusalDiagnostics and TestWorkMaterializationNoCallerGitWrites both reproduced
the ancestor-repository and writable-parent failures (rc=1, 0.503 seconds of package execution).
This demonstrates those two environmental blockers at BASE; other failures remain retained unknowns.
The local outcome is blocked pending host-side whole-package qualification.

| Verification log | Bytes | SHA-256 |
| --- | ---: | --- |
| `corvint-whole.log` | 67613 | `b8f8b121e0af64d2b67b500f8d1ad099f366f7ab31febb64fbb651c3c4bf9dd4` |
| `base-environment-failures.log` | 2879 | `92b494abb5be04ef4cac60fc7e216616a359960a67fdc6303edcf553c1da5d40` |
| `focused-fixed.log` | 1016 | `2fdaa966c083a74eb9a9d9d16d7ab6ea85db45d5882bbd8427a397e4592e1723` |
| `testsupport-final.log` | 1016 | `e97df490ee9eb60b2053a1dea22afda5903db03dbec3cb5a2fb0102a270cef3b` |
| `testsource.log` | 885 | `37d0cdc7ad9832d6f194a1465f0acc0fa2873742f561667e9e8a6802f594eca0` |
| `frontiernextrepo.log` | 850 | `7441e3590be7e51335992dc171b046c890a0d9ce26af7ebfbd260b8725042b2d` |
| `tracemigraterepo.log` | 850 | `f4013d580030c7f12f072406bcd2b0e7af0f3b8766922c2c72df2af3cf1c53d7` |
| `criterionexperiment.log` | 1028 | `87afe5f8e9c72ba5223eac7250cb2bbd04fba16eaca8b163c5453069b43e0514` |
| `dashboard-roadmap.log` | 983 | `60d70c19286993f9ceda7a969d257eb870da87f0469f0c0ff50fa98a951bb6dd` |
| `gitstatus.log` | 1018 | `c5ae914d27e98e58c7137d33d3aadab87c16ec824e11d7c962ca896853f80392` |
| `intake.log` | 1011 | `a68201c3ffdecd5b7d289e33a8a112835192a3594318990c2707f68edc18e28c` |
| `postmergeworkflow.log` | 1216 | `ef0430ba80f53ccec06405be64d69fe3de638942209f4645e0680e1eeeae641e` |
| `slotlearn.log` | 975 | `bfe3a976769ca73cc452041d1ec577114e59ff4511626efa2596ba1fd99b643d` |
| `stepverify.log` | 1154 | `50dc86c77d66ce44815a3b3aeefaa110ae0637a5c19f4fc81df44b72c8c2a1a7` |
| `windows-vet-final.log` | 419 | `3b82a9be5ade16beb7ff1018b4dac5b36cd8d1d76237cfdc20ad708585ee3a29` |
| `line-citations-check-final.log` | 118 | `855a4551b199f407e4e7cec705511746022a39aa0f70383fc13aac9acc748bba` |
