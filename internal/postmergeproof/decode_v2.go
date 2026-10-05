// SPDX-License-Identifier: AGPL-3.0-or-later

package postmergeproof

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Outcome classes follow PMR-V2-009. NOT_OBSERVED is only reported by
// collection on an unsupported host; the offline verifier never returns it.
const (
	OutcomeBlockedV2     = "BLOCKED"
	OutcomeRejectedV2    = "REJECTED"
	OutcomeNotObservedV2 = "NOT_OBSERVED"
)

// ProcessErrorV2 is a bounded refusal. Code names the first failed check;
// Detail is diagnostic only and never an acceptance input.
type ProcessErrorV2 struct {
	Outcome string
	Code    string
	Detail  string
}

func (e *ProcessErrorV2) Error() string {
	if e.Detail == "" {
		return "postmerge process proof " + e.Outcome + ": " + e.Code
	}
	return "postmerge process proof " + e.Outcome + ": " + e.Code + ": " + e.Detail
}

func blocked(code, detail string) error {
	return &ProcessErrorV2{Outcome: OutcomeBlockedV2, Code: code, Detail: detail}
}

func rejected(code, detail string) error {
	return &ProcessErrorV2{Outcome: OutcomeRejectedV2, Code: code, Detail: detail}
}

const (
	maxDocumentBytes   = 32 << 20
	maxExecutableBytes = 1 << 30
	maxJSONDepth       = 64
	maxStringRunes     = 4096
	maxBase64Length    = 22369624
	maxInt64           = int64(^uint64(0) >> 1)
)

var shaPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func domainSHA256(domain string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(domain))
	h.Write([]byte{0})
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// jsonNode is a parsed JSON value that keeps numbers as their exact source
// token and refuses duplicate keys, so no float conversion or alias occurs.
type jsonNode struct {
	kind byte // 'n' null, 'b' bool, 's' string, 'd' number, 'o' object, 'a' array
	b    bool
	str  string
	num  string
	obj  map[string]*jsonNode
	keys []string
	arr  []*jsonNode
}

// parseJSONTree parses exactly one UTF-8 JSON value. strictStrings also refuses
// the replacement character, which encoding/json substitutes for invalid escapes.
func parseJSONTree(data []byte, strictStrings bool) (*jsonNode, error) {
	if !utf8.Valid(data) || bytes.HasPrefix(data, []byte("\xef\xbb\xbf")) {
		return nil, errors.New("invalid UTF-8 or byte order mark")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	node, err := parseJSONValue(dec, 0, strictStrings)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("trailing JSON value")
	}
	return node, nil
}

func parseJSONValue(dec *json.Decoder, depth int, strict bool) (*jsonNode, error) {
	if depth > maxJSONDepth {
		return nil, errors.New("JSON nesting exceeds bound")
	}
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("JSON syntax: %w", err)
	}
	switch v := tok.(type) {
	case nil:
		return &jsonNode{kind: 'n'}, nil
	case bool:
		return &jsonNode{kind: 'b', b: v}, nil
	case string:
		if strict && strings.ContainsRune(v, utf8.RuneError) {
			return nil, errors.New("invalid Unicode in string")
		}
		return &jsonNode{kind: 's', str: v}, nil
	case json.Number:
		return &jsonNode{kind: 'd', num: v.String()}, nil
	case json.Delim:
		switch v {
		case '{':
			node := &jsonNode{kind: 'o', obj: map[string]*jsonNode{}}
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return nil, fmt.Errorf("JSON syntax: %w", err)
				}
				key, ok := keyTok.(string)
				if !ok {
					return nil, errors.New("JSON object key is not a string")
				}
				if strict && strings.ContainsRune(key, utf8.RuneError) {
					return nil, errors.New("invalid Unicode in key")
				}
				if _, dup := node.obj[key]; dup {
					return nil, fmt.Errorf("duplicate JSON key %q", key)
				}
				child, err := parseJSONValue(dec, depth+1, strict)
				if err != nil {
					return nil, err
				}
				node.obj[key] = child
				node.keys = append(node.keys, key)
			}
			if _, err := dec.Token(); err != nil {
				return nil, fmt.Errorf("JSON syntax: %w", err)
			}
			return node, nil
		case '[':
			node := &jsonNode{kind: 'a', arr: []*jsonNode{}}
			for dec.More() {
				child, err := parseJSONValue(dec, depth+1, strict)
				if err != nil {
					return nil, err
				}
				node.arr = append(node.arr, child)
			}
			if _, err := dec.Token(); err != nil {
				return nil, fmt.Errorf("JSON syntax: %w", err)
			}
			return node, nil
		}
	}
	return nil, errors.New("unexpected JSON token")
}

