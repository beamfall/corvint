package cli

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// createTemplateFill names the payload members a caller replaces before
// submitting the template (CAL-V0-073).
var createTemplateFill = []string{"acceptanceCriteria", "body", "title"}

// createTemplate is the read-only `ticket create --template` (CAL-V0-073):
// a canonical CREATE payload for the current queue plus a field table that
// names every type, enum and nullable key. It reads only the intent store:
// no journal, lock, stdin or write.
func createTemplate(env Env, cmd []string) *wire.Result {
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	st, err := intent.Load(repo.PrimaryWorktree)
	if err != nil {
		return errorResult(cmd, err)
	}
	queueID := st.Queue.QueueID.Raw
	payload := mutation.PayloadValue(createTemplatePayload(queueID))
	fields := createTemplateFields(queueID, st.Policy)
	nullable := []string{}
	for _, k := range fields.SortedKeys() {
		v, _ := fields.Get(k)
		if n, _ := v.Obj.Get("nullable"); n.Bool {
			nullable = append(nullable, k)
		}
	}
	o := wire.NewObject()
	o.Set("operation", wire.String(mutation.OpCreate))
	o.Set("queueId", wire.String(queueID))
	o.Set("payload", payload)
	o.Set("payloadCanonical", wire.String(string(wire.Encode(payload))))
	o.Set("fill", wire.Strings(createTemplateFill))
	o.Set("fields", wire.ObjectValue(fields))
	o.Set("nullableKeys", wire.Strings(nullable))
	o.Set("optionalKeys", wire.Strings([]string{"localToken", "requiredRoles", "requiresPool"}))
	o.Set("usage", wire.String("corvint-tasks ticket create --request-id ID --payload-stdin < FILE"))
	o.Set("note", wire.String("payload is canonical and valid for this queue except title, which is empty so an unedited template refuses. Replace title, body and acceptanceCriteria (an empty acceptanceCriteria leaves the ticket DRAFT), keep sorted keys and compact JSON, e.g. jq -c '.items[0].payload | .title=\"...\" | .body=\"...\" | .acceptanceCriteria=[\"...\"]'. Nothing was read from the journal, locked or written."))
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}}
}

// createTemplatePayload renders through the CREATE codec's own encoder, so
// the template's key set cannot drift from the closed payload.
func createTemplatePayload(queueID string) *mutation.CreatePayload {
	body := ""
	return &mutation.CreatePayload{
		Title:              "",
		Body:               &body,
		Kind:               "FEATURE",
		Priority:           "P2",
		Order:              "0",
		Labels:             []string{},
		Dependencies:       []ticket.Dependency{},
		AcceptanceCriteria: []string{},
		RequirementRefs:    []string{},
		Source:             ticket.Source{Kind: "NATIVE", SourceQueueID: queueID},
		Effects:            ticket.Effects{Coverage: "INCOMPLETE", TouchPaths: []string{}, Resources: []ticket.Resource{}},
		Capabilities:       []string{},
		RequiredGates:      []string{},
		ExecutionClass:     "AUTONOMOUS",
	}
}

func templateField(typ string, nullable bool, values []string, note string) wire.Value {
	o := wire.NewObject()
	o.Set("type", wire.String(typ))
	o.Set("nullable", wire.Bool(nullable))
	if values != nil {
		o.Set("values", wire.Strings(slices.Clone(values)))
	}
	if note != "" {
		o.Set("note", wire.String(note))
	}
	return wire.ObjectValue(o)
}

// withValues adds a non-enum value list under key: elementValues is the
// closed set each array element is drawn from, current is this queue's
// value of an open field. values always means a closed enum for the key.
func withValues(v wire.Value, key string, vals []string) wire.Value {
	v.Obj.Set(key, wire.Strings(slices.Clone(vals)))
	return v
}

