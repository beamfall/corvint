// SPDX-License-Identifier: AGPL-3.0-or-later

package postmergeproof

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
)

// VerifyProcessV2 is the concrete PMR-V2-006 process verifier. It checks the
// parent admission and its exact qualification report, strictly decodes the
// closed policy and proof, verifies every raw preimage against the store and
// the independently constructed expected graph, and only then mints the
// opaque token. It runs no observer, author command or network operation.
func VerifyProcessV2(ctx context.Context, admission ProcessAdmissionV2,
	expected GraphBindingV2, policyBytes, qualificationBytes, proofBytes []byte,
	store ArtifactReaderV2) (VerifiedProcessesV2, error) {
	if ctx == nil || store == nil {
		return VerifiedProcessesV2{}, blocked("process-artifact-unavailable", "context and artifact reader are required")
	}
	if err := ctx.Err(); err != nil {
		return VerifiedProcessesV2{}, blocked("process-verification-cancelled", err.Error())
	}
	if !supportedHostV2(admission.HostTuple) {
		return VerifiedProcessesV2{}, blocked("process-host-unsupported", "only the linux amd64/arm64 procfs profile is defined")
	}
	if !admittedBytes(admission.Policy, policyBytes) {
		return VerifiedProcessesV2{}, rejected("process-policy-digest-mismatch", "policy bytes do not match the parent-pinned policy reference")
	}
	if admission.Qualification == (ArtifactRefV2{}) || len(qualificationBytes) == 0 {
		return VerifiedProcessesV2{}, blocked("process-qualification-unavailable", "no host qualification report is admitted for this policy, implementation and host tuple")
	}
	if !admittedBytes(admission.Qualification, qualificationBytes) {
		return VerifiedProcessesV2{}, rejected("process-qualification-invalid", "qualification bytes do not match the parent-pinned reference")
	}
	policy, err := decodePolicyV2(policyBytes, admission)
	if err != nil {
		return VerifiedProcessesV2{}, err
	}
	policySHA := admission.Policy.SHA256
	if err := verifyQualificationV2(ctx, admission, policy, policySHA, qualificationBytes, store); err != nil {
		return VerifiedProcessesV2{}, err
	}
	result, err := verifyRawProcessV2(ctx, policy, policySHA, expected, proofBytes, store)
	if err != nil {
		return VerifiedProcessesV2{}, err
	}
	return VerifiedProcessesV2{
		verified:            true,
		executionID:         expected.ExecutionID,
		graphBindingSHA256:  result.graphSHA,
		policySHA256:        policySHA,
		qualificationSHA256: admission.Qualification.SHA256,
		proofSHA256:         sha256Hex(proofBytes),
		logical:             cloneLogicalV2(result.logical),
	}, nil
}

// ValidFor reports whether a nonzero token binds exactly these identities.
func (v VerifiedProcessesV2) ValidFor(executionID, graphSHA256, policySHA256, qualificationSHA256, proofSHA256 string) bool {
	return v.verified && v.executionID != "" && v.executionID == executionID && v.graphBindingSHA256 == graphSHA256 &&
		v.policySHA256 == policySHA256 && v.qualificationSHA256 == qualificationSHA256 && v.proofSHA256 == proofSHA256
}

// LogicalProcesses returns a deep copy of the verified logical facts.
func (v VerifiedProcessesV2) LogicalProcesses() ([]LogicalProcessV2, error) {
	if !v.verified {
		return nil, blocked("process-token-invalid", "zero process token")
	}
	return cloneLogicalV2(v.logical), nil
}

func cloneLogicalV2(in []LogicalProcessV2) []LogicalProcessV2 {
	out := make([]LogicalProcessV2, len(in))
	for i, p := range in {
		out[i] = p
		if p.Parent != nil {
			parent := *p.Parent
			out[i].Parent = &parent
		}
		out[i].BirthDistinctFrom = slices.Clone(p.BirthDistinctFrom)
		if out[i].BirthDistinctFrom == nil {
			out[i].BirthDistinctFrom = []string{}
		}
	}
	return out
}

func supportedHostV2(host HostTupleV2) bool {
	return host.OS == "linux" && (host.Architecture == "amd64" || host.Architecture == "arm64")
}

func admittedBytes(ref ArtifactRefV2, data []byte) bool {
	return ref.ID != "" && shaPattern.MatchString(ref.SHA256) && ref.Bytes == int64(len(data)) &&
		len(data) <= maxDocumentBytes && sha256Hex(data) == ref.SHA256
}

var (
	ownedLaunchRoles = []string{"host-supervisor", "workflow-root", "assessment-worker", "runner", "server", "observer", "attestor", "control-hook", "cleanup-hook"}
	browserRoles     = []string{"browser-main", "browser-zygote", "browser-renderer", "browser-gpu", "browser-utility"}
	// environmentKeys is the fixed invocation environment allowlist: the
	// testacceptance safe environment keys plus TZ for the native start tool.
	environmentKeys = []string{"LANG", "LC_ALL", "PATH", "TMPDIR", "TZ"}
	phaseRank       = map[string]int{"start-barrier": 0, "during-run": 1, "final-sweep": 2, "retirement": 3}
	emptySHA256     = sha256Hex(nil)
)

func emptyRef(ref ArtifactRefV2) bool {
	return ref.ID == "" && ref.SHA256 == emptySHA256 && ref.Bytes == 0
}

func sameContent(a, b ArtifactRefV2) bool { return a.SHA256 == b.SHA256 && a.Bytes == b.Bytes }

