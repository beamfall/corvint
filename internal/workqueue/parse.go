package workqueue

import (
	"bytes"
	"sort"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

const (
	maxSnapshotBytes = 16 << 20
	maxEnvelopeBytes = 1 << 20
	maxArrayItems    = 10_000
	maxDecodedDepth  = 24
	maxDecodedNodes  = 250_000
	maxTouchPaths    = 256
)

var lifecycleValues = map[string]bool{
	"READY": true, "BLOCKED": true, "HELD": true, "ACTIVE": true,
	"REVIEW": true, "REPAIR": true, "DONE": true, "RETIRED": true, "UNKNOWN": true,
}

func ParseSnapshot(raw []byte) (*Snapshot, error) {
	root, err := parseCanonicalDocument(raw, maxSnapshotBytes)
	if err != nil {
		return nil, err
	}
	fields := []string{
		"accessContextId", "capacityClasses", "checkpoint", "detailRequestTicketVersionIds",
		"id", "leases", "policyId", "profile", "queueAuthorityId", "repositoryAuthorityId",
		"repositorySource", "scope", "tickets",
	}
	value, err := exactObject(root, fields, "snapshot")
	if err != nil {
		return nil, err
	}
	snapshot := &Snapshot{}
	if snapshot.AccessContextID, err = qualifiedMember(value, "accessContextId", "access"); err != nil {
		return nil, err
	}
	if snapshot.ID, err = contentIDMember(value, "id", "work-queue-snapshot"); err != nil {
		return nil, err
	}
	if snapshot.PolicyID, err = contentIDMember(value, "policyId", "work-queue-policy"); err != nil {
		return nil, err
	}
	if snapshot.Profile, err = exactStringMember(value, "profile", SnapshotProfile); err != nil {
		return nil, err
	}
	if snapshot.RepositoryAuthorityID, err = repositoryMember(value, "repositoryAuthorityId"); err != nil {
		return nil, err
	}
	if snapshot.QueueAuthorityID, err = queueMember(value, "queueAuthorityId"); err != nil {
		return nil, err
	}
	if snapshot.RepositorySource, err = parseRepositorySource(member(value, "repositorySource")); err != nil {
		return nil, err
	}
	if snapshot.Checkpoint, err = parseCheckpoint(member(value, "checkpoint")); err != nil {
		return nil, err
	}
	if snapshot.Scope, err = parseScope(member(value, "scope")); err != nil {
		return nil, err
	}
	if snapshot.CapacityClasses, err = parseCapacityClasses(member(value, "capacityClasses")); err != nil {
		return nil, err
	}
	if snapshot.DetailRequestTicketVersionIDs, err = parseSortedStrings(member(value, "detailRequestTicketVersionIds"), maxArrayItems, contentIDValidator("ticket-version"), "detailRequestTicketVersionIds"); err != nil {
		return nil, err
	}
	if snapshot.Leases, err = parseLeases(member(value, "leases")); err != nil {
		return nil, err
	}
	if snapshot.Tickets, err = parseTickets(member(value, "tickets")); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func ParseEnvelope(raw []byte) (*CapacityEnvelope, error) {
	root, err := parseCanonicalDocument(raw, maxEnvelopeBytes)
	if err != nil {
		return nil, err
	}
	value, err := exactObject(root, []string{"available", "capabilities", "id", "profile", "repositoryAuthorityId"}, "capacity envelope")
	if err != nil {
		return nil, err
	}
	envelope := &CapacityEnvelope{}
	if envelope.Available, err = parseCapacityClasses(member(value, "available")); err != nil {
		return nil, err
	}
	if envelope.Capabilities, err = parseSortedStrings(member(value, "capabilities"), maxArrayItems, qualifiedValidator("capability"), "capabilities"); err != nil {
		return nil, err
	}
	if envelope.ID, err = contentIDMember(value, "id", "work-capacity-envelope"); err != nil {
		return nil, err
	}
	if envelope.Profile, err = exactStringMember(value, "profile", EnvelopeProfile); err != nil {
		return nil, err
	}
	if envelope.RepositoryAuthorityID, err = repositoryMember(value, "repositoryAuthorityId"); err != nil {
		return nil, err
	}
	want := contentIdentity("work-capacity-envelope", EnvelopeProfile, envelopeValue(envelope, false))
	if envelope.ID != want {
		return nil, fail(CodeConflicted, "capacity envelope identity does not rederive")
	}
	return envelope, nil
}

func parseCanonicalDocument(raw []byte, byteLimit int) (wire.Value, error) {
	if len(raw) > byteLimit {
		return wire.Value{}, fail(CodeInputLimit, "document exceeds %d bytes", byteLimit)
	}
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return wire.Value{}, fail(CodeMalformedInput, "document must end in exactly one LF")
	}
	body := raw[:len(raw)-1]
	if len(body) > 0 && body[len(body)-1] == '\n' {
		return wire.Value{}, fail(CodeMalformedInput, "document has extra framing whitespace")
	}
	value, err := wire.Parse(body)
	if err != nil {
		return wire.Value{}, fail(CodeMalformedInput, "invalid JSON")
	}
	if value.Kind != wire.KindObject {
		return wire.Value{}, fail(CodeMalformedInput, "document root must be an object")
	}
	if !bytes.Equal(body, wire.CanonicalValue(value)) {
		return wire.Value{}, fail(CodeMalformedInput, "document is not canonical JSON")
	}
	depth, nodes, arraysOK := valueBounds(value)
	if depth > maxDecodedDepth || nodes > maxDecodedNodes || !arraysOK {
		return wire.Value{}, fail(CodeInputLimit, "decoded document exceeds a structural bound")
	}
	return value, nil
}

func valueBounds(value wire.Value) (int, int, bool) {
	depth := 1
	nodes := 1
	arraysOK := true
	children := value.Arr
	if value.Kind == wire.KindObject {
		children = make([]wire.Value, 0, len(value.Obj.Keys))
		for _, key := range value.Obj.Keys {
			children = append(children, value.Obj.Values[key])
		}
	}
	if value.Kind == wire.KindArray && len(value.Arr) > maxArrayItems {
		arraysOK = false
	}
	for _, child := range children {
		childDepth, childNodes, childArraysOK := valueBounds(child)
		if childDepth+1 > depth {
			depth = childDepth + 1
		}
		nodes += childNodes
		arraysOK = arraysOK && childArraysOK
	}
	return depth, nodes, arraysOK
}

func exactObject(value wire.Value, fields []string, subject string) (*wire.Object, error) {
	if value.Kind != wire.KindObject {
		return nil, fail(CodeMalformedInput, "%s must be an object", subject)
	}
	expected := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		expected[field] = struct{}{}
		if _, ok := value.Obj.Get(field); !ok {
			return nil, fail(CodeMalformedInput, "%s is missing %s", subject, field)
		}
	}
	for _, key := range value.Obj.Keys {
		if _, ok := expected[key]; !ok {
			return nil, fail(CodeMalformedInput, "%s has an unknown key", subject)
		}
	}
	return value.Obj, nil
}

