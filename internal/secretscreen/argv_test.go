package secretscreen

import "testing"

func TestMatchArgvStructuralCredentials(t *testing.T) {
	for _, argv := range [][]string{{"x", "--password", "correct horse"}, {"x", "--db-token", "$LITERAL1"}, {"x", "--account_key=secret"}, {"curl", "-u", "user:pass word"}, {"docker", "login", "-p", "p word"}} {
		if !MatchArgv(argv) {
			t.Errorf("missed %+v", argv)
		}
	}
	for _, argv := range [][]string{{"git", "log", "-p", "revision123"}, {"x", "port", "8080"}, {"x", "{\"key\":\"value\"}"}, {"x", "opaque-base64-YWJjZA=="}, {"x", "--password"}} {
		if MatchArgv(argv) {
			t.Errorf("false match %+v", argv)
		}
	}
}
