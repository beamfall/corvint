// Package extevidence attaches external evidence provider records to the
// path-impact receipt as a separated section (docs/specs/external-evidence-provider-v0.md).
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

// Schema is the only record schema V0 accepts (EEP-V0-001).
const Schema = "external-evidence-provider/0"

// Authority is the label Core assigns to every external item (EEP-V0-007).
const Authority = "external-provider"

// Bounds from EEP-V0-012 and EEP-V0-013.
const (
	MaxProviders   = 4
	MaxRecordBytes = 1 << 20
	maxEntities    = 1000
	maxRelations   = 4000
	maxText        = 512
	maxIdentifier  = 128
	maxPath        = 1024
	maxType        = 64
)

// Evidence kinds a relation may carry (EEP-V0-007).
const (
	EvidenceDeclared = "declared"
	EvidenceObserved = "observed"
	EvidenceInferred = "inferred"
)

// Record is one decoded provider record.
type Record struct {
	Schema     string     `json:"schema"`
	Provider   Identity   `json:"provider"`
	Repository Repository `json:"repository"`
	Entities   []Entity   `json:"entities"`
	Relations  []Relation `json:"relations"`
}

// Identity names a provider and its own revision.
type Identity struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
}

// Repository names the repository revision the provider observed.
type Repository struct {
	Revision string `json:"revision"`
}

// Entity is one externally documented thing.
type Entity struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Summary string `json:"summary"`
}

// Relation is one typed, directed link between two endpoints.
type Relation struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Type      string `json:"type"`
	Evidence  string `json:"evidence"`
	Rule      string `json:"rule"`
	Reference string `json:"reference"`
	Blob      string `json:"blob,omitempty"`
}

var (
	identifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)
	typePattern       = regexp.MustCompile(`^([a-z0-9-]+:)?[a-z0-9-]+$`)
	blobPattern       = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
)

// Decode parses one record strictly: unknown members, duplicate entity ids,
// and any field outside the V0 bounds make the whole record invalid (EEP-V0-001).
func Decode(data []byte) (Record, error) {
	var record Record
	if len(data) > MaxRecordBytes {
		return record, fmt.Errorf("record exceeds %d bytes", MaxRecordBytes)
	}
	// The decoder would coerce invalid sequences to U+FFFD; EEP-V0-013 wants
	// them refused, so the bytes are checked before decoding.
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
	return record, validate(record)
}

func validate(record Record) error {
	if record.Schema != Schema {
		return fmt.Errorf("schema must be %q", Schema)
	}
	if err := checkIdentifier("provider.id", record.Provider.ID); err != nil {
		return err
	}
	if record.Provider.ID == "path" {
		return errors.New("provider.id \"path\" is reserved for path endpoints")
	}
	if err := checkIdentifier("provider.revision", record.Provider.Revision); err != nil {
		return err
	}
	if err := checkIdentifier("repository.revision", record.Repository.Revision); err != nil {
		return err
	}
	if len(record.Entities) > maxEntities {
		return fmt.Errorf("entities exceed %d", maxEntities)
	}
	if len(record.Relations) > maxRelations {
		return fmt.Errorf("relations exceed %d", maxRelations)
	}
	seen := make(map[string]struct{}, len(record.Entities))
	for position, entity := range record.Entities {
		if err := validateEntity(position, entity); err != nil {
			return err
		}
		if _, duplicate := seen[entity.ID]; duplicate {
			return fmt.Errorf("entities[%d]: duplicate id %q", position, entity.ID)
		}
		seen[entity.ID] = struct{}{}
	}
	for position, relation := range record.Relations {
		if err := validateRelation(position, relation); err != nil {
			return err
		}
	}
	return nil
}

func validateEntity(position int, entity Entity) error {
	prefix := fmt.Sprintf("entities[%d].", position)
	if err := checkIdentifier(prefix+"id", entity.ID); err != nil {
		return err
	}
	if err := checkIdentifier(prefix+"kind", entity.Kind); err != nil {
		return err
	}
	return checkText(prefix+"summary", entity.Summary)
}

func validateRelation(position int, relation Relation) error {
	prefix := fmt.Sprintf("relations[%d].", position)
	if err := checkEndpointText(prefix+"from", relation.From); err != nil {
		return err
	}
	if err := checkEndpointText(prefix+"to", relation.To); err != nil {
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
	if err := checkText(prefix+"reference", relation.Reference); err != nil {
		return err
	}
	if relation.Blob != "" && !blobPattern.MatchString(relation.Blob) {
		return fmt.Errorf("%sblob must be a lowercase hex object id", prefix)
	}
	return nil
}

func checkIdentifier(field, value string) error {
	if value == "" || len(value) > maxIdentifier || !identifierPattern.MatchString(value) {
		return fmt.Errorf("%s must be a non-empty identifier of at most %d bytes", field, maxIdentifier)
	}
	return nil
}

func checkText(field, value string) error {
	if value == "" || len(value) > maxText || !utf8.ValidString(value) {
		return fmt.Errorf("%s must be non-empty valid UTF-8 of at most %d bytes", field, maxText)
	}
	return nil
}

func checkEndpointText(field, value string) error {
	if value == "" || len(value) > maxPath+len("path:") || !utf8.ValidString(value) || strings.ContainsAny(value, "\x00\n") {
		return fmt.Errorf("%s must be a non-empty endpoint of at most %d bytes", field, maxPath+len("path:"))
	}
	return nil
}

func trimJSONError(err error) string {
	return strings.TrimPrefix(err.Error(), "json: ")
}
