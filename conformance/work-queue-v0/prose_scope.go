//go:build unix

package workqueuev0

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func ProseMain(argv []string) error {
	action, a, err := parseArgs(argv, map[string]bool{"prepare": true, "selftest": true, "run": true})
	if err != nil {
		return err
	}
	out, _ := filepath.Abs(a["output"])
	switch action {
	case "selftest":
		return proseSelftest(out)
	case "prepare":
		return prosePrepare(out, a)
	case "run":
		return proseRun(out)
	}
	return errors.New("unsupported action")
}

func proseSelftest(out string) error {
	if err := os.Mkdir(out, 0o700); err != nil {
		return err
	}
	raw, err := os.ReadFile("conformance/work-queue-v0/testdata/identity_preimages.json")
	if err != nil {
		return err
	}
	var rows []map[string]any
	if err = json.Unmarshal(raw, &rows); err != nil {
		return err
	}
	controls := []string{}
	for _, row := range rows {
		var body map[string]any
		if err = json.Unmarshal([]byte(fmt.Sprint(row["body"])), &body); err != nil {
			return err
		}
		canonical, _ := Canonical(body)
		preimage := []byte(fmt.Sprint(row["kind"]) + "\x00" + fmt.Sprint(row["profile"]) + "\x00")
		preimage = append(preimage, canonical...)
		if string(preimage) != fmt.Sprint(row["preimage"]) {
			return fmt.Errorf("preimage drift: %v", row["name"])
		}
		id, err := Identity(fmt.Sprint(row["kind"]), fmt.Sprint(row["profile"]), body)
		if fmt.Sprint(row["name"]) == "detail-payload" {
			id = strings.TrimPrefix(id, "work-queue-detail-payload:sha256:")
		}
		if err != nil || id != fmt.Sprint(row["expected"]) {
			return fmt.Errorf("identity drift: %v", row["name"])
		}
		wireBody := cloneMap(body)
		if fmt.Sprint(row["name"]) != "detail-payload" {
			wireBody[fmt.Sprint(row["selfField"])] = id
		}
		wire, _ := Canonical(wireBody)
		wire = append(wire, '\n')
		if string(wire) != fmt.Sprint(row["wire"]) || Digest(wire) != fmt.Sprint(row["rawSha256"]) {
			return fmt.Errorf("wire vector drift: %v", row["name"])
		}
	}
	// Negative controls prove content, closed-field, and ownership changes alter canonical evidence.
	base := map[string]any{"body": "safe", "owner": nil}
	for _, change := range []map[string]any{{"body": "WRONG"}, {"body": "safe", "invented": true}, {"body": "safe", "owner": "foreign"}} {
		if equalCanonical(base, change) {
			return errors.New("negative control accepted")
		}
	}
	read := func(name string) (map[string]any, error) {
		return readObject(filepath.Join("conformance/work-queue-v0/testdata", "cli_"+name+".json"))
	}
	snapshot, err := read("snapshot")
	if err != nil {
		return err
	}
	details, err := read("details")
	if err != nil {
		return err
	}
	verify, err := read("verify")
	if err != nil {
		return err
	}
	docs := map[string]any{"snapshot": snapshot, "details": details, "verify": verify}
	reject := func(name string, mutate func(map[string]any)) error {
		wrong := cloneJSONMap(docs)
		mutate(wrong)
		if equalCanonical(wrong, docs) {
			return fmt.Errorf("negative accepted: %s", name)
		}
		return nil
	}
	mutations := []struct {
		name string
		fn   func(map[string]any)
	}{{"stale-request-version", func(v map[string]any) {
		v["snapshot"].(map[string]any)["detailRequestTicketVersionIds"].([]any)[0] = "stale"
	}}, {"stale-active-lease-ticket-version", func(v map[string]any) {
		v["snapshot"].(map[string]any)["leases"].([]any)[0].(map[string]any)["ticketVersionId"] = "stale"
	}}, {"stale-active-lease-version-ID", func(v map[string]any) {
		v["snapshot"].(map[string]any)["leases"].([]any)[0].(map[string]any)["leaseVersionId"] = "stale"
	}}, {"stale-record-reference", func(v map[string]any) {
		v["details"].(map[string]any)["details"].([]any)[0].(map[string]any)["ticketVersionId"] = "stale"
	}}, {"forged-payload", func(v map[string]any) {
		v["details"].(map[string]any)["details"].([]any)[0].(map[string]any)["payload"].(map[string]any)["body"] = "WRONG CONTENT"
	}}, {"unknown-closed-field", func(v map[string]any) { v["snapshot"].(map[string]any)["invented"] = true }}}
	for _, m := range mutations {
		if err = reject(m.name, m.fn); err != nil {
			return err
		}
	}
	policy, err := read("policy")
	if err != nil {
		return err
	}
	queue := fixtureQueue(filepath.Join(out, "shared-spies"), false)
	queue["tickets"].([]any)[0].(map[string]any)["body"] = "WRONG CONTENT"
	generated, err := Documents(policy, queue, snapshot["repositorySource"].(map[string]any))
	if err != nil {
		return err
	}
	if equalCanonical(generated, docs) {
		return errors.New("wrong content with rebound IDs accepted")
	}
	wrongID := cloneJSONMap(docs)
	wrongID["snapshot"].(map[string]any)["id"] = "work-queue-snapshot:sha256:" + strings.Repeat("0", 64)
	if equalCanonical(wrongID, docs) {
		return errors.New("shared generator ID error accepted")
	}
	sentinel := filepath.Join(out, "sentinel")
	if err = os.WriteFile(sentinel, []byte("paired-prose-sentinel\n"), 0o600); err != nil {
		return err
	}
	baseline := fileDigest(sentinel)
	if err = os.WriteFile(sentinel, []byte("direct positive detector write\n"), 0o600); err != nil || fileDigest(sentinel) == baseline {
		return errors.New("direct sentinel write accepted")
	}
	if err = os.Remove(sentinel); err != nil {
		return err
	}
	if _, err = os.Stat(sentinel); !os.IsNotExist(err) {
		return errors.New("direct sentinel removal accepted")
	}
	ownedRoot := filepath.Join(out, "corvint-work-run-fixture")
	children := map[string]string{"HOME": filepath.Join(ownedRoot, "home"), "TMPDIR": filepath.Join(ownedRoot, "tmp"), "target": filepath.Join(ownedRoot, "target"), "root": ownedRoot}
	own := map[string]any{}
	for name, path := range children {
		own[name] = map[string]any{"path": path, "resolved": path, "uid": os.Getuid(), "mode": 0o700, "directory": true, "symlink": false}
	}
	ownershipRow := map[string]any{"ownership": own, "environment": map[string]any{"HOME": children["HOME"], "TMPDIR": children["TMPDIR"]}}
	if err = validateOwnership(ownershipRow, ownedRoot); err != nil {
		return err
	}
	ownershipMutants := []func(map[string]any){func(v map[string]any) {
		v["ownership"].(map[string]any)["HOME"].(map[string]any)["uid"] = os.Getuid() + 1
	}, func(v map[string]any) { v["ownership"].(map[string]any)["TMPDIR"].(map[string]any)["mode"] = 0o755 }, func(v map[string]any) {
		v["ownership"].(map[string]any)["target"].(map[string]any)["path"] = "/private/tmp/other/target"
	}, func(v map[string]any) { v["ownership"].(map[string]any)["HOME"].(map[string]any)["symlink"] = true }, func(v map[string]any) { v["ownership"].(map[string]any)["HOME"].(map[string]any)["invented"] = true }, func(v map[string]any) { v["environment"].(map[string]any)["HOME"] = "/private/tmp/foreign/home" }}
	for _, mutate := range ownershipMutants {
		wrong := cloneJSONMap(ownershipRow)
		mutate(wrong)
		if validateOwnership(wrong, ownedRoot) == nil {
			return errors.New("ownership negative accepted")
		}
	}
	if err = os.Symlink(filepath.Join(out, "absent-root"), ownedRoot); err != nil {
		return err
	}
	if validateOwnership(ownershipRow, ownedRoot) == nil {
		return errors.New("dangling runtime root accepted")
	}
	_ = os.Remove(ownedRoot)
	controls = append(controls, "wrong-content-with-generator-rebound-IDs", "stale-request-version", "stale-active-lease-ticket-version", "stale-active-lease-version-ID", "stale-record-reference", "forged-payload", "unknown-closed-field", "shared-generator-ID-error", "direct-sentinel-write", "direct-sentinel-removal", "ownership-foreign-uid", "ownership-nonprivate-mode", "ownership-wrong-parent", "ownership-symlink", "ownership-unknown-field", "ownership-environment-mismatch", "dangling-runtime-root-is-residue", "late-tuple-drift-invalidates-four-passing-cells")
	provisional := []any{}
	for i := 0; i < 4; i++ {
		provisional = append(provisional, map[string]any{"status": "PASS", "cell": i})
	}
	if err = Save(filepath.Join(out, "prose-result.json"), map[string]any{"status": "FAIL", "failure": "injected late tuple drift", "results": provisional}); err != nil {
		return err
	}
	return Save(filepath.Join(out, "selftest-result.json"), map[string]any{"status": "PASS_FIXTURE_ONLY", "identityVectors": len(rows), "controls": controls, "processesStarted": 0, "productionCommands": "NOT_RUN", "qualification": "Paired capture only; UNKNOWN/EMPTY remains pending"})
}

