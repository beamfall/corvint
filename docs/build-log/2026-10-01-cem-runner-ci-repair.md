# CEM runner CI repair

Date: 2026-10-01
Native work: V1-0615; experimental foundation PR423.

## Decision and bounded change

The hosted PR423 run36884053265 failed `internal/specindex` metadata checks and
`TestGracefulInterruptLifecycle/normal` under the race runtime. The new spec headers,
five-field Agent digests and README summaries are made consistent with the existing
index; the generated requirement line references follow the ten-line stable spec
header insertion. Requirement text and proposed/experimental/planned authority are
preserved. No stable promotion is established.

The normal lifecycle helper receives a five-second timeout because the Go race
runtime delays successful process exit by one second. Timeout, interruption,
overflow and uncooperative helper cases retain their one-second bounds. Production
execution behavior is unchanged.

## Evidence and rollback

Before source admission, Corvint query, impact and affected receipts were retained
against immutable base05d66c36abf3f8c051ce12fa83670a728a98b770. The six-file patch
3a2e7368f1416d77de9f5808bc53b50a51ed47da3fccd2eb344e45e0a9ddf1bb received independent
proposal review with no findings. The unmodified normal race test failed and the
normal-only timeout proposal passed. In the admitted leaf, the spec-index test, all-mode
lifecycle race test and requirement/citation/Go-format checks passed. CEM binding and
sealing follow the ordered post-commit workflow. Hosted rerun outcomes remain separate.

Rollback reverts the source repair while preserving archived evidence and the original
experimental packets. Stable wire implementation, all runtime qualifications,
external interoperability and matched external outcomes remain open requirements.
