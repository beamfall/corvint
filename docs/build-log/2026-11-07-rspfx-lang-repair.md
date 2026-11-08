# Language runner integration repair

- Base: `fd95d92413829a09718c323ddbb00c5980d8ae40` (integration lineage; earlier commits are outside this repair).
- Authority: owner lane charter, decision 0592, RSP-V0-001/002/005/048 and LGP-V0-009.
- Execution status: build-only. Regression tests are written, not executed. Host verification and independent review remain outstanding. No qualification or narrowing claim is made.

The repair binds .NET restore assets and ancestor build configuration to loaded blobs, retains an incomplete record for an inventory binding without collection, bounds mutation line indexing, and refuses ambiguous Coverlet attachments. Existing CR-only normalization and TRX attachment resolution were already present at the base; their original defect descriptions were stale. The attachment resolver remains based on captured TRX paths, without filesystem IO inside the plugin.

Ruby runners now resolve through global selection lookup and match the language manifests. Global instances support captured inventory and result parsing; commands and native evidence still require the existing immutable, core-bound factory inputs. Qualification stays absent and plans stay FULL. The owner explicitly authorized updating the all-absent assertion to the registered set. Evidence normalizes RSpec's `./` package prefix exactly as inventory does; test identities retain their native spelling.

Python coverage ignores project configuration. The existing command already writes to its supplied RunDir staging directory; a new regression separates that directory from the checkout and asserts that neither coverage.json nor a coverage database appears in the checkout. The Swift test moved to swiftselection in the integrated base; it now clears native components before decoding while preserving absent core components. Declared Go script commands reuse sessiontest.AdapterGo; the literal baseline is unchanged.

## Bounded Rust decision

The existing libtest parser supports one library target. Rather than broaden its native output grammar, the explicitly allowed fail-closed repair refuses cargo-test inventory and commands with `rust-integration-tests-unsupported` when captured source paths or Cargo metadata name integration tests. An incomplete source/target universe refuses with `rust-library-target-universe-incomplete`. Nextest retains its existing target model. Ancestor Cargo configuration is added to loaded from the immutable source snapshot. Reverting these lane changes rolls back this repair; it does not qualify the preceding behavior.

JVM collection captures inherited JAVA_TOOL_OPTIONS when the factory is constructed and appends the JaCoCo option, preserving the inherited bytes. No wire field or golden changes are involved.

## Verification boundary

The owner reserved execution, live qualification, independent review, generated spec indexes and CEM binding/sealing for the host. Context used the specified original source and spec route. Dogfood context/binding, affected test selection, broad evaluations and publication are deferred under the final build-only instruction. Exact local check results and host follow-up commands are retained in the lane handback outside the repository.

Local checks passed: gofmt on changed Go files; Go vet through script/heavy.sh for registry, swiftselection and langplugin; spec README and generated-index/worktree drift checks; and line-citations-check. The first vet attempts identified a missing filepath import and an unused strings import in tests; both were corrected and the affected vet commands passed. Line repinning found no moved citations. The complete spec-requirements-check Make target remains NOT_RUN because its recipe executes internal/specindex and internal/lrfrepo Go tests, prohibited by the build-only override. Linux vet is not Darwin execution evidence.