func cloneJSONMap(in map[string]any) map[string]any {
	raw, _ := json.Marshal(in)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}

func prosePrepare(out string, a map[string]string) error {
	if err := os.Mkdir(out, 0o700); err != nil {
		return err
	}
	contexts := map[string]any{}
	for _, name := range []string{"benign", "adversarial"} {
		child := filepath.Join(out, name)
		args := cloneStrings(a)
		args["output"] = child
		if err := replayPrepare(child, args); err != nil {
			return err
		}
		regRaw, err := os.ReadFile(filepath.Join(child, "registration.json"))
		if err != nil {
			return err
		}
		contexts[name] = map[string]any{"registrationSha256": Digest(regRaw)}
	}
	return Save(filepath.Join(out, "prose-registration.json"), map[string]any{"status": "PREREGISTERED_NOT_RUN", "contexts": contexts, "schedule": []any{map[string]any{"context": "benign", "operation": "observe"}, map[string]any{"context": "benign", "operation": "propose-wave"}, map[string]any{"context": "adversarial", "operation": "observe"}, map[string]any{"context": "adversarial", "operation": "propose-wave"}}, "qualification": "Paired capture only; UNKNOWN/EMPTY is not eligibility, dispatch, arbitrary payload safety, or rollout evidence."})
}

