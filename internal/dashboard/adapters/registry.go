package adapters

import "github.com/Beamfall/corvint/internal/dashboard/model"

// AdapterID identifies one compiled dashboard adapter. Values are closed by
// Registry and cannot be selected by artifact contents.
type AdapterID string

const (
	AdapterStableRead     AdapterID = "stable-read-v0"
	AdapterLocalTrace     AdapterID = "local-trace-v1"
	AdapterCEMOCMBundle   AdapterID = "cem-ocm-bundle-v0"
	AdapterQueryEnvelope  AdapterID = "query-envelope-v1"
	AdapterImpactEnvelope AdapterID = "impact-envelope-v1"
	AdapterHeadSpecIndex  AdapterID = "head-spec-index-v0"
	AdapterBeamfallShadow AdapterID = "beamfall-shadow-v0"
	AdapterPulseDogfood   AdapterID = "pulse-dogfood-v0"
	AdapterHarnessUsage   AdapterID = "harness-usage-v0"
	AdapterGoLiveUsage    AdapterID = "go-live-usage-v0"
	AdapterFrontierUsage  AdapterID = "frontier-usage-v0"
)

type DeliveryStage string

const (
	DeliveryNotStarted  DeliveryStage = "NOT_STARTED"
	DeliveryUnsupported DeliveryStage = "UNSUPPORTED"
)

// Descriptor is the complete compiled registry row. Slices returned by
// Registry are independent copies so callers cannot mutate registry truth.
type Descriptor struct {
	AdapterID        AdapterID
	AcceptedProfiles []string
	DefaultLocation  *string
	DeliveryStage    DeliveryStage
	IssueCodes       []string
	MaxBytes         string
	SourceKind       string
	VerifierID       string
}

type UnavailableState struct {
	AdapterID      AdapterID
	Validity       string
	EpistemicClass string
	AuthorityClass string
	Completeness   string
	DeliveryStage  DeliveryStage
}

var registry = []Descriptor{
	{
		AdapterID: AdapterBeamfallShadow, AcceptedProfiles: nil, DefaultLocation: nil,
		DeliveryStage: DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"},
		MaxBytes: "0", SourceKind: "BEAMFALL_SHADOW", VerifierID: "unsupported",
	},
	{
		AdapterID:        AdapterCEMOCMBundle,
		AcceptedProfiles: []string{"cem/0.1+ocm/0.1", "cem/0.2+ocm/0.1"},
		DefaultLocation:  nil, DeliveryStage: DeliveryNotStarted,
		IssueCodes: []string{"SOURCE_UNSUPPORTED"},
		MaxBytes:   "0", SourceKind: "CEM_OCM_BUNDLE", VerifierID: "go-cem-ocm-bundle-v0",
	},
	{
		AdapterID: AdapterFrontierUsage, AcceptedProfiles: nil, DefaultLocation: nil,
		DeliveryStage: DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"},
		MaxBytes: "0", SourceKind: "FRONTIER_RECEIPT", VerifierID: "unsupported",
	},
	{
		AdapterID: AdapterGoLiveUsage, AcceptedProfiles: nil, DefaultLocation: nil,
		DeliveryStage: DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"},
		MaxBytes: "0", SourceKind: "GO_LIVE_RECEIPT", VerifierID: "unsupported",
	},
	{
		AdapterID: AdapterHarnessUsage, AcceptedProfiles: nil, DefaultLocation: nil,
		DeliveryStage: DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"},
		MaxBytes: "0", SourceKind: "HARNESS_RECEIPT", VerifierID: "unsupported",
	},
	{
		AdapterID: AdapterHeadSpecIndex, AcceptedProfiles: nil, DefaultLocation: nil,
		DeliveryStage: DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"},
		MaxBytes: "0", SourceKind: "HEAD_SPEC_INDEX", VerifierID: "unsupported",
	},
	{
		AdapterID: AdapterImpactEnvelope, AcceptedProfiles: nil, DefaultLocation: nil,
		DeliveryStage: DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"},
		MaxBytes: "0", SourceKind: "IMPACT_ENVELOPE", VerifierID: "unsupported",
	},
	{
		AdapterID: AdapterLocalTrace, AcceptedProfiles: []string{"corvint-local-trace/1"},
		DefaultLocation: stringPointer(".context-corvint/traces"), DeliveryStage: DeliveryNotStarted,
		IssueCodes: []string{"OBSERVATION_TIME_UNKNOWN", "REPOSITORY_OBJECT_UNAVAILABLE", "SOURCE_CHANGED_DURING_READ", "SOURCE_INACCESSIBLE", "SOURCE_INVALID_IDENTITY", "SOURCE_INVALID_SCHEMA", "SOURCE_MULTILINK_UNQUALIFIED", "SOURCE_NOT_PRESENT", "SOURCE_OVERSIZED", "SOURCE_SPECIAL_FILE", "SOURCE_SYMLINK", "STORE_CHANGED", "TRACE_ANCESTRY_BOUND", "TRACE_STORE_BOUND", "UNSUPPORTED_OBJECT_ALTERNATES", "VERIFIER_REJECTED"},
		MaxBytes:   "16777216", SourceKind: "LOCAL_TRACE_STORE", VerifierID: "go-local-trace-v1",
	},
	{
		AdapterID: AdapterPulseDogfood, AcceptedProfiles: nil, DefaultLocation: nil,
		DeliveryStage: DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"},
		MaxBytes: "0", SourceKind: "PULSE_RECEIPT", VerifierID: "unsupported",
	},
	{
		AdapterID: AdapterQueryEnvelope, AcceptedProfiles: nil, DefaultLocation: nil,
		DeliveryStage: DeliveryUnsupported, IssueCodes: []string{"SOURCE_UNSUPPORTED"},
		MaxBytes: "0", SourceKind: "QUERY_ENVELOPE", VerifierID: "unsupported",
	},
	{
		AdapterID: AdapterStableRead, AcceptedProfiles: []string{"dashboard-stable-read/0"},
		DefaultLocation: nil, DeliveryStage: DeliveryNotStarted,
		IssueCodes: []string{"SOURCE_CHANGED_DURING_READ", "SOURCE_INACCESSIBLE", "SOURCE_MULTILINK_UNQUALIFIED", "SOURCE_NOT_PRESENT", "SOURCE_OVERSIZED", "SOURCE_SPECIAL_FILE", "SOURCE_SYMLINK"},
		MaxBytes:   "16777216", SourceKind: "INTERNAL", VerifierID: "go-stable-read-v0",
	},
}

