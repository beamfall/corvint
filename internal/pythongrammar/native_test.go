package pythongrammar

import "testing"

func TestNativeLexerPositionsAreOneBased(t *testing.T) {
	tokens, reason := LexPython312([]byte("import alpha\n"))
	if reason != "" || len(tokens) == 0 {
		t.Fatalf("native lexer=%s tokens=%#v", reason, tokens)
	}
	if tokens[0].line != 1 || tokens[0].column != 1 {
		t.Fatalf("native token position=%#v", tokens[0])
	}
}
