package sessiontest

import (
	"fmt"
	"slices"
)

// AdapterManifestProfile versions the capability manifest every registered
// runner adapter carries (STR-V0-004): the discover, plan, execute and cancel
// subset of the LPCV-V0-021 runtime-provider operations. The manifest is held
// in process and checked before discovery; no document carries it.
const AdapterManifestProfile = "corvint-test-adapter/0"

// The LPCV-V0-021 operations STR-V0-004 requires of an adapter.
const (
	OperationCancel   = "cancel"
	OperationDiscover = "discover"
	OperationExecute  = "execute"
	OperationPlan     = "plan"
)

// adapterOperations is the operation set of AdapterManifestProfile, sorted.
// A manifest states exactly this set: a missing operation is unsupported,
// never inferred, and an operation the profile does not define is rejected
// (LPCV-V0-021).
var adapterOperations = []string{OperationCancel, OperationDiscover, OperationExecute, OperationPlan}

// AdapterManifest is one runner adapter's versioned capability manifest.
type AdapterManifest struct {
	Adapter    string
	Operations []string
	Profile    string
}

// AdapterManifests returns the manifest of every registered adapter, keyed by
// adapter name. For the Go adapter, discover is the `go env GOVERSION` probe
// and the `go list ./...` universe check, plan the typed `go test` argv of
// Invocations, execute `go test -json` and cancel the process-group kill. For
// the declared adapter, discover is the runner's version probe, plan the
// declared full command, execute that command and cancel the same kill.
func AdapterManifests() map[string]AdapterManifest {
	manifests := map[string]AdapterManifest{}
	for _, adapter := range []string{AdapterGo, AdapterDeclared} {
		manifests[adapter] = AdapterManifest{Adapter: adapter, Operations: slices.Clone(adapterOperations), Profile: AdapterManifestProfile}
	}
	return manifests
}

// adapterRefusal names why adapter cannot execute under manifests, or ""
// when its manifest is registered, of this version and states exactly the
// required operations.
func adapterRefusal(manifests map[string]AdapterManifest, adapter string) string {
	manifest, ok := manifests[adapter]
	switch {
	case !ok:
		return adapter + ": no registered capability manifest; nothing launched for it"
	case manifest.Adapter != adapter:
		return fmt.Sprintf("%s: its capability manifest names adapter %q; nothing launched for it", adapter, manifest.Adapter)
	case manifest.Profile != AdapterManifestProfile:
		return fmt.Sprintf("%s: capability manifest %q is not %s; nothing launched for it", adapter, manifest.Profile, AdapterManifestProfile)
	}
	for _, operation := range adapterOperations {
		if !slices.Contains(manifest.Operations, operation) {
			return adapter + ": its capability manifest does not state " + operation + "; nothing launched for it"
		}
	}
	if !slices.Equal(manifest.Operations, adapterOperations) {
		return fmt.Sprintf("%s: capability manifest operations %v are not exactly %v; nothing launched for it", adapter, manifest.Operations, adapterOperations)
	}
	return ""
}

// refuseUnmanifested records an INCOMPLETE reason for each adapter of shard
// whose manifest adapterRefusal refuses, and returns planned without that
// adapter's invocations. Their units keep no outcome, so Finish leaves the
// run INCOMPLETE; other adapters still run.
func refuseUnmanifested(receipt *Receipt, manifests map[string]AdapterManifest, m Manifest, shard int, planned []Planned) []Planned {
	refused, checked := map[string]bool{}, map[string]bool{}
	for _, unit := range m.Units {
		if unit.Shard != shard || checked[unit.Adapter] {
			continue
		}
		checked[unit.Adapter] = true
		if reason := adapterRefusal(manifests, unit.Adapter); reason != "" {
			refused[unit.Adapter] = true
			receipt.Incomplete = append(receipt.Incomplete, reason)
		}
	}
	return slices.DeleteFunc(planned, func(invocation Planned) bool { return refused[invocation.Adapter] })
}