// createTemplateFields documents each CREATE member by dotted path. Enum
// values come from the vocabularies the decoder enforces; gate and pool
// values come from the current policy.
func createTemplateFields(queueID string, policy *intent.Policy) *wire.Object {
	set := func(item string, max int) string {
		return fmt.Sprintf("array of %s, sorted canonical bytes without duplicates, at most %d", item, max)
	}
	gates := slices.Sorted(maps.Keys(policy.GateIDs()))
	pools := []string{}
	for _, p := range policy.Pools {
		pools = append(pools, p.ID)
	}
	slices.Sort(pools)
	f := wire.NewObject()
	f.Set("acceptanceCriteria", templateField(fmt.Sprintf("ordered array of prose strings (each at most %d bytes), at most %d", wire.MaxCriterionBytes, wire.MaxAcceptanceCriteria), false, nil, "fill in; empty leaves the ticket DRAFT"))
	f.Set("body", templateField(fmt.Sprintf("prose string, at most %d bytes", wire.MaxBodyBytes), true, nil, "fill in"))
	f.Set("capabilities", templateField(set("Identifier", wire.MaxCapabilities), false, nil, ""))
	f.Set("dependencies", templateField(fmt.Sprintf("ordered array of {gateId, obligation, ticketId} objects, at most %d", wire.MaxDependencies), false, nil, "each ticketId must exist in the queue"))
	f.Set("dependencies[].gateId", templateField("Label naming a policy gate", true, gates, "null iff obligation is COMPLETED; a gate id iff GATE_PASSED"))
	f.Set("dependencies[].obligation", templateField("enum", false, ticket.Obligations, ""))
	f.Set("dependencies[].ticketId", templateField("TicketID ticket:AUTHORITY:QUEUE:LOCAL", false, nil, ""))
	f.Set("dueDate", templateField("date YYYY-MM-DD", true, nil, ""))
	f.Set("effects", templateField("object {coverage, externalUnbounded, resources, touchPaths}", false, nil, ""))
	f.Set("effects.coverage", templateField("enum", false, ticket.Coverages, "QUALIFIED asserts the declared touchPaths and resources are complete"))
	f.Set("effects.externalUnbounded", templateField("boolean", false, nil, ""))
	f.Set("effects.resources", templateField(fmt.Sprintf("array of {class, key} objects, sorted canonical bytes without duplicates, at most %d", wire.MaxResources), false, nil, ""))
	f.Set("effects.resources[].class", templateField("enum", false, ticket.ResourceClasses, ""))
	f.Set("effects.resources[].key", templateField("Identifier; a repository Path when class is PATH", false, nil, ""))
	f.Set("effects.touchPaths", templateField(set("repository Path", wire.MaxTouchPaths), false, nil, ""))
	f.Set("estimateMinutes", templateField("Count decimal string", true, nil, ""))
	f.Set("executionClass", templateField("enum", false, ticket.ExecutionClasses, ""))
	f.Set("kind", templateField("enum", false, ticket.Kinds, ""))
	f.Set("labels", templateField(set("Label", wire.MaxLabels), false, nil, ""))
	f.Set("localToken", templateField("optional queue-local token string", false, nil, "omit for automatic allocation; collisions refuse"))
	f.Set("milestone", templateField("Label", true, nil, ""))
	f.Set("order", templateField("Count decimal string", false, nil, "rank within priority; a string such as \"0\", not a number"))
	f.Set("owner", templateField("Label", true, nil, ""))
	f.Set("priority", templateField("enum", false, ticket.Priorities, ""))
	f.Set("requiredGates", withValues(templateField(set("Label naming a policy gate", wire.MaxRequiredGates), false, nil, ""), "elementValues", gates))
	f.Set("requiredRoles", withValues(templateField("optional object {implement, review, integrate}, each a non-empty sorted array of roles", false, nil, "keys: "+strings.Join(ticket.Stages, ", ")), "elementValues", ticket.StageRoles))
	f.Set("requirementRefs", templateField(set("Identifier", wire.MaxRequirementRefs), false, nil, ""))
	f.Set("requiresPool", templateField("optional Label naming a policy pool", false, pools, ""))
	f.Set("source", templateField("object {kind, sourceItemId, sourceQueueId, sourceRevisionSha256}", false, nil, ""))
	f.Set("source.kind", templateField("enum", false, ticket.SourceKinds, "NATIVE for tickets created here"))
	f.Set("source.sourceItemId", templateField("Identifier", true, nil, ""))
	f.Set("source.sourceQueueId", withValues(templateField("Identifier", false, nil, "this queue's id for NATIVE"), "current", []string{queueID}))
	f.Set("source.sourceRevisionSha256", templateField("lowercase sha256 hex digest", true, nil, ""))
	f.Set("supersededBy", templateField("TicketID of an existing ticket", true, nil, ""))
	f.Set("supersedes", templateField("TicketID of an existing ticket", true, nil, ""))
	f.Set("title", templateField(fmt.Sprintf("prose string, 1..%d bytes", wire.MaxTitleBytes), false, nil, "fill in; empty in the template so an unedited submit refuses"))
	return f
}
