package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// mutationVerbs maps each non-CREATE mutation verb to its operation, in an
// order one ticket can take them in sequence.
var mutationVerbs = []struct{ verb, op string }{
	{"prioritize", mutation.OpPrioritize},
	{"set-dependencies", mutation.OpSetDependencies},
	{"set-gates", mutation.OpSetGates},
	{"set-effects", mutation.OpSetEffects},
	{"attach-evidence", mutation.OpAttachEvidence},
	{"progress", mutation.OpProgressSet},
	{"hold", mutation.OpHold},
	{"release-hold", mutation.OpReleaseHold},
	{"grant-approval", mutation.OpGrantApproval},
	{"revoke-approval", mutation.OpRevokeApproval},
	{"refine", mutation.OpRefine},
	{"complete-manual", mutation.OpCompleteManual},
	{"reopen", mutation.OpReopen},
	{"archive", mutation.OpArchive},
	{"restore", mutation.OpRestore},
}

// TestTCLV0610_HelpNamesPayloadKeyTypes: every mutation verb's help maps
// each closed payload key to a nonempty type string.
func TestTCLV0610_HelpNamesPayloadKeyTypes(t *testing.T) {
	r, _, _ := papercutRepo(t)
	for _, m := range mutationVerbs {
		help := atm(t, r.Root, nil, "ticket", m.verb, "--help", "--verbose")
		if help.res.Outcome != wire.OutcomeOK {
			t.Fatalf("%s --help: %s", m.verb, help.stdout)
		}
		types := field(help.res.Items[0], "payloadKeyTypes")
		for _, k := range mutation.PayloadKeys[m.op] {
			if v, ok := types.Obj.Get(k); !ok || v.Str == "" {
				t.Errorf("ticket %s help: payloadKeyTypes lacks %q: %s", m.verb, k, wire.Encode(types))
			}
		}
	}
}

// TestTCLV0611_EveryMutationTemplateRuns: each verb's --template is a
// read-only OK result whose payload, submitted through the verb's derived
// help example with its placeholders filled, is accepted (V1-3714 AC2,
// V1-3738 AC1).
func TestTCLV0611_EveryMutationTemplateRuns(t *testing.T) {
	r, local, _ := papercutRepo(t)
	var out bytes.Buffer
	if code := cli.Run(cli.Env{Cwd: r.Root, Args: []string{"help", "--json"}, Stdout: &out, Stderr: &out}); code != 0 {
		t.Fatalf("help --json: %d %s", code, out.Bytes())
	}
	var tree any
	if err := json.Unmarshal(out.Bytes(), &tree); err != nil {
		t.Fatal(err)
	}
	ex := map[string]string{}
	examples(tree, ex)
	for i, m := range mutationVerbs {
		tpl := atm(t, r.Root, nil, "ticket", m.verb, "--template")
		if tpl.res.Outcome != wire.OutcomeOK || len(tpl.res.Items) != 1 {
			t.Fatalf("ticket %s --template: %s", m.verb, tpl.stdout)
		}
		item := tpl.res.Items[0]
		if got := field(item, "operation").Str; got != m.op {
			t.Errorf("ticket %s --template operation = %q", m.verb, got)
		}
		payload := field(item, "payloadCanonical").Str
		if payload != string(wire.Encode(field(item, "payload"))) {
			t.Errorf("ticket %s --template payloadCanonical is not the canonical payload", m.verb)
		}
		if m.op == mutation.OpGrantApproval {
			// targetRevision is ticket state the template cannot know.
			show := atm(t, r.Root, nil, "ticket", "show", local)
			rev := field(field(show.res.Items[0], "record"), "acceptanceRevision").Str
			if rev == "" {
				t.Fatalf("ticket show %s: no record.acceptanceRevision: %s", local, show.stdout)
			}
			payload = strings.Replace(payload, `"targetRevision":"0"`, `"targetRevision":"`+rev+`"`, 1)
		}
		fields := field(item, "fields")
		for _, k := range mutation.PayloadKeys[m.op] {
			if _, ok := fields.Obj.Get(k); !ok {
				t.Errorf("ticket %s --template fields lacks %q", m.verb, k)
			}
		}
		example, ok := ex[m.verb]
		if !ok {
			t.Errorf("no help example for ticket %s", m.verb)
			continue
		}
		argv := strings.Fields(example)[1:]
		fill := map[string]string{"ID": "tpl-" + m.verb, "JSON": payload, "TICKET": local, "N": "current"}
		for j, a := range argv {
			if v, ok := fill[a]; ok {
				argv[j] = v
			}
		}
		if x := atm(t, r.Root, nil, argv...); x.res.Outcome != wire.OutcomeOK {
			t.Errorf("step %d: filled ticket %s example %v: %s", i, m.verb, argv, x.stdout)
		}
	}
}

// TestTCLV0612_WideFieldsRefusalNamesTheMissingKey: when list or search
// --fields widens to the show items and a key is still missing, the refusal
// names that key, not a show-only key the narrow items lack.
func TestTCLV0612_WideFieldsRefusalNamesTheMissingKey(t *testing.T) {
	r := papercutStore(t)
	for _, argv := range [][]string{
		{"ticket", "list", "--fields", "record.labels,noSuchKey"},
		{"ticket", "search", "--status", "OPEN", "--fields", "record.labels,noSuchKey"},
	} {
		x := atm(t, r.Root, nil, argv...)
		if x.res.Outcome == wire.OutcomeOK {
			t.Fatalf("%v accepted an unknown key", argv)
		}
		w := string(x.stdout)
		if !strings.Contains(w, `noSuchKey\"`) {
			t.Errorf("%v: refusal does not name the missing key: %q", argv, w)
		}
	}
}
