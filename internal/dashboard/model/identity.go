package model

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

const (
	cohortDomain            = "corvint-dashboard-cohort/0"
	configuredSourcesDomain = "corvint-dashboard-configured-sources/0"
	issueDomain             = "corvint-dashboard-issue/0"
	registryDomain          = "corvint-dashboard-adapter-registry/0"
	repositoryReadsDomain   = "corvint-dashboard-repository-reads/0"
	snapshotDomain          = "corvint-dashboard-snapshot/0"
	sourceDomain            = "corvint-dashboard-source/0"
	traceStoreDomain        = "trace-store/0"
)

type SourceIdentity struct {
	AdapterID             string  `json:"adapterId"`
	ConfiguredOrdinal     string  `json:"configuredOrdinal"`
	ContentSHA256         *string `json:"contentSha256"`
	Profile               string  `json:"profile"`
	RepositoryReadsSHA256 *string `json:"repositoryReadsSha256"`
}

type configuredSourceIdentity struct {
	AdapterID         string  `json:"adapterId"`
	ConfiguredOrdinal string  `json:"configuredOrdinal"`
	ContentSHA256     *string `json:"contentSha256"`
}

func ComputeCohortID(identity CohortIdentity) (string, error) {
	return domainHash("dashboard-cohort:sha256:", cohortDomain, identity)
}

func ComputeSourceID(identity SourceIdentity) (string, error) {
	return domainHash("dashboard-source:sha256:", sourceDomain, identity)
}

func ComputeRepositoryReadsSHA256(witnesses []RepositoryWitness) (string, error) {
	values, err := normalizeRepositoryWitnesses(witnesses)
	if err != nil {
		return "", err
	}
	return domainHash("sha256:", repositoryReadsDomain, values)
}

func normalizeRepositoryWitnesses(witnesses []RepositoryWitness) ([]RepositoryWitness, error) {
	values := append([]RepositoryWitness(nil), witnesses...)
	for _, witness := range values {
		if !oneOf(witness.ObjectFormat, "sha1", "sha256") || !validateObjectID(witness.ObjectID, &witness.ObjectFormat) {
			return nil, invalidArgument()
		}
		switch witness.Kind {
		case "SNAPSHOT_HEAD":
			if witness.ObjectType != "commit" || witness.Revision != nil {
				return nil, invalidArgument()
			}
		case "TRACE_REVISION":
			if witness.ObjectType != "commit" || witness.Revision == nil || *witness.Revision != witness.ObjectID {
				return nil, invalidArgument()
			}
		case "TRACE_PATH_OBJECT":
			if witness.ObjectType != "blob" || witness.Revision == nil || !validateObjectID(*witness.Revision, &witness.ObjectFormat) {
				return nil, invalidArgument()
			}
		default:
			return nil, invalidArgument()
		}
	}
	sort.Slice(values, func(i, j int) bool {
		left, _ := canonicalJSON(values[i])
		right, _ := canonicalJSON(values[j])
		return string(left) < string(right)
	})
	result := values[:0]
	var previous string
	for index, witness := range values {
		encoded, _ := canonicalJSON(witness)
		key := string(encoded)
		if index == 0 || key != previous {
			result = append(result, witness)
			previous = key
		}
	}
	return result, nil
}

func NewIssue(input IssueInput) (Issue, error) {
	issue := Issue{
		Code: input.Code, Severity: input.Severity, SourceID: cloneString(input.SourceID),
		Observed: cloneString(input.Observed), Limit: cloneString(input.Limit),
	}
	if err := validateIssue(issue); err != nil {
		return Issue{}, err
	}
	preimage := struct {
		Code     string   `json:"code"`
		Severity Severity `json:"severity"`
		SourceID *string  `json:"sourceId"`
		Observed *string  `json:"observed"`
		Limit    *string  `json:"limit"`
	}{issue.Code, issue.Severity, issue.SourceID, issue.Observed, issue.Limit}
	id, err := domainHash("dashboard-issue:sha256:", issueDomain, preimage)
	if err != nil {
		return Issue{}, err
	}
	issue.ID = id
	return issue, nil
}

func domainHash(prefix, domain string, value any) (string, error) {
	canonical, ok := value.(jsonRaw)
	if !ok {
		var err error
		canonical, err = canonicalJSON(value)
		if err != nil {
			return "", invalidArgument()
		}
	}
	hash := sha256.New()
	hash.Write([]byte(domain))
	hash.Write([]byte{0})
	hash.Write(canonical)
	return prefix + hex.EncodeToString(hash.Sum(nil)), nil
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