var canonicalInteger = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)

func (n *jsonNode) integer() (int64, bool) {
	if n == nil || n.kind != 'd' || !canonicalInteger.MatchString(n.num) || n.num == "-0" {
		return 0, false
	}
	v, err := strconv.ParseInt(n.num, 10, 64)
	return v, err == nil
}

// canonicalJSON encodes keys in byte order, preserves array order, admits only
// canonical integers and uses the tasks/wire string escaping, with no trailing LF.
func canonicalJSON(n *jsonNode) ([]byte, error) {
	var b []byte
	var walk func(*jsonNode) error
	walk = func(n *jsonNode) error {
		switch n.kind {
		case 'n':
			b = append(b, "null"...)
		case 'b':
			b = strconv.AppendBool(b, n.b)
		case 's':
			b = appendCanonicalString(b, n.str)
		case 'd':
			if _, ok := n.integer(); !ok {
				return errors.New("non-integer number in canonical JSON")
			}
			b = append(b, n.num...)
		case 'a':
			b = append(b, '[')
			for i, child := range n.arr {
				if i > 0 {
					b = append(b, ',')
				}
				if err := walk(child); err != nil {
					return err
				}
			}
			b = append(b, ']')
		case 'o':
			keys := slices.Clone(n.keys)
			slices.Sort(keys)
			b = append(b, '{')
			for i, key := range keys {
				if i > 0 {
					b = append(b, ',')
				}
				b = appendCanonicalString(b, key)
				b = append(b, ':')
				if err := walk(n.obj[key]); err != nil {
					return err
				}
			}
			b = append(b, '}')
		default:
			return errors.New("invalid JSON node")
		}
		return nil
	}
	if err := walk(n); err != nil {
		return nil, err
	}
	return b, nil
}

func appendCanonicalString(b []byte, s string) []byte {
	const hexdigits = "0123456789abcdef"
	b = append(b, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			b = append(b, '\\', '"')
		case c == '\\':
			b = append(b, '\\', '\\')
		case c == '\t':
			b = append(b, '\\', 't')
		case c == '\n':
			b = append(b, '\\', 'n')
		case c == '\r':
			b = append(b, '\\', 'r')
		case c < 0x20:
			b = append(b, '\\', 'u', '0', '0', hexdigits[c>>4], hexdigits[c&0xF])
		default:
			b = append(b, c)
		}
	}
	return append(b, '"')
}

// Field kinds of the closed process schemas.
const (
	kString = iota
	kSHA
	kBool
	kInt
	kBytes
	kObject
	kStringArray
	kObjectArray
)

type fieldSpec struct {
	name      string
	kind      int
	ref       string // object or object-array element type
	nullable  bool
	enum      []string
	constant  string
	constBool *bool
	constInt  *int64
	min, max  int64
	minItems  int
	maxItems  int
}

var (
	processRoles = []string{"host-supervisor", "workflow-root", "assessment-worker", "runner", "server", "observer", "attestor", "control-hook", "cleanup-hook", "playwright-worker", "browser-main", "browser-zygote", "browser-renderer", "browser-gpu", "browser-utility"}
	parentRoles  = append(slices.Clone(processRoles), "outside")
	trueValue    = true
	interval20   = int64(20)
)

func str(name string) fieldSpec { return fieldSpec{name: name, kind: kString} }
func sha(name string) fieldSpec { return fieldSpec{name: name, kind: kSHA} }
func boolean(name string) fieldSpec {
	return fieldSpec{name: name, kind: kBool}
}
func nonNegative(name string) fieldSpec {
	return fieldSpec{name: name, kind: kInt, min: 0, max: maxInt64}
}
func nullableIndex(name string) fieldSpec {
	return fieldSpec{name: name, kind: kInt, min: 0, max: maxInt64, nullable: true}
}
func raw(name string) fieldSpec { return fieldSpec{name: name, kind: kBytes} }
func object(name, ref string) fieldSpec {
	return fieldSpec{name: name, kind: kObject, ref: ref}
}
func objects(name, ref string, maxItems int) fieldSpec {
	return fieldSpec{name: name, kind: kObjectArray, ref: ref, maxItems: maxItems}
}
func strings4096(name string, maxItems int) fieldSpec {
	return fieldSpec{name: name, kind: kStringArray, maxItems: maxItems}
}
func enum(name string, values ...string) fieldSpec {
	return fieldSpec{name: name, kind: kString, enum: values}
}
func constant(name, value string) fieldSpec {
	return fieldSpec{name: name, kind: kString, constant: value}
}

