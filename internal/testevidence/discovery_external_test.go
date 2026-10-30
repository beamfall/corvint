package testevidence_test

import (
	"testing"

	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testvaliditydoc"
)

// These tests import testvaliditydoc, which imports rubytestprovider, which
// imports testevidence, so they live in the external test package (V1-1811).

// V1-1811, V1-1842: testvaliditydoc aliases the leaf constant; this guard is
// trivial and stays only to name the binding.
func TestEvidenceDirectoryMatchesDiscovery(t *testing.T) {
	if testevidence.EvidenceDirectory != testvaliditydoc.EvidenceDirectory {
		t.Fatalf("testevidence %q, testvaliditydoc %q", testevidence.EvidenceDirectory, testvaliditydoc.EvidenceDirectory)
	}
}

// LPCV-V0-084: after the first retention writes its ignore file, discovery
// still selects the retained document.
func TestLPCVV0084RetentionIsDiscovered(t *testing.T) {
	root, _ := testevidence.GitRepository(t)
	name, err := testevidence.Retain(root, "corvint-js-test-provider", testevidence.RetainedDocument(t, root))
	if err != nil {
		t.Fatal(err)
	}
	discovered, err := testvaliditydoc.Discover(root)
	if err != nil || discovered.Discovery == nil || discovered.Discovery.Evidence != testvaliditydoc.EvidenceDirectory+"/"+name {
		t.Fatalf("discovery after retention: %+v %v", discovered.Discovery, err)
	}
}
