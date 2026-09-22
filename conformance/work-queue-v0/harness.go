//go:build unix

// Package workqueuev0 implements the native work-queue qualification fixtures.
package workqueuev0

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	maxOutput = 64 << 20
	maxError  = 1 << 20
	maxSpies  = 4096
)

func Canonical(v any) ([]byte, error) {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	if err := e.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

func Digest(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }

func Identity(kind, profile string, body any) (string, error) {
	b, err := Canonical(body)
	if err != nil {
		return "", err
	}
	return kind + ":sha256:" + Digest(append(append([]byte(kind+"\x00"+profile+"\x00"), b...), nil...)), nil
}

func Identified(kind, profile, field string, body map[string]any) (map[string]any, error) {
	id, err := Identity(kind, profile, body)
	if err != nil {
		return nil, err
	}
	out := cloneMap(body)
	out[field] = id
	return out, nil
}

func Save(path string, value any) error {
	b, err := Canonical(value)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o600)
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func readObject(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err = d.Decode(&v); err != nil {
		return nil, err
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, errors.New("trailing JSON")
	}
	return v, nil
}

type Receipt struct {
	Argv                      []string `json:"argv"`
	StartedNS                 int64    `json:"startedNs"`
	Exit                      int      `json:"exit"`
	Failure                   any      `json:"failure"`
	Waited                    bool     `json:"waited"`
	StreamObservationComplete bool     `json:"streamObservationComplete"`
	Cleanup                   string   `json:"cleanup"`
	StdoutBytes               int      `json:"stdoutBytes"`
	StdoutRawSHA256           string   `json:"stdoutRawSha256"`
	StderrBytes               int      `json:"stderrBytes"`
	StderrRawSHA256           string   `json:"stderrRawSha256"`
	Stderr                    string   `json:"stderr"`
}

func RunBounded(argv []string, cwd string, env []string, timeout time.Duration) ([]byte, Receipt, error) {
	return runBoundedLimits(argv, cwd, env, timeout, maxOutput, maxError)
}

func runBoundedLimits(argv []string, cwd string, env []string, timeout time.Duration, stdoutLimit, stderrLimit int) ([]byte, Receipt, error) {
	r := Receipt{Argv: argv, StartedNS: time.Now().UnixNano(), Failure: nil}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = cwd
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var stdout, stderr limitBuffer
	stdout.limit = stdoutLimit
	stderr.limit = stderrLimit
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		r.Failure = err.Error()
		return nil, r, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var waitErr error
	select {
	case waitErr = <-done:
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		waitErr = <-done
		r.Failure = "TIMEOUT"
	}
	r.Waited = true
	r.StreamObservationComplete = true
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	r.Cleanup = "NO_OWNED_LIVE_DESCENDANTS"
	if cmd.ProcessState != nil {
		r.Exit = cmd.ProcessState.ExitCode()
	}
	r.StdoutBytes = stdout.count
	r.StdoutRawSHA256 = streamDigest(&stdout)
	r.StderrBytes = stderr.count
	r.StderrRawSHA256 = streamDigest(&stderr)
	r.Stderr = string(stderr.buf)
	if stdout.exceeded || stderr.exceeded {
		r.Failure = "OUTPUT_LIMIT"
		return stdout.buf, r, errors.New("output limit")
	}
	if waitErr != nil && r.Exit < 0 && r.Failure == nil {
		r.Failure = waitErr.Error()
	}
	return stdout.buf, r, nil
}

type limitBuffer struct {
	buf      []byte
	limit    int
	count    int
	hash     hash.Hash
	exceeded bool
}

func (b *limitBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if b.hash == nil {
		b.hash = sha256.New()
	}
	_, _ = b.hash.Write(p)
	b.count += n
	if len(b.buf) < b.limit {
		take := b.limit - len(b.buf)
		if take > n {
			take = n
		}
		b.buf = append(b.buf, p[:take]...)
	}
	b.exceeded = b.count > b.limit
	return n, nil
}

func streamDigest(b *limitBuffer) string {
	if b.hash == nil {
		return Digest(nil)
	}
	return hex.EncodeToString(b.hash.Sum(nil))
}

func WriteSpy(dir, operation string, ownership bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) >= maxSpies {
		return errors.New("fixture spy record limit")
	}
	for i, e := range entries {
		if e.Name() != fmt.Sprintf("%06d.json", i) || e.Type()&os.ModeSymlink != 0 || e.IsDir() {
			return errors.New("invalid immutable spy sequence")
		}
		info, err := e.Info()
		if err != nil || info.Size() > 65536 {
			return errors.New("invalid immutable spy sequence")
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		var prior map[string]any
		if err = json.Unmarshal(raw, &prior); err != nil {
			return errors.New("invalid immutable spy sequence")
		}
		sequence, ok := prior["sequence"].(float64)
		if !ok || int(sequence) != i {
			return errors.New("invalid immutable spy sequence")
		}
		canonical, err := Canonical(prior)
		if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
			return errors.New("noncanonical immutable spy record")
		}
	}
	env := map[string]any{}
	for _, k := range EnvironmentKeys {
		if v, ok := os.LookupEnv(k); ok {
			env[k] = v
		} else {
			env[k] = nil
		}
	}
	record := map[string]any{"argv": []any{operation}, "pid": os.Getpid(), "operation": operation, "environment": env, "sequence": len(entries)}
	if ownership {
		own, err := ownershipSnapshot()
		if err != nil {
			return err
		}
		record["ownership"] = own
	}
	b, err := Canonical(record)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if len(b) > 65536 {
		return errors.New("fixture spy byte limit")
	}
	p := filepath.Join(dir, fmt.Sprintf("%06d.json", len(entries)))
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	return errors.Join(err, f.Close())
}

