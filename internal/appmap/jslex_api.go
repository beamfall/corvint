package appmap

import "strings"

// This file exposes the application map's JavaScript/TypeScript lexer to the static test-source
// analysis (docs/specs/test-source-analysis-v0.md, TSA-V0-010) without a second lexer: the tokens
// are the ones every application-map reader uses, so a lexing repair reaches both at once.

// JSTokenKind classifies one exported token.
type JSTokenKind int

const (
	JSIdent JSTokenKind = iota + 1
	JSString
	JSTemplate
	JSNumber
	JSPunct
)

// JSToken is one token: an identifier, punctuation character, number, or a string or template
// literal's decoded body. Literal is true only for a string or substitution-free template whose
// value is exact; a regular expression is a JSString that is never Literal. Glued marks
// punctuation written directly after the previous punctuation (the `>` of `=>`). Source keeps
// what Text loses: a regular expression's source, slashes and flags included, or a template with
// substitutions from its opening backtick, its substitution expressions written as in the file.
type JSToken struct {
	Kind    JSTokenKind
	Text    string
	Line    int
	Literal bool
	Glued   bool
	Source  string
}

// JSComment is one comment: the line it starts on and its text, delimiters included.
type JSComment struct {
	Line int
	Text string
}

// LexJS tokenizes JavaScript or TypeScript source. unsure reports that the lexer met source it
// cannot place (an unterminated literal or comment, a `/` it cannot classify), so the tokens may
// not show all the code and a reader that must see every construct fails closed.
func LexJS(text string) (tokens []JSToken, comments []JSComment, unsure bool) {
	tokens, comments, line := LexJSUnsureLine(text)
	return tokens, comments, line != 0
}

// LexJSUnsureLine is LexJS reporting, instead of unsure, the first line on which the lexer met
// source it cannot place (0 when it placed everything), so a reader that fails closed can name it.
func LexJSUnsureLine(text string) (tokens []JSToken, comments []JSComment, unsureLine int) {
	collected := []jsComment{}
	toks, _ := lexJSComments(text, &collected)
	tokens = make([]JSToken, 0, len(toks))
	for _, t := range toks {
		kind := map[tokKind]JSTokenKind{tokIdent: JSIdent, tokString: JSString, tokTemplate: JSTemplate, tokNumber: JSNumber, tokPunct: JSPunct}[t.kind]
		source := t.regex
		if t.kind == tokTemplate && t.subst {
			prefix, _, _ := strings.Cut(t.text, substMark)
			source = "`" + prefix + t.code
		}
		tokens = append(tokens, JSToken{Kind: kind, Text: t.text, Line: t.line, Literal: literal(t), Glued: t.glued, Source: source})
		at := 0
		switch {
		case t.doubtLine != 0:
			at = t.doubtLine
		case t.unsure:
			at = t.line
		}
		if at != 0 && (unsureLine == 0 || at < unsureLine) {
			unsureLine = at
		}
	}
	comments = make([]JSComment, 0, len(collected))
	for _, c := range collected {
		comments = append(comments, JSComment{Line: c.line, Text: c.text})
	}
	return tokens, comments, unsureLine
}