func member(object *wire.Object, name string) wire.Value {
	value, _ := object.Get(name)
	return value
}

func stringMember(object *wire.Object, name string) (string, error) {
	value := member(object, name)
	if value.Kind != wire.KindString {
		return "", fail(CodeMalformedInput, "%s must be a string", name)
	}
	return value.Str, nil
}

func identifierMember(object *wire.Object, name string) (string, error) {
	value, err := stringMember(object, name)
	if err != nil {
		return "", err
	}
	if err := ValidateIdentifier(value); err != nil {
		return "", err
	}
	return value, nil
}

func exactStringMember(object *wire.Object, name, expected string) (string, error) {
	value, err := stringMember(object, name)
	if err != nil || value != expected {
		return "", fail(CodeMalformedInput, "%s must be %q", name, expected)
	}
	return value, nil
}

func contentIDMember(object *wire.Object, name, kind string) (string, error) {
	value, err := identifierMember(object, name)
	if err != nil {
		return "", err
	}
	if !validContentID(value, kind) {
		return "", fail(CodeMalformedInput, "%s must be a %s content ID", name, kind)
	}
	return value, nil
}

func repositoryMember(object *wire.Object, name string) (string, error) {
	value, err := identifierMember(object, name)
	if err != nil {
		return "", err
	}
	if _, valid := splitRepositoryID(value); !valid {
		return "", fail(CodeMalformedInput, "%s must be a repository authority ID", name)
	}
	return value, nil
}