var EnvironmentKeys = []string{"GIT_CONFIG_COUNT", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_KEY_0", "GIT_CONFIG_SYSTEM", "GIT_CONFIG_VALUE_0", "GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE", "HOME", "LANG", "LC_ALL", "PATH", "SOURCE_DATE_EPOCH", "TMPDIR", "TZ"}

func ownershipSnapshot() (map[string]any, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	home := os.Getenv("HOME")
	tmp := os.Getenv("TMPDIR")
	paths := map[string]string{"HOME": home, "TMPDIR": tmp, "target": cwd, "root": filepath.Dir(home)}
	out := map[string]any{}
	for k, p := range paths {
		info, err := os.Lstat(p)
		if err != nil {
			return nil, err
		}
		resolved, err := filepath.EvalSymlinks(p)
		if err != nil {
			return nil, err
		}
		st := info.Sys().(*syscall.Stat_t)
		out[k] = map[string]any{"path": p, "resolved": resolved, "uid": int(st.Uid), "mode": int(info.Mode().Perm()), "directory": info.IsDir(), "symlink": info.Mode()&os.ModeSymlink != 0}
	}
	return out, nil
}

func git(root string, args ...string) ([]byte, error) {
	base := []string{"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "-c", "core.excludesFile="}
	base = append(base, args...)
	env := []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_REPLACE_OBJECTS=1", "GIT_NO_LAZY_FETCH=1"}
	raw, r, err := RunBounded(append([]string{"/usr/bin/git"}, base...), root, env, 10*time.Second)
	if err != nil || r.Exit != 0 {
		return nil, fmt.Errorf("fixture Git failed: %s", r.Stderr)
	}
	return raw, nil
}

func source(root string) (map[string]any, error) {
	raw, err := git(root, "rev-parse", "--show-object-format", "HEAD", "HEAD^{tree}")
	if err != nil {
		return nil, err
	}
	parts := strings.Fields(string(raw))
	if len(parts) != 3 {
		return nil, errors.New("unexpected git identity")
	}
	status, err := git(root, "status", "--porcelain=v2", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	material, err := materialization(root, parts[2])
	if err != nil {
		return nil, err
	}
	return Identified("repository-source", "repository-source/0", "id", map[string]any{"objectFormat": parts[0], "commit": parts[1], "tree": parts[2], "materializationSha256": material, "statusSha256": Digest(status)})
}

func materialization(root, tree string) (string, error) {
	raw, err := git(root, "ls-tree", "-r", "-z", "--full-tree", tree)
	if err != nil {
		return "", err
	}
	items := bytes.Split(bytes.TrimSuffix(raw, []byte{0}), []byte{0})
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		pair := bytes.SplitN(item, []byte{'\t'}, 2)
		if len(pair) != 2 {
			return "", errors.New("invalid tree")
		}
		fields := strings.Fields(string(pair[0]))
		if len(fields) != 3 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
			return "", errors.New("unsupported tree entry")
		}
		name := string(pair[1])
		content, err := git(root, "show", tree+":"+name)
		if err != nil {
			return "", err
		}
		disk, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || !bytes.Equal(content, disk) {
			return "", errors.New("materialization differs from immutable blob")
		}
		rows = append(rows, map[string]any{"blobOid": fields[2], "mode": fields[0], "path": name, "rawSha256": Digest(content)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["path"].(string) < rows[j]["path"].(string) })
	encoded, err := Canonical(rows)
	if err != nil {
		return "", err
	}
	return Digest(append([]byte("verification-materialization\x00verification-materialization/0\x00"), encoded...)), nil
}

func installSignals() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() { s := <-ch; os.Exit(128 + int(s.(syscall.Signal))) }()
}

func requireTmp(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(abs, "/private/tmp/") && !strings.HasPrefix(abs, "/tmp/") {
		return errors.New("raw evidence belongs under /private/tmp")
	}
	return nil
}

func parseArgs(argv []string, actions map[string]bool) (string, map[string]string, error) {
	if len(argv) < 1 || !actions[argv[0]] {
		return "", nil, errors.New("unsupported action")
	}
	m := map[string]string{}
	for i := 1; i < len(argv); i += 2 {
		if i+1 >= len(argv) || !strings.HasPrefix(argv[i], "--") {
			return "", nil, errors.New("invalid arguments")
		}
		m[strings.TrimPrefix(argv[i], "--")] = argv[i+1]
	}
	if m["output"] == "" {
		return "", nil, errors.New("--output is required")
	}
	if err := requireTmp(m["output"]); err != nil {
		return "", nil, err
	}
	return argv[0], m, nil
}

func runtimePlatform() string {
	if runtime.GOOS == "darwin" {
		return "DARWIN_PROCESS_GROUP_UNQUALIFIED"
	}
	return "LINUX_PROCESS_GROUP_UNQUALIFIED"
}
func atoi(v any) (int, error) { return strconv.Atoi(fmt.Sprint(v)) }

// AdapterMain is the independent repository-owned WQO producer.
func AdapterMain(argv []string) error {
	if len(argv) != 1 || (argv[0] != "snapshot" && argv[0] != "details" && argv[0] != "verify") {
		return errors.New("usage: cli-adapter snapshot|details|verify")
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	queue, err := readObject(filepath.Join(root, "docs/cli_queue.json"))
	if err != nil {
		return err
	}
	policy, err := readObject(filepath.Join(root, ".corvint/work-queue-policy.json"))
	if err != nil {
		return err
	}
	own, _ := queue["recordOwnership"].(bool)
	if err = WriteSpy(fmt.Sprint(queue["spyDirectory"]), argv[0], own); err != nil {
		return err
	}
	src, err := source(root)
	if err != nil {
		return err
	}
	docs, err := Documents(policy, queue, src)
	if err != nil {
		return err
	}
	b, err := Canonical(docs[argv[0]])
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(b, '\n'))
	return err
}

func Documents(policy, queue, src map[string]any) (map[string]map[string]any, error) {
	items, ok := queue["tickets"].([]any)
	if !ok {
		return nil, errors.New("tickets must be array")
	}
	tickets := make([]any, 0, len(items))
	details := []any{}
	versions := map[string]string{}
	limit, err := atoi(policy["detailLimit"])
	if err != nil {
		return nil, err
	}
	for rank, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			return nil, errors.New("ticket must be object")
		}
		id := fmt.Sprint(row["id"])
		payload := map[string]any{"acceptanceCriteria": []any{"Retain repository authority"}, "body": row["body"], "displayKey": nil, "evidenceHandles": []any{}, "owner": nil, "title": row["title"]}
		payloadID, err := Identity("work-queue-detail-payload", "work-queue-detail-payload/0", payload)
		if err != nil {
			return nil, err
		}
		payloadDigest := strings.TrimPrefix(payloadID, "work-queue-detail-payload:sha256:")
		vb := map[string]any{"declaredVersion": "fixture-v1", "queueAuthorityId": policy["queueAuthorityId"], "repositoryAuthorityId": policy["repositoryAuthorityId"], "ticketContentSha256": mustDigest(row), "ticketId": id}
		version, err := Identity("ticket-version", "ticket-version/0", vb)
		if err != nil {
			return nil, err
		}
		versions[id] = version
		t := cloneMap(vb)
		for k, v := range map[string]any{"ticketVersionId": version, "rank": strconv.Itoa(rank), "authority": "COMPLETE", "atomicRepositoryAuthorityIds": []any{policy["repositoryAuthorityId"]}, "capacityUses": row["capacityUses"], "collisionGroupIds": row["collisionGroupIds"], "dependencyTicketIds": []any{}, "detailPayloadSha256": payloadDigest, "lifecycle": row["lifecycle"], "routeAlternatives": row["routeAlternatives"], "touchPaths": row["touchPaths"], "selectionFacts": map[string]any{"approvals": "CLEAR", "dependencies": "SATISFIED", "holds": "CLEAR", "lease": map[bool]string{true: "PRESENT", false: "ABSENT"}[row["leased"] == true]}} {
			t[k] = v
		}
		tickets = append(tickets, t)
		if row["lifecycle"] == "READY" && len(details) < limit {
			d, err := Identified("work-queue-detail-record", "work-queue-detail-record/0", "detailId", map[string]any{"payload": payload, "payloadSha256": payloadDigest, "repositoryAuthorityId": policy["repositoryAuthorityId"], "ticketId": id, "ticketVersionId": version})
			if err != nil {
				return nil, err
			}
			details = append(details, d)
		}
	}
	leases := []any{}
	for _, item := range items {
		row := item.(map[string]any)
		if row["leased"] != true {
			continue
		}
		id := fmt.Sprint(row["id"])
		l, err := Identified("lease-version", "lease-version/0", "leaseVersionId", map[string]any{"blocksSelection": true, "capacityUses": row["capacityUses"], "collisionGroupIds": row["collisionGroupIds"], "holderId": "holder:fixture:queue:owner", "leaseId": "lease:fixture:queue:active", "lifecycle": "ACTIVE", "queueAuthorityId": policy["queueAuthorityId"], "repositoryAuthorityId": policy["repositoryAuthorityId"], "ticketId": id, "ticketVersionId": versions[id]})
		if err != nil {
			return nil, err
		}
		leases = append(leases, l)
	}
	checkpoint := map[string]any{"id": "checkpoint:fixture:queue", "version": src["tree"]}
	requested := []string{}
	for _, d := range details {
		requested = append(requested, d.(map[string]any)["ticketVersionId"].(string))
	}
	sort.Strings(requested)
	req := make([]any, len(requested))
	for i, v := range requested {
		req[i] = v
	}
	sort.Slice(details, func(i, j int) bool {
		a, _ := Canonical(details[i])
		b, _ := Canonical(details[j])
		return bytes.Compare(a, b) < 0
	})
	snapshot, err := Identified("work-queue-snapshot", "work-queue-snapshot/0", "id", map[string]any{"accessContextId": policy["accessContextId"], "capacityClasses": queue["capacityClasses"], "checkpoint": checkpoint, "detailRequestTicketVersionIds": req, "leases": leases, "policyId": policy["id"], "profile": "work-queue-snapshot/0", "queueAuthorityId": policy["queueAuthorityId"], "repositoryAuthorityId": policy["repositoryAuthorityId"], "repositorySource": src, "scope": map[string]any{"complete": true, "id": policy["scopeId"], "ticketCount": strconv.Itoa(len(tickets))}, "tickets": tickets})
	if err != nil {
		return nil, err
	}
	detailDoc, err := Identified("work-queue-details", "work-queue-detail/0", "id", map[string]any{"details": details, "profile": "work-queue-detail/0", "snapshotId": snapshot["id"]})
	if err != nil {
		return nil, err
	}
	verify, err := Identified("work-queue-checkpoint", "work-queue-checkpoint/0", "id", map[string]any{"checkpoint": checkpoint, "policyId": policy["id"], "profile": "work-queue-checkpoint/0", "repositorySource": src, "snapshotId": snapshot["id"]})
	if err != nil {
		return nil, err
	}
	return map[string]map[string]any{"snapshot": snapshot, "details": detailDoc, "verify": verify}, nil
}

