package appmap

import "testing"

// TestLexJSComments proves comments are returned apart from the tokens, so a mention inside a
// comment never reads as code and annotations can be read from the comments alone (TSA-V0-011).
func TestLexJSComments(t *testing.T) {
	tokens, comments, unsure := LexJS("const a = 'x'; // corvint-lint-allow fixed-wait: why\n/* import './seed'; */\nf(() => a);\n")
	if unsure || len(comments) != 2 || comments[0].Line != 1 || comments[1].Line != 2 ||
		comments[0].Text != "// corvint-lint-allow fixed-wait: why" || comments[1].Text != "/* import './seed'; */" {
		t.Fatalf("comments = %+v unsure=%v", comments, unsure)
	}
	for _, token := range tokens {
		if token.Text == "import" || token.Text == "corvint" {
			t.Fatalf("comment text leaked into tokens: %+v", token)
		}
	}
	last := tokens[len(tokens)-1]
	arrow := false
	for i := 1; i < len(tokens); i++ {
		arrow = arrow || (tokens[i-1].Text == "=" && tokens[i].Text == ">" && tokens[i].Glued)
	}
	if !arrow || last.Kind != JSPunct || last.Text != ";" || last.Line != 3 || !tokens[3].Literal || tokens[3].Kind != JSString {
		t.Fatalf("tokens = %+v", tokens)
	}
	if _, _, unsure := LexJS("const a = 'unterminated\n"); !unsure {
		t.Fatal("an unterminated string was not unsure")
	}
}

// TestLexJSUnsureLine proves the lexer names the first line it cannot place, and 0 when it places
// everything (TSA-V0-648).
func TestLexJSUnsureLine(t *testing.T) {
	if _, _, line := LexJSUnsureLine("const a = 1;\nconst b = 'x';\n"); line != 0 {
		t.Fatalf("clean source unsure at line %d", line)
	}
	if _, _, line := LexJSUnsureLine("const a = 1;\nconst b = 2;\nconst c = 'open\n"); line != 3 {
		t.Fatalf("unterminated string unsure at line %d, want 3", line)
	}
	if _, _, line := LexJSUnsureLine("const a = 1;\n/* open\ncomment\n"); line != 2 {
		t.Fatalf("unterminated comment unsure at line %d, want 2", line)
	}
}
