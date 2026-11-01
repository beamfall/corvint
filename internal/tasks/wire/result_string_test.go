package wire

import "testing"

// encodedResult returns r's encoded envelope as a string and fails the test when
// Encode refuses it, so a test asserting on the bytes does not hand-convert the
// ([]byte, error) pair (V1-3783).
func encodedResult(t testing.TB, r Result) string {
	t.Helper()
	b, err := r.Encode()
	if err != nil {
		t.Fatalf("encode %v result: %v", r.Command, err)
	}
	return string(b)
}

func TestEncodedResultMatchesEncode(t *testing.T) {
	r := Result{Command: []string{"queue", "status"}, Outcome: OutcomeOK}
	want, err := r.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if got := encodedResult(t, r); got != string(want) {
		t.Fatalf("encodedResult = %s, want %s", got, want)
	}
}