// wireTypesV2 transcribes the closed $defs of the frozen process schemas.
// TestWireTableMatchesFrozenSchemas checks it against those files.
var wireTypesV2 = map[string][]fieldSpec{
	"ArtifactRefV2": {str("id"), sha("sha256"), nonNegative("bytes")},
	"ImplementationV2": {str("source_commit"), str("source_tree"), sha("verifier_sha256"),
		sha("observer_sha256"), sha("supervisor_sha256")},
	"HostTupleV2": {constant("os", "linux"), enum("architecture", "amd64", "arm64"), str("kernel_release"),
		sha("image_manifest_sha256"), sha("launcher_policy_sha256")},
	"RoleRuleV2": {enum("role", processRoles...), enum("derivation", "owned-launch/0", "playwright-node-worker/0", "chromium-switch-role/0"),
		object("executable", "ArtifactRefV2"), enum("launch_purpose", processRoles...), enum("parent_role", parentRoles...),
		enum("slot", "main", "default", "sandboxed", "unsandboxed", "network", "storage"),
		strings4096("required_switches", 32), strings4096("forbidden_switches", 32)},
	"ObservationPolicyV2": {constant("profile", "linux-procfs-birth-command/0"), constant("scope", "observed-owned-process-tree"),
		{name: "target_interval_ms", kind: kInt, constInt: &interval20, min: 20, max: 20},
		{name: "capture_limit", kind: kInt, min: 1, max: 16384}, {name: "process_limit", kind: kInt, min: 1, max: 4096},
		{name: "require_paired_stat", kind: kBool, constBool: &trueValue},
		{name: "require_final_independent_sweep", kind: kBool, constBool: &trueValue}, strings4096("limitations", 4096)},
	"ProcessPolicyV2": {constant("profile", "postmerge-process-policy/2"), object("implementation", "ImplementationV2"),
		object("host", "HostTupleV2"), objects("rules", "RoleRuleV2", 32), object("observation", "ObservationPolicyV2")},
	"SourceBindingV2": {enum("kind", "request", "report", "provider", "hook", "control", "attestation-output", "producer-job"),
		object("artifact", "ArtifactRefV2")},
	"RunContextV2": {nullableIndex("run_index"), enum("run_kind", "workflow", "repeat", "probe-original", "probe-reversed", "probe-isolated", "control"),
		nonNegative("run_ordinal"), strings4096("test_ids", 4096), nullableIndex("control_index"),
		nullableIndex("control_ordinal"), nullableIndex("attempt_ordinal")},
	"GraphBindingV2": {constant("profile", "postmerge-native-graph-binding/2"), str("execution_id"), object("request", "ArtifactRefV2"),
		object("report", "ArtifactRefV2"), str("product_commit"), str("test_commit"), str("draft_commit"),
		objects("sources", "SourceBindingV2", 4096), objects("contexts", "RunContextV2", 4096)},
	"ProcCaptureV2": {nonNegative("index"), enum("phase", "start-barrier", "during-run", "retirement", "final-sweep"),
		raw("boot_id_bytes"), raw("namespace_link_bytes"), nonNegative("namespace_device"), nonNegative("namespace_inode"),
		raw("stat_before"), raw("cmdline"), raw("cmdline_after"), raw("executable_link_bytes"), raw("executable_link_after_bytes"),
		object("executable", "ArtifactRefV2"), object("executable_after", "ArtifactRefV2"), raw("stat_after"),
		raw("native_start_output"), object("native_start_tool", "ArtifactRefV2"),
		{name: "exit_code", kind: kInt, min: -1, max: 255, nullable: true},
		{name: "exit_signal", kind: kInt, min: 1, max: 128, nullable: true}},
	"OwnedLaunchV2": {nonNegative("index"), nonNegative("context_index"), enum("purpose", processRoles...),
		object("invocation", "ArtifactRefV2"), object("stdin", "ArtifactRefV2"), nullableIndex("parent_capture_index"),
		nonNegative("start_capture_index"), nonNegative("executed_capture_index"), nullableIndex("retirement_capture_index"),
		boolean("completed"), boolean("cancelled"), boolean("timed_out")},
	"NativeProcessReferenceV2": {str("artifact_id"), str("pointer"), enum("kind", "fresh-process", "app-instance", "descendant"),
		nonNegative("context_index"), nonNegative("capture_index")},
	"SweepProcessV2": {nonNegative("pid"), raw("stat_bytes")},
	"AbsenceSweepV2": {object("observer_invocation", "ArtifactRefV2"), object("observer_stdout", "ArtifactRefV2"),
		raw("boot_id_bytes"), raw("namespace_link_bytes"), raw("pid_directory_bytes"),
		objects("processes", "SweepProcessV2", 4096), strings4096("read_failures", 4096)},
	"CleanupFactsV2": {boolean("owned_group"), enum("status", "owned-process-group", "descendant-cleanup-unsupported", "ancestor-process-group"),
		enum("derived_state", "observed-absent", "unknown", "survivors"), boolean("descendants_present"), boolean("absent"),
		str("qualification"), boolean("cancelled"), boolean("timed_out"), str("scope"), nonNegative("interval_ms"),
		strings4096("failures", 4096), strings4096("limitations", 4096)},
	"CleanupJoinV2": {str("artifact_id"), str("pointer"), nonNegative("context_index"), object("facts", "CleanupFactsV2")},
	"ProcessProofV2": {constant("profile", "postmerge-process-proof/2"), str("execution_id"), sha("policy_sha256"),
		sha("graph_binding_sha256"), object("trusted_start_invocation", "ArtifactRefV2"), object("trusted_start_stdout", "ArtifactRefV2"),
		objects("captures", "ProcCaptureV2", 16384), objects("launches", "OwnedLaunchV2", 4096),
		objects("native_references", "NativeProcessReferenceV2", 16384), object("final_sweep", "AbsenceSweepV2"),
		objects("cleanup", "CleanupJoinV2", 4096)},
	"LogicalProcessV2": {str("node"), enum("role", processRoles...), str("run_kind"), nonNegative("run_ordinal"),
		{name: "parent", kind: kString, nullable: true}, str("state"), strings4096("birth_distinct_from", 4096)},
	"InvocationV2": {constant("profile", "postmerge-owned-invocation/2"), str("execution_id"), enum("purpose", processRoles...),
		object("executable", "ArtifactRefV2"), strings4096("argv", 4096), objects("environment", "EnvironmentV2", 4096),
		sha("stdin_sha256"), nonNegative("context_index"), nonNegative("launch_index")},
	"EnvironmentV2": {str("key"), str("value")},
	"TrustedStartV2": {constant("profile", "postmerge-trusted-start/2"), str("execution_id"), sha("policy_sha256"),
		sha("request_sha256"), sha("supervisor_sha256"), sha("observer_sha256"), sha("host_tuple_sha256"),
		nonNegative("root_capture_index")},
	"AbsenceSweepDocumentV2": {constant("profile", "postmerge-absence-sweep/2"), str("execution_id"), raw("boot_id_bytes"),
		raw("namespace_link_bytes"), raw("pid_directory_bytes"), objects("processes", "SweepProcessV2", 4096),
		strings4096("read_failures", 4096)},
	"QualificationCaseV2": {enum("id", qualificationCaseIDs...),
		{name: "graph_bindings", kind: kObjectArray, ref: "ArtifactRefV2", minItems: 1, maxItems: 2},
		object("policy", "ArtifactRefV2"), {name: "proofs", kind: kObjectArray, ref: "ArtifactRefV2", minItems: 1, maxItems: 2},
		object("harness_invocation", "ArtifactRefV2"), object("harness_stdout", "ArtifactRefV2")},
	"ProcessQualificationV2": {constant("profile", "postmerge-process-qualification/2"), sha("policy_sha256"),
		object("implementation", "ImplementationV2"), object("host", "HostTupleV2"),
		objects("cases", "QualificationCaseV2", 10), strings4096("limitations", 4096)},
}

