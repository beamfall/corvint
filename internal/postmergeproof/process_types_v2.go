// SPDX-License-Identifier: AGPL-3.0-or-later

// Package postmergeproof owns the closed post-merge /2 process proof wire and
// its concrete offline verifier (PMR-V2-006). It grants no authority, runs no
// observer, author command or network operation, and is the only package that
// can mint a nonzero VerifiedProcessesV2 token.
package postmergeproof

// The types below are the frozen tagged contract in
// conformance/postmerge-runtime-v2/process-types.contract.go.txt.

// ArtifactReaderV2 returns exact immutable bounded bytes. It never chooses an executable.
type ArtifactReaderV2 interface {
	ReadArtifact(id, sha256 string, byteCount int64) ([]byte, error)
}

type ArtifactRefV2 struct {
	ID     string `json:"id"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type ImplementationV2 struct {
	SourceCommit     string `json:"source_commit"`
	SourceTree       string `json:"source_tree"`
	VerifierSHA256   string `json:"verifier_sha256"`
	ObserverSHA256   string `json:"observer_sha256"`
	SupervisorSHA256 string `json:"supervisor_sha256"`
}

type HostTupleV2 struct {
	OS                   string `json:"os"`
	Architecture         string `json:"architecture"`
	KernelRelease        string `json:"kernel_release"`
	ImageManifestSHA256  string `json:"image_manifest_sha256"`
	LauncherPolicySHA256 string `json:"launcher_policy_sha256"`
}

type RoleRuleV2 struct {
	Role              string        `json:"role"`
	Derivation        string        `json:"derivation"`
	Executable        ArtifactRefV2 `json:"executable"`
	LaunchPurpose     string        `json:"launch_purpose"`
	ParentRole        string        `json:"parent_role"`
	Slot              string        `json:"slot"`
	RequiredSwitches  []string      `json:"required_switches"`
	ForbiddenSwitches []string      `json:"forbidden_switches"`
}

type ObservationPolicyV2 struct {
	Profile                      string   `json:"profile"`
	Scope                        string   `json:"scope"`
	TargetIntervalMS             int      `json:"target_interval_ms"`
	CaptureLimit                 int      `json:"capture_limit"`
	ProcessLimit                 int      `json:"process_limit"`
	RequirePairedStat            bool     `json:"require_paired_stat"`
	RequireFinalIndependentSweep bool     `json:"require_final_independent_sweep"`
	Limitations                  []string `json:"limitations"`
}

type ProcessPolicyV2 struct {
	Profile        string              `json:"profile"`
	Implementation ImplementationV2    `json:"implementation"`
	Host           HostTupleV2         `json:"host"`
	Rules          []RoleRuleV2        `json:"rules"`
	Observation    ObservationPolicyV2 `json:"observation"`
}

// SourceBindingV2 and GraphBindingV2 exclude process/host proof, attachment and
// canonical requests. The producer constructs them from verified native inputs.
type SourceBindingV2 struct {
	Kind     string        `json:"kind"`
	Artifact ArtifactRefV2 `json:"artifact"`
}

type RunContextV2 struct {
	RunIndex       *int     `json:"run_index"`
	RunKind        string   `json:"run_kind"`
	RunOrdinal     int      `json:"run_ordinal"`
	TestIDs        []string `json:"test_ids"`
	ControlIndex   *int     `json:"control_index"`
	ControlOrdinal *int     `json:"control_ordinal"`
	AttemptOrdinal *int     `json:"attempt_ordinal"`
}

type GraphBindingV2 struct {
	Profile       string            `json:"profile"`
	ExecutionID   string            `json:"execution_id"`
	Request       ArtifactRefV2     `json:"request"`
	Report        ArtifactRefV2     `json:"report"`
	ProductCommit string            `json:"product_commit"`
	TestCommit    string            `json:"test_commit"`
	DraftCommit   string            `json:"draft_commit"`
	Sources       []SourceBindingV2 `json:"sources"`
	Contexts      []RunContextV2    `json:"contexts"`
}

// ProcCaptureV2 and the types below are exact fresh physical observations,
// never decision facts.
type ProcCaptureV2 struct {
	Index                    int           `json:"index"`
	Phase                    string        `json:"phase"`
	BootIDBytes              []byte        `json:"boot_id_bytes"`
	NamespaceLinkBytes       []byte        `json:"namespace_link_bytes"`
	NamespaceDevice          uint64        `json:"namespace_device"`
	NamespaceInode           uint64        `json:"namespace_inode"`
	StatBefore               []byte        `json:"stat_before"`
	Cmdline                  []byte        `json:"cmdline"`
	CmdlineAfter             []byte        `json:"cmdline_after"`
	ExecutableLinkBytes      []byte        `json:"executable_link_bytes"`
	ExecutableLinkAfterBytes []byte        `json:"executable_link_after_bytes"`
	Executable               ArtifactRefV2 `json:"executable"`
	ExecutableAfter          ArtifactRefV2 `json:"executable_after"`
	StatAfter                []byte        `json:"stat_after"`
	NativeStartOutput        []byte        `json:"native_start_output"`
	NativeStartTool          ArtifactRefV2 `json:"native_start_tool"`
	ExitCode                 *int          `json:"exit_code"`
	ExitSignal               *int          `json:"exit_signal"`
}

type OwnedLaunchV2 struct {
	Index                  int           `json:"index"`
	ContextIndex           int           `json:"context_index"`
	Purpose                string        `json:"purpose"`
	Invocation             ArtifactRefV2 `json:"invocation"`
	Stdin                  ArtifactRefV2 `json:"stdin"`
	ParentCaptureIndex     *int          `json:"parent_capture_index"`
	StartCaptureIndex      int           `json:"start_capture_index"`
	ExecutedCaptureIndex   int           `json:"executed_capture_index"`
	RetirementCaptureIndex *int          `json:"retirement_capture_index"`
	Completed              bool          `json:"completed"`
	Cancelled              bool          `json:"cancelled"`
	TimedOut               bool          `json:"timed_out"`
}

type NativeProcessReferenceV2 struct {
	ArtifactID   string `json:"artifact_id"`
	Pointer      string `json:"pointer"`
	Kind         string `json:"kind"`
	ContextIndex int    `json:"context_index"`
	CaptureIndex int    `json:"capture_index"`
}

// SweepProcessV2 is one row of a second observer's raw /proc PID inventory.
type SweepProcessV2 struct {
	PID       int    `json:"pid"`
	StatBytes []byte `json:"stat_bytes"`
}

type AbsenceSweepV2 struct {
	ObserverInvocation ArtifactRefV2    `json:"observer_invocation"`
	ObserverStdout     ArtifactRefV2    `json:"observer_stdout"`
	BootIDBytes        []byte           `json:"boot_id_bytes"`
	NamespaceLinkBytes []byte           `json:"namespace_link_bytes"`
	PIDDirectoryBytes  []byte           `json:"pid_directory_bytes"`
	Processes          []SweepProcessV2 `json:"processes"`
	ReadFailures       []string         `json:"read_failures"`
}

type CleanupFactsV2 struct {
	OwnedGroup         bool     `json:"owned_group"`
	Status             string   `json:"status"`
	DerivedState       string   `json:"derived_state"`
	DescendantsPresent bool     `json:"descendants_present"`
	Absent             bool     `json:"absent"`
	Qualification      string   `json:"qualification"`
	Cancelled          bool     `json:"cancelled"`
	TimedOut           bool     `json:"timed_out"`
	Scope              string   `json:"scope"`
	IntervalMS         int      `json:"interval_ms"`
	Failures           []string `json:"failures"`
	Limitations        []string `json:"limitations"`
}

type ProcessProofV2 struct {
	Profile                string                     `json:"profile"`
	ExecutionID            string                     `json:"execution_id"`
	PolicySHA256           string                     `json:"policy_sha256"`
	GraphBindingSHA256     string                     `json:"graph_binding_sha256"`
	TrustedStartInvocation ArtifactRefV2              `json:"trusted_start_invocation"`
	TrustedStartStdout     ArtifactRefV2              `json:"trusted_start_stdout"`
	Captures               []ProcCaptureV2            `json:"captures"`
	Launches               []OwnedLaunchV2            `json:"launches"`
	NativeReferences       []NativeProcessReferenceV2 `json:"native_references"`
	FinalSweep             AbsenceSweepV2             `json:"final_sweep"`
	Cleanup                []CleanupJoinV2            `json:"cleanup"`
}

// ProcessAdmissionV2 inputs come from the parent-verified manifest/policy,
// never from proof JSON or expected fixture data.
type ProcessAdmissionV2 struct {
	Policy         ArtifactRefV2
	Qualification  ArtifactRefV2
	HostTuple      HostTupleV2
	Implementation ImplementationV2
}

type LogicalProcessV2 struct {
	Node              string   `json:"node"`
	Role              string   `json:"role"`
	RunKind           string   `json:"run_kind"`
	RunOrdinal        int      `json:"run_ordinal"`
	Parent            *string  `json:"parent"`
	State             string   `json:"state"`
	BirthDistinctFrom []string `json:"birth_distinct_from"`
}

// VerifiedProcessesV2 is the opaque process token. Only the final successful
// branch of VerifyProcessV2 in process_verify_v2.go initializes it; there is
// no exported constructor taking a logical map or bool.
type VerifiedProcessesV2 struct {
	verified            bool
	executionID         string
	graphBindingSHA256  string
	policySHA256        string
	qualificationSHA256 string
	proofSHA256         string
	logical             []LogicalProcessV2
}

// InvocationV2 and the documents below are exact retained preimages; their
// fields are checked, never treated as assertions.
type InvocationV2 struct {
	Profile      string          `json:"profile"`
	ExecutionID  string          `json:"execution_id"`
	Purpose      string          `json:"purpose"`
	Executable   ArtifactRefV2   `json:"executable"`
	Argv         []string        `json:"argv"`
	Environment  []EnvironmentV2 `json:"environment"`
	StdinSHA256  string          `json:"stdin_sha256"`
	ContextIndex int             `json:"context_index"`
	LaunchIndex  int             `json:"launch_index"`
}

type EnvironmentV2 struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type TrustedStartV2 struct {
	Profile          string `json:"profile"`
	ExecutionID      string `json:"execution_id"`
	PolicySHA256     string `json:"policy_sha256"`
	RequestSHA256    string `json:"request_sha256"`
	SupervisorSHA256 string `json:"supervisor_sha256"`
	ObserverSHA256   string `json:"observer_sha256"`
	HostTupleSHA256  string `json:"host_tuple_sha256"`
	RootCaptureIndex int    `json:"root_capture_index"`
}

type AbsenceSweepDocumentV2 struct {
	Profile            string           `json:"profile"`
	ExecutionID        string           `json:"execution_id"`
	BootIDBytes        []byte           `json:"boot_id_bytes"`
	NamespaceLinkBytes []byte           `json:"namespace_link_bytes"`
	PIDDirectoryBytes  []byte           `json:"pid_directory_bytes"`
	Processes          []SweepProcessV2 `json:"processes"`
	ReadFailures       []string         `json:"read_failures"`
}

type CleanupJoinV2 struct {
	ArtifactID   string         `json:"artifact_id"`
	Pointer      string         `json:"pointer"`
	ContextIndex int            `json:"context_index"`
	Facts        CleanupFactsV2 `json:"facts"`
}

type QualificationCaseV2 struct {
	ID                string          `json:"id"`
	GraphBindings     []ArtifactRefV2 `json:"graph_bindings"`
	Policy            ArtifactRefV2   `json:"policy"`
	Proofs            []ArtifactRefV2 `json:"proofs"`
	HarnessInvocation ArtifactRefV2   `json:"harness_invocation"`
	HarnessStdout     ArtifactRefV2   `json:"harness_stdout"`
}

type ProcessQualificationV2 struct {
	Profile        string                `json:"profile"`
	PolicySHA256   string                `json:"policy_sha256"`
	Implementation ImplementationV2      `json:"implementation"`
	Host           HostTupleV2           `json:"host"`
	Cases          []QualificationCaseV2 `json:"cases"`
	Limitations    []string              `json:"limitations"`
}
