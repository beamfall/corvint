// Package authorityevent defines the experimental native Stop transport. It
// grants no authority: only a protected host may supply a Resolver. The ordinary
// corvint command deliberately has no resolver until root admission exists.
package authorityevent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

const Profile = "corvint-authority-event/0"
const MaxInput = 4096

type Event struct {
	Profile          string `json:"profile"`
	Event            string `json:"event"`
	EnrollmentHandle string `json:"enrollmentHandle"`
	StopHookActive   bool   `json:"stopHookActive"`
}

// Resolution is an in-process trusted-boundary result, never an accepted wire
// input. Resolve must recompute the universe from protected enrollment, current
// policy and actual target; it must not read a producer's EMPTY field. The host
// digest identifies an independently admitted exact native tuple and complete
// AHI qualification, not a caller's version string or test fixture certificate.
// RootCurrent requires fresh, non-revoked, monotonic protected policy. The caller
// must execute this package from its immutable protected consumer release.
// QualificationExercise is supplied only by the protected resolver after
// current operator campaign admission. It is not part of any input schema.
type QualificationExercise struct {
	CampaignID    string
	StopPermitted bool
}

type Resolution struct {
	Exercise            *QualificationExercise
	SupportScope        string
	QualifiedSurfaces   []string
	State               string
	UniverseSHA256      string
	RootCurrent         bool
	QualifiedHostSHA256 string
	RemediationAllowed  bool
}

type Resolver func(context.Context, string) (Resolution, error)

type Result struct {
	Qualification       string   `json:"qualification,omitempty"`
	CampaignID          string   `json:"campaignId,omitempty"`
	SupportScope        string   `json:"supportScope,omitempty"`
	QualifiedSurfaces   []string `json:"qualifiedSurfaces,omitempty"`
	EventSurface        string   `json:"eventSurface,omitempty"`
	Profile             string   `json:"profile"`
	RequestSHA256       string   `json:"requestSHA256"`
	Authority           string   `json:"authority"`
	Support             string   `json:"support"`
	State               string   `json:"state"`
	UniverseSHA256      string   `json:"universeSHA256,omitempty"`
	QualifiedHostSHA256 string   `json:"qualifiedHostSHA256,omitempty"`
	Decision            string   `json:"decision"`
	Reason              string   `json:"reason"`
}

func digest(value []byte) string { sum := sha256.Sum256(value); return hex.EncodeToString(sum[:]) }
func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// Parse rejects duplicates, unknown fields, null/missing values and trailing
// documents. Host event bodies are projected before reaching this boundary.
func Parse(raw []byte) (Event, error) {
	var event Event
	invalid := errors.New("invalid-authority-event")
	if len(raw) > MaxInput || !utf8.Valid(raw) {
		return event, invalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return event, invalid
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return event, invalid
		}
		name, ok := key.(string)
		if !ok {
			return event, invalid
		}
		if _, exists := fields[name]; exists {
			return event, invalid
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return event, invalid
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return event, invalid
		}
		fields[name] = value
	}
	if _, err = decoder.Token(); err != nil {
		return event, invalid
	}
	if _, err = decoder.Token(); err != io.EOF {
		return event, invalid
	}
	if len(fields) != 4 {
		return event, invalid
	}
	for _, name := range []string{"profile", "event", "enrollmentHandle", "stopHookActive"} {
		if _, ok := fields[name]; !ok {
			return event, invalid
		}
	}
	clean, _ := json.Marshal(fields)
	if json.Unmarshal(clean, &event) != nil {
		return event, invalid
	}
	if event.Profile != Profile || event.Event != "stop" || !validDigest(event.EnrollmentHandle) {
		return Event{}, invalid
	}
	return event, nil
}

