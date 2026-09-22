package workqueue

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

const (
	PolicyProfile      = "work-queue-policy/0"
	DetailsProfile     = "work-queue-detail/0"
	CheckpointProfile  = "work-queue-checkpoint/0"
	ObservationProfile = "work-queue-observation/0"
	CommandProfile     = "work-command-result/0"
)

type PolicyOperations struct {
	Details  []string `json:"details"`
	Snapshot []string `json:"snapshot"`
	Verify   []string `json:"verify"`
}

type Policy struct {
	AccessContextID       string           `json:"accessContextId"`
	AdapterPath           string           `json:"adapterPath"`
	AdapterProfile        string           `json:"adapterProfile"`
	DetailLimit           string           `json:"detailLimit"`
	ID                    string           `json:"id"`
	MappingVersion        string           `json:"mappingVersion"`
	Operations            PolicyOperations `json:"operations"`
	Profile               string           `json:"profile"`
	QueueAuthorityID      string           `json:"queueAuthorityId"`
	RepositoryAuthorityID string           `json:"repositoryAuthorityId"`
	ScopeID               string           `json:"scopeId"`
}

type DetailPayload struct {
	AcceptanceCriteria []string `json:"acceptanceCriteria"`
	Body               *string  `json:"body"`
	DisplayKey         *string  `json:"displayKey"`
	EvidenceHandles    []string `json:"evidenceHandles"`
	Owner              *string  `json:"owner"`
	Title              *string  `json:"title"`
}

type Detail struct {
	DetailID              string        `json:"detailId"`
	Payload               DetailPayload `json:"payload"`
	PayloadSHA256         string        `json:"payloadSha256"`
	RepositoryAuthorityID string        `json:"repositoryAuthorityId"`
	TicketID              string        `json:"ticketId"`
	TicketVersionID       string        `json:"ticketVersionId"`
}

type DetailsDocument struct {
	Details    []Detail `json:"details"`
	ID         string   `json:"id"`
	Profile    string   `json:"profile"`
	SnapshotID string   `json:"snapshotId"`
}

type CheckpointDocument struct {
	Checkpoint       Checkpoint       `json:"checkpoint"`
	ID               string           `json:"id"`
	PolicyID         string           `json:"policyId"`
	Profile          string           `json:"profile"`
	RepositorySource RepositorySource `json:"repositorySource"`
	SnapshotID       string           `json:"snapshotId"`
}

type ExecutableIdentity struct {
	FileSHA256 string `json:"fileSha256"`
	Mode       string `json:"mode"`
	PathSHA256 string `json:"pathSha256"`
}

type AdapterReceipt struct {
	AdapterBlobOID          string               `json:"adapterBlobOid"`
	AdapterFileSHA256       string               `json:"adapterFileSha256"`
	AdapterMode             string               `json:"adapterMode"`
	Argv                    []string             `json:"argv"`
	ContainmentClass        string               `json:"containmentClass"`
	ExecutableQualification string               `json:"executableQualification"`
	ExitCode                *Count               `json:"-"`
	ID                      string               `json:"id"`
	InterpreterChain        []ExecutableIdentity `json:"interpreterChain"`
	Operation               string               `json:"operation"`
	PolicyID                string               `json:"policyId"`
	Signal                  *string              `json:"signal"`
	State                   string               `json:"state"`
	StderrBytes             Count                `json:"-"`
	StderrRawSHA256         string               `json:"stderrRawSha256"`
	StdoutBytes             Count                `json:"-"`
	StdoutRawSHA256         string               `json:"stdoutRawSha256"`
}