func expectedCommands(reg map[string]any) (map[string]map[string]any, error) {
	receipts := []any{}
	for _, op := range []string{"snapshot", "details", "verify"} {
		inputs := reg["inputs"].(map[string]any)[op].(map[string]any)
		body := map[string]any{"adapterBlobOid": reg["adapterBlobOid"], "adapterFileSha256": reg["adapterSha256"], "adapterMode": "100755", "argv": []any{op}, "containmentClass": reg["containmentClass"], "executableQualification": "UNQUALIFIED", "exitCode": "0", "interpreterChain": []any{map[string]any{"fileSha256": reg["interpreterSha256"], "mode": reg["interpreterMode"], "pathSha256": Digest([]byte(fmt.Sprint(reg["interpreter"])))}}, "operation": op, "policyId": reg["expectedPolicyId"], "signal": nil, "state": "PASSED", "stderrBytes": "0", "stderrRawSha256": Digest(nil), "stdoutBytes": fmt.Sprint(inputs["bytes"]), "stdoutRawSha256": inputs["sha256"]}
		receipt, err := Identified("adapter-execution", "adapter-execution/0", "id", body)
		if err != nil {
			return nil, err
		}
		receipts = append(receipts, receipt)
	}
	observation, err := Identified("work-queue-observation", "work-queue-observation/0", "id", map[string]any{"adapterReceipts": receipts, "containmentClass": reg["containmentClass"], "detailIds": reg["expectedDetails"], "endCheckpoint": reg["expectedCheckpoint"], "mutationState": "UNKNOWN", "networkState": "HOST_UNOBSERVED", "policyId": reg["expectedPolicyId"], "profile": "work-queue-observation/0", "queueSourceId": reg["expectedQueueSourceId"], "snapshotId": reg["inputs"].(map[string]any)["snapshot"].(map[string]any)["id"], "startCheckpoint": reg["expectedCheckpoint"], "state": "UNKNOWN", "unknowns": reg["expectedUnknowns"]})
	if err != nil {
		return nil, err
	}
	proposal, err := Identified("work-wave-proposal", "work-wave-proposal/0", "id", map[string]any{"capacityEnvelopeId": reg["expectedEnvelopeId"], "collisionClosure": reg["expectedCollisionClosure"], "entries": []any{}, "mutationAuthority": false, "observationId": observation["id"], "profile": "work-wave-proposal/0", "queueSourceId": reg["expectedQueueSourceId"], "state": "EMPTY", "unknowns": reg["expectedUnknowns"], "waveLimit": "3", "waveOptimality": "NONE"})
	if err != nil {
		return nil, err
	}
	observe, err := Identified("work-command-result", "work-command-result/0", "id", map[string]any{"errorCode": nil, "observation": observation, "profile": "work-command-result/0", "proposal": nil, "state": "OK"})
	if err != nil {
		return nil, err
	}
	propose, err := Identified("work-command-result", "work-command-result/0", "id", map[string]any{"errorCode": nil, "observation": nil, "profile": "work-command-result/0", "proposal": proposal, "state": "OK"})
	if err != nil {
		return nil, err
	}
	return map[string]map[string]any{"observe": observe, "propose-wave": propose}, nil
}

func mustDigest(v any) string {
	b, err := Canonical(v)
	if err != nil {
		panic(err)
	}
	return Digest(b)
}
