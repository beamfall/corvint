// SPDX-License-Identifier: Apache-2.0
// release-binary builds, checksums and records one product binary at a time, so a release tag
// can ship one executable without rebuilding the others (decision 0594 item 8, V1-5236;
// ARTIFACT-BIN-V0-001..007 in docs/specs/release-artifact-integrity-v0.md).
//
// It reads the committed catalog binaries.json, builds the named package for the release
// targets from a clean HEAD, assembles archives with the same layout as the corvint archives,
// merges per-binary entries into release-manifest.json and refuses undeclared version skew
// between binaries of one version set. It has no network, tag, signing or transfer code:
// publication is an owner-approved step outside this tool.
package main