func queueMember(object *wire.Object, name string) (string, error) {
	value, err := identifierMember(object, name)
	if err != nil {
		return "", err
	}
	if _, _, valid := splitQueueID(value); !valid {
		return "", fail(CodeMalformedInput, "%s must be a queue authority ID", name)
	}
	return value, nil
}

func qualifiedMember(object *wire.Object, name, kind string) (string, error) {
	value, err := identifierMember(object, name)
	if err != nil {
		return "", err
	}
	if _, _, _, valid := splitQualifiedID(value, kind); !valid {
		return "", fail(CodeMalformedInput, "%s must be a %s ID", name, kind)
	}
	return value, nil
}

func countMember(object *wire.Object, name string, positive bool) (Count, error) {
	value, err := stringMember(object, name)
	if err != nil {
		return 0, err
	}
	count, err := ParseCount(value)
	if err != nil || (positive && count == 0) {
		return 0, fail(CodeMalformedInput, "%s must be a valid Count", name)
	}
	return count, nil
}

func rankMember(object *wire.Object, name string) (Rank, error) {
	value, err := stringMember(object, name)
	if err != nil {
		return 0, err
	}
	rank, err := ParseRank(value)
	if err != nil {
		return 0, fail(CodeMalformedInput, "%s must be a valid Rank", name)
	}
	return rank, nil
}

func boolMember(object *wire.Object, name string) (bool, error) {
	value := member(object, name)
	if value.Kind != wire.KindBool {
		return false, fail(CodeMalformedInput, "%s must be a boolean", name)
	}
	return value.Bool, nil
}

func parseRepositorySource(value wire.Value) (RepositorySource, error) {
	object, err := exactObject(value, []string{"commit", "id", "materializationSha256", "objectFormat", "statusSha256", "tree"}, "repositorySource")
	if err != nil {
		return RepositorySource{}, err
	}
	result := RepositorySource{}
	if result.Commit, err = stringMember(object, "commit"); err != nil {
		return result, err
	}
	if result.ID, err = contentIDMember(object, "id", "repository-source"); err != nil {
		return result, err
	}
	if result.MaterializationSHA256, err = digestMember(object, "materializationSha256"); err != nil {
		return result, err
	}
	if result.ObjectFormat, err = stringMember(object, "objectFormat"); err != nil {
		return result, err
	}
	if result.ObjectFormat != "sha1" && result.ObjectFormat != "sha256" {
		return result, fail(CodeMalformedInput, "objectFormat is unsupported")
	}
	if result.StatusSHA256, err = digestMember(object, "statusSha256"); err != nil {
		return result, err
	}
	if result.Tree, err = stringMember(object, "tree"); err != nil {
		return result, err
	}
	oidLength := 40
	if result.ObjectFormat == "sha256" {
		oidLength = 64
	}
	if !validOID(result.Commit, oidLength) || !validOID(result.Tree, oidLength) {
		return result, fail(CodeMalformedInput, "repository source OID is invalid")
	}
	return result, nil
}

func validOID(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, current := range value {
		if (current < '0' || current > '9') && (current < 'a' || current > 'f') {
			return false
		}
	}
	return true
}

func digestMember(object *wire.Object, name string) (string, error) {
	value, err := stringMember(object, name)
	if err != nil || !validDigest(value) {
		return "", fail(CodeMalformedInput, "%s must be a SHA-256 digest", name)
	}
	return value, nil
}

func parseCheckpoint(value wire.Value) (Checkpoint, error) {
	object, err := exactObject(value, []string{"id", "version"}, "checkpoint")
	if err != nil {
		return Checkpoint{}, err
	}
	result := Checkpoint{}
	if result.ID, err = qualifiedMember(object, "id", "checkpoint"); err != nil {
		return result, err
	}
	if result.Version, err = identifierMember(object, "version"); err != nil {
		return result, err
	}
	return result, nil
}

