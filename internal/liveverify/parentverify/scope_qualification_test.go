package parentverify

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/godiscovery"
	"github.com/Beamfall/corvint/internal/liveverify/gorunner"
	"github.com/Beamfall/corvint/internal/liveverify/provider"
)

// This minimal, zero-event run only compares frozen scope inventories. It does
// not simulate a non-empty exclusion or an unsupported shadow execution mode.
func composeScopeQualificationReceipt(t *testing.T, snapshot Snapshot, request provider.CanonicalBindingRequest, binding provider.CanonicalBinding) map[string]any {
	t.Helper()
	pathSHA, err := godiscovery.Hash("go-logical-path", "go-logical-path/0", []byte(`{"namespace":"SOURCE","path":"probe"}`))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := godiscovery.Decode(strings.NewReader(`{"Dir":"/workspace/probe","ImportPath":"example.test/probe","Name":"probe","Match":["example.test/probe"]}`), godiscovery.Commitments{
		DependencyMaterializationSHA256: snapshot.Binding.DependencyIdentity,
		ModuleMode:                      godiscovery.ModuleModeModule, SourceWSI: snapshot.Binding.SourceIdentity,
		Packages: []godiscovery.PackageMaterial{{ImportPath: "example.test/probe", RawDir: "/workspace/probe", DirNamespace: godiscovery.NamespaceSource, DirLogicalPath: "probe", DirPathSHA256: pathSHA}},
	})
	if err != nil {
		t.Fatal(err)
	}
	discovery, canonical, err := godiscovery.Compose(manifest, godiscovery.DocumentInput{EnvironmentSHA256: request.EnvironmentSHA256, PackagePatterns: request.PackagePatterns, RawStderrSHA256: digestText(""), ToolchainID: snapshot.Binding.ToolchainIdentity})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := provider.ComposeReceipt(provider.ReceiptInput{
		ActualEnvironmentSHA256: request.EnvironmentSHA256, CapabilityID: binding.CapabilityID, PlanID: binding.PlanID, Nonce: strings.Repeat("1", 32),
		Discovery: discovery, DiscoveryCanonical: canonical, Decoder: provider.DecoderRejected,
		SourcePreSHA256: snapshot.SourceObservation, SourcePostSHA256: snapshot.SourceObservation,
		ToolchainPreSHA256: snapshot.ToolchainObservation, ToolchainPostSHA256: snapshot.ToolchainObservation, EphemeralDeletionComplete: true,
		Runner: gorunner.Result{Started: true, Exited: true, ProcessCleanupDone: true, Containment: gorunner.ContainmentProcessGroupBestEffort, Stdout: gorunner.StreamResult{Drained: true, RawSHA256: digestText("")}, Stderr: gorunner.StreamResult{Drained: true, RawSHA256: digestText("")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var run map[string]any
	if err := json.Unmarshal(receipt.Run, &run); err != nil {
		t.Fatal(err)
	}
	return run["scope"].(map[string]any)
}
