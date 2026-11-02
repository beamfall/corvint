// SPDX-License-Identifier: Apache-2.0

package contextindex

import (
	"context"
	"reflect"
	"testing"
)

// GPK-V0-1950: decoding requires a complete, valid JSON boundary.
func TestTaskScanTextDecodesOnlyAnEscapedTask_GPKV01950(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, in, want string }{
		{"escaped body", `"**Title**\n\nThe \"DirectedGraph\" path\tbreaks\nnow\\n stays"`, "**Title**\n\nThe \"DirectedGraph\" path\tbreaks\nnow\\n stays"},
		{"quoted escape is not serialization", `print("a\nb") fails`, `print("a\nb") fails`},
		{"a raw line break keeps every backslash", "Title\nprintf(\"a\\nb\\n\") fails", "Title\nprintf(\"a\\nb\\n\") fails"},
		{"unserialized escapes are kept", `C:\data\nodes\n\u00e9 end\`, `C:\data\nodes\n\u00e9 end\`},
		{"Windows path", `Fix C:\newParser\nodeCache`, `Fix C:\newParser\nodeCache`},
		{"regex", `Fix regex \newParser\nodeCache`, `Fix regex \newParser\nodeCache`},
		{"quoted code", `Fix print("a\nnewParser\nnodeCache")`, `Fix print("a\nnewParser\nnodeCache")`},
		{"unquoted escaped body is ambiguous", `Title\n\nopenLedgerFile`, `Title\n\nopenLedgerFile`},
		{"invalid string literal", `"C:\newParser\nodeCache\q"`, `"C:\newParser\nodeCache\q"`},
		{"unicode", `"Fix \u006fpenLedgerFile"`, "Fix openLedgerFile"},
		{"envelope values", `{"EnvelopeField":"Fix openLedgerFile\n\nagain"}`, "Fix openLedgerFile\n\nagain"},
	} {
		if got := taskScanText(test.in); got != test.want {
			t.Errorf("%s: taskScanText(%q) = %q, want %q", test.name, test.in, got, test.want)
		}
	}
}

// Ordinary task text must keep its literal backslashes even when it contains
// multiple newline-like sequences. Check the searched and credited identifiers.
func TestTaskLexicalChannelPreservesLiteralEscapes_GPKV01950(t *testing.T) {
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, map[string]string{
		"go.mod":       "module example.test/channel\n\ngo 1.27.0\n",
		"literal.go":   "package sample\n// newParser nodeCache nnewParser nnodeCache\n",
		"corrupted.go": "package sample\n// ewParser odeCache\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"", "ident"} {
		t.Run("terms="+mode, func(t *testing.T) {
			t.Setenv("CORVINT_CONTEXT_TERMS", mode)
			for _, test := range []struct {
				name, task string
				want       []string
			}{
				{"Windows path", `Fix C:\newParser\nodeCache`, []string{"newParser", "nodeCache"}},
				{"regex", `Fix regex \newParser\nodeCache`, []string{"newParser", "nodeCache"}},
				{"quoted code", `Fix print("a\nnewParser\nnodeCache")`, []string{"nnewParser", "nnodeCache"}},
			} {
				t.Run(test.name, func(t *testing.T) {
					channel, err := TaskLexicalChannel(context.Background(), index, test.task, 5)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(channel.Identifiers, test.want) {
						t.Fatalf("searched %v, want %v", channel.Identifiers, test.want)
					}
					if len(channel.Candidates) == 0 || channel.Candidates[0].Path != "literal.go" || !reflect.DeepEqual(channel.Candidates[0].Identifiers, test.want) {
						t.Fatalf("candidates = %+v, want literal.go first naming %v", channel.Candidates, test.want)
					}
				})
			}
		})
	}
}

// JSON extraction precedes lexical scanning: keys never become identifiers,
// unicode is decoded, and literal backslashes in decoded values stay literal.
func TestTaskLexicalChannelExtractsJSONValues_GPKV01950(t *testing.T) {
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, map[string]string{
		"go.mod":      "module example.test/channel\n\ngo 1.27.0\n",
		"ledger.go":   "package sample\nfunc openLedgerFile() {}\n",
		"envelope.go": "package sample\nfunc EnvelopeField() {}\n",
		"path.go":     "package sample\n// newParser nodeCache\n",
		"code.go":     "package sample\n// nopenLedgerFile\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"", "ident"} {
		t.Run("terms="+mode, func(t *testing.T) {
			t.Setenv("CORVINT_CONTEXT_TERMS", mode)
			for _, test := range []struct {
				name, task, path string
				want             []string
			}{
				{"escaped envelope value", `{"EnvelopeField":"Fix openLedgerFile\n\nagain"}`, "ledger.go", []string{"openLedgerFile"}},
				{"unicode envelope value", `{"EnvelopeField":"Fix \u006fpenLedgerFile\n\nagain"}`, "ledger.go", []string{"openLedgerFile"}},
				{"unicode string", `"Fix \u006fpenLedgerFile"`, "ledger.go", []string{"openLedgerFile"}},
				{"serialized Windows path", `"Fix C:\\newParser\\nodeCache"`, "path.go", []string{"newParser", "nodeCache"}},
				{"quoted code value decoded once", `{"EnvelopeField":"\"a\\nopenLedgerFile\\n\""}`, "code.go", []string{"nopenLedgerFile"}},
			} {
				t.Run(test.name, func(t *testing.T) {
					channel, err := TaskLexicalChannel(context.Background(), index, test.task, 5)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(channel.Identifiers, test.want) {
						t.Fatalf("searched %v, want %v", channel.Identifiers, test.want)
					}
					for _, candidate := range channel.Candidates {
						if candidate.Path == "envelope.go" {
							t.Fatalf("envelope key admitted candidate: %+v", candidate)
						}
					}
					if len(channel.Candidates) == 0 || channel.Candidates[0].Path != test.path || !reflect.DeepEqual(channel.Candidates[0].Identifiers, test.want) {
						t.Fatalf("candidates = %+v, want %s first naming %v", channel.Candidates, test.path, test.want)
					}
				})
			}
		})
	}
}

// GPK-V0-1950: the lexical channel of an escaped task searches for the
// identifier the task wrote and discloses no `n`-prefixed artefact.
func TestTaskLexicalChannelDecodesAnEscapedTask_GPKV01950(t *testing.T) {
	t.Parallel()
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, map[string]string{
		"go.mod":          "module example.test/channel\n\ngo 1.27.0\n",
		"store/ledger.go": "package store\n\n// openLedgerFile opens the ledger.\nfunc openLedgerFile() {}\n",
		"store/cache.go":  "package store\n\nfunc warm() {}\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	channel, err := TaskLexicalChannel(context.Background(), index, `"## Title\n\nThe ledger opens twice\n\nopenLedgerFile runs again"`, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(channel.Identifiers, []string{"openLedgerFile"}) {
		t.Fatalf("channel searched %v, want only openLedgerFile", channel.Identifiers)
	}
	if len(channel.Candidates) == 0 || channel.Candidates[0].Path != "store/ledger.go" || !reflect.DeepEqual(channel.Candidates[0].Identifiers, []string{"openLedgerFile"}) {
		t.Fatalf("candidates = %+v, want store/ledger.go first naming openLedgerFile", channel.Candidates)
	}
}