// decodePolicyV2 strictly decodes the parent-pinned policy and checks its
// finite role table. Rules are fixed recipes; nothing is registered dynamically.
func decodePolicyV2(data []byte, admission ProcessAdmissionV2) (ProcessPolicyV2, error) {
	var policy ProcessPolicyV2
	if err := decodeWire(data, "ProcessPolicyV2", &policy); err != nil {
		return policy, err
	}
	if policy.Host != admission.HostTuple || policy.Implementation != admission.Implementation {
		return policy, rejected("process-admission-mismatch", "policy host tuple or implementation differs from the parent admission")
	}
	seen := map[string]bool{}
	for _, rule := range policy.Rules {
		key := rule.Role + "\x00" + rule.Slot
		if seen[key] {
			return policy, blocked("process-role-ambiguous", "duplicate role/slot rule "+rule.Role+"/"+rule.Slot)
		}
		seen[key] = true
		if rule.LaunchPurpose != rule.Role || rule.Executable.ID == "" || rule.Executable.Bytes == 0 {
			return policy, blocked("process-role-unsupported", "rule "+rule.Role+" has an unsupported purpose or executable")
		}
		switch rule.Derivation {
		case "owned-launch/0":
			outside := rule.Role == "host-supervisor"
			if !slices.Contains(ownedLaunchRoles, rule.Role) || (rule.ParentRole == "outside") != outside ||
				len(rule.RequiredSwitches) != 0 || len(rule.ForbiddenSwitches) != 0 {
				return policy, blocked("process-role-unsupported", "owned-launch rule "+rule.Role+" is not a supported form")
			}
		case "chromium-switch-role/0":
			if !slices.Contains(browserRoles, rule.Role) || rule.ParentRole == "outside" || !switchTokens(rule.RequiredSwitches) || !switchTokens(rule.ForbiddenSwitches) {
				return policy, blocked("process-role-unsupported", "chromium rule "+rule.Role+" is not a supported form")
			}
			for _, required := range rule.RequiredSwitches {
				if slices.Contains(rule.ForbiddenSwitches, required) {
					return policy, blocked("process-role-ambiguous", "switch is both required and forbidden")
				}
			}
		default:
			// playwright-node-worker/0 needs the native worker/project join,
			// which no qualified producer supplies yet.
			return policy, blocked("process-role-unsupported", "derivation "+rule.Derivation+" is not implemented")
		}
	}
	return policy, nil
}

func switchTokens(tokens []string) bool {
	for i, token := range tokens {
		if !strings.HasPrefix(token, "--") || len(token) < 3 || strings.ContainsAny(token, "\x00 ") || slices.Contains(tokens[:i], token) {
			return false
		}
	}
	return true
}

type birthKey struct {
	boot  string
	nsDev uint64
	nsIno uint64
	pid   int
	start uint64
}

type captureV2 struct {
	raw   *ProcCaptureV2
	stat  procStatV2
	birth birthKey
	live  bool
	// The native lstart output under each native observer's own profile:
	// freshness/app-instance trim it; procgroup descendants join its fields.
	startTrim   string
	startFields string
}

type birthV2 struct {
	key      birthKey
	captures []int
	ppid     int
	launch   int // -1 when the birth has no owned launch
	mapped   bool
	infra    bool // trusted-start or final-sweep observer witness
	role     string
	slot     string
	context  int
	node     string
	parent   *birthV2
}

type rawProcessResultV2 struct {
	graphSHA   string
	logical    []LogicalProcessV2
	workload   []birthKey
	references int
	cleanups   int
}

type verifierV2 struct {
	ctx         context.Context
	store       ArtifactReaderV2
	policy      ProcessPolicyV2
	policySHA   string
	graph       GraphBindingV2
	graphSHA    string
	proof       ProcessProofV2
	documents   map[string][]byte
	executables map[string]bool
	captures    []captureV2
	births      map[birthKey]*birthV2
	order       []*birthV2
	invocations []InvocationV2
	trusted     int
	sweeper     int
	native      *nativeSourcesV2
}

func refKey(ref ArtifactRefV2) string {
	return ref.ID + "\x00" + ref.SHA256 + "\x00" + strconv.FormatInt(ref.Bytes, 10)
}

func (v *verifierV2) fetch(ref ArtifactRefV2, limit int64) ([]byte, error) {
	if err := v.ctx.Err(); err != nil {
		return nil, blocked("process-verification-cancelled", err.Error())
	}
	if emptyRef(ref) {
		return []byte{}, nil
	}
	if ref.ID == "" {
		return nil, rejected("process-artifact-digest-mismatch", "non-empty artifact reference has no ID")
	}
	if ref.Bytes > limit {
		return nil, blocked("process-bound-exceeded", "artifact "+ref.ID+" exceeds its byte bound")
	}
	data, err := v.store.ReadArtifact(ref.ID, ref.SHA256, ref.Bytes)
	if err != nil {
		return nil, blocked("process-artifact-unavailable", ref.ID)
	}
	if int64(len(data)) != ref.Bytes || sha256Hex(data) != ref.SHA256 {
		return nil, rejected("process-artifact-digest-mismatch", ref.ID)
	}
	return data, nil
}

func (v *verifierV2) readDocument(ref ArtifactRefV2) ([]byte, error) {
	key := refKey(ref)
	if data, ok := v.documents[key]; ok {
		return data, nil
	}
	data, err := v.fetch(ref, maxDocumentBytes)
	if err == nil {
		v.documents[key] = data
	}
	return data, err
}

// verifyExecutable checks full executable bytes without retaining them.
func (v *verifierV2) verifyExecutable(ref ArtifactRefV2) error {
	key := refKey(ref)
	if v.executables[key] {
		return nil
	}
	if emptyRef(ref) {
		return rejected("process-executable-mismatch", "executable bytes are empty")
	}
	if _, err := v.fetch(ref, maxExecutableBytes); err != nil {
		return err
	}
	v.executables[key] = true
	return nil
}

// legacySampleV2 recognizes the current sampled DescendantObservation (and
// other profile-less PID row) shapes, which cannot populate a /2 proof.
func legacySampleV2(data []byte) bool {
	tree, err := parseJSONTree(data, false)
	if err != nil {
		return false
	}
	if tree.kind == 'a' {
		return true
	}
	_, profile := tree.obj["profile"]
	_, processes := tree.obj["processes"]
	return tree.kind == 'o' && !profile && processes
}