func parseScope(value wire.Value) (Scope, error) {
	object, err := exactObject(value, []string{"complete", "id", "ticketCount"}, "scope")
	if err != nil {
		return Scope{}, err
	}
	result := Scope{}
	if result.Complete, err = boolMember(object, "complete"); err != nil {
		return result, err
	}
	if result.ID, err = qualifiedMember(object, "id", "scope"); err != nil {
		return result, err
	}
	if result.TicketCount, err = countMember(object, "ticketCount", false); err != nil {
		return result, err
	}
	return result, nil
}

func parseCapacityClasses(value wire.Value) ([]CapacityClass, error) {
	return parseSortedArray(value, maxArrayItems, "capacity classes", func(item wire.Value) (CapacityClass, error) {
		object, err := exactObject(item, []string{"availableUnits", "id"}, "capacity class")
		if err != nil {
			return CapacityClass{}, err
		}
		result := CapacityClass{}
		if result.AvailableUnits, err = countMember(object, "availableUnits", false); err != nil {
			return result, err
		}
		if result.ID, err = qualifiedMember(object, "id", "capacity"); err != nil {
			return result, err
		}
		return result, nil
	})
}

func parseCapacityUses(value wire.Value) ([]CapacityUse, error) {
	return parseSortedArray(value, maxArrayItems, "capacity uses", func(item wire.Value) (CapacityUse, error) {
		object, err := exactObject(item, []string{"classId", "units"}, "capacity use")
		if err != nil {
			return CapacityUse{}, err
		}
		result := CapacityUse{}
		if result.ClassID, err = qualifiedMember(object, "classId", "capacity"); err != nil {
			return result, err
		}
		if result.Units, err = countMember(object, "units", true); err != nil {
			return result, err
		}
		return result, nil
	})
}

func parseRoutes(value wire.Value) ([]RouteAlternative, error) {
	return parseSortedArray(value, maxArrayItems, "route alternatives", func(item wire.Value) (RouteAlternative, error) {
		object, err := exactObject(item, []string{"id", "requires"}, "route alternative")
		if err != nil {
			return RouteAlternative{}, err
		}
		result := RouteAlternative{}
		if result.ID, err = qualifiedMember(object, "id", "route"); err != nil {
			return result, err
		}
		if result.Requires, err = parseSortedStrings(member(object, "requires"), maxArrayItems, qualifiedValidator("capability"), "route requirements"); err != nil {
			return result, err
		}
		return result, nil
	})
}

func parseSelectionFacts(value wire.Value) (SelectionFacts, error) {
	object, err := exactObject(value, []string{"approvals", "dependencies", "holds", "lease"}, "selection facts")
	if err != nil {
		return SelectionFacts{}, err
	}
	result := SelectionFacts{}
	if result.Approvals, err = enumMember(object, "approvals", "CLEAR", "BLOCKED", "UNKNOWN"); err != nil {
		return result, err
	}
	if result.Dependencies, err = enumMember(object, "dependencies", "SATISFIED", "BLOCKED", "UNKNOWN"); err != nil {
		return result, err
	}
	if result.Holds, err = enumMember(object, "holds", "CLEAR", "HELD", "UNKNOWN"); err != nil {
		return result, err
	}
	if result.Lease, err = enumMember(object, "lease", "ABSENT", "PRESENT", "UNKNOWN"); err != nil {
		return result, err
	}
	return result, nil
}

func enumMember(object *wire.Object, name string, allowed ...string) (string, error) {
	value, err := stringMember(object, name)
	if err != nil {
		return "", err
	}
	for _, candidate := range allowed {
		if value == candidate {
			return value, nil
		}
	}
	return "", fail(CodeMalformedInput, "%s has an unknown enum value", name)
}

func parseTickets(value wire.Value) ([]TicketSummary, error) {
	if value.Kind != wire.KindArray {
		return nil, fail(CodeMalformedInput, "tickets must be an array")
	}
	if len(value.Arr) > maxArrayItems {
		return nil, fail(CodeInputLimit, "tickets must contain at most %d entries", maxArrayItems)
	}
	result := make([]TicketSummary, 0, len(value.Arr))
	for _, item := range value.Arr {
		ticket, err := parseTicket(item)
		if err != nil {
			return nil, err
		}
		result = append(result, ticket)
	}
	for index := 1; index < len(result); index++ {
		previous := result[index-1]
		current := result[index]
		if current.Rank < previous.Rank {
			return nil, fail(CodeMalformedInput, "tickets are not in rank order")
		}
		if current.Rank == previous.Rank && current.TicketVersionID < previous.TicketVersionID {
			return nil, fail(CodeMalformedInput, "tickets are not in version digest order")
		}
	}
	return result, nil
}

