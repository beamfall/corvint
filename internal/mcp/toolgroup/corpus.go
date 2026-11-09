package toolgroup

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/doccorpus"
	"github.com/Beamfall/corvint/internal/mcp/bridge"
	"github.com/Beamfall/corvint/internal/mcp/corpusbridge"
	"github.com/Beamfall/corvint/internal/mcp/protocol"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

const (
	// CallDeadline bounds one corpus call so a slow request returns a typed
	// timeout while the server keeps serving others (DCP-V1-145).
	CallDeadline = 50 * time.Second
	// TimingMeta is the result _meta key carrying the call's cost (DCP-V1-145).
	TimingMeta = "corvint/timing"
	// SearchTool answers with the compact receipt by default (DCP-V1-653).
	SearchTool = "corvint.docs_search"
	// corpusRefused is the code every corpus call answers while the configured
	// artifact cannot be opened (MMR-V0-014).
	corpusRefused = "corpus-refused"
)

// Corpus answers the artifact-declared corvint.docs_* tools of one opened
// corpus artifact. A zero Deadline means CallDeadline.
type Corpus struct {
	Registry *corpusbridge.Registry
	Deadline time.Duration
}

func (group Corpus) Tools() []bridge.ToolDescriptor { return group.Registry.Tools() }

func (group Corpus) Call(ctx context.Context, name string, arguments map[string]any) (map[string]any, *protocol.RPCError) {
	raw, err := json.Marshal(arguments)
	if err != nil {
		return nil, protocol.InvalidParams("Invalid params")
	}
	deadline := group.Deadline
	if deadline <= 0 {
		deadline = CallDeadline
	}
	callCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()
	structured, text, toolFailure, transportErr, timing := group.Registry.CallTimed(callCtx, name, raw)
	if transportErr != nil {
		if transportErr.Code == "invalid-arguments" || transportErr.Code == "unsupported-tool" {
			return nil, protocol.InvalidParams("Invalid params")
		}
		if ctx.Err() != nil {
			return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
		}
	}
	if (transportErr != nil || toolFailure != nil) && ctx.Err() == nil && callCtx.Err() != nil {
		return timedFailure(name, "deadline-exceeded", "call did not finish within "+deadline.String()+"; retry with fewer paths", timing)
	}
	if transportErr != nil {
		if transportErr.Code == "cancelled" {
			return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
		}
		return timedFailure(name, transportErr.Code, transportErr.Code, timing)
	}
	if toolFailure != nil {
		return timedFailure(name, toolFailure.Code, toolFailure.Message, timing)
	}
	// DCP-V1-653..655: docs_search answers with the compact receipt unless
	// the caller asks for the full one.
	if name == SearchTool && arguments["receipt"] != corpusbridge.FullView {
		compact, err := corpusbridge.CompactReceipt(structured, includes(arguments, "capabilities"))
		if err != nil {
			return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
		}
		encoded, err := json.Marshal(compact)
		if err != nil {
			return nil, protocol.NewError(protocol.CodeInternalError, "Internal error")
		}
		structured, text = compact, string(encoded)
	}
	// Model-facing evidence uses the shared untrusted-data envelope over the
	// same receipt as the structured content.
	framed, frameErr := repoenvelope.Frame(text)
	if frameErr != nil {
		return CorpusFailure(name, repoenvelope.CollisionCode, repoenvelope.CollisionCode)
	}
	return map[string]any{
		"content":           []any{map[string]any{"type": "text", "text": framed}},
		"isError":           false,
		"structuredContent": structured,
		"_meta":             map[string]any{TimingMeta: timing},
	}, nil
}

// includes reports whether a validated include argument names value.
func includes(arguments map[string]any, value string) bool {
	list, _ := arguments["include"].([]any)
	for _, x := range list {
		if x == value {
			return true
		}
	}
	return false
}

// timedFailure reports the call's cost in result _meta, outside the evidence
// receipt, so receipts stay byte-identical to the CLI (DCP-V1-145).
func timedFailure(name, code, message string, timing corpusbridge.Timing) (map[string]any, *protocol.RPCError) {
	result, rpcErr := CorpusFailure(name, code, message)
	if result != nil {
		result["_meta"] = map[string]any{TimingMeta: timing}
	}
	return result, rpcErr
}

// CorpusFailure is the corpus tool-error result: the closed error object plus
// an all-unknown trust envelope. It always sets isError: true; a business or
// transport refusal from the native docs compiler is never reported as success.
func CorpusFailure(name, code, message string) (map[string]any, *protocol.RPCError) {
	return failureResult(map[string]any{
		"trust_envelope": map[string]any{"content_status": "unknown", "source_revision": "unknown", "corpus_revision": "unknown", "freshness": "unknown", "retirement": "unknown", "verified_at": nil, "citations": []any{}, "limitations": []string{"operation failed; no evidence answer", doccorpus.RefusalUnverifiedLimitation}},
		"abstention":     map[string]any{"active": true, "reason": "OPERATION_FAILED"},
		"code":           code, "error": message, "mutates": false, "profile": CorpusErrorProfile, "tool": name,
	})
}

// CorpusArtifact is the corpus group of the unified server (MMR-V0-014). The
// artifact opened at startup lists exactly its declared tools, as the dedicated
// server does. One that could not be opened lists every corpus tool and is
// opened again at each call: until it opens, and for a tool the opened artifact
// does not declare, the call is the corpus-refused tool error, never a startup
// failure, so the other groups keep serving. A stale artifact is refused by
// the registry itself, as corpus-refused. The root's identity is pinned at
// startup: a later open, or a call through an opened registry, under a root
// replaced since is corpus-refused, and so is a call whose artifact has been
// removed after it opened (MMR-V0-017).
type CorpusArtifact struct {
	root, path string
	rootID     os.FileInfo
	atStartup  bool
	mu         sync.Mutex
	registry   *corpusbridge.Registry
}

