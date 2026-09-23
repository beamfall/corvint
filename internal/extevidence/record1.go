package extevidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Schema1 is the multi-repository record schema (EEP-V1-001).
const Schema1 = "external-evidence-provider/1"

// Schema2 is the V1 record shape with the path-relation profile opted in: it
// alone composes path-to-path relations (EEP-V2-001).
const Schema2 = "external-evidence-provider/2"

// Bounds from EEP-V1-004 and EEP-V1-009.
const (
	MaxRepositories = 8
	MaxCheckouts    = 8
	maxRemote       = 256
)

// Record1 is one decoded multi-repository provider record.
type Record1 struct {
	Schema       string        `json:"schema"`
	Provider     Identity      `json:"provider"`
	Repositories []Repository1 `json:"repositories"`
	Entities     []Entity      `json:"entities"`
	Relations    []Relation1   `json:"relations"`
	Capabilities *Capabilities `json:"capabilities,omitempty"`
}

// Repository1 declares one repository: `id` and `origin` are authoritative,
// `remote` and `role` are advisory and never bind (EEP-V1-002).
type Repository1 struct {
	ID       string `json:"id"`
	Origin   string `json:"origin,omitempty"`
	Remote   string `json:"remote,omitempty"`
	Revision string `json:"revision"`
	Tree     string `json:"tree,omitempty"`
	Role     string `json:"role,omitempty"`
}

// Endpoint1 is either a repository-qualified path or a provider-qualified entity.
type Endpoint1 struct {
	Repository string `json:"repository,omitempty"`
	Path       string `json:"path,omitempty"`
	Blob       string `json:"blob,omitempty"`
	Provider   string `json:"provider,omitempty"`
	Entity     string `json:"entity,omitempty"`
}

// Relation1 is one typed, directed link between two structured endpoints.
type Relation1 struct {
	From      Endpoint1 `json:"from"`
	To        Endpoint1 `json:"to"`
	Type      string    `json:"type"`
	Evidence  string    `json:"evidence"`
	Rule      string    `json:"rule"`
	Reference string    `json:"reference"`
}