// verifyRawProcessV2 validates one raw proof against an expected graph and
// policy without minting a token or requiring qualification, so qualification
// can use it without calling itself.
func verifyRawProcessV2(ctx context.Context, policy ProcessPolicyV2, policySHA string, expected GraphBindingV2,
	proofBytes []byte, store ArtifactReaderV2) (rawProcessResultV2, error) {
	v := &verifierV2{ctx: ctx, store: store, policy: policy, policySHA: policySHA, graph: expected,
		documents: map[string][]byte{}, executables: map[string]bool{}, births: map[birthKey]*birthV2{}}
	if err := ctx.Err(); err != nil {
		return rawProcessResultV2{}, blocked("process-verification-cancelled", err.Error())
	}
	if len(proofBytes) > maxDocumentBytes {
		return rawProcessResultV2{}, blocked("process-bound-exceeded", "proof exceeds its byte bound")
	}
	if legacySampleV2(proofBytes) {
		return rawProcessResultV2{}, blocked("process-legacy-sample-unsupported", "sampled PID/start rows have no kernel birth or command evidence")
	}
	if err := decodeWire(proofBytes, "ProcessProofV2", &v.proof); err != nil {
		return rawProcessResultV2{}, err
	}
	graphSHA, err := GraphBindingSHA256V2(expected)
	if err != nil {
		return rawProcessResultV2{}, blocked("process-graph-invalid", err.Error())
	}
	v.graphSHA = graphSHA
	if v.proof.ExecutionID != expected.ExecutionID || v.proof.GraphBindingSHA256 != graphSHA {
		return rawProcessResultV2{}, rejected("process-graph-substituted", "proof is bound to another execution or native graph")
	}
	if v.proof.PolicySHA256 != policySHA {
		return rawProcessResultV2{}, rejected("process-policy-digest-mismatch", "proof is bound to another policy")
	}
	steps := []func() error{
		func() error { native, err := v.loadNativeSourcesV2(); v.native = native; return err },
		v.checkCapturesV2,
		v.checkLaunchesV2,
		v.checkTrustedStartV2,
		v.mapObservedBirthsV2,
		v.checkNativeReferencesV2,
		v.checkCleanupV2,
		v.checkFinalSweepV2,
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return rawProcessResultV2{}, err
		}
	}
	return v.logicalV2(), nil
}

var exitUnknown = "exit:unknown"

func exitFactsV2(c *ProcCaptureV2) (string, error) {
	switch {
	case c.ExitCode == nil && c.ExitSignal == nil:
		return exitUnknown, nil
	case c.ExitSignal == nil && *c.ExitCode >= 0:
		return "exit:" + strconv.Itoa(*c.ExitCode), nil
	case c.ExitSignal != nil && (c.ExitCode == nil || *c.ExitCode == -1):
		return "signal:" + strconv.Itoa(*c.ExitSignal), nil
	}
	return "", rejected("process-retirement-invalid", "exit status and signal are inconsistent")
}

func namespaceLinkInode(link []byte) (uint64, bool) {
	text := string(link)
	if !strings.HasPrefix(text, "pid:[") || !strings.HasSuffix(text, "]") {
		return 0, false
	}
	digits := []byte(text[5 : len(text)-1])
	if len(digits) == 0 || len(digits) > 20 || (len(digits) > 1 && digits[0] == '0') {
		return 0, false
	}
	value, err := strconv.ParseUint(string(digits), 10, 64)
	return value, err == nil
}

// normalizeNativeStart returns the native `ps -o lstart=` output as the
// freshness observer trims it and as procgroup joins its descendant columns.
func normalizeNativeStart(output []byte) (trimmed, joined string, ok bool) {
	fields := strings.Fields(string(output))
	if len(fields) != 5 || !bytes.HasSuffix(output, []byte("\n")) {
		return "", "", false
	}
	return strings.TrimSpace(string(output)), strings.Join(fields, " "), true
}

// statRefusalV2 maps a stat parse failure: a pre-3.5 kernel layout is an
// unsupported host, anything else is malformed evidence.
func statRefusalV2(i int, which string, err error) error {
	detail := "capture " + strconv.Itoa(i) + " " + which + ": " + err.Error()
	if errors.Is(err, errStatLegacyLayout) {
		return blocked("process-host-unsupported", detail)
	}
	return rejected("process-stat-malformed", detail)
}