type Observation struct {
	AdapterReceipts  []AdapterReceipt `json:"adapterReceipts"`
	ContainmentClass string           `json:"containmentClass"`
	DetailIDs        []string         `json:"detailIds"`
	EndCheckpoint    Checkpoint       `json:"endCheckpoint"`
	ID               string           `json:"id"`
	MutationState    string           `json:"mutationState"`
	NetworkState     string           `json:"networkState"`
	PolicyID         string           `json:"policyId"`
	Profile          string           `json:"profile"`
	QueueSourceID    string           `json:"queueSourceId"`
	SnapshotID       string           `json:"snapshotId"`
	StartCheckpoint  Checkpoint       `json:"startCheckpoint"`
	State            string           `json:"state"`
	Unknowns         []string         `json:"unknowns"`
}

type CommandResult struct {
	ErrorCode   *string      `json:"errorCode"`
	ID          string       `json:"id"`
	Observation *Observation `json:"observation"`
	Profile     string       `json:"profile"`
	Proposal    *Proposal    `json:"proposal"`
	State       string       `json:"state"`
}

func SHA256Hex(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func canonicalAny(value any) []byte {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
	encoded := bytes.TrimSuffix(output.Bytes(), []byte{'\n'})
	// encoding/json always writes U+2028/U+2029 as optional \u escapes, which
	// §4.1 forbids; the wire encoder is the one canonical profile.
	parsed, err := wire.Parse(encoded)
	if err != nil {
		return encoded
	}
	return wire.CanonicalValue(parsed)
}

func identityAny(kind, profile string, body any) string {
	return kind + ":sha256:" + digestAny(kind, profile, body)
}

func digestAny(kind, profile string, body any) string {
	digest := sha256.New()
	digest.Write([]byte(kind))
	digest.Write([]byte{0})
	digest.Write([]byte(profile))
	digest.Write([]byte{0})
	digest.Write(canonicalAny(body))
	return hex.EncodeToString(digest.Sum(nil))
}

func policyBody(policy *Policy) map[string]any {
	return map[string]any{
		"accessContextId": policy.AccessContextID, "adapterPath": policy.AdapterPath,
		"adapterProfile": policy.AdapterProfile, "detailLimit": policy.DetailLimit,
		"mappingVersion": policy.MappingVersion, "operations": policy.Operations,
		"profile": policy.Profile, "queueAuthorityId": policy.QueueAuthorityID,
		"repositoryAuthorityId": policy.RepositoryAuthorityID, "scopeId": policy.ScopeID,
	}
}

func (policy *Policy) RefreshIdentity() {
	policy.ID = identityAny("work-queue-policy", PolicyProfile, policyBody(policy))
}
func (policy *Policy) Canonical() []byte { return append(canonicalAny(policy), '\n') }

func ParsePolicy(raw []byte) (*Policy, error) {
	root, err := parseCanonicalDocument(raw, 64<<10)
	if err != nil {
		return nil, err
	}
	object, err := exactObject(root, []string{"accessContextId", "adapterPath", "adapterProfile", "detailLimit", "id", "mappingVersion", "operations", "profile", "queueAuthorityId", "repositoryAuthorityId", "scopeId"}, "policy")
	if err != nil {
		return nil, err
	}
	policy := &Policy{}
	if policy.AccessContextID, err = qualifiedMember(object, "accessContextId", "access"); err != nil {
		return nil, err
	}
	if policy.AdapterPath, err = stringMember(object, "adapterPath"); err != nil {
		return nil, err
	}
	if err = ValidatePath(policy.AdapterPath); err != nil {
		return nil, err
	}
	if policy.AdapterPath[len(policy.AdapterPath)-1] == '/' {
		return nil, fail(CodeMalformedInput, "adapterPath must name a file")
	}
	if policy.AdapterProfile, err = exactStringMember(object, "adapterProfile", "repository-work-queue-adapter/0"); err != nil {
		return nil, err
	}
	limit, err := countMember(object, "detailLimit", false)
	if err != nil {
		return nil, err
	}
	if limit > 512 {
		return nil, fail(CodeMalformedInput, "detailLimit exceeds policy bound")
	}
	policy.DetailLimit = limit.String()
	if policy.ID, err = contentIDMember(object, "id", "work-queue-policy"); err != nil {
		return nil, err
	}
	if policy.MappingVersion, err = identifierMember(object, "mappingVersion"); err != nil {
		return nil, err
	}
	if policy.Operations, err = parsePolicyOperations(member(object, "operations")); err != nil {
		return nil, err
	}
	if policy.Profile, err = exactStringMember(object, "profile", PolicyProfile); err != nil {
		return nil, err
	}
	if policy.QueueAuthorityID, err = queueMember(object, "queueAuthorityId"); err != nil {
		return nil, err
	}
	if policy.RepositoryAuthorityID, err = repositoryMember(object, "repositoryAuthorityId"); err != nil {
		return nil, err
	}
	if policy.ScopeID, err = qualifiedMember(object, "scopeId", "scope"); err != nil {
		return nil, err
	}
	if policy.ID != identityAny("work-queue-policy", PolicyProfile, policyBody(policy)) {
		return nil, fail(CodeConflicted, "policy identity does not rederive")
	}
	return policy, nil
}

func parsePolicyOperations(value wire.Value) (PolicyOperations, error) {
	object, err := exactObject(value, []string{"details", "snapshot", "verify"}, "operations")
	result := PolicyOperations{}
	if err != nil {
		return result, err
	}
	if result.Details, err = parseOperation(member(object, "details")); err != nil {
		return result, err
	}
	if result.Snapshot, err = parseOperation(member(object, "snapshot")); err != nil {
		return result, err
	}
	if result.Verify, err = parseOperation(member(object, "verify")); err != nil {
		return result, err
	}
	return result, nil
}

func parseOperation(value wire.Value) ([]string, error) {
	if value.Kind != wire.KindArray || len(value.Arr) < 1 || len(value.Arr) > 8 {
		return nil, fail(CodeMalformedInput, "operation must have 1..8 arguments")
	}
	result := make([]string, 0, len(value.Arr))
	for _, argument := range value.Arr {
		if argument.Kind != wire.KindString {
			return nil, fail(CodeMalformedInput, "operation argument must be an Identifier")
		}
		if err := ValidateIdentifier(argument.Str); err != nil {
			return nil, err
		}
		result = append(result, argument.Str)
	}
	return result, nil
}

func RefreshRepositorySource(source *RepositorySource) {
	source.ID = contentIdentity("repository-source", "repository-source/0", repositorySourceValue(*source, false))
}

func RefreshTicket(ticket *TicketSummary) {
	ticket.TicketVersionID = contentIdentity("ticket-version", "ticket-version/0", ticketVersionBody(*ticket))
}

func RefreshLease(lease *LeaseSummary) {
	lease.LeaseVersionID = contentIdentity("lease-version", "lease-version/0", leaseValue(*lease, false))
}

func RefreshSnapshot(snapshot *Snapshot) {
	snapshot.ID = contentIdentity("work-queue-snapshot", SnapshotProfile, snapshotValue(snapshot, false))
}

func RefreshEnvelope(envelope *CapacityEnvelope) {
	envelope.ID = contentIdentity("work-capacity-envelope", EnvelopeProfile, envelopeValue(envelope, false))
}

func DetailPayloadDigest(payload DetailPayload) string {
	return digestAny("work-queue-detail-payload", "work-queue-detail-payload/0", payload)
}

func detailBody(detail Detail) map[string]any {
	return map[string]any{"payload": detail.Payload, "payloadSha256": detail.PayloadSHA256,
		"repositoryAuthorityId": detail.RepositoryAuthorityID, "ticketId": detail.TicketID,
		"ticketVersionId": detail.TicketVersionID}
}

func RefreshDetail(detail *Detail) {
	detail.PayloadSHA256 = DetailPayloadDigest(detail.Payload)
	detail.DetailID = identityAny("work-queue-detail-record", "work-queue-detail-record/0", detailBody(*detail))
}

func detailsBody(document *DetailsDocument) map[string]any {
	return map[string]any{"details": document.Details, "profile": document.Profile, "snapshotId": document.SnapshotID}
}

func RefreshDetails(document *DetailsDocument) {
	document.Profile = DetailsProfile
	sort.Slice(document.Details, func(i, j int) bool {
		return bytes.Compare(canonicalAny(document.Details[i]), canonicalAny(document.Details[j])) < 0
	})
	document.ID = identityAny("work-queue-details", DetailsProfile, detailsBody(document))
}

func (document *DetailsDocument) Canonical() []byte { return append(canonicalAny(document), '\n') }

func ParseDetails(raw []byte) (*DetailsDocument, error) {
	root, err := parseCanonicalDocument(raw, 32<<20)
	if err != nil {
		return nil, err
	}
	object, err := exactObject(root, []string{"details", "id", "profile", "snapshotId"}, "details document")
	if err != nil {
		return nil, err
	}
	document := &DetailsDocument{}
	if document.Details, err = parseSortedArray(member(object, "details"), 512, "details", parseDetail); err != nil {
		return nil, err
	}
	if document.ID, err = contentIDMember(object, "id", "work-queue-details"); err != nil {
		return nil, err
	}
	if document.Profile, err = exactStringMember(object, "profile", DetailsProfile); err != nil {
		return nil, err
	}
	if document.SnapshotID, err = contentIDMember(object, "snapshotId", "work-queue-snapshot"); err != nil {
		return nil, err
	}
	// The entire closed document is structurally valid before identity use.
	for _, detail := range document.Details {
		if detail.PayloadSHA256 != DetailPayloadDigest(detail.Payload) || detail.DetailID != identityAny("work-queue-detail-record", "work-queue-detail-record/0", detailBody(detail)) {
			return nil, fail(CodeConflicted, "detail identity does not rederive")
		}
	}
	if document.ID != identityAny("work-queue-details", DetailsProfile, detailsBody(document)) {
		return nil, fail(CodeConflicted, "details identity does not rederive")
	}
	return document, nil
}

func parseDetail(value wire.Value) (Detail, error) {
	result := Detail{}
	if len(wire.CanonicalValue(value)) > 1<<20 {
		return result, fail(CodeInputLimit, "detail exceeds byte bound")
	}
	object, err := exactObject(value, []string{"detailId", "payload", "payloadSha256", "repositoryAuthorityId", "ticketId", "ticketVersionId"}, "detail")
	if err != nil {
		return result, err
	}
	if result.DetailID, err = contentIDMember(object, "detailId", "work-queue-detail-record"); err != nil {
		return result, err
	}
	if result.Payload, err = parseDetailPayload(member(object, "payload")); err != nil {
		return result, err
	}
	if result.PayloadSHA256, err = digestMember(object, "payloadSha256"); err != nil {
		return result, err
	}
	if result.RepositoryAuthorityID, err = repositoryMember(object, "repositoryAuthorityId"); err != nil {
		return result, err
	}
	if result.TicketID, err = qualifiedMember(object, "ticketId", "ticket"); err != nil {
		return result, err
	}
	if result.TicketVersionID, err = contentIDMember(object, "ticketVersionId", "ticket-version"); err != nil {
		return result, err
	}
	return result, nil
}

func parseDetailPayload(value wire.Value) (DetailPayload, error) {
	result := DetailPayload{}
	object, err := exactObject(value, []string{"acceptanceCriteria", "body", "displayKey", "evidenceHandles", "owner", "title"}, "detail payload")
	if err != nil {
		return result, err
	}
	if result.AcceptanceCriteria, err = parsePayloadStrings(member(object, "acceptanceCriteria"), validateProse); err != nil {
		return result, err
	}
	if result.Body, err = nullablePayloadString(member(object, "body"), validateProse); err != nil {
		return result, err
	}
	if result.DisplayKey, err = nullablePayloadString(member(object, "displayKey"), validateLabel); err != nil {
		return result, err
	}
	if result.EvidenceHandles, err = parsePayloadStrings(member(object, "evidenceHandles"), validateLabel); err != nil {
		return result, err
	}
	if result.Owner, err = nullablePayloadString(member(object, "owner"), validateLabel); err != nil {
		return result, err
	}
	if result.Title, err = nullablePayloadString(member(object, "title"), validateProse); err != nil {
		return result, err
	}
	return result, nil
}

func parsePayloadStrings(value wire.Value, validate func(string) error) ([]string, error) {
	return parseSortedArray(value, maxArrayItems, "payload strings", func(item wire.Value) (string, error) {
		if item.Kind != wire.KindString {
			return "", fail(CodeMalformedInput, "payload array member must be a string")
		}
		if err := validate(item.Str); err != nil {
			return "", err
		}
		return item.Str, nil
	})
}

func nullablePayloadString(value wire.Value, validate func(string) error) (*string, error) {
	if value.Kind == wire.KindNull {
		return nil, nil
	}
	if value.Kind != wire.KindString {
		return nil, fail(CodeMalformedInput, "payload member must be a string or null")
	}
	if err := validate(value.Str); err != nil {
		return nil, err
	}
	return &value.Str, nil
}

func validateProse(value string) error {
	if len(value) > 256<<10 {
		return fail(CodeInputLimit, "prose exceeds byte bound")
	}
	if !utf8.ValidString(value) {
		return fail(CodeMalformedInput, "prose encoding is invalid")
	}
	for _, current := range value {
		if current == '\t' || current == '\n' || current == '\r' {
			continue
		}
		if forbiddenIdentifierRune(current) {
			return fail(CodeHostileInput, "prose contains a forbidden code point")
		}
	}
	return nil
}

func validateLabel(value string) error {
	if len(value) > 32<<10 {
		return fail(CodeInputLimit, "label exceeds byte bound")
	}
	if len(value) == 0 || !utf8.ValidString(value) {
		return fail(CodeMalformedInput, "label length or encoding is invalid")
	}
	for _, current := range value {
		if forbiddenIdentifierRune(current) {
			return fail(CodeHostileInput, "label contains a forbidden code point")
		}
	}
	return nil
}

func checkpointBody(document *CheckpointDocument) map[string]any {
	return map[string]any{"checkpoint": checkpointAny(document.Checkpoint), "policyId": document.PolicyID,
		"profile": document.Profile, "repositorySource": repositorySourceAny(document.RepositorySource), "snapshotId": document.SnapshotID}
}

func RefreshCheckpoint(document *CheckpointDocument) {
	document.Profile = CheckpointProfile
	document.ID = identityAny("work-queue-checkpoint", CheckpointProfile, checkpointBody(document))
}

func (document *CheckpointDocument) Canonical() []byte {
	body := checkpointBody(document)
	body["id"] = document.ID
	return append(canonicalAny(body), '\n')
}

func ParseCheckpoint(raw []byte) (*CheckpointDocument, error) {
	root, err := parseCanonicalDocument(raw, 64<<10)
	if err != nil {
		return nil, err
	}
	object, err := exactObject(root, []string{"checkpoint", "id", "policyId", "profile", "repositorySource", "snapshotId"}, "checkpoint document")
	if err != nil {
		return nil, err
	}
	document := &CheckpointDocument{}
	if document.Checkpoint, err = parseCheckpoint(member(object, "checkpoint")); err != nil {
		return nil, err
	}
	if document.ID, err = contentIDMember(object, "id", "work-queue-checkpoint"); err != nil {
		return nil, err
	}
	if document.PolicyID, err = contentIDMember(object, "policyId", "work-queue-policy"); err != nil {
		return nil, err
	}
	if document.Profile, err = exactStringMember(object, "profile", CheckpointProfile); err != nil {
		return nil, err
	}
	if document.RepositorySource, err = parseRepositorySource(member(object, "repositorySource")); err != nil {
		return nil, err
	}
	if document.SnapshotID, err = contentIDMember(object, "snapshotId", "work-queue-snapshot"); err != nil {
		return nil, err
	}
	if document.RepositorySource.ID != contentIdentity("repository-source", "repository-source/0", repositorySourceValue(document.RepositorySource, false)) {
		return nil, fail(CodeConflicted, "repository source identity does not rederive")
	}
	if document.ID != identityAny("work-queue-checkpoint", CheckpointProfile, checkpointBody(document)) {
		return nil, fail(CodeConflicted, "checkpoint identity does not rederive")
	}
	return document, nil
}

func receiptBody(receipt AdapterReceipt) map[string]any {
	var exit any
	if receipt.ExitCode != nil {
		exit = receipt.ExitCode.String()
	}
	return map[string]any{
		"adapterBlobOid": receipt.AdapterBlobOID, "adapterFileSha256": receipt.AdapterFileSHA256,
		"adapterMode": receipt.AdapterMode, "argv": receipt.Argv, "containmentClass": receipt.ContainmentClass,
		"executableQualification": receipt.ExecutableQualification, "exitCode": exit,
		"interpreterChain": receipt.InterpreterChain, "operation": receipt.Operation,
		"policyId": receipt.PolicyID, "signal": receipt.Signal, "state": receipt.State,
		"stderrBytes": receipt.StderrBytes.String(), "stderrRawSha256": receipt.StderrRawSHA256,
		"stdoutBytes": receipt.StdoutBytes.String(), "stdoutRawSha256": receipt.StdoutRawSHA256,
	}
}

func RefreshReceipt(receipt *AdapterReceipt) {
	receipt.ID = identityAny("adapter-execution", "adapter-execution/0", receiptBody(*receipt))
}

func observationBody(observation *Observation) map[string]any {
	receipts := make([]any, len(observation.AdapterReceipts))
	for index, receipt := range observation.AdapterReceipts {
		body := receiptBody(receipt)
		body["id"] = receipt.ID
		receipts[index] = body
	}
	return map[string]any{
		"adapterReceipts": receipts, "containmentClass": observation.ContainmentClass,
		"detailIds": observation.DetailIDs, "endCheckpoint": checkpointAny(observation.EndCheckpoint),
		"mutationState": observation.MutationState, "networkState": observation.NetworkState,
		"policyId": observation.PolicyID, "profile": observation.Profile,
		"queueSourceId": observation.QueueSourceID, "snapshotId": observation.SnapshotID,
		"startCheckpoint": checkpointAny(observation.StartCheckpoint), "state": observation.State, "unknowns": observation.Unknowns,
	}
}

func RefreshObservation(observation *Observation) {
	observation.Profile = ObservationProfile
	sort.Strings(observation.DetailIDs)
	observation.Unknowns = sortedUnique(observation.Unknowns)
	observation.ID = identityAny("work-queue-observation", ObservationProfile, observationBody(observation))
}

func QueueSourceIdentity(policy *Policy, snapshot *Snapshot) string {
	body := map[string]any{"accessContextId": policy.AccessContextID, "adapterProfile": policy.AdapterProfile,
		"checkpoint": checkpointAny(snapshot.Checkpoint), "complete": snapshot.Scope.Complete, "policyId": policy.ID,
		"repositorySourceId": snapshot.RepositorySource.ID, "scopeId": policy.ScopeID, "snapshotId": snapshot.ID}
	return identityAny("queue-source", "queue-source/0", body)
}

func checkpointAny(checkpoint Checkpoint) map[string]any {
	return map[string]any{"id": checkpoint.ID, "version": checkpoint.Version}
}

func repositorySourceAny(source RepositorySource) map[string]any {
	return map[string]any{"commit": source.Commit, "id": source.ID, "materializationSha256": source.MaterializationSHA256,
		"objectFormat": source.ObjectFormat, "statusSha256": source.StatusSHA256, "tree": source.Tree}
}

func commandBody(result *CommandResult) map[string]any {
	var observation any
	if result.Observation != nil {
		observation = observationBody(result.Observation)
		observation.(map[string]any)["id"] = result.Observation.ID
	}
	var proposal any
	if result.Proposal != nil {
		var decoded any
		_ = json.Unmarshal(bytes.TrimSuffix(result.Proposal.Canonical(), []byte{'\n'}), &decoded)
		proposal = decoded
	}
	return map[string]any{"errorCode": result.ErrorCode, "observation": observation, "profile": result.Profile, "proposal": proposal, "state": result.State}
}

func RefreshCommandResult(result *CommandResult) {
	result.Profile = CommandProfile
	result.ID = identityAny("work-command-result", CommandProfile, commandBody(result))
}

func (result *CommandResult) Canonical() []byte {
	body := commandBody(result)
	body["id"] = result.ID
	return append(canonicalAny(body), '\n')
}

func ErrorCode(err error) string {
	var typed *Error
	if ok := errorAs(err, &typed); ok {
		return typed.Code
	}
	return CodeMalformedInput
}

func errorAs(err error, target **Error) bool {
	for err != nil {
		if typed, ok := err.(*Error); ok {
			*target = typed
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			break
		}
		err = unwrapper.Unwrap()
	}
	return false
}

func ValidateDetailCoverage(snapshot *Snapshot, details *DetailsDocument) ValidationResult {
	result := validation{unknowns: map[string]struct{}{}}
	if snapshot == nil {
		result.conflicted = true
		return result.finish()
	}
	if details == nil {
		result.unknown(UnknownAdapterInvalid)
		return result.finish()
	}
	if details.SnapshotID != snapshot.ID {
		result.conflicted = true
	}
	wanted := map[string]TicketSummary{}
	requested := map[string]struct{}{}
	for _, version := range snapshot.DetailRequestTicketVersionIDs {
		if duplicate(requested, version) {
			result.conflicted = true
		}
	}
	for _, ticket := range snapshot.Tickets {
		if _, ok := requested[ticket.TicketVersionID]; !ok {
			continue
		}
		if _, found := wanted[ticket.TicketVersionID]; found {
			result.conflicted = true
		}
		wanted[ticket.TicketVersionID] = ticket
	}
	for version := range requested {
		if _, ok := wanted[version]; !ok {
			result.unknown(UnknownReference)
		}
	}
	authority, queue, _ := authorityTokens(snapshot.RepositoryAuthorityID, snapshot.QueueAuthorityID)
	seen := map[string]struct{}{}
	for _, detail := range details.Details {
		if detail.RepositoryAuthorityID != snapshot.RepositoryAuthorityID {
			result.unknown(UnknownMultiRepoUnsupported)
		}
		markForeign(detail.TicketID, "ticket", authority, queue, &result)
		ticket, ok := wanted[detail.TicketVersionID]
		if !ok {
			result.conflicted = true
			continue
		}
		// A present but contradictory tuple is not a missing record.
		repeated := duplicate(seen, detail.TicketVersionID)
		if repeated || ticket.TicketID != detail.TicketID || ticket.DetailPayloadSHA256 == nil || (ticket.DetailPayloadSHA256 != nil && *ticket.DetailPayloadSHA256 != detail.PayloadSHA256) {
			result.conflicted = true
		}
	}
	for version := range wanted {
		if _, ok := seen[version]; !ok {
			result.partial = true
			result.unknowns[UnknownDetailMissing] = struct{}{}
		}
	}
	return result.finish()
}

func ValidateCheckpoint(document *CheckpointDocument, policy *Policy, snapshot *Snapshot, source RepositorySource) error {
	if document.PolicyID != policy.ID || document.SnapshotID != snapshot.ID {
		return fmt.Errorf("checkpoint binding mismatch")
	}
	if document.RepositorySource != source {
		return fmt.Errorf("repository source changed")
	}
	return nil
}