func Handle(ctx context.Context, event Event, resolve Resolver) Result {
	encoded, _ := json.Marshal(event)
	result := Result{Profile: Profile, RequestSHA256: digest(encoded), Authority: "NONE", Support: "UNAVAILABLE", State: "UNKNOWN", Decision: "release", Reason: "protected-authority-unavailable"}
	if _, err := Parse(encoded); err != nil {
		result.Reason = "invalid-authority-event"
		return result
	}
	if resolve == nil {
		return result
	}
	if ctx.Err() != nil {
		return result
	}
	resolution, err := resolve(ctx, event.EnrollmentHandle)
	if err != nil || ctx.Err() != nil {
		return result
	}
	if !resolution.RootCurrent {
		return result
	}
	if resolution.Exercise != nil {
		exercise := resolution.Exercise
		if !validDigest(exercise.CampaignID) || resolution.QualifiedHostSHA256 != "" || resolution.SupportScope != "" || len(resolution.QualifiedSurfaces) != 0 || !validDigest(resolution.UniverseSHA256) || (resolution.State != "OPEN" && resolution.State != "EMPTY") {
			result.Reason = "invalid-qualification-exercise"
			return result
		}
		result.Authority = "VERIFIED"
		result.Support = "FALLBACK"
		result.Qualification = "UNQUALIFIED"
		result.CampaignID = exercise.CampaignID
		result.EventSurface = "unattributed"
		result.State = resolution.State
		result.UniverseSHA256 = resolution.UniverseSHA256
		result.Decision, result.Reason = frontierDecision(resolution.State, event.StopHookActive, exercise.StopPermitted)
		result.Reason = "unqualified-exercise-" + result.Reason
		return result
	}
	if !validDigest(resolution.QualifiedHostSHA256) {
		result.Reason = "native-tuple-unqualified"
		return result
	}
	if !validDigest(resolution.UniverseSHA256) {
		return result
	}
	if resolution.State != "EMPTY" && resolution.State != "OPEN" {
		return result
	}
	result.Authority = "VERIFIED"
	result.Support = "FULL"
	if resolution.SupportScope == "qualified-shared-runtime" {
		result.SupportScope = resolution.SupportScope
		result.QualifiedSurfaces = append([]string(nil), resolution.QualifiedSurfaces...)
		result.EventSurface = "unattributed"
	}
	result.State = resolution.State
	result.UniverseSHA256 = resolution.UniverseSHA256
	result.QualifiedHostSHA256 = resolution.QualifiedHostSHA256
	result.Decision, result.Reason = frontierDecision(resolution.State, event.StopHookActive, resolution.RemediationAllowed)
	return result
}

// Both completed qualification and temporary exercise use this one rule.
func frontierDecision(state string, active, permitted bool) (string, string) {
	if state == "EMPTY" {
		return "release", "computed-frontier-empty"
	}
	if state != "OPEN" {
		return "release", "protected-authority-unavailable"
	}
	if active {
		return "release", "continuation-limit"
	}
	if permitted {
		return "block", "computed-frontier-open"
	}
	return "release", "computed-frontier-open"
}

// NativeOutput contains only fixed host-facing text, never repository content.
// A display envelope is not a cryptographic receipt or evidence of host uptake.
func NativeOutput(result Result) map[string]string {
	output := nativeOutput(result)
	if result.SupportScope == "qualified-shared-runtime" && result.Authority == "VERIFIED" {
		output["systemMessage"] += " Qualification covers the admitted shared runtime surfaces; this event's client surface is unattributed."
	}
	return output
}
func nativeOutput(result Result) map[string]string {
	exercise := result.Support == "FALLBACK" && result.Qualification == "UNQUALIFIED" && validDigest(result.CampaignID) && result.QualifiedHostSHA256 == "" && len(result.QualifiedSurfaces) == 0 && result.SupportScope == ""
	qualified := result.Support == "FULL" && result.Qualification == "" && result.CampaignID == ""
	if result.Authority != "VERIFIED" || (!qualified && !exercise) {
		return map[string]string{"systemMessage": "Corvint protected authority unavailable; Frontier UNKNOWN. Native qualification is incomplete."}
	}
	prefix := ""
	if exercise {
		prefix = "UNQUALIFIED native qualification exercise: "
	}
	if result.State == "EMPTY" {
		return map[string]string{"systemMessage": prefix + "Corvint protected consumer computed Frontier EMPTY for the enrolled universe."}
	}
	if result.State != "OPEN" {
		return map[string]string{"systemMessage": prefix + "Corvint protected authority unavailable; Frontier UNKNOWN."}
	}
	if result.Decision == "block" {
		return map[string]string{"decision": "block", "reason": prefix + "Corvint protected consumer computed Frontier OPEN. Complete the enrolled obligations and inspect the protected result before stopping."}
	}
	return map[string]string{"systemMessage": prefix + "Corvint protected Frontier remains OPEN; this Stop releases without a closure claim."}
}

// DecisionForState shares the native Stop rule with the additive qualified
// lifecycle renderer. Callers must establish protected state independently.
func DecisionForState(state string, active, permitted bool) (string, string) {
	return frontierDecision(state, active, permitted)
}