func validateWire(n *jsonNode, typeName, path string) error {
	fields, ok := wireTypesV2[typeName]
	if !ok {
		return fmt.Errorf("%s: unknown wire type %s", path, typeName)
	}
	if n.kind != 'o' {
		return fmt.Errorf("%s: %s must be an object", path, typeName)
	}
	if len(n.obj) != len(fields) {
		return fmt.Errorf("%s: %s requires exactly %d members", path, typeName, len(fields))
	}
	for _, f := range fields {
		child, present := n.obj[f.name]
		if !present {
			return fmt.Errorf("%s: missing member %s", path, f.name)
		}
		if err := validateField(child, f, path+"/"+f.name); err != nil {
			return err
		}
	}
	return nil
}

func validateField(n *jsonNode, f fieldSpec, path string) error {
	if n.kind == 'n' {
		if f.nullable {
			return nil
		}
		return fmt.Errorf("%s: null is not admitted", path)
	}
	switch f.kind {
	case kString, kSHA:
		if n.kind != 's' || utf8.RuneCountInString(n.str) > maxStringRunes {
			return fmt.Errorf("%s: bounded string required", path)
		}
		if f.kind == kSHA && !shaPattern.MatchString(n.str) {
			return fmt.Errorf("%s: lowercase SHA-256 required", path)
		}
		if f.constant != "" && n.str != f.constant {
			return fmt.Errorf("%s: unsupported value", path)
		}
		if f.enum != nil && !slices.Contains(f.enum, n.str) {
			return fmt.Errorf("%s: unsupported value", path)
		}
	case kBool:
		if n.kind != 'b' || (f.constBool != nil && n.b != *f.constBool) {
			return fmt.Errorf("%s: boolean required", path)
		}
	case kInt:
		v, ok := n.integer()
		if !ok || v < f.min || v > f.max || (f.constInt != nil && v != *f.constInt) {
			return fmt.Errorf("%s: bounded integer required", path)
		}
	case kBytes:
		if n.kind != 's' || len(n.str) > maxBase64Length || strings.ContainsAny(n.str, "\r\n") {
			return fmt.Errorf("%s: base64 bytes required", path)
		}
		if _, err := base64.StdEncoding.Strict().DecodeString(n.str); err != nil {
			return fmt.Errorf("%s: canonical base64 required", path)
		}
	case kObject:
		return validateWire(n, f.ref, path)
	case kStringArray, kObjectArray:
		if n.kind != 'a' || len(n.arr) > f.maxItems || len(n.arr) < f.minItems {
			return fmt.Errorf("%s: bounded array required", path)
		}
		for i, child := range n.arr {
			elementPath := path + "/" + strconv.Itoa(i)
			if f.kind == kObjectArray {
				if err := validateWire(child, f.ref, elementPath); err != nil {
					return err
				}
			} else if child.kind != 's' || utf8.RuneCountInString(child.str) > maxStringRunes {
				return fmt.Errorf("%s: bounded string required", elementPath)
			}
		}
	}
	return nil
}