// checkCapturesV2 parses every bracketed capture and groups them by birth key.
func (v *verifierV2) checkCapturesV2() error {
	captures := v.proof.Captures
	if len(captures) > v.policy.Observation.CaptureLimit {
		return blocked("process-bound-exceeded", "captures exceed the policy capture limit")
	}
	if len(captures) == 0 {
		return blocked("process-role-unsupported", "proof has no captures")
	}
	v.captures = make([]captureV2, len(captures))
	var nativeTool *ArtifactRefV2
	for i := range captures {
		c := &captures[i]
		if c.Index != i {
			return rejected("process-capture-malformed", "capture indexes are not contiguous")
		}
		before, err := parseProcStatV2(c.StatBefore)
		if err != nil {
			return statRefusalV2(i, "stat_before", err)
		}
		after, err := parseProcStatV2(c.StatAfter)
		if err != nil {
			return statRefusalV2(i, "stat_after", err)
		}
		if before.pid != after.pid || before.startTicks != after.startTicks {
			return rejected("process-birth-changed", "capture "+strconv.Itoa(i)+" changed birth within its bracket")
		}
		if before.zombie() != after.zombie() || !bytes.Equal(c.Cmdline, c.CmdlineAfter) ||
			!bytes.Equal(c.ExecutableLinkBytes, c.ExecutableLinkAfterBytes) || c.Executable != c.ExecutableAfter {
			return rejected("process-bracket-changed", "capture "+strconv.Itoa(i)+" changed within its bracket")
		}
		// A PPID-only change is the kernel reparenting a birth whose parent
		// exited mid-bracket: benign, but the parent edge is then unverified.
		if before.ppid != after.ppid {
			return blocked("process-parent-unverified", "capture "+strconv.Itoa(i)+" was reparented within its bracket")
		}
		inode, ok := namespaceLinkInode(c.NamespaceLinkBytes)
		if len(c.BootIDBytes) == 0 || !ok || inode != c.NamespaceInode {
			return rejected("process-capture-malformed", "capture "+strconv.Itoa(i)+" boot or PID namespace identity is malformed")
		}
		first := &captures[0]
		if !bytes.Equal(c.BootIDBytes, first.BootIDBytes) || !bytes.Equal(c.NamespaceLinkBytes, first.NamespaceLinkBytes) ||
			c.NamespaceDevice != first.NamespaceDevice || c.NamespaceInode != first.NamespaceInode {
			return rejected("process-namespace-mismatch", "captures span boots or PID namespaces")
		}
		live := !before.zombie()
		if live {
			if len(c.Cmdline) == 0 || c.Cmdline[len(c.Cmdline)-1] != 0 || len(c.ExecutableLinkBytes) == 0 {
				return rejected("process-capture-malformed", "capture "+strconv.Itoa(i)+" argv or executable link is malformed")
			}
			if err := v.verifyExecutable(c.Executable); err != nil {
				return err
			}
		} else if len(c.Cmdline) != 0 || len(c.ExecutableLinkBytes) != 0 || !emptyRef(c.Executable) {
			return rejected("process-capture-malformed", "zombie capture "+strconv.Itoa(i)+" claims a command or executable")
		}
		trimmed, joined := "", ""
		if len(c.NativeStartOutput) != 0 || !emptyRef(c.NativeStartTool) {
			if trimmed, joined, ok = normalizeNativeStart(c.NativeStartOutput); !ok || emptyRef(c.NativeStartTool) {
				return rejected("process-capture-malformed", "capture "+strconv.Itoa(i)+" native start output is malformed")
			}
			// The tool is the native-reference join key, so every capture
			// must use one; qualification does not yet pin which one.
			if nativeTool == nil {
				nativeTool = &c.NativeStartTool
			} else if *nativeTool != c.NativeStartTool {
				return rejected("process-capture-malformed", "capture "+strconv.Itoa(i)+" uses another native start tool")
			}
			if err := v.verifyExecutable(c.NativeStartTool); err != nil {
				return err
			}
		}
		if c.ExitCode != nil || c.ExitSignal != nil {
			if c.Phase != "retirement" || live {
				return rejected("process-retirement-invalid", "exit facts appear outside a reaped retirement capture")
			}
		}
		if _, err := exitFactsV2(c); err != nil {
			return err
		}
		key := birthKey{boot: string(c.BootIDBytes), nsDev: c.NamespaceDevice, nsIno: c.NamespaceInode, pid: before.pid, start: before.startTicks}
		v.captures[i] = captureV2{raw: c, stat: before, birth: key, live: live, startTrim: trimmed, startFields: joined}
		birth := v.births[key]
		if birth == nil {
			if len(v.births) >= v.policy.Observation.ProcessLimit {
				return blocked("process-bound-exceeded", "distinct births exceed the policy process limit")
			}
			birth = &birthV2{key: key, ppid: before.ppid, launch: -1}
			v.births[key] = birth
		} else {
			previous := &v.captures[birth.captures[len(birth.captures)-1]]
			if previous.raw.Phase == "retirement" || (!previous.live && live) || phaseRank[c.Phase] < phaseRank[previous.raw.Phase] {
				return rejected("process-retirement-invalid", "capture "+strconv.Itoa(i)+" contradicts its birth's observed phase order")
			}
			if birth.ppid != before.ppid {
				return blocked("process-parent-unverified", "birth was reparented between captures")
			}
		}
		birth.captures = append(birth.captures, i)
	}
	v.order = make([]*birthV2, 0, len(v.births))
	for _, birth := range v.births {
		v.order = append(v.order, birth)
	}
	slices.SortFunc(v.order, func(a, b *birthV2) int { return compareBirth(a.key, b.key) })
	return nil
}

func compareBirth(a, b birthKey) int {
	switch {
	case a.pid != b.pid:
		return a.pid - b.pid
	case a.start != b.start:
		if a.start < b.start {
			return -1
		}
		return 1
	}
	return strings.Compare(a.boot, b.boot)
}

func argvBytes(argv []string) []byte {
	var b []byte
	for _, arg := range argv {
		b = append(b, arg...)
		b = append(b, 0)
	}
	return b
}

// invocationV2 decodes and verifies one retained supervisor invocation preimage.
func (v *verifierV2) invocationV2(ref ArtifactRefV2) (InvocationV2, error) {
	var inv InvocationV2
	data, err := v.readDocument(ref)
	if err != nil {
		return inv, err
	}
	if err := decodeWire(data, "InvocationV2", &inv); err != nil {
		return inv, err
	}
	if inv.ExecutionID != v.graph.ExecutionID || len(inv.Argv) == 0 {
		return inv, rejected("process-invocation-mismatch", "invocation execution or argv is invalid")
	}
	for _, arg := range inv.Argv {
		if strings.IndexByte(arg, 0) >= 0 {
			return inv, rejected("process-invocation-mismatch", "argv contains NUL")
		}
	}
	for i, env := range inv.Environment {
		if !slices.Contains(environmentKeys, env.Key) || strings.IndexByte(env.Value, 0) >= 0 ||
			(i > 0 && inv.Environment[i-1].Key >= env.Key) {
			return inv, rejected("process-invocation-mismatch", "invocation environment is outside the fixed sorted allowlist")
		}
	}
	return inv, v.verifyExecutable(inv.Executable)
}

func (v *verifierV2) capture(index int) (*captureV2, bool) {
	if index < 0 || index >= len(v.captures) {
		return nil, false
	}
	return &v.captures[index], true
}

