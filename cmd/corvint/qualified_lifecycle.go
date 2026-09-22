package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/authorityevent"
	"github.com/Beamfall/corvint/internal/authoritystore"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/localcompletion"
	"github.com/Beamfall/corvint/internal/repoenvelope"
)

const qualifiedLifecycleProfile = "corvint-qualified-lifecycle/0"
const directQualifiedLifecycleProfile = "corvint-qualified-lifecycle/1"
const qualifiedLifecycleDigest = "qualified-lifecycle:sha256:"

var errQualifiedLifecycle = errors.New("qualified-lifecycle-unavailable")

type qualifiedRequest struct {
	profile string
	event   string
	input   map[string]any
}

type qualifiedResolver func(context.Context, bool, authoritystore.LifecycleObserver) (authoritystore.LifecycleResolution, error)

func parseQualifiedRequest(raw []byte) (qualifiedRequest, error) {
	var request qualifiedRequest
	if len(raw) > gokernel.MaxInputBytes || !utf8.Valid(raw) {
		return request, errQualifiedLifecycle
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return request, errQualifiedLifecycle
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		token, err = decoder.Token()
		name, ok := token.(string)
		if err != nil || !ok || (name != "profile" && name != "event" && name != "input") || fields[name] != nil {
			return request, errQualifiedLifecycle
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return request, errQualifiedLifecycle
		}
		fields[name] = value
	}
	if _, err = decoder.Token(); err != nil {
		return request, errQualifiedLifecycle
	}
	if _, err = decoder.Token(); err != io.EOF || len(fields) != 3 {
		return request, errQualifiedLifecycle
	}
	var profile string
	if json.Unmarshal(fields["profile"], &profile) != nil || (profile != qualifiedLifecycleProfile && profile != directQualifiedLifecycleProfile) || json.Unmarshal(fields["event"], &request.event) != nil {
		return request, errQualifiedLifecycle
	}
	request.profile = profile
	request.input, err = dogfoodEventInput(request.event, fields["input"])
	if err != nil || (request.event == "stop" && request.input["stopHookActive"] == nil) {
		return qualifiedRequest{}, errQualifiedLifecycle
	}
	return request, nil
}

func runProtectedEvent(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if args[0] == "qualified-event" {
		return runQualifiedLifecycle(ctx, args[1:], stdin, stdout, stderr)
	}
	return runAuthorityEvent(ctx, args[1:], stdin, stdout, stderr)
}

func runQualifiedLifecycle(parent context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	native := len(args) == 3 && args[2] == "--native-output"
	fallback := func(reason string) int {
		if native {
			if emit(stdout, qualifiedFallback(reason)) == nil {
				return 0
			}
		} else {
			message := reason
			if reason == "corvint-invocation-timeout" {
				message = "corvint-invocation-timeout: the 1600 ms invocation deadline was exceeded; this is a time bound, not a diagnosed fault"
			}
			emitError(stderr, &gokernel.Error{Code: reason, Message: message})
		}
		return 2
	}
	if (len(args) != 2 && !native) || args[0] != "--input" || args[1] != "-" {
		return fallback("invalid-qualified-lifecycle-arguments")
	}
	ctx, cancel := context.WithTimeout(parent, 1600*time.Millisecond)
	defer cancel()
	raw, err := readInputBounded(ctx, stdin, gokernel.MaxInputBytes)
	if err != nil {
		return fallback(qualifiedFailureReason(ctx, "qualified-lifecycle-input-unavailable"))
	}
	request, err := parseQualifiedRequest(raw)
	if err != nil {
		return fallback("invalid-qualified-lifecycle-input")
	}
	resolver := authoritystore.ResolveLifecycle
	if request.profile == directQualifiedLifecycleProfile {
		resolver = authoritystore.ResolveDirectLifecycle
	}
	result, err := qualifiedLifecycle(ctx, request, resolver)
	if err != nil {
		return fallback(qualifiedFailureReason(ctx, "qualified-lifecycle-unavailable"))
	}
	encoded, err := qualifiedLifecycleBytes(result, dogfoodEventMaxBytes)
	if err == nil {
		_, err = validateQualifiedResult(encoded)
	}
	if err == nil && native {
		encoded, err = qualifiedNativeBytes(encoded)
	}
	if errors.Is(err, repoenvelope.ErrTerminatorCollision) {
		return fallback(repoenvelope.CollisionCode)
	}
	if err != nil || len(encoded) > dogfoodEventMaxBytes || ctx.Err() != nil {
		return fallback(qualifiedFailureReason(ctx, "qualified-lifecycle-output-unavailable"))
	}
	if _, err = stdout.Write(encoded); err != nil {
		return 2
	}
	return 0
}