var (
	roles = map[string]struct{}{"application": {}, "test": {}, "documentation": {}, "contract": {}, "other": {}}
	// A remote is already normalized by its author: host[:port]/path with no
	// scheme, credentials, query, or `.git` suffix. Core refuses, never repairs.
	remotePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?(:[0-9]{1,5})?(/[A-Za-z0-9._~-]+)+$`)
)

// Decode1 parses one multi-repository record strictly (EEP-V1-001, EEP-V2-001).
func Decode1(data []byte) (Record1, error) {
	var record Record1
	if len(data) > MaxRecordBytes {
		return record, fmt.Errorf("record exceeds %d bytes", MaxRecordBytes)
	}
	if !utf8.Valid(data) {
		return record, errors.New("record is not valid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return record, fmt.Errorf("record is not a strict JSON document: %s", trimJSONError(err))
	}
	if decoder.More() {
		return record, errors.New("record has trailing content after the document")
	}
	return record, validate1(record)
}

func validate1(record Record1) error {
	if record.Schema != Schema1 && record.Schema != Schema2 {
		return fmt.Errorf("schema must be %q or %q", Schema1, Schema2)
	}
	if err := checkIdentifier("provider.id", record.Provider.ID); err != nil {
		return err
	}
	if err := checkIdentifier("provider.revision", record.Provider.Revision); err != nil {
		return err
	}
	if len(record.Repositories) == 0 || len(record.Repositories) > MaxRepositories {
		return fmt.Errorf("repositories must list 1 to %d repositories", MaxRepositories)
	}
	if len(record.Entities) > maxEntities {
		return fmt.Errorf("entities exceed %d", maxEntities)
	}
	if len(record.Relations) > maxRelations {
		return fmt.Errorf("relations exceed %d", maxRelations)
	}
	repositories := make(map[string]struct{}, len(record.Repositories))
	for position, repository := range record.Repositories {
		if err := validateRepository(position, repository); err != nil {
			return err
		}
		if _, duplicate := repositories[repository.ID]; duplicate {
			return fmt.Errorf("repositories[%d]: duplicate id %q", position, repository.ID)
		}
		repositories[repository.ID] = struct{}{}
	}
	entities := make(map[string]struct{}, len(record.Entities))
	for position, entity := range record.Entities {
		if err := validateEntity(position, entity); err != nil {
			return err
		}
		if _, duplicate := entities[entity.ID]; duplicate {
			return fmt.Errorf("entities[%d]: duplicate id %q", position, entity.ID)
		}
		entities[entity.ID] = struct{}{}
	}
	for position, relation := range record.Relations {
		if err := validateRelation1(position, relation); err != nil {
			return err
		}
		if err := checkScope2(record.Schema, position, relation); err != nil {
			return err
		}
	}
	return validateCapabilities(record.Capabilities)
}

// checkScope2 refuses a pinned blob on a V2 directory scope: a path ending in
// "/" names a tree, never one blob (EEP-V2-012). V1 keeps its rules.
func checkScope2(schema string, position int, relation Relation1) error {
	if schema != Schema2 {
		return nil
	}
	for field, side := range []Endpoint1{relation.From, relation.To} {
		if strings.HasSuffix(side.Path, "/") && side.Blob != "" {
			return fmt.Errorf("relations[%d].%s: a directory scope must not pin a blob", position, []string{"from", "to"}[field])
		}
	}
	return nil
}

func validateRepository(position int, repository Repository1) error {
	prefix := fmt.Sprintf("repositories[%d].", position)
	if err := checkIdentifier(prefix+"id", repository.ID); err != nil {
		return err
	}
	if err := checkIdentifier(prefix+"revision", repository.Revision); err != nil {
		return err
	}
	if repository.Origin != "" && !blobPattern.MatchString(repository.Origin) {
		return fmt.Errorf("%sorigin must be a full lowercase hex commit id", prefix)
	}
	if repository.Tree != "" && !blobPattern.MatchString(repository.Tree) {
		return fmt.Errorf("%stree must be a full lowercase hex tree id", prefix)
	}
	if repository.Remote != "" && (len(repository.Remote) > maxRemote || !remotePattern.MatchString(repository.Remote) || hasGitSuffix(repository.Remote)) {
		return fmt.Errorf("%sremote must be a normalized host/path of at most %d bytes without scheme, credentials, or .git suffix", prefix, maxRemote)
	}
	if _, known := roles[repository.Role]; repository.Role != "" && !known {
		return fmt.Errorf("%srole must be application, test, documentation, contract, or other", prefix)
	}
	return nil
}

func hasGitSuffix(remote string) bool {
	return len(remote) >= 4 && remote[len(remote)-4:] == ".git"
}

func validateRelation1(position int, relation Relation1) error {
	prefix := fmt.Sprintf("relations[%d].", position)
	if err := checkEndpoint1(prefix+"from", relation.From); err != nil {
		return err
	}
	if err := checkEndpoint1(prefix+"to", relation.To); err != nil {
		return err
	}
	if len(relation.Type) > maxType || !typePattern.MatchString(relation.Type) {
		return fmt.Errorf("%stype must be lowercase kebab-case of at most %d bytes", prefix, maxType)
	}
	if err := checkIdentifier(prefix+"evidence", relation.Evidence); err != nil {
		return err
	}
	if err := checkText(prefix+"rule", relation.Rule); err != nil {
		return err
	}
	return checkText(prefix+"reference", relation.Reference)
}

// checkEndpoint1 admits exactly one endpoint shape; semantic resolution
// (declared repository, declared entity, path bounds) happens in composition.
func checkEndpoint1(field string, endpoint Endpoint1) error {
	isPath := endpoint.Repository != "" || endpoint.Path != "" || endpoint.Blob != ""
	isEntity := endpoint.Provider != "" || endpoint.Entity != ""
	if isPath == isEntity {
		return fmt.Errorf("%s must be exactly one of {repository, path[, blob]} or {provider, entity}", field)
	}
	if isEntity {
		if err := checkIdentifier(field+".provider", endpoint.Provider); err != nil {
			return err
		}
		return checkIdentifier(field+".entity", endpoint.Entity)
	}
	if err := checkIdentifier(field+".repository", endpoint.Repository); err != nil {
		return err
	}
	if err := checkEndpointText(field+".path", endpoint.Path); err != nil {
		return err
	}
	if endpoint.Blob != "" && !blobPattern.MatchString(endpoint.Blob) {
		return fmt.Errorf("%s.blob must be a lowercase hex object id", field)
	}
	return nil
}
