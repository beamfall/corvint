package cli

import (
	"errors"
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
	st, err := intent.Load(repo.IntentRoot())
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
	fill := createTemplateFill
	if st.Policy.MilestoneRequired() {
		fill = []string{"acceptanceCriteria", "body", "milestone", "title"}
	}
	o.Set("fill", wire.Strings(fill))
	o.Set("fields", wire.ObjectValue(fields))
	o.Set("nullableKeys", wire.Strings(nullable))
	o.Set("optionalKeys", wire.Strings([]string{"effects.verdictPaths", "localToken", "requiredRoles", "requiresPool"}))
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
	f.Set("effects", templateField("object {coverage, externalUnbounded, resources, touchPaths} with optional verdictPaths", false, nil, ""))
	f.Set("effects.coverage", templateField("enum", false, ticket.Coverages, "QUALIFIED asserts the declared touchPaths and resources are complete"))
	f.Set("effects.externalUnbounded", templateField("boolean", false, nil, ""))
	f.Set("effects.resources", templateField(fmt.Sprintf("array of {class, key} objects, sorted canonical bytes without duplicates, at most %d", wire.MaxResources), false, nil, ""))
	f.Set("effects.resources[].class", templateField("enum", false, ticket.ResourceClasses, ""))
	f.Set("effects.resources[].key", templateField("Identifier; a repository Path when class is PATH", false, nil, ""))
	f.Set("effects.touchPaths", templateField(set("repository Path", wire.MaxTouchPaths), false, nil, ""))
	f.Set("effects.verdictPaths", templateField("optional nonempty "+set("repository Path", wire.MaxTouchPaths), false, nil, "the cem-verdict predicate domain; every path must lie under a touchPaths key; omitted, the gate governs touchPaths (TCB-V0-035)"))
	f.Set("estimateMinutes", templateField("Count decimal string", true, nil, ""))
	f.Set("executionClass", templateField("enum", false, ticket.ExecutionClasses, ""))
	f.Set("kind", templateField("enum", false, ticket.Kinds, ""))
	f.Set("labels", templateField(set("Label", wire.MaxLabels), false, nil, ""))
	f.Set("localToken", templateField("optional queue-local token string", false, nil, "omit for automatic allocation; collisions refuse"))
	if policy.MilestoneRequired() {
		f.Set("milestone", templateField("Label", false, nil, "required by policy milestones.required (CAL-V0-195)"))
	} else {
		f.Set("milestone", templateField("Label", true, nil, ""))
	}
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

// createOptionalKeys are the top-level CREATE members a payload may omit.
var createOptionalKeys = []string{"localToken", "requiredRoles", "requiresPool"}

// createSkeleton is `ticket create --template --skeleton` (TCT-V0-001): the
// template's complete payload alone as items[0], every closed key present
// with its default, ready to edit and pipe to --payload-stdin.
func createSkeleton(env Env, cmd []string) *wire.Result {
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	st, err := intent.Load(repo.IntentRoot())
	if err != nil {
		return errorResult(cmd, err)
	}
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{mutation.PayloadValue(createTemplatePayload(st.Queue.QueueID.Raw))}}
}

// diagnoseCreatePayload turns a MALFORMED CREATE payload into one MALFORMED
// that names every missing, unknown and invalid top-level key with its
// expected shape (TCT-V0-002). Each present key is checked alone by
// substituting it into the queue's skeleton, so one bad key cannot hide
// another. The closed contract is unchanged: this only explains a refusal
// the codec already made, and any other failure is returned as it was.
func diagnoseCreatePayload(env Env, payload wire.Value, orig error) error {
	var we *wire.Error
	if !errors.As(orig, &we) || we.Code != wire.CodeMalformed || payload.Kind != wire.KindObject || payload.Obj == nil {
		return orig
	}
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return orig
	}
	st, err := intent.Load(repo.IntentRoot())
	if err != nil {
		return orig
	}
	queueID := st.Queue.QueueID.Raw
	fields := createTemplateFields(queueID, st.Policy)
	shape := func(k string) string {
		if v, ok := fields.Get(k); ok {
			t, _ := v.Obj.Get("type")
			s := t.Str
			if n, _ := v.Obj.Get("nullable"); n.Bool {
				s += " or null"
			}
			if vals, ok := v.Obj.Get("values"); ok && len(vals.Arr) > 0 {
				s += " " + string(wire.Encode(vals))
			}
			return s
		}
		return "see ticket create --template"
	}
	skeleton := func() *wire.Object {
		v := mutation.PayloadValue(createTemplatePayload(queueID))
		v.Obj.Set("title", wire.String("x"))
		return v.Obj
	}
	defaults := skeleton()
	required := mutation.PayloadKeys[mutation.OpCreate]
	issues := []string{}
	for _, k := range slices.Sorted(slices.Values(required)) {
		if _, ok := payload.Obj.Get(k); !ok {
			d, _ := defaults.Get(k)
			if k == "title" {
				d = wire.String("")
			}
			issues = append(issues, fmt.Sprintf("missing %s: expected %s; skeleton default %s", k, shape(k), wire.Encode(d)))
		}
	}
	for _, k := range payload.Obj.SortedKeys() {
		if !slices.Contains(required, k) && !slices.Contains(createOptionalKeys, k) {
			issues = append(issues, "unknown key "+k)
			continue
		}
		v, _ := payload.Obj.Get(k)
		copied, err := wire.ParseInput(wire.Encode(v))
		if err != nil {
			continue
		}
		trial := skeleton()
		trial.Set(k, copied)
		if _, err := mutation.CanonicalPayload(mutation.OpCreate, wire.ObjectValue(trial)); err != nil {
			issues = append(issues, fmt.Sprintf("invalid %s (%v): expected %s", k, err, shape(k)))
		}
	}
	if len(issues) == 0 {
		return orig
	}
	return &wire.Error{Code: wire.CodeMalformed, Where: "/payload", Msg: fmt.Sprintf("CREATE payload has %d problem(s): %s; corvint-tasks ticket create --template --skeleton prints a complete payload for this queue", len(issues), strings.Join(issues, "; "))}
}