func qualifiedLifecycle(ctx context.Context, request qualifiedRequest, resolve qualifiedResolver) (map[string]any, error) {
	var result map[string]any
	observed := false
	resolution, err := resolve(ctx, request.event == "stop", func(ctx context.Context, scope authoritystore.LifecycleScope) error {
		if observed || scope.RepositoryRoot == "" || scope.Direct != (request.profile == directQualifiedLifecycleProfile) {
			return errQualifiedLifecycle
		}
		observed = true
		eventOptions := options{root: scope.RepositoryRoot, event: request.event, budgetBytes: dogfoodEventMaxBytes}
		var readErr error
		result, readErr = localEventRead(ctx, eventOptions, request.input, func(options options, input map[string]any, repo gokernel.Repository, evaluation localcompletion.Evaluation) map[string]any {
			return qualifiedEnvelope(options, input, repo, evaluation, scope)
		}, qualifiedLifecycleContext, scope.ExpectedTarget)
		return readErr
	})
	if err != nil || !observed || result == nil || ctx.Err() != nil {
		return nil, errQualifiedLifecycle
	}
	if request.event == "stop" {
		if err = qualifiedStop(result, request.input, resolution); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func qualifiedEnvelope(options options, input map[string]any, repo gokernel.Repository, evaluation localcompletion.Evaluation, scope authoritystore.LifecycleScope) map[string]any {
	legacy := dogfoodEventEnvelope(options, input, repo, evaluation)
	profile := qualifiedLifecycleProfile
	if scope.Direct {
		profile = directQualifiedLifecycleProfile
	}
	requestBytes, _ := gokernel.CanonicalJSON(map[string]any{"profile": profile, "event": options.event, "input": input})
	surfaces := make([]map[string]any, 0, len(scope.QualifiedSurfaces))
	for _, surface := range scope.QualifiedSurfaces {
		surfaces = append(surfaces, map[string]any{"surface": surface.Surface, "evidenceSHA256": surface.EvidenceSHA256})
	}
	support, qualification, degradations := "FULL", "QUALIFIED", []string{}
	if scope.QualifiedHostSHA256 == "" {
		support, qualification, degradations = "FALLBACK", "UNQUALIFIED", []string{"native-tuple-unqualified"}
	}
	result := map[string]any{
		"profile": profile, "ok": true, "mutates": false,
		"event": options.event, "requestProvenance": "caller-asserted", "requestSha256": dogfoodSHA(requestBytes),
		"support": support, "qualification": qualification, "degradations": degradations,
		"qualifiedHost": map[string]any{"host": "codex", "digest": scope.QualifiedHostSHA256, "evidenceSHA256": scope.EvidenceSHA256,
			"appSHA256": scope.AppSHA256, "engineSHA256": scope.EngineSHA256, "adapterSHA256": scope.AdapterSHA256,
			"osBuild": scope.OSBuild, "architecture": scope.Architecture, "supportScope": scope.SupportScope,
			"eventSurface": "unattributed", "qualifiedSurfaces": surfaces},
		"repository": legacy["repository"], "policy": legacy["policy"], "completion": legacy["completion"],
		"decision":  legacy["completion"],
		"authority": "NONE", "frontier": map[string]any{"state": "NOT_EVALUATED", "universeSHA256": "", "decision": "release", "reason": "not-stop-event"},
	}
	if scope.Direct {
		host := result["qualifiedHost"].(map[string]any)
		delete(host, "appSHA256")
		delete(host, "engineSHA256")
		host["hostSHA256"] = scope.HostSHA256
		host["runtimeAdmissionEvidenceSHA256"] = scope.RuntimeAdmissionEvidenceSHA256
	}
	return result
}

func qualifiedStop(result map[string]any, input map[string]any, resolution authoritystore.LifecycleResolution) error {
	stop := resolution.Stop
	if !stop.RootCurrent || (stop.State != "OPEN" && stop.State != "EMPTY") || !dogfoodSessionPattern.MatchString(stop.UniverseSHA256) {
		return errQualifiedLifecycle
	}
	active, _ := input["stopHookActive"].(bool)
	permitted := stop.RemediationAllowed
	if stop.Exercise != nil {
		if stop.QualifiedHostSHA256 != "" || !dogfoodSessionPattern.MatchString(stop.Exercise.CampaignID) {
			return errQualifiedLifecycle
		}
		permitted = stop.Exercise.StopPermitted
		result["support"], result["qualification"] = "FALLBACK", "UNQUALIFIED"
	} else if stop.QualifiedHostSHA256 != resolution.Scope.QualifiedHostSHA256 || !dogfoodSessionPattern.MatchString(stop.QualifiedHostSHA256) {
		return errQualifiedLifecycle
	}
	decision, reason := authorityevent.DecisionForState(stop.State, active, permitted)
	result["authority"] = "VERIFIED"
	result["frontier"] = map[string]any{"state": stop.State, "universeSHA256": stop.UniverseSHA256, "decision": decision, "reason": reason}
	local, ok := result["completion"].(map[string]any)
	if !ok {
		return errQualifiedLifecycle
	}
	if active {
		result["decision"] = map[string]any{"decision": "release", "reason": "continuation-limit"}
	} else if local["decision"] != "block" && decision == "block" {
		result["decision"] = map[string]any{"decision": "block", "reason": "protected-frontier-open"}
	}
	return nil
}

func qualifiedLifecycleContext(ctx context.Context, options options, input map[string]any, evaluation localcompletion.Evaluation, envelope map[string]any, repo gokernel.Repository) (map[string]any, error) {
	return localEventContext(ctx, options, input, evaluation, envelope, repo, true, qualifiedLifecycleBytes, func(encoded []byte) (int, error) {
		value, err := qualifiedNativeBytes(encoded)
		size := len(value)
		// A candidate carries no completed qualification digest/surfaces. Keep
		// 512 native bytes for their maximum two-surface representation so a
		// successful candidate packet cannot exceed the final FULL budget.
		if envelope["support"] == "FALLBACK" {
			size += 512
		}
		return size, err
	})
}

func qualifiedLifecycleBytes(result map[string]any, budget int) ([]byte, error) {
	copy := make(map[string]any, len(result)+1)
	for key, value := range result {
		if key != "resultDigest" {
			copy[key] = value
		}
	}
	basis, err := gokernel.CanonicalJSON(copy)
	if err != nil {
		return nil, errQualifiedLifecycle
	}
	profile, ok := copy["profile"].(string)
	if !ok || (profile != qualifiedLifecycleProfile && profile != directQualifiedLifecycleProfile) {
		return nil, errQualifiedLifecycle
	}
	copy["resultDigest"] = qualifiedLifecycleDigest + dogfoodSHA(append([]byte(profile+"\x00"), basis...))
	encoded, err := gokernel.CanonicalJSON(copy)
	if err != nil || len(encoded)+1 > budget {
		return nil, errQualifiedLifecycle
	}
	return append(encoded, '\n'), nil
}

// The new top-level receipt and all authority-bearing members are closed typed
// objects. Context retains the existing read-only local prompt packet profile.
type qualifiedDecision struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}
type qualifiedHost struct {
	HostSHA256                     string                                `json:"hostSHA256,omitempty"`
	RuntimeAdmissionEvidenceSHA256 string                                `json:"runtimeAdmissionEvidenceSHA256,omitempty"`
	Host                           string                                `json:"host"`
	Digest                         string                                `json:"digest"`
	EvidenceSHA256                 string                                `json:"evidenceSHA256"`
	AppSHA256                      string                                `json:"appSHA256,omitempty"`
	EngineSHA256                   string                                `json:"engineSHA256,omitempty"`
	AdapterSHA256                  string                                `json:"adapterSHA256"`
	OSBuild                        string                                `json:"osBuild"`
	Architecture                   string                                `json:"architecture"`
	SupportScope                   string                                `json:"supportScope"`
	EventSurface                   string                                `json:"eventSurface"`
	QualifiedSurfaces              []authoritystore.SurfaceQualification `json:"qualifiedSurfaces"`
}
type qualifiedResult struct {
	Profile           string        `json:"profile"`
	OK                bool          `json:"ok"`
	Mutates           bool          `json:"mutates"`
	Event             string        `json:"event"`
	RequestProvenance string        `json:"requestProvenance"`
	RequestSHA256     string        `json:"requestSha256"`
	Support           string        `json:"support"`
	Qualification     string        `json:"qualification"`
	Degradations      []string      `json:"degradations"`
	QualifiedHost     qualifiedHost `json:"qualifiedHost"`
	Repository        struct {
		CommitRevision   string `json:"commitRevision"`
		TreeRevision     string `json:"treeRevision"`
		ObjectFormat     string `json:"objectFormat"`
		WorktreeState    string `json:"worktreeState"`
		DirtyPathCount   int    `json:"dirtyPathCount"`
		DirtyPathsSHA256 string `json:"dirtyPathsSha256"`
	} `json:"repository"`
	Policy struct {
		Lifecycle       string   `json:"lifecycle"`
		Satisfied       bool     `json:"satisfied"`
		Unmet           []string `json:"unmet"`
		Base            string   `json:"base"`
		Target          string   `json:"target"`
		PlanDigest      string   `json:"planDigest"`
		ReportSetDigest string   `json:"reportSetDigest"`
	} `json:"policy"`
	Completion qualifiedDecision `json:"completion"`
	Decision   qualifiedDecision `json:"decision"`
	Authority  string            `json:"authority"`
	Frontier   struct {
		State          string `json:"state"`
		UniverseSHA256 string `json:"universeSHA256"`
		Decision       string `json:"decision"`
		Reason         string `json:"reason"`
	} `json:"frontier"`
	Context      map[string]any `json:"context,omitempty"`
	ResultDigest string         `json:"resultDigest"`
}

func validateQualifiedResult(encoded []byte) (qualifiedResult, error) {
	var result qualifiedResult
	raw := bytes.TrimSuffix(encoded, []byte{'\n'})
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if len(encoded) > dogfoodEventMaxBytes || decoder.Decode(&result) != nil {
		return result, errQualifiedLifecycle
	}
	// Round-trip all typed required fields through maps for sorted JSON keys.
	// Unlike protected execution WP3 receipts, local context uses numeric counts.
	typed, err := json.Marshal(result)
	var fields map[string]any
	if err != nil || json.Unmarshal(typed, &fields) != nil {
		return result, errQualifiedLifecycle
	}
	canonical, err := gokernel.CanonicalJSON(fields)
	if err != nil || !bytes.Equal(raw, canonical) ||
		(result.Profile != qualifiedLifecycleProfile && result.Profile != directQualifiedLifecycleProfile) || !result.OK || result.Mutates || result.RequestProvenance != "caller-asserted" ||
		!dogfoodSessionPattern.MatchString(result.RequestSHA256) {
		return result, errQualifiedLifecycle
	}
	host := result.QualifiedHost
	if host.Host != "codex" || host.EventSurface != "unattributed" || host.QualifiedSurfaces == nil || result.Degradations == nil {
		return result, errQualifiedLifecycle
	}
	// The support code leads; only the compact start's own compaction-* codes follow it (decision 0241).
	supportCodes := 0
	if result.Profile == directQualifiedLifecycleProfile {
		if !validDirectQualifiedHost(result) {
			return result, errQualifiedLifecycle
		}
		if result.Support == "FALLBACK" {
			supportCodes = 1
		}
	} else {
		if host.HostSHA256 != "" || host.RuntimeAdmissionEvidenceSHA256 != "" {
			return result, errQualifiedLifecycle
		}
		switch result.Support {
		case "FULL":
			if result.Qualification != "QUALIFIED" || host.OSBuild == "" || (host.Architecture != "arm64" && host.Architecture != "amd64") {
				return result, errQualifiedLifecycle
			}
			for _, digest := range []string{host.Digest, host.EvidenceSHA256, host.AppSHA256, host.EngineSHA256, host.AdapterSHA256} {
				if !dogfoodSessionPattern.MatchString(digest) {
					return result, errQualifiedLifecycle
				}
			}
			if host.SupportScope == "qualified-native-runtime" {
				if len(host.QualifiedSurfaces) != 1 || host.QualifiedSurfaces[0].Surface != "codex-desktop" || host.QualifiedSurfaces[0].EvidenceSHA256 != host.EvidenceSHA256 {
					return result, errQualifiedLifecycle
				}
			} else if host.SupportScope == "qualified-shared-runtime" {
				if len(host.QualifiedSurfaces) != 2 {
					return result, errQualifiedLifecycle
				}
				seen := map[string]bool{}
				for _, surface := range host.QualifiedSurfaces {
					if (surface.Surface != "codex-desktop" && surface.Surface != "codex-cli") || seen[surface.Surface] || !dogfoodSessionPattern.MatchString(surface.EvidenceSHA256) {
						return result, errQualifiedLifecycle
					}
					seen[surface.Surface] = true
				}
			} else {
				return result, errQualifiedLifecycle
			}
		case "FALLBACK":
			supportCodes = 1
			if result.Qualification != "UNQUALIFIED" || len(result.Degradations) == 0 || result.Degradations[0] != "native-tuple-unqualified" || host.Digest != "" || host.EvidenceSHA256 != "" || len(host.QualifiedSurfaces) != 0 || (host.SupportScope != "candidate-native-runtime" && host.SupportScope != "candidate-shared-runtime") {
				return result, errQualifiedLifecycle
			}
			for _, digest := range []string{host.AppSHA256, host.EngineSHA256, host.AdapterSHA256} {
				if !dogfoodSessionPattern.MatchString(digest) {
					return result, errQualifiedLifecycle
				}
			}
			if host.OSBuild == "" || (host.Architecture != "arm64" && host.Architecture != "amd64") {
				return result, errQualifiedLifecycle
			}
		default:
			return result, errQualifiedLifecycle
		}

	}
	if !qualifiedCompactionCodes(result, result.Degradations[supportCodes:]) {
		return result, errQualifiedLifecycle
	}
	for _, decision := range []qualifiedDecision{result.Completion, result.Decision} {
		if (decision.Decision != "release" && decision.Decision != "block") || decision.Reason == "" {
			return result, errQualifiedLifecycle
		}
	}
	if result.Event == "stop" {
		if result.Context != nil || result.Authority != "VERIFIED" || (result.Frontier.State != "OPEN" && result.Frontier.State != "EMPTY") || !dogfoodSessionPattern.MatchString(result.Frontier.UniverseSHA256) || (result.Frontier.Decision != "release" && result.Frontier.Decision != "block") || (result.Frontier.State == "EMPTY" && result.Frontier.Decision != "release") {
			return result, errQualifiedLifecycle
		}
	} else {
		if result.Authority != "NONE" || result.Frontier.State != "NOT_EVALUATED" || result.Frontier.UniverseSHA256 != "" || result.Frontier.Decision != "release" || result.Frontier.Reason != "not-stop-event" || result.Completion != (qualifiedDecision{"release", "not-stop-event"}) || result.Decision != result.Completion {
			return result, errQualifiedLifecycle
		}
		if result.Event == "session-start" || result.Event == "user-prompt" {
			if result.Context == nil || result.Context["profile"] != "corvint-dogfood-prompt/0" {
				return result, errQualifiedLifecycle
			}
		} else if result.Event != "session-end" || result.Context != nil {
			return result, errQualifiedLifecycle
		}
	}
	return result, nil
}

// qualifiedCompactionCodes admits exactly the compaction-* codes that the session-start
// context's own compaction block raises, in gokernel.CompactionDegradations order.
func qualifiedCompactionCodes(result qualifiedResult, codes []string) bool {
	compaction, present := result.Context["compaction"]
	block, _ := compaction.(map[string]any)
	if present && (result.Event != "session-start" || block == nil) {
		return false
	}
	want := gokernel.CompactionDegradations(result.Event, block)
	if len(codes) != len(want) {
		return false
	}
	for index, code := range want {
		if codes[index] != code {
			return false
		}
	}
	return true
}

func qualifiedNativeBytes(encoded []byte) ([]byte, error) {
	var result map[string]any
	if json.Unmarshal(encoded, &result) != nil || (result["profile"] != qualifiedLifecycleProfile && result["profile"] != directQualifiedLifecycleProfile) || result["requestProvenance"] != "caller-asserted" || result["ok"] != true || result["mutates"] != false {
		return nil, errQualifiedLifecycle
	}
	again, err := qualifiedLifecycleBytes(result, dogfoodEventMaxBytes)
	if err != nil || !bytes.Equal(again, encoded) {
		return nil, errQualifiedLifecycle
	}
	validated, err := validateQualifiedResult(encoded)
	if err != nil {
		return nil, err
	}
	event := validated.Event
	var native any
	switch event {
	case "session-start", "user-prompt":
		nativeEvent := "SessionStart"
		if event == "user-prompt" {
			nativeEvent = "UserPromptSubmit"
		}
		framed, err := repoenvelope.Frame(string(bytes.TrimSuffix(encoded, []byte{'\n'})))
		if err != nil {
			return nil, err
		}
		native = map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": nativeEvent, "additionalContext": framed}}
	case "stop":
		status := "Corvint qualified lifecycle " + validated.Support + "/" + validated.Qualification +
			" (" + validated.QualifiedHost.SupportScope + "); Frontier " + validated.Frontier.State +
			" (" + validated.Authority + "); local completion " + validated.Completion.Decision +
			" (" + validated.Completion.Reason + "); request provenance caller-asserted; result " + validated.ResultDigest + "."
		if validated.Decision.Decision == "block" {
			native = map[string]any{"decision": "block", "reason": status + " One remediation is requested by the enrolled local policy or protected Frontier permission."}
		} else {
			native = map[string]any{"systemMessage": status + " This Stop releases without a closure claim."}
		}
	case "session-end":
		native = map[string]any{}
	default:
		return nil, errQualifiedLifecycle
	}
	raw, err := gokernel.CanonicalJSON(native)
	if err != nil {
		return nil, errQualifiedLifecycle
	}
	return append(raw, '\n'), nil
}

func qualifiedFailureReason(ctx context.Context, fallback string) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "corvint-invocation-timeout"
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return "qualified-lifecycle-cancelled"
	}
	return fallback
}

