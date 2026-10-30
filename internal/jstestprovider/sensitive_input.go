package jstestprovider

import (
	"errors"
	"fmt"
	"strings"
)

const (
	SensitiveExternalProfile              = "corvint-playwright-external/2"
	SensitiveInputRedactionMarker         = "[REDACTED]"
	SensitiveInputUnredacted              = "sensitive-input-unredacted"
	SensitiveInputDocumentInvalid         = "sensitive-input-document-invalid"
	SensitiveInputActionSyntaxUnsupported = "sensitive-input-action-syntax-unsupported"
	SensitiveInputDepthExceeded           = "sensitive-input-depth-exceeded"
	SensitiveInputStepBoundExceeded       = "sensitive-input-step-bound-exceeded"
	SensitiveInputStringBoundExceeded     = "sensitive-input-string-bound-exceeded"
	SensitiveInputFindingBoundExceeded    = "sensitive-input-finding-bound-exceeded"
	sensitiveInputMaxDepth                = 32
	sensitiveInputMaxSteps                = 4096
	sensitiveInputMaxStringBytes          = 64 << 10
	sensitiveInputMaxFindings             = 64
)

var (
	defaultSensitiveActionPatterns = []string{"fill", "type", "inserttext", "insert-text", "insert text", "presssequentially", "press sequentially"}
	defaultSensitiveFields         = []string{"value", "text", "inputvalue", "input-value"}
)

// SensitiveInputFinding is a typed, value-free validation result. Path names
// the structural location only; it never includes the rejected value.
type SensitiveInputFinding struct {
	Code string `json:"code"`
	Path string `json:"path"`
}

type SensitiveInputValidationError struct {
	Findings []SensitiveInputFinding
}

func (e *SensitiveInputValidationError) Error() string {
	if len(e.Findings) == 0 {
		return "sensitive input validation failed"
	}
	return fmt.Sprintf("%s: %d finding(s)", e.Findings[0].Code, len(e.Findings))
}

type normalizedSensitivePolicy struct {
	actions []string
	fields  map[string]bool
}

func sensitivePolicy(additions *SensitiveInputPolicy) (normalizedSensitivePolicy, error) {
	p := normalizedSensitivePolicy{actions: append([]string{}, defaultSensitiveActionPatterns...), fields: map[string]bool{}}
	for _, field := range defaultSensitiveFields {
		p.fields[field] = true
	}
	if additions == nil {
		return p, nil
	}
	if len(additions.AdditionalActionPatterns) > 16 || len(additions.AdditionalSensitiveFields) > 32 {
		return p, errors.New("sensitive-input-policy-invalid")
	}
	for _, pattern := range additions.AdditionalActionPatterns {
		pattern = strings.ToLower(strings.TrimSpace(pattern))
		if boundaryWords(pattern) == "" || len(pattern) > 128 {
			return p, errors.New("sensitive-input-policy-invalid")
		}
		p.actions = append(p.actions, pattern)
	}
	for _, field := range additions.AdditionalSensitiveFields {
		field = strings.ToLower(strings.TrimSpace(field))
		if field == "" || len(field) > 128 {
			return p, errors.New("sensitive-input-policy-invalid")
		}
		p.fields[field] = true
	}
	return p, nil
}