// checkLaunchesV2 joins every owned launch to its retained invocation/stdin,
// birth bracket, executable bytes, rule, parent capture and retirement.
func (v *verifierV2) checkLaunchesV2() error {
	launches := v.proof.Launches
	impl := v.policy.Implementation
	v.invocations = make([]InvocationV2, len(launches))
	for i := range launches {
		l := &launches[i]
		where := "launch " + strconv.Itoa(i)
		if l.Index != i || l.ContextIndex >= len(v.graph.Contexts) {
			return rejected("process-invocation-mismatch", where+" index or context is invalid")
		}
		inv, err := v.invocationV2(l.Invocation)
		if err != nil {
			return err
		}
		v.invocations[i] = inv
		if inv.Purpose != l.Purpose || inv.ContextIndex != l.ContextIndex || inv.LaunchIndex != i || inv.StdinSHA256 != l.Stdin.SHA256 {
			return rejected("process-invocation-mismatch", where+" does not match its invocation preimage")
		}
		if _, err := v.readDocument(l.Stdin); err != nil {
			return err
		}
		start, okStart := v.capture(l.StartCaptureIndex)
		executed, okExecuted := v.capture(l.ExecutedCaptureIndex)
		if !okStart || !okExecuted || l.StartCaptureIndex > l.ExecutedCaptureIndex {
			return rejected("process-invocation-mismatch", where+" capture indexes are invalid")
		}
		if start.birth != executed.birth {
			return rejected("process-birth-changed", where+" start and executed captures are different births")
		}
		if !executed.live || !sameContent(executed.raw.Executable, inv.Executable) || !bytes.Equal(executed.raw.Cmdline, argvBytes(inv.Argv)) {
			return rejected("process-executable-mismatch", where+" executed image differs from its invocation")
		}
		if (l.Purpose == "host-supervisor" && inv.Executable.SHA256 != impl.SupervisorSHA256) ||
			(l.Purpose == "observer" && inv.Executable.SHA256 != impl.ObserverSHA256) {
			return rejected("process-executable-mismatch", where+" is not the admitted implementation")
		}
		birth := v.births[executed.birth]
		for _, index := range birth.captures {
			if c := &v.captures[index]; index > l.ExecutedCaptureIndex && c.live &&
				(!sameContent(c.raw.Executable, executed.raw.Executable) || !bytes.Equal(c.raw.Cmdline, executed.raw.Cmdline)) {
				return rejected("process-executable-mismatch", where+" image changed after exec")
			}
		}
		rule, err := v.matchRule(func(r RoleRuleV2) bool {
			return r.Derivation == "owned-launch/0" && r.Role == l.Purpose && sameContent(r.Executable, inv.Executable)
		})
		if err != nil {
			return err
		}
		if birth.launch >= 0 {
			return blocked("process-role-ambiguous", "one birth joins more than one owned launch")
		}
		birth.launch, birth.mapped, birth.role, birth.slot, birth.context = i, true, rule.Role, rule.Slot, l.ContextIndex
		if l.RetirementCaptureIndex != nil {
			retirement, ok := v.capture(*l.RetirementCaptureIndex)
			if !ok || *l.RetirementCaptureIndex < l.ExecutedCaptureIndex {
				return rejected("process-retirement-invalid", where+" retirement capture is invalid")
			}
			if retirement.birth != executed.birth {
				return rejected("process-birth-changed", where+" retirement capture is another birth")
			}
			if retirement.raw.Phase != "retirement" {
				return rejected("process-retirement-invalid", where+" retirement capture has another phase")
			}
		}
		if l.Completed {
			if l.RetirementCaptureIndex == nil {
				return rejected("process-retirement-invalid", where+" completed without a reaped retirement")
			}
			if facts, _ := exitFactsV2(v.captures[*l.RetirementCaptureIndex].raw); facts == exitUnknown {
				return rejected("process-retirement-invalid", where+" completed without an exit status")
			}
		}
	}
	// Parent edges are resolved after every launch birth has its role.
	for i := range launches {
		l := &launches[i]
		birth := v.births[v.captures[l.ExecutedCaptureIndex].birth]
		rule, _ := v.matchRule(func(r RoleRuleV2) bool { return r.Role == birth.role && r.Slot == birth.slot })
		if rule.ParentRole == "outside" {
			if l.ParentCaptureIndex != nil || l.ContextIndex != 0 {
				return rejected("process-invocation-mismatch", "host supervisor must be the outside workflow boundary")
			}
			continue
		}
		if l.ParentCaptureIndex == nil {
			return blocked("process-parent-unverified", "launch "+strconv.Itoa(i)+" has no parent capture")
		}
		parentCapture, ok := v.capture(*l.ParentCaptureIndex)
		if !ok {
			return rejected("process-invocation-mismatch", "launch "+strconv.Itoa(i)+" parent capture is invalid")
		}
		parent := v.births[parentCapture.birth]
		if parent == birth || parent.key.pid != birth.ppid || parent.key.start > birth.key.start || parent.launch < 0 || parent.role != rule.ParentRole {
			return blocked("process-parent-unverified", "launch "+strconv.Itoa(i)+" parent is not the mapped "+rule.ParentRole)
		}
		birth.parent = parent
	}
	return nil
}

func (v *verifierV2) matchRule(match func(RoleRuleV2) bool) (RoleRuleV2, error) {
	var found []RoleRuleV2
	for _, rule := range v.policy.Rules {
		if match(rule) {
			found = append(found, rule)
		}
	}
	switch len(found) {
	case 0:
		return RoleRuleV2{}, blocked("process-role-unsupported", "no admitted rule derives this birth")
	case 1:
		return found[0], nil
	}
	return RoleRuleV2{}, blocked("process-role-ambiguous", "more than one admitted rule derives this birth")
}