func qualifiedFallback(reason string) map[string]any {
	if reason == "corvint-invocation-timeout" {
		return map[string]any{"systemMessage": "Corvint qualified lifecycle FALLBACK: corvint-invocation-timeout: the 1600 ms invocation deadline was exceeded. This is a time bound, not a diagnosed fault. Unrelated coding continues."}
	}
	return map[string]any{"systemMessage": "Corvint qualified lifecycle FALLBACK: " + reason + "; native qualification or current evidence is unavailable. Unrelated coding continues."}
}

func validDirectQualifiedHost(r qualifiedResult) bool {
	h := r.QualifiedHost
	if h.AppSHA256 != "" || h.EngineSHA256 != "" || h.Architecture != "arm64" || h.OSBuild == "" {
		return false
	}
	for _, d := range []string{h.HostSHA256, h.RuntimeAdmissionEvidenceSHA256, h.AdapterSHA256} {
		if !dogfoodSessionPattern.MatchString(d) {
			return false
		}
	}
	switch r.Support {
	case "FULL":
		return r.Qualification == "QUALIFIED" && dogfoodSessionPattern.MatchString(h.Digest) && dogfoodSessionPattern.MatchString(h.EvidenceSHA256) && h.SupportScope == "qualified-direct-native-runtime" && len(h.QualifiedSurfaces) == 1 && h.QualifiedSurfaces[0].Surface == "codex-cli" && h.QualifiedSurfaces[0].EvidenceSHA256 == h.EvidenceSHA256
	case "FALLBACK":
		return r.Qualification == "UNQUALIFIED" && h.Digest == "" && h.EvidenceSHA256 == "" && h.SupportScope == "candidate-direct-native-runtime" && len(h.QualifiedSurfaces) == 0 && len(r.Degradations) > 0 && r.Degradations[0] == "native-tuple-unqualified"
	}
	return false
}
