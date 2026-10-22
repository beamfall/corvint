package corpusserve

import (
	json "encoding/json/v2"
	"io"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/doccorpus"
)

func postQuery(t *testing.T, server *httptest.Server, body string) (int, map[string]any) {
	t.Helper()
	response, e := server.Client().Post(server.URL+"/query", "application/json", strings.NewReader(body))
	if e != nil {
		t.Fatal(e)
	}
	data, e := io.ReadAll(response.Body)
	response.Body.Close()
	var out map[string]any
	if e != nil || json.Unmarshal(data, &out) != nil {
		t.Fatal(e, string(data))
	}
	return response.StatusCode, out
}

// TestDCPV1320And321HTTPRefusalAndGapKinds: an HTTP refusal envelope carries a null
// verified_at and the refusal limitation; gaps admits a bounded kinds filter with
// per-kind totals and refuses an empty one as a query refusal.
func TestDCPV1320And321HTTPRefusalAndGapKinds(t *testing.T) {
	server := httptest.NewServer(Handler(portable(t)))
	defer server.Close()
	status, refused := postQuery(t, server, `{"operation":"shell"}`)
	envelope, _ := refused["trust_envelope"].(map[string]any)
	verified, present := envelope["verified_at"]
	limitations, _ := envelope["limitations"].([]any)
	if status != 400 || !present || verified != nil || !slices.Contains(limitations, any(doccorpus.RefusalUnverifiedLimitation)) {
		t.Fatalf("refusal envelope: %d %v", status, refused)
	}
	status, gaps := postQuery(t, server, `{"operation":"gaps","kinds":["no-such-kind"]}`)
	totals, _ := gaps["kind_totals"].(map[string]any)
	if status != 200 || len(totals) != 1 || totals["no-such-kind"] != float64(0) {
		t.Fatalf("gaps kinds: %d %v", status, gaps)
	}
	if status, out := postQuery(t, server, `{"operation":"gaps","kinds":[]}`); status != 422 || out["code"] != "query-refused" {
		t.Fatalf("empty kinds admitted: %d %v", status, out)
	}
}
