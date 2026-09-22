package main

const (
	manifestLimit          = 1_048_576
	artifactLimit          = 8_388_608
	implementationLimit    = 67_108_864
	observationLimit       = 131_072
	captureLimit           = 65_536
	caseTimeoutSeconds     = 25
	totalTimeoutSeconds    = 900
	lockTimeoutSeconds     = 5
	maxRuns                = 2
	maxJSONDepth           = 64
	maxJSONIntegerDigits   = 128
	maxPacketBytes         = 33_554_432
	expectedManifestSHA256 = "2655258e73d569e35dffb36cc6d3ae2e738801848d9f57decb774463b92f34bd"
	lfOverflowRecipe       = "cem/0.1-lf-overflow-x-lines"
	lfOverflowBytes        = 524_290
	lfOverflowLines        = 262_145
	lfOverflowSHA256       = "cfecb16854630a4d141b429fdfdcdc73bb48124ac47375b5896c634037c9eee6"
)

var expectedCounts = map[string]int{"valid": 7, "invalid": 19, "drift": 6}
var expectedMatrix = [][2]string{{"valid", "supported-sha1"}, {"valid", "supported-sha256"}, {"valid", "unknown-create-delete-modified-rename"}, {"valid", "mechanical-whitespace"}, {"valid", "mechanical-line-ending-crlf-to-lf"}, {"valid", "unicode-and-no-final-newline"}, {"valid", "overlap-evidence-base"}, {"invalid", "duplicate-json-key"}, {"invalid", "unknown-field"}, {"invalid", "unknown-spec"}, {"invalid", "patch-digest"}, {"invalid", "evidence-id"}, {"invalid", "hunk-id"}, {"invalid", "hunk-range"}, {"invalid", "path-traversal"}, {"invalid", "supported-without-basis"}, {"invalid", "orphan-evidence"}, {"invalid", "surplus-hunk-payload"}, {"invalid", "mechanical-line-join"}, {"invalid", "header-only-create"}, {"invalid", "header-only-delete"}, {"invalid", "special-index-mode"}, {"invalid", "contradictory-create-index-mode"}, {"invalid", "lf-tokenizer-line-overflow"}, {"invalid", "integer-resource-bound"}, {"invalid", "utf8-path-resource-bound"}, {"drift", "stable"}, {"drift", "relocated-whitespace-outside"}, {"drift", "stale-whitespace-inside"}, {"drift", "ambiguous-overlapping-matches"}, {"drift", "deleted"}, {"drift", "deleted-symlink-changed"}}
var packetIdentityFiles = []string{"ADAPTER.md", "ALGORITHMS.md", "IMPLEMENTATIONS.md", "START-HERE.md", "SUBMIT.md", "manifest.json", "observation.schema.json", "runner.py"}