// Registry returns all registry rows sorted by adapter ID.
func Registry() []Descriptor {
	result := make([]Descriptor, len(registry))
	for index, item := range registry {
		result[index] = item
		result[index].AcceptedProfiles = append([]string(nil), item.AcceptedProfiles...)
		result[index].IssueCodes = append([]string(nil), item.IssueCodes...)
		if item.DefaultLocation != nil {
			value := *item.DefaultLocation
			result[index].DefaultLocation = &value
		}
	}
	return result
}

// Registrations converts the closed registry into model inputs. The model is
// responsible for canonical validation and the registry-domain hash.
func Registrations() []model.AdapterRegistration {
	descriptors := Registry()
	result := make([]model.AdapterRegistration, len(descriptors))
	for index, descriptor := range descriptors {
		result[index] = model.AdapterRegistration{
			AdapterID: string(descriptor.AdapterID), AcceptedProfiles: descriptor.AcceptedProfiles,
			DefaultLocation: descriptor.DefaultLocation,
			DeliveryStage:   model.DeliveryStage(descriptor.DeliveryStage),
			IssueCodes:      descriptor.IssueCodes, MaxBytes: descriptor.MaxBytes,
			SourceKind: descriptor.SourceKind, VerifierID: descriptor.VerifierID,
		}
	}
	return result
}

func Lookup(adapterID AdapterID) (Descriptor, bool) {
	for _, descriptor := range Registry() {
		if descriptor.AdapterID == adapterID {
			return descriptor, true
		}
	}
	return Descriptor{}, false
}

// Unavailable returns the required fail-closed base state for a registered
// adapter that has no delivered verifier. It deliberately accepts no path or
// artifact bytes, so filename discovery cannot upgrade delivery.
func Unavailable(adapterID AdapterID) (UnavailableState, bool) {
	descriptor, ok := Lookup(adapterID)
	if !ok || adapterID == AdapterStableRead || adapterID == AdapterLocalTrace {
		return UnavailableState{}, false
	}
	return UnavailableState{
		AdapterID: adapterID, Validity: "UNSUPPORTED", EpistemicClass: "NOT_OBSERVED",
		AuthorityClass: "NONE", Completeness: "UNKNOWN",
		DeliveryStage: descriptor.DeliveryStage,
	}, true
}

func stringPointer(value string) *string { return &value }