// decodeWire strictly validates the closed type before typed decoding; the
// typed decode cannot then meet unknown, duplicate, case-aliased or null members.
func decodeWire(data []byte, typeName string, out any) error {
	tree, err := parseJSONTree(data, true)
	if err != nil {
		return rejected("process-wire-invalid", typeName+": "+err.Error())
	}
	if err := validateWire(tree, typeName, ""); err != nil {
		return rejected("process-wire-invalid", typeName+err.Error())
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return rejected("process-wire-invalid", typeName+": "+err.Error())
	}
	return nil
}

// canonicalWireSHA256 hashes a Go value of a closed wire type under a domain:
// it is marshalled, validated against the closed table and canonicalized.
func canonicalWireSHA256(domain, typeName string, value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	tree, err := parseJSONTree(data, true)
	if err != nil {
		return "", err
	}
	if err := validateWire(tree, typeName, ""); err != nil {
		return "", errors.New(typeName + err.Error())
	}
	body, err := canonicalJSON(tree)
	if err != nil {
		return "", err
	}
	return domainSHA256(domain, body), nil
}

// GraphBindingSHA256V2 returns the PMR-V2 native graph binding identity:
// SHA-256 of `postmerge-native-graph-binding/2`, NUL and canonical JSON.
func GraphBindingSHA256V2(graph GraphBindingV2) (string, error) {
	return canonicalWireSHA256("postmerge-native-graph-binding/2", "GraphBindingV2", graph)
}

// HostTupleSHA256V2 returns the trusted-start host tuple identity:
// SHA-256 of `postmerge-host-tuple/2`, NUL and canonical JSON.
func HostTupleSHA256V2(host HostTupleV2) (string, error) {
	return canonicalWireSHA256("postmerge-host-tuple/2", "HostTupleV2", host)
}