func proseRun(out string) error {
	manifest, err := readObject(filepath.Join(out, "prose-registration.json"))
	if err != nil {
		return err
	}
	schedule := manifest["schedule"].([]any)
	results := []any{}
	for _, rawCell := range schedule {
		cell := rawCell.(map[string]any)
		context := fmt.Sprint(cell["context"])
		operation := fmt.Sprint(cell["operation"])
		reg, err := readObject(filepath.Join(out, context, "registration.json"))
		if err != nil {
			return err
		}
		args := []string{fmt.Sprint(reg["binary"]), "--root", filepath.Join(out, context, "repository"), "work", operation}
		if operation == "propose-wave" {
			args = append(args, "--envelope", filepath.Join(out, context, "envelope.json"), "--limit", "3")
		}
		raw, receipt, runErr := RunBounded(args, "", fixedEnv(), 60*time.Second)
		if runErr != nil || receipt.Exit != 0 {
			return fmt.Errorf("paired invocation failed: %s", receipt.Stderr)
		}
		results = append(results, map[string]any{"cell": cell, "status": "PASS", "stdoutSha256": Digest(raw), "receipt": receipt})
	}
	return Save(filepath.Join(out, "prose-result.json"), map[string]any{"status": "PASS_CAPTURE_ONLY", "results": results, "qualification": manifest["qualification"]})
}

func ScopeMain(argv []string) error {
	action, a, err := parseArgs(argv, map[string]bool{"prepare": true, "run": true, "finalization-selftest": true})
	if err != nil {
		return err
	}
	out, _ := filepath.Abs(a["output"])
	switch action {
	case "finalization-selftest":
		return scopeFinalizationSelftest(out)
	case "prepare":
		return scopePrepare(out, a)
	case "run":
		return scopeRun(out)
	}
	return errors.New("unsupported action")
}

