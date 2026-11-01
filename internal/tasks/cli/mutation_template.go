package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// mutationTemplateFill is the placeholder prose a caller replaces before
// submitting a non-CREATE template (TCL-V0-611); it decodes, so the
// template is a working payload, and the fill list names where it sits.
const mutationTemplateFill = "REPLACE: why"

// emptySHA256 is the digest of the empty byte string, a well-formed
// placeholder for evidence digests.
const emptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// mutationFieldTable documents every top-level and dotted payload member a
// ticket mutation takes (TCL-V0-610): the CREATE table plus the members
// only other operations carry. op selects the operation-specific spelling
// of a key whose shape differs between operations (reason, evidence,
// operation). A nil policy documents gate and pool values as open.
func mutationFieldTable(op string, policy *intent.Policy) *wire.Object {
	if policy == nil {
		policy = &intent.Policy{}
	}
	f := createTemplateFields("", policy)
	f.Set("executionPrerequisites", templateField(fmt.Sprintf("optional nonempty array of {gateId, obligation, stages, ticketId} objects, sorted canonical bytes, at most %d", wire.MaxDependencies), false, nil, "replaces the record's set; omit it to keep the set (CAL-V0-099)"))
	f.Set("executionPrerequisites[].stages", withValues(templateField("nonempty sorted array of stages", false, nil, ""), "elementValues", ticket.Stages))
	f.Set("holdId", templateField("Label", false, nil, "names the hold; release-hold takes the same id"))
	f.Set("condition", templateField("optional object {kind, ref, commit|path|symbol}", false, ticket.HoldConditionKinds, "values are the closed kinds; ref is a full refname under refs/ (SCC-V0-010)"))
	f.Set("reason", templateField(fmt.Sprintf("prose string, at most %d bytes", wire.MaxProseBytes), false, nil, "fill in"))
	f.Set("evidence", templateField("array of lowercase sha256 hex digests, may be empty", false, nil, ""))
	f.Set("grantId", templateField("Label", false, nil, "revoke-approval takes the same id"))
	f.Set("actor", templateField("Label naming the approved actor id", false, nil, ""))
	f.Set("operation", templateField("enum", false, ticket.ApprovalOps, ""))
	f.Set("targetRevision", templateField("Count decimal string", false, nil, "the ticket revision the grant applies to"))
	f.Set("scope", templateField("array of Identifier", false, nil, ""))
	switch op {
	case mutation.OpAttachEvidence:
		f.Set("evidence", templateField(fmt.Sprintf("array of 1..%d lowercase sha256 hex digests, sorted without duplicates", wire.AttachedEvidenceMaxDigests), false, nil, "fill in"))
		f.Set("reason", templateField(fmt.Sprintf("nonblank prose string, at most %d bytes", wire.AttachedEvidenceMaxReasonBytes), false, nil, "fill in"))
	case mutation.OpProgressSet:
		f.Set("phase", templateField("enum", false, wire.ProgressPhases, "DELIVERED_ON_BRANCH needs branch and head, IN_REVIEW and IN_MERGE_QUEUE need pr, MERGED_AWAITING_CLOSEOUT needs head, BLOCKED needs reason (TLF-V0-880)"))
		f.Set("lane", templateField("Label", true, nil, ""))
		f.Set("host", templateField("Label", true, nil, ""))
		f.Set("branch", templateField("Identifier", true, nil, ""))
		f.Set("head", templateField("Git object id", true, nil, ""))
		f.Set("pr", templateField("Count decimal string", true, nil, ""))
		f.Set("reason", templateField(fmt.Sprintf("nonblank prose string, at most %d bytes", wire.ProgressMaxReasonBytes), true, nil, ""))
	}
	return f
}

// opFields is the part of the mutation field table that op's payload keys
// (and their dotted members) select.
func opFields(op string, policy *intent.Policy) *wire.Object {
	all := mutationFieldTable(op, policy)
	keys := mutationTemplateKeys(op)
	out := wire.NewObject()
	for _, k := range all.SortedKeys() {
		top, _, _ := strings.Cut(k, ".")
		top, _, _ = strings.Cut(top, "[")
		if slices.Contains(keys, top) {
			v, _ := all.Get(k)
			out.Set(k, v)
		}
	}
	return out
}

// mutationTemplateKeys is op's closed payload keys plus its optional ones.
func mutationTemplateKeys(op string) []string {
	keys := slices.Clone(mutation.PayloadKeys[op])
	switch op {
	case mutation.OpCreate:
		keys = append(keys, createOptionalKeys...)
	case mutation.OpHold:
		keys = append(keys, "condition")
	}
	return keys
}

// payloadKeyTypes is the help's key-to-type map for op (TCL-V0-610): the
// type of each top-level payload key, policy-independent so help reads
// nothing.
func payloadKeyTypes(op string) wire.Value {
	fields := mutationFieldTable(op, nil)
	o := wire.NewObject()
	for _, k := range mutationTemplateKeys(op) {
		v, ok := fields.Get(k)
		if !ok {
			continue
		}
		typ, _ := v.Obj.Get("type")
		s := typ.Str
		if vals, ok := v.Obj.Get("values"); ok && typ.Str == "enum" {
			names := make([]string, 0, len(vals.Arr))
			for _, e := range vals.Arr {
				names = append(names, e.Str)
			}
			s = "enum " + strings.Join(names, "|")
		}
		if n, _ := v.Obj.Get("nullable"); n.Bool {
			s += " or null"
		}
		o.Set(k, wire.String(s))
	}
	return wire.ObjectValue(o)
}

