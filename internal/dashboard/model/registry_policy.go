package model

type adapterPolicy struct {
	profiles   []string
	stage      DeliveryStage
	verifier   string
	maxBytes   uint64
	issueCodes []string
}

var adapterPolicies = map[string]adapterPolicy{
	"beamfall-shadow-v0": {stage: DeliveryUnsupported, verifier: "unsupported", issueCodes: []string{"SOURCE_UNSUPPORTED"}},
	"cem-ocm-bundle-v0":  {profiles: []string{"cem/0.1+ocm/0.1", "cem/0.2+ocm/0.1"}, stage: DeliveryNotStarted, verifier: "go-cem-ocm-bundle-v0", issueCodes: []string{"SOURCE_UNSUPPORTED"}},
	"frontier-usage-v0":  {stage: DeliveryUnsupported, verifier: "unsupported", issueCodes: []string{"SOURCE_UNSUPPORTED"}},
	"go-live-usage-v0":   {stage: DeliveryUnsupported, verifier: "unsupported", issueCodes: []string{"SOURCE_UNSUPPORTED"}},
	"harness-usage-v0":   {stage: DeliveryUnsupported, verifier: "unsupported", issueCodes: []string{"SOURCE_UNSUPPORTED"}},
	"head-spec-index-v0": {stage: DeliveryUnsupported, verifier: "unsupported", issueCodes: []string{"SOURCE_UNSUPPORTED"}},
	"impact-envelope-v1": {stage: DeliveryUnsupported, verifier: "unsupported", issueCodes: []string{"SOURCE_UNSUPPORTED"}},
	"local-trace-v1": {
		profiles: []string{"corvint-local-trace/1"}, stage: DeliveryNotStarted,
		verifier: "go-local-trace-v1", maxBytes: 16 << 20,
		issueCodes: []string{"OBSERVATION_TIME_UNKNOWN", "REPOSITORY_OBJECT_UNAVAILABLE", "SOURCE_CHANGED_DURING_READ", "SOURCE_INACCESSIBLE", "SOURCE_INVALID_IDENTITY", "SOURCE_INVALID_SCHEMA", "SOURCE_MULTILINK_UNQUALIFIED", "SOURCE_NOT_PRESENT", "SOURCE_OVERSIZED", "SOURCE_SPECIAL_FILE", "SOURCE_SYMLINK", "STORE_CHANGED", "TRACE_ANCESTRY_BOUND", "TRACE_STORE_BOUND", "UNSUPPORTED_OBJECT_ALTERNATES", "VERIFIER_REJECTED"},
	},
	"pulse-dogfood-v0":  {stage: DeliveryUnsupported, verifier: "unsupported", issueCodes: []string{"SOURCE_UNSUPPORTED"}},
	"query-envelope-v1": {stage: DeliveryUnsupported, verifier: "unsupported", issueCodes: []string{"SOURCE_UNSUPPORTED"}},
	"stable-read-v0": {
		profiles: []string{"dashboard-stable-read/0"}, stage: DeliveryNotStarted,
		verifier: "go-stable-read-v0", maxBytes: 16 << 20,
		issueCodes: []string{"SOURCE_CHANGED_DURING_READ", "SOURCE_INACCESSIBLE", "SOURCE_MULTILINK_UNQUALIFIED", "SOURCE_NOT_PRESENT", "SOURCE_OVERSIZED", "SOURCE_SPECIAL_FILE", "SOURCE_SYMLINK"},
	},
}

func validateDecodedSourcePolicy(source Source) error {
	policy, ok := adapterPolicies[source.AdapterID]
	if !ok || source.DeliveryStage != policy.stage || source.VerifierID != policy.verifier {
		return invalidArgument()
	}
	profileAllowed := source.Profile == "unsupported" && policy.stage == DeliveryUnsupported && policy.verifier == "unsupported"
	for _, profile := range policy.profiles {
		if source.Profile == profile {
			profileAllowed = true
			break
		}
	}
	if !profileAllowed {
		return invalidArgument()
	}
	if policy.maxBytes == 0 && (source.ByteCount != nil || source.ContentSHA256 != nil) {
		return invalidArgument()
	}
	if source.ByteCount != nil {
		count, valid := parseDecimal(*source.ByteCount)
		if !valid || count > policy.maxBytes {
			return invalidArgument()
		}
	}
	return nil
}