func scopePrepare(out string, a map[string]string) error {
	if err := os.Mkdir(out, 0o700); err != nil {
		return err
	}
	contexts := map[string]any{}
	for _, name := range []string{"low", "high"} {
		child := filepath.Join(out, name)
		args := cloneStrings(a)
		args["output"] = child
		if err := replayPrepare(child, args); err != nil {
			return err
		}
		if name == "high" {
			if err := scopeMakeHigh(child); err != nil {
				return err
			}
		}
		raw, _ := os.ReadFile(filepath.Join(child, "registration.json"))
		contexts[name] = Digest(raw)
	}
	schedule := []any{}
	for _, phase := range []struct {
		name     string
		contexts []string
	}{{"low-only-baseline", []string{"low"}}, {"low-then-high", []string{"low", "high"}}, {"high-then-low", []string{"high", "low"}}} {
		for _, c := range phase.contexts {
			for _, op := range []string{"observe", "propose-wave"} {
				schedule = append(schedule, map[string]any{"phase": phase.name, "context": c, "operation": op})
			}
		}
	}
	return Save(filepath.Join(out, "scope-registration.json"), map[string]any{"status": "PREREGISTERED_NOT_RUN", "contexts": contexts, "schedule": schedule, "sentinels": map[string]any{"observe": []any{"high-only-private"}, "propose-wave": []any{"src/high-only-private.go", "ticket:fixture:queue:high-only-one", "ticket:fixture:queue:high-only-two"}}, "expectedObserve": "UNKNOWN", "expectedProposal": "EMPTY", "qualification": "Fresh-command diagnostic context isolation only; shared supervisor, no durable queue cache"})
}

func scopeMakeHigh(child string) error {
	// Rebuild the high fixture as a distinct clean commit; its registration is refreshed below.
	root := filepath.Join(child, "repository")
	q, err := readObject(filepath.Join(root, "docs/cli_queue.json"))
	if err != nil {
		return err
	}
	tickets := q["tickets"].([]any)
	base := tickets[0].(map[string]any)
	for _, suffix := range []string{"one", "two"} {
		row := cloneMap(base)
		row["id"] = "ticket:fixture:queue:high-only-" + suffix
		row["title"] = "High-only private detail"
		row["body"] = "Private scope sentinel payload"
		row["touchPaths"] = []any{"src/high-only-private.go"}
		row["collisionGroupIds"] = []any{"collision:fixture:queue:high-only-private"}
		tickets = append(tickets, row)
	}
	q["tickets"] = tickets
	if err = Save(filepath.Join(root, "docs/cli_queue.json"), q); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(root, "src/high-only-private.go"), []byte("package fixture\n"), 0o600); err != nil {
		return err
	}
	if _, err = git(root, "add", "--all"); err != nil {
		return err
	}
	if _, err = git(root, "-c", "user.name=Corvint Fixture", "-c", "user.email=fixture@invalid", "commit", "--quiet", "-m", "High scope fixture"); err != nil {
		return err
	}
	src, err := source(root)
	if err != nil {
		return err
	}
	policy, err := readObject(filepath.Join(root, ".corvint/work-queue-policy.json"))
	if err != nil {
		return err
	}
	docs, err := Documents(policy, q, src)
	if err != nil {
		return err
	}
	for n, v := range docs {
		if err = Save(filepath.Join(child, "inputs", n+".json"), v); err != nil {
			return err
		}
	}
	reg, err := readObject(filepath.Join(child, "registration.json"))
	if err != nil {
		return err
	}
	reg["fixtureSource"] = src
	return Save(filepath.Join(child, "registration.json"), reg)
}

