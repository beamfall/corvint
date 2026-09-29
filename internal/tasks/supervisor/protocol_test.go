package supervisor

import (
	"os"
	"testing"
)

func TestSupervisorNativeSelfIdentity(t *testing.T) {
	a, e := ProcessIdentity(os.Getpid())
	if e != nil || a == "" {
		t.Fatalf("self identity %q %v", a, e)
	}
	b, e := ProcessIdentity(os.Getpid())
	if e != nil || a != b {
		t.Fatalf("identity changed %q %q %v", a, b, e)
	}
	t.Logf("pid=%d start=%s", os.Getpid(), a)
}
func TestSupervisorImmutableAck(t *testing.T) {
	d := t.TempDir()
	b := Boot{Effect: "bound", PID: 1, Started: "start"}
	if e := Publish(d, "ack", Ack{Boot: b, Accepted: true}); e != nil {
		t.Fatal(e)
	}
	if e := Publish(d, "ack", Ack{}); e == nil {
		t.Fatal("ack overwrite")
	}
}

func TestSupervisorUsageUnknownAndBoundedOutput(t *testing.T) {
	i, o, known := ObservedUsage([]byte("{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":42,\"output_tokens\":7}}\n"))
	if !known || i != 42 || o != 7 {
		t.Fatal(i, o, known)
	}
	for _, raw := range []string{"", `{"type":"turn.completed"}`, `{"type":"turn.completed","usage":{"input_tokens":-1,"output_tokens":7}}`} {
		if _, _, known := ObservedUsage([]byte(raw)); known {
			t.Fatal("invented observed usage")
		}
	}
}
