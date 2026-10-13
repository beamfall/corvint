package cemcandidate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stableFixture(t *testing.T, format string) (StableRequest, string) {
	t.Helper()
	r, out := fixture(t, format)
	// The reviewed stable prototype admits literal generated configurations in
	// one section order. Keep that narrower fixture qualification explicit.
	if format == "sha256" {
		p := filepath.Join(r.Repository, ".git/config")
		b, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		prefix := "[extensions]\n\tobjectformat = sha256\n"
		if strings.HasPrefix(string(b), prefix) {
			if e := os.WriteFile(p, []byte(strings.TrimPrefix(string(b), prefix)+prefix), 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
	snapshot := save(t, filepath.Join(realTemp(t), "snapshot.json"), []byte("{\"profile\":\"manufactured-snapshot-not-native-authority\"}\n"))
	return StableRequest{Profile: StableAssemblyProfile, Repository: r.Repository, ExpectedBase: r.ExpectedBase, Target: r.Target, TicketID: r.TicketID, AttemptID: r.AttemptID, SourcePrefix: r.SourcePrefix, SourceMap: r.SourceMap, Capture: r.Capture, Verification: r.Verification, SnapshotHeadArtifact: snapshot, RunnerPlan: r.RunnerPlan, RunnerReceipt: r.RunnerReceipt, Links: r.Links}, out
}
