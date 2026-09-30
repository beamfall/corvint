package dogfoodflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const queryAbstentionReason = "authority-start-trace-state-abstention"
const queryAbstentionEnvelope = "{\"code\": \"unsupported-query-trace-state\", \"error\": \"native Go authority-start query requires an absent clean-tree local trace store\", \"ok\": false}\n"
const queryStep = "coordination-time-query"

// coordinationQuery preserves the original task bytes. Only the intentional
// authority-start refusal qualifies; another error with the same code does not.
func (c *change) coordinationQuery(task string) {
	c.queryAbstentionSHA = ""
	stem := c.evidence + "/" + queryStep
	failed := func() { c.addStep(queryStep, "NOT_PRODUCED", "query-abstention-evidence-failed") }
	if removeFile(stem+"-abstention.json") != nil {
		failed()
		return
	}
	argv := []string{c.options.Steps.Path, "--root", c.root, "query", "--task", task, "--limit", "1"}
	rawArgv := []byte(strings.Join(argv, "\x00") + "\x00")
	if _, ok := queryTask(rawArgv, c.root); !ok || writePrivate(stem+".argv", rawArgv) != nil {
		failed()
		return
	}
	status := c.exec(argv[3:], stem+".json", stem+".stderr")
	if status == 0 {
		c.addStep(queryStep, "PRODUCED", "none")
		return
	}
	stdout, outErr := os.ReadFile(stem + ".json")
	stderr, errErr := os.ReadFile(stem + ".stderr")
	if outErr != nil || errErr != nil {
		failed()
		return
	}
	if status != 2 || len(stdout) != 0 || string(stderr) != queryAbstentionEnvelope {
		reason := failureReason(stderr, status)
		if reason == queryAbstentionReason {
			reason = "query-abstention-invalid"
		}
		c.addStep(queryStep, "NOT_PRODUCED", reason)
		return
	}
	record := queryAbstentionArtifact(rawArgv, task, c.base, c.target, stdout, stderr)
	if writePrivate(stem+"-abstention.json", record) != nil {
		failed()
		return
	}
	c.queryAbstentionSHA = sha256Hex(record)
	c.addStep(queryStep, "NOT_PRODUCED", queryAbstentionReason)
}

func queryAbstentionArtifact(argv []byte, task, base, target string, stdout, stderr []byte) []byte {
	// Canonical exact bytes also reject duplicate/extra members and alternate
	// encodings during verification; all interpolated values are immutable hashes.
	return []byte(fmt.Sprintf("{\"argvSha256\":\"sha256:%s\",\"base\":\"%s\",\"exitStatus\":\"2\",\"profile\":\"corvint-dogfood-query-abstention/0\",\"reason\":\"%s\",\"status\":\"NOT_PRODUCED\",\"stderrSha256\":\"sha256:%s\",\"stdoutSha256\":\"sha256:%s\",\"step\":\"%s\",\"target\":\"%s\",\"taskSha256\":\"sha256:%s\"}\n", sha256Hex(argv), base, queryAbstentionReason, sha256Hex(stderr), sha256Hex(stdout), queryStep, target, sha256Hex([]byte(task))))
}

// queryTask validates the recorded invocation without executing any of it.
// Replay constructs a fixed command through separately selected verifiers.
func queryTask(data []byte, root string) (string, bool) {
	if len(data) == 0 || data[len(data)-1] != 0 {
		return "", false
	}
	argv := strings.Split(string(data[:len(data)-1]), "\x00")
	if len(argv) != 8 || argv[0] == "" || argv[1] != "--root" || !sameDirectory(argv[2], root) || argv[3] != "query" || argv[4] != "--task" || argv[5] == "" || argv[6] != "--limit" || argv[7] != "1" {
		return "", false
	}
	return argv[5], true
}

func (c *change) rowFailing(row step) bool {
	if row.reason == queryAbstentionReason {
		return row != (step{queryStep, "NOT_PRODUCED", queryAbstentionReason}) || c.queryAbstentionSHA == ""
	}
	return failing(row)
}

// uniqueObject rejects duplicate keys before decoding the report fields used
// here; json.Unmarshal alone silently keeps the last value.
func uniqueObject(data []byte) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, fmt.Errorf("expected object")
	}
	result := map[string]json.RawMessage{}
	for d.More() {
		token, err = d.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("invalid key")
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate key")
		}
		var raw json.RawMessage
		if err := d.Decode(&raw); err != nil {
			return nil, err
		}
		result[key] = raw
	}
	if _, err := d.Token(); err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing value")
	}
	return result, nil
}

// checkQueryAbstention preserves reports predating the optional digest member.
// Every claimed exemption requires exactly one matching row and retained bytes.
func (c *check) checkQueryAbstention(report []byte) (string, bool) {
	fail := func() { c.fail("query-abstention-evidence-drift") }
	object, err := uniqueObject(report)
	if err != nil {
		fail()
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(object["steps"], &rows); err != nil {
		fail()
	}
	var digest string
	rawDigest, present := object["queryAbstentionEvidenceSha256"]
	if present && string(rawDigest) != "null" {
		if json.Unmarshal(rawDigest, &digest) != nil || digest == "" {
			fail()
		}
	}
	queryCount, claimed := 0, false
	for _, raw := range rows {
		members, err := uniqueObject(raw)
		if err != nil || len(members) != 3 {
			fail()
		}
		var row step
		if json.Unmarshal(members["name"], &row.name) != nil || json.Unmarshal(members["status"], &row.status) != nil || json.Unmarshal(members["reason"], &row.reason) != nil {
			fail()
		}
		if row.name == queryStep {
			queryCount++
		}
		if row == (step{queryStep, "NOT_PRODUCED", queryAbstentionReason}) {
			claimed = true
			continue
		}
		if row.reason == queryAbstentionReason || failing(row) {
			fail()
		}
	}
	stem := c.evidence + "/" + queryStep
	if queryCount > 1 {
		fail()
	}
	if !claimed {
		if digest != "" || exists(stem+"-abstention.json") {
			fail()
		}
		return "", false
	}
	if queryCount != 1 || digest == "" {
		fail()
	}
	files := make([][]byte, 4)
	for i, suffix := range []string{".argv", ".json", ".stderr", "-abstention.json"} {
		if !isRegular(stem+suffix) || isSymlink(stem+suffix) {
			fail()
		}
		data, err := os.ReadFile(stem + suffix)
		if err != nil {
			fail()
		}
		files[i] = data
	}
	argv, stdout, stderr, artifact := files[0], files[1], files[2], files[3]
	task, ok := queryTask(argv, c.root)
	if !ok || len(stdout) != 0 || string(stderr) != queryAbstentionEnvelope || digest != "sha256:"+sha256Hex(artifact) || !bytes.Equal(artifact, queryAbstentionArtifact(argv, task, c.base, c.target, stdout, stderr)) {
		fail()
	}
	return task, true
}

func (c *check) verifyQueryAbstention(task string) {
	result, agreed := c.verifyAll("query", "--task", task, "--limit", "1")
	if !agreed || result.status != 2 || len(result.stdout) != 0 || string(result.stderr) != queryAbstentionEnvelope {
		c.fail("verifier-disagreement")
	}
	c.say("dogfood-check: NOTE %s NOT_PRODUCED %s\n", queryStep, queryAbstentionReason)
}