// checkTrustedStartV2 verifies the barrier observer's invocation and output
// and identifies the trusted-start and final-sweep observer witnesses.
func (v *verifierV2) checkTrustedStartV2() error {
	impl := v.policy.Implementation
	inv, err := v.invocationV2(v.proof.TrustedStartInvocation)
	if err != nil {
		return err
	}
	if inv.Purpose != "observer" || inv.ContextIndex != 0 || inv.Executable.SHA256 != impl.ObserverSHA256 ||
		inv.LaunchIndex >= len(v.proof.Launches) || v.proof.Launches[inv.LaunchIndex].Invocation != v.proof.TrustedStartInvocation {
		return rejected("process-trusted-start-mismatch", "trusted start invocation is not the admitted barrier observer launch")
	}
	v.trusted = inv.LaunchIndex
	data, err := v.readDocument(v.proof.TrustedStartStdout)
	if err != nil {
		return err
	}
	var start TrustedStartV2
	if err := decodeWire(data, "TrustedStartV2", &start); err != nil {
		return err
	}
	hostSHA, err := HostTupleSHA256V2(v.policy.Host)
	if err != nil {
		return rejected("process-admission-mismatch", err.Error())
	}
	if start.ExecutionID != v.graph.ExecutionID || start.PolicySHA256 != v.policySHA || start.RequestSHA256 != v.graph.Request.SHA256 ||
		start.SupervisorSHA256 != impl.SupervisorSHA256 || start.ObserverSHA256 != impl.ObserverSHA256 || start.HostTupleSHA256 != hostSHA {
		return rejected("process-trusted-start-mismatch", "trusted start identities differ from the admitted execution")
	}
	roots := 0
	for _, l := range v.proof.Launches {
		if l.Purpose == "workflow-root" && l.ContextIndex == 0 {
			roots++
			if l.StartCaptureIndex != start.RootCaptureIndex || v.captures[l.StartCaptureIndex].raw.Phase != "start-barrier" {
				return rejected("process-trusted-start-mismatch", "trusted start root capture is not the workflow root start barrier")
			}
		}
	}
	if roots != 1 {
		return blocked("process-role-ambiguous", "exactly one workflow root launch is required")
	}
	v.sweeper = -1
	for i, l := range v.proof.Launches {
		if l.Invocation == v.proof.FinalSweep.ObserverInvocation {
			if v.sweeper >= 0 {
				return rejected("process-sweep-invalid", "final sweep observer invocation joins more than one launch")
			}
			v.sweeper = i
		}
	}
	if v.sweeper < 0 || v.sweeper == v.trusted || v.proof.Launches[v.sweeper].Purpose != "observer" || v.proof.Launches[v.sweeper].ContextIndex != 0 {
		return rejected("process-sweep-invalid", "final sweep is not an independent admitted observer launch")
	}
	for _, index := range []int{v.trusted, v.sweeper} {
		v.births[v.captures[v.proof.Launches[index].ExecutedCaptureIndex].birth].infra = true
	}
	return nil
}

// mapObservedBirthsV2 derives chromium-switch-role births from their actual
// mapped parent, executable bytes and argv. Births are visited by physical
// birth key, so raw witness order cannot change the result.
func (v *verifierV2) mapObservedBirthsV2() error {
	var pending []*birthV2
	for _, birth := range v.order {
		if !birth.mapped {
			pending = append(pending, birth)
		}
	}
	for len(pending) > 0 {
		var next []*birthV2
		for _, birth := range pending {
			parent, err := v.observedParent(birth)
			if err != nil {
				return err
			}
			if !parent.mapped {
				next = append(next, birth)
				continue
			}
			if err := v.deriveSwitchRole(birth, parent); err != nil {
				return err
			}
		}
		if len(next) == len(pending) {
			return blocked("process-parent-unverified", "observed births have no mapped owner")
		}
		pending = next
	}
	nodes := map[string]bool{}
	for _, birth := range v.order {
		if birth.infra {
			continue
		}
		birth.node = "proc/" + strconv.Itoa(birth.context) + "/" + birth.role + "/" + birth.slot
		if nodes[birth.node] {
			return blocked("process-role-ambiguous", "more than one birth derives "+birth.node)
		}
		nodes[birth.node] = true
	}
	for _, birth := range v.order {
		seen := map[*birthV2]bool{}
		for at := birth; at != nil; at = at.parent {
			if seen[at] {
				return blocked("process-parent-unverified", "parent edges form a cycle")
			}
			seen[at] = true
		}
	}
	return nil
}

func (v *verifierV2) observedParent(birth *birthV2) (*birthV2, error) {
	var found []*birthV2
	for _, candidate := range v.order {
		if candidate != birth && candidate.key.pid == birth.ppid && candidate.key.start <= birth.key.start {
			found = append(found, candidate)
		}
	}
	if len(found) != 1 {
		return nil, blocked("process-parent-unverified", "observed birth "+strconv.Itoa(birth.key.pid)+" has no unique captured parent")
	}
	return found[0], nil
}

func (v *verifierV2) deriveSwitchRole(birth, parent *birthV2) error {
	var image *captureV2
	for _, index := range birth.captures {
		c := &v.captures[index]
		if !c.live {
			continue
		}
		if image != nil && (!sameContent(image.raw.Executable, c.raw.Executable) || !bytes.Equal(image.raw.Cmdline, c.raw.Cmdline)) {
			return blocked("process-role-unsupported", "observed birth changed image between captures")
		}
		image = c
	}
	if image == nil || parent.infra {
		return blocked("process-role-unsupported", "observed birth has no live image or supported owner")
	}
	argv := strings.Split(strings.TrimSuffix(string(image.raw.Cmdline), "\x00"), "\x00")
	rule, err := v.matchRule(func(r RoleRuleV2) bool {
		return r.Derivation == "chromium-switch-role/0" && r.ParentRole == parent.role && sameContent(r.Executable, image.raw.Executable) &&
			allSwitches(argv, r.RequiredSwitches, true) && allSwitches(argv, r.ForbiddenSwitches, false)
	})
	if err != nil {
		return err
	}
	birth.mapped, birth.role, birth.slot, birth.context, birth.parent = true, rule.Role, rule.Slot, parent.context, parent
	return nil
}

