package docmaintain

import (
	"strings"
	"testing"
)

const blindedKey = `{"profile":"corvint-docs-blinded-answer-key/0","answers":{"q1":["Preview"],"q2":["cmd/corvint","internal/companionrelease"]}}` //corvint:fixture-path

func blindedTasks(keySHA string) []byte {
	return []byte(`{"profile":"corvint-docs-blinded-task-set/0","reference":{"source":"s.md","package":"p","revision":"r"},` +
		`"answer_key_sha256":"` + keySHA + `","pass_numerator":1,"pass_denominator":1,` +
		`"questions":[{"id":"q1","text":"a"},{"id":"q2","text":"b"}]}`)
}

// SDD-V0-015: the scorer refuses a key the task set did not commit to, scores every required fact
// case-insensitively, and records a failing run as a result rather than an error.
func TestSDDV0015BlindedScoring(t *testing.T) {
	tasks := blindedTasks(digestHex([]byte(blindedKey)))
	if _, err := ScoreBlinded(blindedTasks(strings.Repeat("0", 64)), []byte(blindedKey), nil); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("a key the task set did not commit to was accepted: %v", err)
	}
	pass := `{"profile":"corvint-docs-blinded-responses/0","participant":"p1","condition":"generated-reference",` +
		`"answers":{"q1":"  preview  ","q2":"CMD/corvint and internal/companionrelease"}}` //corvint:fixture-path
	res, err := ScoreBlinded(tasks, []byte(blindedKey), []byte(pass))
	if err != nil || res.Verdict != "PASS" || res.Correct != 2 || res.Profile != BlindedResultProfile {
		t.Fatalf("pass run: %+v %v", res, err)
	}
	fail := `{"profile":"corvint-docs-blinded-responses/0","participant":"p2","condition":"generated-reference","answers":{"q2":"cmd/corvint"}}`
	res, err = ScoreBlinded(tasks, []byte(blindedKey), []byte(fail))
	if err != nil || res.Verdict != "FAIL" || res.Correct != 0 || len(res.Scores[1].Missing) != 1 || res.Scores[1].Missing[0] != "internal/companionrelease" { //corvint:fixture-path
		t.Fatalf("fail run: %+v %v", res, err)
	}
	if res.ResponsesSHA256 != digestHex([]byte(fail)) || res.TaskSetSHA256 != digestHex(tasks) {
		t.Fatal("the result does not bind its inputs")
	}
}