// mutationTemplatePayload is a decodable example payload for op: valid
// shapes with neutral values, placeholders where the caller must decide.
func mutationTemplatePayload(op string) wire.Value {
	o := wire.NewObject()
	str := wire.String
	empty := func() wire.Value { return wire.Strings([]string{}) }
	switch op {
	case mutation.OpRefine:
		o.Set("title", str(mutationTemplateFill))
	case mutation.OpPrioritize:
		o.Set("order", str("0"))
		o.Set("priority", str("P2"))
	case mutation.OpSetDependencies:
		o.Set("dependencies", empty())
	case mutation.OpSetGates:
		o.Set("requiredGates", empty())
	case mutation.OpSetEffects:
		eff := wire.NewObject()
		eff.Set("coverage", str("INCOMPLETE"))
		eff.Set("externalUnbounded", wire.Bool(false))
		eff.Set("resources", wire.Array())
		eff.Set("touchPaths", empty())
		o.Set("capabilities", empty())
		o.Set("effects", wire.ObjectValue(eff))
		o.Set("executionClass", str("AUTONOMOUS"))
	case mutation.OpHold:
		o.Set("holdId", str("hold-1"))
		o.Set("reason", str(mutationTemplateFill))
	case mutation.OpReleaseHold:
		o.Set("holdId", str("hold-1"))
	case mutation.OpReopen, mutation.OpArchive, mutation.OpRestore:
		o.Set("reason", str(mutationTemplateFill))
	case mutation.OpCompleteManual:
		o.Set("evidence", empty())
		o.Set("reason", str(mutationTemplateFill))
	case mutation.OpGrantApproval:
		// The grant names the invoking actor (mutation apply refuses any
		// other), read from the environment as the write would read it.
		actor := "OWNER"
		if b, err := initActor(""); err == nil {
			actor = b.ID
		}
		o.Set("actor", str(actor))
		o.Set("grantId", str("grant-1"))
		o.Set("operation", str("COMPLETE"))
		o.Set("scope", empty())
		o.Set("targetRevision", str("0"))
	case mutation.OpRevokeApproval:
		o.Set("grantId", str("grant-1"))
		o.Set("reason", str(mutationTemplateFill))
	case mutation.OpAttachEvidence:
		o.Set("evidence", wire.Strings([]string{emptySHA256}))
		o.Set("reason", str(mutationTemplateFill))
	case mutation.OpProgressSet:
		for _, k := range ticket.ProgressMarkKeys {
			o.Set(k, wire.Null())
		}
		o.Set("phase", str("IN_PROGRESS"))
	}
	return wire.ObjectValue(o)
}

// mutationTemplateFillKeys names the template members a caller decides.
var mutationTemplateFillKeys = map[string][]string{
	mutation.OpRefine:          {"title"},
	mutation.OpHold:            {"holdId", "reason"},
	mutation.OpReleaseHold:     {"holdId"},
	mutation.OpReopen:          {"reason"},
	mutation.OpArchive:         {"reason"},
	mutation.OpRestore:         {"reason"},
	mutation.OpCompleteManual:  {"evidence", "reason"},
	mutation.OpGrantApproval:   {"actor", "grantId", "operation", "scope", "targetRevision"},
	mutation.OpRevokeApproval:  {"grantId", "reason"},
	mutation.OpAttachEvidence:  {"evidence", "reason"},
	mutation.OpProgressSet:     {"phase"},
	mutation.OpPrioritize:      {"order", "priority"},
	mutation.OpSetDependencies: {"dependencies"},
	mutation.OpSetGates:        {"requiredGates"},
	mutation.OpSetEffects:      {"capabilities", "effects", "executionClass"},
}

// mutationTemplate is the read-only `ticket VERB --template` for every
// mutation other than CREATE (TCL-V0-611): a decodable example payload and
// the typed field table for the verb's keys. It reads only the intent store
// (for the queue id and the policy's gate and pool values) and writes
// nothing.
func mutationTemplate(env Env, cmd []string, op string) *wire.Result {
	repo, err := intent.Resolve(env.Cwd)
	if err != nil {
		return errorResult(cmd, err)
	}
	st, err := intent.Load(repo.IntentRoot())
	if err != nil {
		return errorResult(cmd, err)
	}
	payload := mutationTemplatePayload(op)
	o := wire.NewObject()
	o.Set("operation", wire.String(op))
	o.Set("queueId", wire.String(st.Queue.QueueID.Raw))
	o.Set("payload", payload)
	o.Set("payloadCanonical", wire.String(string(wire.Encode(payload))))
	o.Set("fill", wire.Strings(mutationTemplateFillKeys[op]))
	o.Set("fields", wire.ObjectValue(opFields(op, st.Policy)))
	o.Set("usage", wire.String("corvint-tasks "+strings.Join(cmd, " ")+" --request-id ID --target TICKET|LOCAL --expected-revision N|current --payload-stdin < FILE"))
	note := "payload decodes as is; replace the fill members (prose placeholders read " + mutationTemplateFill + "). Nothing was read from the journal, locked or written."
	if op == mutation.OpRefine {
		note = "REFINE takes a non-empty subset of the keys in fields; the example sets only title. " + note
	}
	o.Set("note", wire.String(note))
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeOK, Items: []wire.Value{wire.ObjectValue(o)}}
}
