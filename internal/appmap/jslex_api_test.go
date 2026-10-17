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