// OpenCorpusArtifact binds the artifact path, already checked by
// RootRelativePath and the local input path rule, pins the root's identity,
// and reports the open failure code when the artifact cannot be opened now.
func OpenCorpusArtifact(root, path string) (*CorpusArtifact, string) {
	artifact := &CorpusArtifact{root: root, path: path}
	if absolute, err := filepath.Abs(root); err == nil {
		artifact.root = absolute
		if identity, err := os.Lstat(absolute); err == nil && identity.IsDir() {
			artifact.rootID = identity
		}
	}
	registry, code := artifact.open()
	if registry == nil {
		return artifact, code
	}
	artifact.atStartup = true
	return artifact, ""
}

func (artifact *CorpusArtifact) Tools() []bridge.ToolDescriptor {
	if artifact.atStartup {
		return artifact.registry.Tools()
	}
	return corpusbridge.AllTools()
}

func (artifact *CorpusArtifact) Call(ctx context.Context, name string, arguments map[string]any) (map[string]any, *protocol.RPCError) {
	if !artifact.atStartup && !listed(corpusbridge.AllTools(), name) {
		return nil, protocol.InvalidParams("Invalid params")
	}
	registry, code := artifact.open()
	if registry == nil {
		return refusedFor(name, code)
	}
	if !artifact.atStartup && !listed(registry.Tools(), name) {
		return CorpusFailure(name, corpusRefused, "the configured corpus artifact does not declare this tool's capability")
	}
	result, rpcErr := Corpus{Registry: registry}.Call(ctx, name, arguments)
	// The registry reports a replaced root or a removed artifact as an
	// unavailable input; the unified server refuses both (MMR-V0-017).
	if failureCode(result) == "corpus-input-unavailable" {
		if code := artifact.unavailable(); code != "" {
			refused, refusedErr := refusedFor(name, code)
			if refused != nil && result["_meta"] != nil {
				refused["_meta"] = result["_meta"]
			}
			return refused, refusedErr
		}
	}
	return result, rpcErr
}

func refusedFor(name, code string) (map[string]any, *protocol.RPCError) {
	return CorpusFailure(name, corpusRefused, "the configured corpus artifact is unavailable ("+code+"); build it and call again")
}

// unavailable names why the pinned root or its artifact cannot serve now, or
// returns "".
func (artifact *CorpusArtifact) unavailable() string {
	if code := artifact.unavailableRoot(); code != "" {
		return code
	}
	if _, err := doccorpus.StatCorpusFile(artifact.root, artifact.path); err != nil {
		return "corpus-unavailable"
	}
	return ""
}

// open returns the opened registry, opening it under the pinned root when
// it has not opened yet. The root is checked before and after the open, so a
// registry is never bound to a root that replaced the one pinned at startup.
func (artifact *CorpusArtifact) open() (*corpusbridge.Registry, string) {
	artifact.mu.Lock()
	defer artifact.mu.Unlock()
	if artifact.registry == nil {
		if code := artifact.unavailableRoot(); code != "" {
			return nil, code
		}
		registry, err := corpusbridge.New(artifact.root, artifact.path)
		if err != nil {
			return nil, err.Code
		}
		if code := artifact.unavailableRoot(); code != "" {
			return nil, code
		}
		artifact.registry = registry
	}
	return artifact.registry, ""
}

// unavailableRoot is "root-changed" unless the root is still the directory
// pinned at startup.
func (artifact *CorpusArtifact) unavailableRoot() string {
	identity, err := os.Lstat(artifact.root)
	if artifact.rootID == nil || err != nil || !os.SameFile(identity, artifact.rootID) {
		return "root-changed"
	}
	return ""
}

// failureCode reads a tool-error result's code, or "".
func failureCode(result map[string]any) string {
	if result == nil || result["isError"] != true {
		return ""
	}
	structured, _ := result["structuredContent"].(map[string]any)
	code, _ := structured["code"].(string)
	return code
}

func listed(tools []bridge.ToolDescriptor, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

// RootRelativePath rewrites an absolute --artifact or --map inside root to the equivalent
// root-relative path, so both spellings open the same file and report the same path
// (DCP-V1-205). Containment is decided lexically first and then on symlink-resolved paths,
// so a root reached through a symlinked ancestor (macOS /var -> /private/var) matches either
// spelling (DCP-V1-206). An absolute path that is not below root is refused with a reason
// instead of the generic repository-unavailable refusal (DCP-V1-207). A relative value is
// returned unchanged; the bounded reader still applies its path rules.
func RootRelativePath(root, artifact string) (string, string) {
	if !filepath.IsAbs(artifact) {
		return artifact, ""
	}
	lexicalRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "--root cannot be made absolute"
	}
	candidate := filepath.Clean(artifact)
	if relative, inside := containedPath(lexicalRoot, candidate); inside {
		return relative, ""
	}
	realRoot, rootErr := filepath.EvalSymlinks(lexicalRoot)
	realParent, parentErr := filepath.EvalSymlinks(filepath.Dir(candidate))
	if rootErr == nil && parentErr == nil {
		if relative, inside := containedPath(realRoot, filepath.Join(realParent, filepath.Base(candidate))); inside {
			return relative, ""
		}
	}
	return "", "absolute path is not below --root"
}

// containedPath reports the slash-separated path of target relative to root when target is
// strictly below root.
func containedPath(root, target string) (string, bool) {
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", false
	}
	return filepath.ToSlash(relative), true
}
