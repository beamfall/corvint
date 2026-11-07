# Fullplan self-review repair

Authority: the owner's Self-review repair request, 2026-11-06, findings P1
and P2 against `8f1605fbc058f8dc3e2d7b3b79a9d7f470a6206e`. The retained
lane base is `9db7a94169821d9b28701eaa88866aee2fec1083`. Requirements:
AZU-V0-9920, AZU-V0-9922, STR-V0-004 and STR-V0-069. No native ticket ID
was supplied; task-store writes, publication and sealing remain prohibited.

## Decision and repair

P1: the maintained Python no-suite assertion contradicted CI's existing
`python3 -B .github/macos/select_test.py` step. The declaration now names
that exact command. `TestFullplanReplay_AZU_V0_9920_9921` reads the maintained
declaration, asserts its executable local invocation and its required CI
check, and retains both disk and immutable graph replays. The fixture's FULL
count now includes the real declaration's shell and TypeScript commands too.
The declaration-checker's exact expected no-suite list drops Python; its
comparison and all refusal assertions remain intact. This updates expected
declaration data to the authorized command, without changing a golden,
wire profile, diagnostic, error code or exit behavior.

P2: the real inventory probe now filters independently enumerated `.github`
children with the source walk's hidden and shared skipped-directory rules,
testdata exclusion and child module boundaries. Recursive patterns rooted
at an independent nested module are never sent to the root module's Go
probe. `TestHiddenHelperModuleBoundary_AZU_V0_9920` builds the real graph
with an ordinary root package, helper, independent nested module and
skipped build/testdata/hidden directories. It pins exactly the ordinary and
helper packages, then exercises the real discovery probe. Existing extra
and missing helper refusal tests are retained.

These decisions refine the existing AZU-V0-9920 and STR-V0-069 clauses;
there is no new capability or acceptance claim beyond the two repairs.
Rollback: revert this repair's declaration, inventory filter, regression
changes and corresponding requirement amendments together. Do not drop the
inventory comparison or restore a fixture-only executable Python command.

## Evidence and limits

Before the production fixes, the new boundary regression failed with the
existing `go list ./... failed; the Go universe cannot be confirmed`
diagnostic, and the replay reading the actual declaration failed because
Python selected no invocation. The initial complete declaration-checker
suite then failed its outdated Python no-suite expectation. Those failures
remain in external logs; no comparison or refusal check was weakened.

Final focused regressions pass. Complete package results, evidence digests,
required spec/citation checks and the final CEM check are retained in the
lane handback. The existing six native Python tests pass. Commit identity
is absent during tests and gates; the heavy semaphore configuration is
unchanged. The final citation repin reported no moved citation content.

The owner's excluded repository-wide selections and gates, native
Darwin/Windows execution, sealing, task-store writes and independent review
are NOT_RUN here. The orchestrator owns integration and independent review.
No new performance, savings, hosted outcome or general zero-unknown
qualification is produced. Original pre-change receipts are reused; no
start-of-change capture was repeated. Raw logs remain outside the repository
at `/home/russell/lanes/fullplan-evidence/`.

| Complete package / check | Result | External log SHA256 |
|---|---|---|
| affected | PASS, 144.898 s | `20572c29099166860864d24b3d92dacac29e7000f922f31615ebcda41bba4607` (`selfreview-affected-whole.log`) |
| sessiontest | PASS, 4.443 s | `3942f423523563fadf99540f130e55237b91c0badad435599d0a9386a0bf1e01` (`selfreview-sessiontest-final.log`) |
| confined cishards | PASS, 9.626 s | `95e57c0c93b82a91e538b547044dcf89f8f83da57a5fa517e8033af0427bf0ac` (`selfreview-cishards-whole.log`) |
| test-commands-check | PASS, 12.148 s | `be94bb7ecd2ee2818e3c710090fb634920cba88f33744474890e3391877a3f4c` (`selfreview-commands-final.log`) |
| Python select_test.py | PASS, 6 tests | `1e43b905e11ca028139fe5078016515111aaf7cebf8d6a3e224b7d03b9cb6ac9` (`selfreview-python.log`) |
| spec and line checks | PASS | `56ec7cb28d005b5e07b9b7606f501c7673bc509aad331f86f078bfe4d70aecd9` (`selfreview-spec-lines.log`) |
