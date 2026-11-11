// SPDX-License-Identifier: Apache-2.0

package affected

// The E2E member of a plan (APK-V0-001 to APK-V0-007, AFU-V1-140 to
// AFU-V1-145): the E2E test files a change requires, each naming the flow,
// variation and changed path that selected it, with the inventory it was
// drawn from and the typed reasons the answer is wider than the evidence.

// Test kinds a selection carries when the repository declares its flows
// (APK-V0-002). A kind is declared or UNKNOWN, never guessed from a name.
const (
	TestKindUnit        = "unit"
	TestKindIntegration = "integration"
	TestKindE2E         = "e2e"
	TestKindUnknown     = "UNKNOWN"
)

// E2E scopes (APK-V0-004): BOUNDED when every unselected inventory file has a
// non-stale flow link explaining it, FULL when every inventory file is
// selected for a typed reason, UNKNOWN when some could not be explained.
const (
	E2EScopeBounded = ScopeBounded
	E2EScopeFull    = "FULL"
	E2EScopeUnknown = ScopeUnknown
)

// Reasons of an E2E selection (APK-V0-003, APK-V0-004, AFU-V1-141,
// AFU-V1-142).
const (
	E2EReasonFlowReach            = "flow-reach"
	E2EReasonTestFileChanged      = "test-file-changed"
	E2EReasonStaticReach          = "static-reach"
	E2EReasonUnlinked             = "unlinked"
	E2EReasonStaleLink            = "stale-link"
	E2EReasonUnreviewedLink       = "unreviewed-link"
	E2EReasonGlobalPathChanged    = "global-path-changed"
	E2EReasonPlanUnbounded        = "plan-unbounded"
	E2EReasonBoundExceeded        = "bound-exceeded"
	E2EReasonInventoryUnderivable = "inventory-underivable"
)

// Unknown reasons of the E2E member (APK-V0-004, AFU-V1-142). They live under
// plan.e2e.unknown and never under plan.unknown.
const (
	UnknownFlowsDeclarationInvalid = "FLOWS_DECLARATION_INVALID"
	// UnknownFlowsSchemaUnsupported: the flows declaration names a
	// well-formed schema version this build does not read (FSV-V1-004).
	UnknownFlowsSchemaUnsupported  = "FLOWS_SCHEMA_UNSUPPORTED"
	UnknownE2EFlowStale            = "E2E_FLOW_STALE"
	UnknownE2EFlowUnreviewed       = "E2E_FLOW_UNREVIEWED"
	UnknownE2EGlobalPath           = "E2E_GLOBAL_PATH"
	UnknownE2EPlanUnbounded        = "E2E_PLAN_UNBOUNDED"
	UnknownE2EBoundExceeded        = "E2E_BOUND_EXCEEDED"
	UnknownE2EInventoryUnderivable = "E2E_INVENTORY_UNDERIVABLE"
)

// Inventory sources (APK-V0 Wire, AFU-V1-140, AFU-V1-145).
const (
	E2EInventoryDiscovery = "discovery"
	E2EInventoryFlows     = "flows"
	E2EInventoryProvider  = "provider"
)

// E2EPlan is the plan's `e2e` member: present only when the repository
// declares its flows (APK-V0-001).
type E2EPlan struct {
	// Flows is the declared intent directory.
	Flows string `json:"flows"`
	// Scope is E2EScopeBounded, E2EScopeFull or E2EScopeUnknown.
	Scope     string       `json:"scope"`
	Inventory E2EInventory `json:"inventory"`
	// Selected is sorted by project then path (APK-V0-006).
	Selected []E2ESelection `json:"selected"`
	// Unknown names why the answer is wider than the evidence; omitted when
	// the scope is BOUNDED.
	Unknown []Unknown `json:"unknown,omitempty"`
}

// E2EInventory says where the E2E test files came from and what state the
// evidence was in.
type E2EInventory struct {
	// Source is E2EInventoryDiscovery, E2EInventoryFlows or
	// E2EInventoryProvider.
	Source string `json:"source"`
	// State is the discovery receipt state (MATCHED, MISSING, MALFORMED,
	// BINDING_MISMATCH), "config:untracked", or "n/a" without a receipt.
	State string `json:"state"`
	Count int    `json:"count"`
	// Unlinked lists, sorted, each inventory path no flow link names
	// (AFU-V1-141); omitted when none.
	Unlinked []string `json:"unlinked,omitempty"`
	// ProviderOmits lists, sorted, each derived inventory path a
	// --provider inventory does not name (AFU-V1-145); omitted when none or
	// without a provider.
	ProviderOmits []string `json:"providerOmits,omitempty"`
}

// E2ESelection is one E2E test file the change requires.
type E2ESelection struct {
	Path string `json:"path"`
	// Project is the Playwright project of a discovered file; omitted for a
	// flows-only inventory.
	Project  string `json:"project,omitempty"`
	TestKind string `json:"testKind"`
	// TestKeys are the flow-link test keys inside the file that the
	// selecting variations attach, sorted; omitted when none.
	TestKeys []string `json:"testKeys,omitempty"`
	// Reasons are the E2EReason* values, sorted.
	Reasons []string `json:"reasons"`
	// Flows is one row per (flow, variation, changed path) that selected the
	// file (APK-V0-003); omitted when no flow hit selected it.
	Flows []E2EFlowHit `json:"flows,omitempty"`
}

// E2EFlowHit names the flow, the selecting variation, the changed path that
// reached the flow and the link target it reached.
type E2EFlowHit struct {
	Flow        string `json:"flow"`
	Variation   string `json:"variation"`
	ChangedPath string `json:"changedPath"`
	Target      string `json:"target"`
}