func parseTicket(value wire.Value) (TicketSummary, error) {
	fields := []string{
		"atomicRepositoryAuthorityIds", "authority", "capacityUses", "collisionGroupIds",
		"declaredVersion", "dependencyTicketIds", "detailPayloadSha256", "lifecycle",
		"queueAuthorityId", "rank", "repositoryAuthorityId", "routeAlternatives",
		"selectionFacts", "ticketContentSha256", "ticketId", "ticketVersionId", "touchPaths",
	}
	object, err := exactObject(value, fields, "ticket")
	if err != nil {
		return TicketSummary{}, err
	}
	result := TicketSummary{}
	if result.AtomicRepositoryAuthorityIDs, err = parseSortedStrings(member(object, "atomicRepositoryAuthorityIds"), maxArrayItems, repositoryValidator, "atomic repository authorities"); err != nil {
		return result, err
	}
	if result.Authority, err = enumMember(object, "authority", "COMPLETE", "UNKNOWN"); err != nil {
		return result, err
	}
	if result.CapacityUses, err = parseCapacityUses(member(object, "capacityUses")); err != nil {
		return result, err
	}
	if result.CollisionGroupIDs, err = parseSortedStrings(member(object, "collisionGroupIds"), maxArrayItems, qualifiedValidator("collision"), "collision group IDs"); err != nil {
		return result, err
	}
	if result.DeclaredVersion, err = identifierMember(object, "declaredVersion"); err != nil {
		return result, err
	}
	if result.DependencyTicketIDs, err = parseSortedStrings(member(object, "dependencyTicketIds"), maxArrayItems, qualifiedValidator("ticket"), "dependency ticket IDs"); err != nil {
		return result, err
	}
	detail := member(object, "detailPayloadSha256")
	if detail.Kind == wire.KindString && validDigest(detail.Str) {
		result.DetailPayloadSHA256 = &detail.Str
	} else if detail.Kind != wire.KindNull {
		return result, fail(CodeMalformedInput, "detailPayloadSha256 must be a digest or null")
	}
	if result.Lifecycle, err = stringMember(object, "lifecycle"); err != nil || !lifecycleValues[result.Lifecycle] {
		return result, fail(CodeMalformedInput, "lifecycle has an unknown enum value")
	}
	if result.QueueAuthorityID, err = queueMember(object, "queueAuthorityId"); err != nil {
		return result, err
	}
	if result.Rank, err = rankMember(object, "rank"); err != nil {
		return result, err
	}
	if result.RepositoryAuthorityID, err = repositoryMember(object, "repositoryAuthorityId"); err != nil {
		return result, err
	}
	if result.RouteAlternatives, err = parseRoutes(member(object, "routeAlternatives")); err != nil {
		return result, err
	}
	if result.SelectionFacts, err = parseSelectionFacts(member(object, "selectionFacts")); err != nil {
		return result, err
	}
	if result.TicketContentSHA256, err = digestMember(object, "ticketContentSha256"); err != nil {
		return result, err
	}
	if result.TicketID, err = qualifiedMember(object, "ticketId", "ticket"); err != nil {
		return result, err
	}
	if result.TicketVersionID, err = contentIDMember(object, "ticketVersionId", "ticket-version"); err != nil {
		return result, err
	}
	if result.TouchPaths, err = parseSortedStrings(member(object, "touchPaths"), maxTouchPaths, ValidatePath, "touch paths"); err != nil {
		return result, err
	}
	return result, nil
}