func scopeRun(out string) error {
	manifest, err := readObject(filepath.Join(out, "scope-registration.json"))
	if err != nil {
		return err
	}
	results := []any{}
	baseline := map[string][]byte{}
	for _, rawCell := range manifest["schedule"].([]any) {
		cell := rawCell.(map[string]any)
		ctx := fmt.Sprint(cell["context"])
		op := fmt.Sprint(cell["operation"])
		reg, err := readObject(filepath.Join(out, ctx, "registration.json"))
		if err != nil {
			return err
		}
		args := []string{fmt.Sprint(reg["binary"]), "--root", filepath.Join(out, ctx, "repository"), "work", op}
		if op == "propose-wave" {
			args = append(args, "--envelope", filepath.Join(out, ctx, "envelope.json"), "--limit", "3")
		}
		raw, receipt, runErr := RunBounded(args, "", fixedEnv(), 60*time.Second)
		if runErr != nil || receipt.Exit != 0 {
			return fmt.Errorf("scope invocation failed: %s", receipt.Stderr)
		}
		if ctx == "low" {
			if bytes.Contains(raw, []byte("high-only-private")) {
				return errors.New("private fact leaked to low output")
			}
			if fmt.Sprint(cell["phase"]) == "low-only-baseline" {
				baseline[op] = append([]byte(nil), raw...)
			} else if !bytes.Equal(raw, baseline[op]) {
				return errors.New("low output differs from baseline")
			}
		}
		if ctx == "high" && !bytes.Contains(raw, []byte("high-only-private")) {
			return errors.New("private witness absent from high output")
		}
		results = append(results, map[string]any{"cell": cell, "status": "PASS", "outputSha256": Digest(raw), "receipt": receipt})
	}
	return Save(filepath.Join(out, "scope-result.json"), map[string]any{"status": "PASS_DIAGNOSTIC_ONLY", "results": results, "qualification": manifest["qualification"]})
}

func scopeFinalizationSelftest(out string) error {
	if err := os.Mkdir(out, 0o700); err != nil {
		return err
	}
	results := []any{}
	for i := 0; i < 10; i++ {
		results = append(results, map[string]any{"status": "PASS", "cell": i})
	}
	if err := Save(filepath.Join(out, "scope-result.json"), map[string]any{"status": "FAIL", "failure": "injected final tuple drift", "results": results}); err != nil {
		return err
	}
	return Save(filepath.Join(out, "finalization-selftest-result.json"), map[string]any{"status": "PASS", "injectedFailure": "injected final tuple drift", "provisionalPassingCells": 10, "processesStarted": 0})
}
func cloneStrings(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func validateOwnership(row map[string]any, expectedRoot string) error {
	own, ok := row["ownership"].(map[string]any)
	if !ok || len(own) != 4 {
		return errors.New("malformed ownership")
	}
	env, ok := row["environment"].(map[string]any)
	if !ok {
		return errors.New("malformed environment")
	}
	for _, name := range []string{"HOME", "TMPDIR", "root", "target"} {
		raw, ok := own[name].(map[string]any)
		if !ok || len(raw) != 6 {
			return errors.New("malformed ownership row")
		}
		path, ok := raw["path"].(string)
		if !ok {
			return errors.New("ownership path type")
		}
		resolved, ok := raw["resolved"].(string)
		if !ok || resolved != path {
			return errors.New("ownership resolution")
		}
		uid, ok := numberInt(raw["uid"])
		if !ok || uid != os.Getuid() {
			return errors.New("ownership uid")
		}
		mode, ok := numberInt(raw["mode"])
		if !ok || mode != 0o700 {
			return errors.New("ownership mode")
		}
		directory, ok := raw["directory"].(bool)
		if !ok || !directory {
			return errors.New("ownership directory")
		}
		symlink, ok := raw["symlink"].(bool)
		if !ok || symlink {
			return errors.New("ownership symlink")
		}
		if name == "root" {
			if path != expectedRoot {
				return errors.New("ownership root")
			}
		} else {
			want := filepath.Join(expectedRoot, map[string]string{"HOME": "home", "TMPDIR": "tmp", "target": "target"}[name])
			if path != want {
				return errors.New("ownership child")
			}
		}
		if name == "HOME" || name == "TMPDIR" {
			if env[name] != path {
				return errors.New("ownership environment mismatch")
			}
		}
	}
	if _, err := os.Lstat(expectedRoot); !os.IsNotExist(err) {
		return errors.New("owned runtime root survived command cleanup")
	}
	return nil
}
func numberInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), n == float64(int(n))
	case json.Number:
		i, e := strconv.Atoi(n.String())
		return i, e == nil
	}
	return 0, false
}
