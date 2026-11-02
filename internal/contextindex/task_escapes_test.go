// SPDX-License-Identifier: Apache-2.0

package contextindex

import (
	"context"
	"reflect"
	"testing"
)

// GPK-V0-1950: only a task with the escaped-string signature is decoded.
func TestTaskScanTextDecodesOnlyAnEscapedTask_GPKV01950(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, in, want string }{
		{"escaped body", `"**Title**\n\nThe \"DirectedGraph\" path\tbreaks\nnow\\n stays"`, "\"**Title**\n\nThe \"DirectedGraph\" path\tbreaks\nnow\\n stays\""},
		{"one escape is not the signature", `print("a\nb") fails`, `print("a\nb") fails`},
		{"a raw line break keeps every backslash", "Title\nprintf(\"a\\nb\\n\") fails", "Title\nprintf(\"a\\nb\\n\") fails"},
		{"unknown escapes are kept", `C:\data\nodes\n\u00e9 end\`, "C:\\data\nodes\n\\u00e9 end\\"},
	} {
		if got := taskScanText(test.in); got != test.want {
			t.Errorf("%s: taskScanText(%q) = %q, want %q", test.name, test.in, got, test.want)
		}
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