// allSwitches reports whether every switch token is present (or absent): a
// token matches an argv element exactly or, without `=`, as its `switch=` prefix.
func allSwitches(argv, switches []string, present bool) bool {
	for _, token := range switches {
		found := false
		for _, arg := range argv[min(1, len(argv)):] {
			if arg == token || (!strings.Contains(token, "=") && strings.HasPrefix(arg, token+"=")) {
				found = true
				break
			}
		}
		if found != present {
			return false
		}
	}
	return true
}

// checkNativeReferencesV2 requires exactly one reference for every finite
// native process locator and joins it to an exact captured birth.
func (v *verifierV2) checkNativeReferencesV2() error {
	used := map[string]bool{}
	servers := map[string]birthKey{}
	for i, ref := range v.proof.NativeReferences {
		where := "native reference " + strconv.Itoa(i)
		key := locatorKey(ref.ArtifactID, ref.Pointer)
		locator, ok := v.native.locators[key]
		if !ok || used[key] {
			return rejected("process-native-reference-invalid", where+" is not a unique recognized native locator")
		}
		used[key] = true
		c, ok := v.capture(ref.CaptureIndex)
		if !ok || locator.kind != ref.Kind || locator.context != ref.ContextIndex {
			return rejected("process-native-reference-invalid", where+" kind, context or capture is invalid")
		}
		birth := v.births[c.birth]
		if birth.infra || birth.context != ref.ContextIndex {
			return rejected("process-native-reference-invalid", where+" capture belongs to another context")
		}
		if c.startTrim == "" {
			return blocked("process-native-start-unavailable", where+" capture has no native start output")
		}
		switch locator.kind {
		case "fresh-process":
			pid, start, ok := freshProcessFragment(locator.node)
			if !ok || pid != int64(c.stat.pid) || start != c.startTrim || birth.role != "server" {
				return rejected("process-native-reference-invalid", where+" does not join the server birth")
			}
		case "app-instance":
			kind, id, generation, ok := appInstanceFragment(locator.node)
			if !ok || kind != "process" || id != strconv.Itoa(c.stat.pid) || generation != c.startTrim || birth.role != "server" {
				return rejected("process-native-reference-invalid", where+" does not join the server birth")
			}
		case "descendant":
			pid, ppid, start, state, ok := descendantFragment(locator.node)
			// procfs state is one byte; ps(1) may append modifiers such as
			// "s" or "+", so only the first character is the kernel state.
			if !ok || pid != int64(c.stat.pid) || ppid != int64(c.stat.ppid) || start != c.startFields || state[0] != c.stat.state {
				return rejected("process-native-reference-invalid", where+" does not join the observed descendant")
			}
		}
		if locator.server != "" {
			if previous, ok := servers[locator.server]; ok && previous != c.birth {
				return rejected("process-native-reference-invalid", where+" joins a different server birth within one provider")
			}
			servers[locator.server] = c.birth
		}
	}
	if len(used) != len(v.native.locators) {
		return blocked("process-native-reference-missing", "a native process locator has no reference")
	}
	return nil
}

func equalFacts(a, b CleanupFactsV2) bool {
	return a.OwnedGroup == b.OwnedGroup && a.Status == b.Status && a.DerivedState == b.DerivedState &&
		a.DescendantsPresent == b.DescendantsPresent && a.Absent == b.Absent && a.Qualification == b.Qualification &&
		a.Cancelled == b.Cancelled && a.TimedOut == b.TimedOut && a.Scope == b.Scope && a.IntervalMS == b.IntervalMS &&
		slices.Equal(a.Failures, b.Failures) && slices.Equal(a.Limitations, b.Limitations)
}

// checkCleanupV2 joins every outer native cleanup object exactly and refuses
// unknown or surviving cleanup, including provider descendant observations.
func (v *verifierV2) checkCleanupV2() error {
	joined := map[string]bool{}
	for i, join := range v.proof.Cleanup {
		target, ok := v.native.cleanups[join.Pointer]
		if !ok || join.ArtifactID != v.graph.Report.ID || joined[join.Pointer] || !slices.Contains(target.context, join.ContextIndex) {
			return rejected("process-cleanup-join-invalid", "cleanup join "+strconv.Itoa(i)+" is not a unique native cleanup object")
		}
		joined[join.Pointer] = true
		if !equalFacts(join.Facts, target.facts) {
			return rejected("process-cleanup-join-invalid", "cleanup join "+strconv.Itoa(i)+" facts differ from the native report")
		}
	}
	if len(joined) != len(v.native.cleanups) {
		return blocked("process-cleanup-unknown", "an outer native cleanup object has no join")
	}
	facts := make([]CleanupFactsV2, 0, len(v.native.cleanups)+len(v.native.providerCleanup))
	for _, join := range v.proof.Cleanup {
		facts = append(facts, join.Facts)
	}
	facts = append(facts, v.native.providerCleanup...)
	for _, f := range facts {
		switch f.DerivedState {
		case "survivors":
			return rejected("process-cleanup-survivors", "native cleanup observed surviving descendants")
		case "unknown":
			return blocked("process-cleanup-unknown", "native cleanup is unknown")
		}
	}
	return nil
}

