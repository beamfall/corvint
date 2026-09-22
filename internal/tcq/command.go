package tcq

import (
	"regexp"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

var (
	oidPattern         = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
	shaPattern         = regexp.MustCompile(`^[0-9a-f]{64}$`)
	runnerIDPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/+:-]{0,127}$`)
	commandIDPattern   = regexp.MustCompile(`^test-command:sha256:[0-9a-f]{64}$`)
	rowIDPattern       = regexp.MustCompile(`^test-row:sha256:[0-9a-f]{64}$`)
	observationIDMatch = regexp.MustCompile(`^test-observation:sha256:[0-9a-f]{64}$`)
)

var commandFields = []string{"argv", "cleanTarget", "cwd", "environment", "id", "runner", "spec", "targetRevision"}

// Clean-target attestations (TCQ-V0-024). Both are unauthenticated caller
// statements; only CALLER_ATTESTED_CLEAN can participate in the V0 relation, and
// neither is verifier evidence.
const (
	CleanTargetAttested    = "CALLER_ATTESTED_CLEAN"
	CleanTargetNotAttested = "NOT_ATTESTED"
)

// command is the verified `test-command/0.1-experimental` artifact. Only its ID,
// target, and clean-target statement leave this struct: TCQ-V0-025 forbids
// persisting, exporting, or echoing argv, which may carry secrets.
type command struct {
	id             string
	targetRevision string
	cwd            string
	cleanTarget    string
}

// parseCommand implements TCQ-V0-023/024/025. Every field is required, no other
// field is allowed, and the self-ID must reproduce from the document without it.
func parseCommand(raw []byte) (command, error) {
	value, err := parseCanonical(raw, commandBounds, CodeInvalidCommand, CodeNoncanonicalCommand)
	if err != nil {
		return command{}, err
	}
	object, err := exactObject(value, commandFields, CodeInvalidCommand)
	if err != nil {
		return command{}, err
	}
	if err := checkCommandFields(object); err != nil {
		return command{}, err
	}
	identity, err := stringField(object, "id", CodeInvalidCommand)
	if err != nil {
		return command{}, err
	}
	if identity != commandPrefix+domainHash(domainCommand, canonicalValue(withoutMember(value, "id"))) {
		return command{}, fail(CodeInvalidCommand)
	}
	return command{
		id:             identity,
		targetRevision: object.Values["targetRevision"].Str,
		cwd:            object.Values["cwd"].Str,
		cleanTarget:    object.Values["cleanTarget"].Str,
	}, nil
}

func checkCommandFields(object *wire.Object) error {
	spec, err := stringField(object, "spec", CodeInvalidCommand)
	if err != nil || spec != CommandSpec {
		return fail(CodeInvalidCommand)
	}
	if err := checkArgv(object.Values["argv"]); err != nil {
		return err
	}
	if err := checkCwd(object); err != nil {
		return err
	}
	if err := checkEnvironment(object.Values["environment"]); err != nil {
		return err
	}
	if err := checkCleanTarget(object); err != nil {
		return err
	}
	if err := checkRunner(object.Values["runner"]); err != nil {
		return err
	}
	target, err := stringField(object, "targetRevision", CodeInvalidCommand)
	if err != nil || !oidPattern.MatchString(target) {
		return fail(CodeInvalidCommand)
	}
	return nil
}

// checkArgv enforces TCQ-V0-023: argv is an array, never a shell string.
func checkArgv(value wire.Value) error {
	if value.Kind != wire.KindArray || len(value.Arr) < 1 || len(value.Arr) > 64 {
		return fail(CodeInvalidCommand)
	}
	total := 0
	for _, item := range value.Arr {
		if item.Kind != wire.KindString || item.Str == "" || hasControl(item.Str) || len(item.Str) > 1024 {
			return fail(CodeInvalidCommand)
		}
		total += len(item.Str)
	}
	if total > 8192 {
		return fail(CodeInvalidCommand)
	}
	return nil
}

func checkCwd(object *wire.Object) error {
	cwd, err := stringField(object, "cwd", CodeInvalidCommand)
	if err != nil {
		return err
	}
	if cwd == "." {
		return nil
	}
	if normalized, ok := normalPath(cwd, 512); !ok || normalized != cwd {
		return fail(CodeInvalidCommand)
	}
	return nil
}

// checkEnvironment enforces TCQ-V0-024: the V0 policy is exactly
// `{"entries":[],"policy":"OMITTED"}`. No environment name or value is ever
// persisted or inferred.
func checkEnvironment(value wire.Value) error {
	object, err := exactObject(value, []string{"entries", "policy"}, CodeInvalidCommand)
	if err != nil {
		return err
	}
	entries := object.Values["entries"]
	if entries.Kind != wire.KindArray || len(entries.Arr) != 0 {
		return fail(CodeInvalidCommand)
	}
	if policy := object.Values["policy"]; policy.Kind != wire.KindString || policy.Str != "OMITTED" {
		return fail(CodeInvalidCommand)
	}
	return nil
}

func checkCleanTarget(object *wire.Object) error {
	value, err := stringField(object, "cleanTarget", CodeInvalidCommand)
	if err != nil {
		return err
	}
	if value != CleanTargetAttested && value != CleanTargetNotAttested {
		return fail(CodeInvalidCommand)
	}
	return nil
}

func checkRunner(value wire.Value) error {
	object, err := exactObject(value, []string{"identity", "version"}, CodeInvalidCommand)
	if err != nil {
		return err
	}
	for _, field := range []string{"identity", "version"} {
		item := object.Values[field]
		if item.Kind != wire.KindString || !runnerIDPattern.MatchString(item.Str) {
			return fail(CodeInvalidCommand)
		}
	}
	return nil
}

// MakeTestCommand builds one canonical `test-command/0.1-experimental` artifact.
// It is a declaration, never an instruction: TCQ neither executes the command
// nor claims that retaining it makes execution reproducible (TCQ-V0-025).
func MakeTestCommand(argv []string, cwd, runnerIdentity, runnerVersion, targetRevision, cleanTarget string) ([]byte, error) {
	document := jsonObject(
		member{"argv", jsonStrings(argv)},
		member{"cleanTarget", jsonString(cleanTarget)},
		member{"cwd", jsonString(cwd)},
		member{"environment", jsonObject(
			member{"entries", jsonArray(nil)},
			member{"policy", jsonString("OMITTED")},
		)},
		member{"runner", jsonObject(
			member{"identity", jsonString(runnerIdentity)},
			member{"version", jsonString(runnerVersion)},
		)},
		member{"spec", jsonString(CommandSpec)},
		member{"targetRevision", jsonString(targetRevision)},
	)
	identity := commandPrefix + domainHash(domainCommand, canonicalValue(document))
	document.Obj.Keys = append(document.Obj.Keys, "id")
	document.Obj.Values["id"] = jsonString(identity)
	raw := canonicalJSON(document)
	if _, err := parseCommand(raw); err != nil {
		return nil, err
	}
	return raw, nil
}
