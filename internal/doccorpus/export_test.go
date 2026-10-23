package doccorpus

import "testing"

// EightSectionFlowForTest exposes the DCP-V1-170 fixture to the external
// MCP bridge test in this directory.
var EightSectionFlowForTest = eightSectionFlow

// TestKnowledgeFixtureForTest exposes the DCP-V1-086 fixture to the external
// MCP bridge test in this directory (V1-1873).
var TestKnowledgeFixtureForTest = testKnowledgeFixture

// MultiRevisionForTest exposes the DCP-V1-308 fixture to the external bridge test.
var MultiRevisionForTest = multiRevisionFixture

// LargeReadFixtureForTest exposes the DCP-V1-418 large read fixture to the external MCP bridge
// test: the large scale is the one the PR measurements use, the default a moderate one.
func LargeReadFixtureForTest(t testing.TB, parent string, large bool) string {
	scale := readFixtureScale{docs: 1200, sections: 4, sources: 1500, additional: 2, additionalFiles: 600}
	if large {
		scale = readFixtureScale{docs: 4000, sections: 6, sources: 6000, additional: 4, additionalFiles: 1500}
	}
	root, _ := largeReadFixture(t, parent, scale)
	return root
}