// checkFinalSweepV2 recomputes absence of every tracked workload birth from
// the independent observer's raw /proc inventory.
func (v *verifierV2) checkFinalSweepV2() error {
	sweep := &v.proof.FinalSweep
	l := &v.proof.Launches[v.sweeper]
	observer := v.births[v.captures[l.ExecutedCaptureIndex].birth]
	if !l.Completed || l.Cancelled || l.TimedOut {
		return blocked("process-sweep-incomplete", "final sweep observer was not waited to completion")
	}
	if facts, _ := exitFactsV2(v.captures[*l.RetirementCaptureIndex].raw); facts != "exit:0" {
		return blocked("process-sweep-incomplete", "final sweep observer did not exit successfully")
	}
	for _, index := range observer.captures {
		if phase := v.captures[index].raw.Phase; phase != "final-sweep" && phase != "retirement" {
			return rejected("process-sweep-invalid", "final sweep observer captured outside the final sweep phase")
		}
	}
	for i, c := range v.captures {
		birth := v.births[c.birth]
		if c.raw.Phase == "final-sweep" && birth != observer {
			return rejected("process-sweep-invalid", "final-sweep phase capture "+strconv.Itoa(i)+" is not the sweep observer")
		}
		if !birth.infra && birth.role != "host-supervisor" && i >= l.StartCaptureIndex {
			return rejected("process-sweep-invalid", "workload capture "+strconv.Itoa(i)+" follows the final sweep observer start")
		}
	}
	data, err := v.readDocument(sweep.ObserverStdout)
	if err != nil {
		return err
	}
	var document AbsenceSweepDocumentV2
	if err := decodeWire(data, "AbsenceSweepDocumentV2", &document); err != nil {
		return err
	}
	if document.ExecutionID != v.graph.ExecutionID || !bytes.Equal(document.BootIDBytes, sweep.BootIDBytes) ||
		!bytes.Equal(document.NamespaceLinkBytes, sweep.NamespaceLinkBytes) || !bytes.Equal(document.PIDDirectoryBytes, sweep.PIDDirectoryBytes) ||
		!slices.EqualFunc(document.Processes, sweep.Processes, func(a, b SweepProcessV2) bool { return a.PID == b.PID && bytes.Equal(a.StatBytes, b.StatBytes) }) ||
		!slices.Equal(document.ReadFailures, sweep.ReadFailures) {
		return rejected("process-sweep-invalid", "final sweep differs from the observer's retained output")
	}
	first := v.captures[0].raw
	if !bytes.Equal(sweep.BootIDBytes, first.BootIDBytes) || !bytes.Equal(sweep.NamespaceLinkBytes, first.NamespaceLinkBytes) {
		return rejected("process-namespace-mismatch", "final sweep observed another boot or PID namespace")
	}
	if len(sweep.ReadFailures) != 0 {
		return blocked("process-sweep-incomplete", "final sweep has read failures")
	}
	lines := strings.Split(string(sweep.PIDDirectoryBytes), "\n")
	if len(sweep.PIDDirectoryBytes) == 0 || lines[len(lines)-1] != "" || len(lines)-1 != len(sweep.Processes) {
		return rejected("process-sweep-invalid", "PID directory inventory does not correspond to the swept rows")
	}
	tracked := map[int]bool{}
	for _, birth := range v.order {
		if !birth.infra {
			tracked[birth.key.pid] = true
		}
	}
	seenPID := map[int]bool{}
	observerSeen := false
	for i, row := range sweep.Processes {
		pid, ok := positiveDecimal([]byte(lines[i]))
		stat, err := parseProcStatV2(row.StatBytes)
		if !ok || err != nil || pid != row.PID || stat.pid != pid || seenPID[pid] {
			return rejected("process-sweep-invalid", "swept row "+strconv.Itoa(i)+" does not match its PID directory entry")
		}
		seenPID[pid] = true
		key := birthKey{boot: string(sweep.BootIDBytes), nsDev: first.NamespaceDevice, nsIno: first.NamespaceInode, pid: pid, start: stat.startTicks}
		if birth, ok := v.births[key]; ok {
			switch {
			case birth == observer:
				observerSeen = true
			case birth.role == "host-supervisor" || birth.infra:
			default:
				return rejected("process-sweep-survivor", birth.node+" is still present at the final sweep")
			}
			continue
		}
		if tracked[stat.ppid] {
			return blocked("process-sweep-scope-ambiguous", "an uncaptured process is a child of a tracked birth")
		}
	}
	if !observerSeen {
		return rejected("process-sweep-invalid", "final sweep does not contain its own observer birth")
	}
	return nil
}

func (v *verifierV2) logicalV2() rawProcessResultV2 {
	result := rawProcessResultV2{graphSHA: v.graphSHA, logical: []LogicalProcessV2{}, workload: []birthKey{},
		references: len(v.proof.NativeReferences), cleanups: len(v.proof.Cleanup)}
	byRole := map[string][]string{}
	for _, birth := range v.order {
		if birth.node != "" {
			byRole[birth.role] = append(byRole[birth.role], birth.node)
		}
	}
	for _, birth := range v.order {
		if birth.node == "" {
			continue
		}
		context := v.graph.Contexts[birth.context]
		p := LogicalProcessV2{Node: birth.node, Role: birth.role, RunKind: context.RunKind, RunOrdinal: context.RunOrdinal, BirthDistinctFrom: []string{}}
		if birth.parent != nil && birth.parent.node != "" {
			parent := birth.parent.node
			p.Parent = &parent
		}
		for _, other := range byRole[birth.role] {
			if other != birth.node {
				p.BirthDistinctFrom = append(p.BirthDistinctFrom, other)
			}
		}
		slices.Sort(p.BirthDistinctFrom)
		p.State = v.logicalState(birth)
		if birth.role != "host-supervisor" {
			result.workload = append(result.workload, birth.key)
		}
		result.logical = append(result.logical, p)
	}
	slices.SortFunc(result.logical, func(a, b LogicalProcessV2) int { return strings.Compare(a.Node, b.Node) })
	return result
}

func (v *verifierV2) logicalState(birth *birthV2) string {
	if birth.role == "host-supervisor" {
		// Nothing observes the supervisor's liveness, so none is claimed.
		return "outside-workload/" + exitUnknown
	}
	if birth.launch < 0 {
		return "observed/" + exitUnknown
	}
	l := v.proof.Launches[birth.launch]
	var outcome []string
	if l.Completed {
		outcome = append(outcome, "completed")
	}
	if l.Cancelled {
		outcome = append(outcome, "cancelled")
	}
	if l.TimedOut {
		outcome = append(outcome, "timed-out")
	}
	if len(outcome) == 0 {
		outcome = append(outcome, "incomplete")
	}
	exit := "unreaped"
	if l.RetirementCaptureIndex != nil {
		exit, _ = exitFactsV2(v.captures[*l.RetirementCaptureIndex].raw)
	}
	return strings.Join(outcome, "+") + "/" + exit
}
