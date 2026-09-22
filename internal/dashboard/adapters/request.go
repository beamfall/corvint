package adapters

import (
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/dashboard/authority"
	"github.com/Beamfall/corvint/internal/dashboard/source"
)

const MaxConfiguredArtifacts = 10_000

type ConfiguredSource struct {
	AdapterID    AdapterID
	RelativePath string
}

type CEMOCMBinding struct {
	CEMPath      string
	OCMPath      string
	ExpectedBase string
	Target       string
	Profile      string
}

type ScanRequest struct {
	Root         string
	Sources      []ConfiguredSource
	CEMOCM       *CEMOCMBinding
	GeneratedAt  *string
	ClockSource  string
	Authority    authority.Authority
	SourceBudget *source.Budget
}

type ScanError struct{ Code string }

func (failure *ScanError) Error() string { return failure.Code }

type plannedSource struct {
	adapterID    AdapterID
	relativePath string
	ordinal      uint64
	profile      string
}

func planRequest(request ScanRequest) ([]plannedSource, *ScanError) {
	configuredCount := len(request.Sources) + 1
	if request.CEMOCM != nil {
		configuredCount++
	}
	if !validRootArgument(request.Root) || configuredCount > MaxConfiguredArtifacts || request.Authority == nil || request.SourceBudget == nil {
		return nil, invalidArgumentError()
	}
	if (request.GeneratedAt == nil && request.ClockSource != "PROCESS") ||
		(request.GeneratedAt != nil && (request.ClockSource != "CALLER" || !validTimestamp(*request.GeneratedAt))) {
		return nil, invalidArgumentError()
	}
	if request.CEMOCM != nil {
		binding := request.CEMOCM
		if !validRelativeArgument(binding.CEMPath) || !validRelativeArgument(binding.OCMPath) || binding.CEMPath == binding.OCMPath ||
			!validAnyObjectID(binding.ExpectedBase) || !validAnyObjectID(binding.Target) || len(binding.ExpectedBase) != len(binding.Target) {
			return nil, invalidArgumentError()
		}
		if binding.Profile != "cem/0.1+ocm/0.1" && binding.Profile != "cem/0.2+ocm/0.1" {
			return nil, invalidArgumentError()
		}
	}
	sources := append([]ConfiguredSource(nil), request.Sources...)
	seen := make(map[string]struct{}, len(sources))
	seen[string(AdapterLocalTrace)+"\x00.context-corvint/traces"] = struct{}{}
	for _, configured := range sources {
		descriptor, registered := Lookup(configured.AdapterID)
		if !registered || configured.AdapterID == AdapterStableRead || !validRelativeArgument(configured.RelativePath) {
			return nil, invalidArgumentError()
		}
		// Input bytes never select a profile or revive a zero-byte adapter.
		if descriptor.MaxBytes == "0" && descriptor.DeliveryStage != DeliveryUnsupported {
			return nil, invalidArgumentError()
		}
		key := string(configured.AdapterID) + "\x00" + configured.RelativePath
		if _, duplicate := seen[key]; duplicate {
			return nil, invalidArgumentError()
		}
		seen[key] = struct{}{}
	}
	sort.Slice(sources, func(left, right int) bool {
		if sources[left].AdapterID != sources[right].AdapterID {
			return sources[left].AdapterID < sources[right].AdapterID
		}
		return sources[left].RelativePath < sources[right].RelativePath
	})
	ordinals := make(map[AdapterID]uint64)
	// The default local trace store is always ordinal zero. Explicit local
	// trace sources therefore start at one even when the directory is absent.
	ordinals[AdapterLocalTrace] = 1
	result := make([]plannedSource, 0, len(sources)+1)
	result = append(result, plannedSource{adapterID: AdapterLocalTrace, relativePath: ".context-corvint/traces", ordinal: 0, profile: "corvint-local-trace/1"})
	for _, configured := range sources {
		profile := "unsupported"
		if configured.AdapterID == AdapterLocalTrace {
			profile = "corvint-local-trace/1"
		}
		result = append(result, plannedSource{
			adapterID: configured.AdapterID, relativePath: configured.RelativePath,
			ordinal: ordinals[configured.AdapterID], profile: profile,
		})
		ordinals[configured.AdapterID]++
	}
	if request.CEMOCM != nil {
		result = append(result, plannedSource{adapterID: AdapterCEMOCMBundle, ordinal: 0, profile: request.CEMOCM.Profile})
	}
	return result, nil
}

func validRelativeArgument(value string) bool {
	if value == "" || value == "." || len(value) > 4096 || !utf8.ValidString(value) ||
		path.IsAbs(value) || filepath.IsAbs(value) || filepath.VolumeName(value) != "" || hasVolumePrefix(value) ||
		path.Clean(value) != value || strings.Contains(value, "\\") {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	for _, character := range value {
		if character == 0 || unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validRootArgument(value string) bool {
	if value == "" || len(value) > 4096 || !utf8.ValidString(value) || !filepath.IsAbs(value) || filepath.Clean(value) != value {
		return false
	}
	for _, character := range value {
		if character == 0 || unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func invalidArgumentError() *ScanError {
	return &ScanError{Code: "DASHBOARD_INVALID_ARGUMENT"}
}