func parseLeases(value wire.Value) ([]LeaseSummary, error) {
	return parseSortedArray(value, maxArrayItems, "leases", func(item wire.Value) (LeaseSummary, error) {
		fields := []string{
			"blocksSelection", "capacityUses", "collisionGroupIds", "holderId", "leaseId",
			"leaseVersionId", "lifecycle", "queueAuthorityId", "repositoryAuthorityId",
			"ticketId", "ticketVersionId",
		}
		object, err := exactObject(item, fields, "lease")
		if err != nil {
			return LeaseSummary{}, err
		}
		result := LeaseSummary{}
		if result.BlocksSelection, err = boolMember(object, "blocksSelection"); err != nil {
			return result, err
		}
		if result.CapacityUses, err = parseCapacityUses(member(object, "capacityUses")); err != nil {
			return result, err
		}
		if result.CollisionGroupIDs, err = parseSortedStrings(member(object, "collisionGroupIds"), maxArrayItems, qualifiedValidator("collision"), "lease collision group IDs"); err != nil {
			return result, err
		}
		if result.HolderID, err = qualifiedMember(object, "holderId", "holder"); err != nil {
			return result, err
		}
		if result.LeaseID, err = qualifiedMember(object, "leaseId", "lease"); err != nil {
			return result, err
		}
		if result.LeaseVersionID, err = contentIDMember(object, "leaseVersionId", "lease-version"); err != nil {
			return result, err
		}
		if result.Lifecycle, err = stringMember(object, "lifecycle"); err != nil || !lifecycleValues[result.Lifecycle] {
			return result, fail(CodeMalformedInput, "lease lifecycle has an unknown enum value")
		}
		if result.QueueAuthorityID, err = queueMember(object, "queueAuthorityId"); err != nil {
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
	})
}

func parseSortedArray[T any](value wire.Value, limit int, subject string, parse func(wire.Value) (T, error)) ([]T, error) {
	if value.Kind != wire.KindArray {
		return nil, fail(CodeMalformedInput, "%s must be an array", subject)
	}
	if len(value.Arr) > limit {
		return nil, fail(CodeInputLimit, "%s exceeds its item bound", subject)
	}
	result := make([]T, 0, len(value.Arr))
	previous := []byte(nil)
	for _, item := range value.Arr {
		canonical := wire.CanonicalValue(item)
		if previous != nil && bytes.Compare(previous, canonical) >= 0 {
			return nil, fail(CodeMalformedInput, "%s must be canonical-sorted and unique", subject)
		}
		parsed, err := parse(item)
		if err != nil {
			return nil, err
		}
		result = append(result, parsed)
		previous = canonical
	}
	return result, nil
}

func parseSortedStrings(value wire.Value, limit int, validate func(string) error, subject string) ([]string, error) {
	return parseSortedArray(value, limit, subject, func(item wire.Value) (string, error) {
		if item.Kind != wire.KindString {
			return "", fail(CodeMalformedInput, "%s contains an invalid value", subject)
		}
		if err := validate(item.Str); err != nil {
			return "", err
		}
		return item.Str, nil
	})
}

func repositoryValidator(value string) error {
	if err := ValidateIdentifier(value); err != nil {
		return err
	}
	if _, valid := splitRepositoryID(value); !valid {
		return fail(CodeMalformedInput, "invalid repository authority ID")
	}
	return nil
}

func contentIDValidator(kind string) func(string) error {
	return func(value string) error {
		if err := ValidateIdentifier(value); err != nil {
			return err
		}
		if !validContentID(value, kind) {
			return fail(CodeMalformedInput, "invalid content ID")
		}
		return nil
	}
}

func qualifiedValidator(kind string) func(string) error {
	return func(value string) error {
		if err := ValidateIdentifier(value); err != nil {
			return err
		}
		if _, _, _, valid := splitQualifiedID(value, kind); !valid {
			return fail(CodeMalformedInput, "invalid qualified ID")
		}
		return nil
	}
}

func canonicalSort[T any](values []T, project func(T) wire.Value) {
	sort.Slice(values, func(left, right int) bool {
		return bytes.Compare(wire.CanonicalValue(project(values[left])), wire.CanonicalValue(project(values[right]))) < 0
	})
}

func authorityTokens(repositoryID, queueID string) (string, string, bool) {
	repositoryAuthority, repositoryOK := splitRepositoryID(repositoryID)
	queueAuthority, queue, queueOK := splitQueueID(queueID)
	return repositoryAuthority, queue, repositoryOK && queueOK && repositoryAuthority == queueAuthority
}

func validateQualifiedAuthority(value, kind, authority, queue string) (foreign bool, malformed bool) {
	same, valid := sameAuthority(value, kind, authority, queue)
	if !valid {
		return false, true
	}
	return !same, false
}
