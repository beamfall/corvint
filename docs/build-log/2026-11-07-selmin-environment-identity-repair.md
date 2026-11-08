# Selmin self-review repair: prove the environment import identity

Requirements: AFP-V0-2200, AFP-V0-9010. Ticket: V1-5029.
Input head: 0f9f001508484fc11cde0604cefe9efa39c555b9.
Base: e19823d919cfa653ee0ed05708d03f5fd12ca929.

The review reproduced an unsound import-only proof: a custom receiver named `os`
can inject an escaped GOFLAGS key through `Environ`, changing `go list -deps`
to JSON output that exposes test filenames even when their imports do not change.
The environment proof trusted a spelling instead of an import identity.

Decision: pass the file's existing unshadowed import map to both environment
proof call sites, command confinement and listing isolation. The closed allowlist
admits only a literal string slice or a non-variadic builtin append of literal
entries to a zero-argument Environ call on an identifier resolving to the standard
os import. Explicit import aliases are valid; declarations shadowing the import
invalidate it. Entries setting GOFLAGS or GOENV remain rejected after string
decoding. Builtin identity remains governed by the existing unit-wide proof.
Every other environment expression or receiver fails closed, keeping whole-module
source observation. No spelling denylist or raw-key exception is added.

`TestReaderEnvironmentIdentity_AFP_V0_9010` covers six forms against two kept-import
test-file additions: standard import, aliased import, literal environment, the
reported custom receiver with an escaped key, shadowed import and unknown receiver
producer. Every case lists cmd/tool, which imports lib. The lib addition retains
selection; the unrelated base addition isolates the whole-module source-reader
edge from binary-execution and path-literal fallback. Dropping its Source witness
must remove that selection. Existing import-only controls remain excluded.

The pre-fix focused run reproduced the custom receiver's false certification.
All 12 final regression cases pass (focused run exit 0, 0.082 s). The required
whole-package run includes every existing test and fails in six existing top-level
tests during Git fixture setup: process-cleanup-failed, /proc quiet proof
unavailable. The new test passes in that run; no assertion or golden was weakened.
This is an external qualification blocker, not a passing package result.

Evidence outside the repository, under ~/lanes/selmin-evidence/:

- self-review-affected.json: package selected, PACKAGE_EMBEDS fallback; SHA-256
  fc784d987eef46b7f8c11a18d0e5e7b39f70f381ea5781e664ecba026150bfcf.
- self-review-regression-after.log: exit 0; SHA-256
  24aed1a216b3834fa3269de8723157ca15981bf5b31868feed6cc9b58f22f57f.
- self-review-package-final.log: exit 1, 7.925 s; SHA-256
  466dae3e6845982fc9727f584655534bc93c01973f7cd7a189625efc66816b36.

No wire shape, field, error/exit code, stderr, golden or core-freeze output changes.
V1-5026 / AFP-V0-9009 and V1-5030 / AFP-V0-9011 are unchanged. Independent review
and host qualification remain orchestrator-owned. Bind/check results and exact
external blockers are retained in the handback, without a seal or Tasks write.
Rollback: revert this repair and rebind the resulting patch; doing so restores
the reported soundness gap, so do not claim import-only proof for that receiver.
