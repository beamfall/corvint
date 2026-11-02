// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func conflictCreatePayload(title string) string {
	return `{"acceptanceCriteria":["x"],"body":"b","capabilities":[],"dependencies":[],"dueDate":null,` +
		`"effects":{"coverage":"QUALIFIED","externalUnbounded":false,"resources":[],"touchPaths":["a.go"]},` +
		`"estimateMinutes":null,"executionClass":"AUTONOMOUS","kind":"BUG","labels":["bugs"],"milestone":null,` +
		`"order":"1","owner":null,"priority":"P1","requiredGates":[],"requirementRefs":[],` +
		`"source":{"kind":"NATIVE","sourceItemId":null,"sourceQueueId":"` + fixture.QueueID + `","sourceRevisionSha256":null},` +
		`"supersededBy":null,"supersedes":null,"title":"` + title + `"}`
}

func conflictCreate(r *fixture.Repo, requestID, title string) []byte {
	return tcxRun(Env{Cwd: r.Root, Args: []string{"ticket", "create", "--request-id", requestID, "--payload-stdin"}, Stdin: strings.NewReader(conflictCreatePayload(title))})
}

var conflictDefaulted = regexp.MustCompile(`defaulted to [0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z`)

// TestLGLV0846ConflictOutputMatchesCompleteRoute: a retry whose request ID
// lies more than MaxWriterTail receipts before the writer checkpoint and
// whose bytes differ only in the defaulted issuedAt is refused with the same
// result, codes and warnings, explanation included, from the writer route's
// binding as from the complete route's audit. Only the defaulted second the
// two runs name differs.
func TestLGLV0846ConflictOutputMatchesCompleteRoute(t *testing.T) {
	r := fixture.TempRepo(t)
	fixture.WriteWritableState(t, r)
	fixture.WriteIntent(t, r, fixture.Ticket("A"))
	fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"intent/tickets/A.json": fixture.Ticket("A").Encode()})
	original := time.Now().UTC().Truncate(time.Second)
	if res, err := wire.DecodeResult(conflictCreate(r, "orig", "original")); err != nil || res.Outcome != wire.OutcomeOK {
		t.Fatalf("original create: %+v %v", res, err)
	}
	for n := range journal.MaxWriterTail + 140 {
		local := fmt.Sprintf("P-%d", n)
		fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"intent/tickets/" + local + ".json": fixture.Ticket(local).Encode()})
	}
	// A complete-route write retains the first writer checkpoint.
	if res, err := wire.DecodeResult(conflictCreate(r, "seed-checkpoint", "seed")); err != nil || res.Outcome != wire.OutcomeOK {
		t.Fatalf("seed create: %+v %v", res, err)
	}
	wcPath := journal.WriterCheckpointPath(r.StateDir)
	if _, err := os.Stat(wcPath); err != nil {
		t.Fatalf("no writer checkpoint retained: %v", err)
	}
	// The retry must default a later issuedAt than the original's.
	time.Sleep(time.Until(original.Add(1100 * time.Millisecond)))
	writer := conflictCreate(r, "orig", "original")
	if !bytes.Contains(writer, []byte("REQUEST_ID_CONFLICT")) || !bytes.Contains(writer, []byte("differs from the recorded request only in issuedAt")) {
		t.Fatalf("writer-route refusal: %s", writer)
	}
	if err := os.Remove(wcPath); err != nil {
		t.Fatal(err)
	}
	complete := conflictCreate(r, "orig", "original")
	if a, b := conflictDefaulted.ReplaceAll(writer, nil), conflictDefaulted.ReplaceAll(complete, nil); !bytes.Equal(a, b) {
		t.Fatalf("writer route:\n%s\ncomplete route:\n%s", writer, complete)
	}
}
