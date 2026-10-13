package console

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// sealStableFixture commits the packet's positive cem/1.0 map as a sealed
// change: the packet target is the bind commit (it already holds the map at
// .corvint/change.cem.json), and one further commit moves it under
// .corvint/changes/<bind>.cem.json the way script/dogfood-seal.sh does. When
// rebind is set, an extra commit first replaces the bound map so the sealed
// file's name no longer matches its bind commit's bytes.
func sealStableFixture(t *testing.T, sealed []byte, rebind bool) *chainFixture {
	t.Helper()
	fixture := materializeStableRead(t)
	root := fixture.repo
	chainGit(t, root, "reset", "-q", "--hard", fixture.target)
	change := fixture.target
	if rebind {
		chainWrite(t, root, boundMapPath, []byte("{}\n"))
		chainGit(t, root, "commit", "-q", "--no-gpg-sign", "-am", "rebind")
		change = chainGit(t, root, "rev-parse", "HEAD")
	}
	chainGit(t, root, "rm", "-q", boundMapPath)
	chainWrite(t, root, changesDir+"/"+change+".cem.json", sealed)
	chainGit(t, root, "add", ".")
	chainGit(t, root, "commit", "-q", "--no-gpg-sign", "-m", "seal")
	return &chainFixture{root: root, base: fixture.expectedBase, change: change, cem: sealed}
}

// stableAppHunk is the packet's supported app.txt hunk, which carries a caller
// discrimination witness with one survived mutant.
const stableAppHunk = "hunk:sha256:e4321d308c1625258ebeb739d317cf47c9d4b28291be46203144c6e7cac5be06"

func stablePacketMap(t *testing.T) []byte {
	t.Helper()
	return materializeStableRead(t).raw
}

// TestConsoleChainStableDisplay covers CEM-V1-077 and CEM-V1-078: a sealed
// cem/1.0 map renders its references as linked rows that each say a valid
// reference is not task completion, shows the unverified envelope, the
// NOT_CHECKED integrity axes, the NOT_OBSERVED authority, criterion and runner
// axes, the sidecar state and the caller's survived discrimination state, and
// opens no artifact.
func TestConsoleChainStableDisplay(t *testing.T) {
	fixture := sealStableFixture(t, stablePacketMap(t), false)
	page := mainOf(fixture.page(t, ""))
	for _, want := range []string{
		"cem/1.0 verification", "stable verification is NOT_RUN here", "verify-stable",
		"sidecar state NOT_CHECKED",
		"changeIntegrity NOT_CHECKED", "referenceIntegrity NOT_CHECKED",
		"nativeAuthority NOT_OBSERVED", "criterionAdequacy NOT_OBSERVED", "criterionDiscrimination NOT_OBSERVED",
		"runnerExecution NOT_OBSERVED", "authentication NOT_OBSERVED",
		"criterionBindings[0]", "runnerReceipts[0]", "criterionLinks[0]", "artifacts[5]",
		"executionAuthority CALLER_OBSERVED", "(content not opened)",
		"a valid reference is not task completion and not criterion satisfaction",
		"task completion NOT_OBSERVED",
		"pinned <code>sha256:" + sha256Hex(string(fixture.cem)) + "</code>",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("stable chain page lacks %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, "gap: stale") || strings.Contains(page, "gap: missing</b></td>\n<td><code>"+fixture.change) {
		t.Fatalf("an exact seal rendered stale or missing:\n%s", page)
	}
	hunk := mainOf(fixture.page(t, "&hunk="+stableAppHunk))
	if !strings.Contains(hunk, "survived 1") || !strings.Contains(hunk, "not a current passing test") {
		t.Fatalf("the stable hunk detail lost the caller's survived witness state:\n%s", hunk)
	}
	if strings.Contains(page, "artifacts[6]") {
		t.Fatal("the pane invented an artifact reference")
	}
}

// TestConsoleChainStableStale covers CEM-V1-078: a sealed stable map whose
// bind commit holds different bytes is drawn as stale, not as a valid change.
func TestConsoleChainStableStale(t *testing.T) {
	fixture := sealStableFixture(t, stablePacketMap(t), true)
	page := mainOf(fixture.page(t, ""))
	if !strings.Contains(page, "gap: stale") {
		t.Fatalf("a rebound stable seal was not stale:\n%s", page)
	}
}

// TestConsoleChainStableRegistry covers CEM-V1-079 and CEM-V1-210: the profile
// registry is closed to cem/1.0. An unsupported profile and a cem/1.0 map the
// strict stable reader refuses draw no chain, and a retired cem/0.x map is
// refused with unsupported-spec and the recovery line.
func TestConsoleChainStableRegistry(t *testing.T) {
	raw := string(stablePacketMap(t))
	for name, text := range map[string]string{
		"future profile":        strings.Replace(raw, `"spec": "cem/1.0"`, `"spec": "cem/2.0"`, 1),
		"near stable profile":   strings.Replace(raw, `"spec": "cem/1.0"`, `"spec": "cem/1.0.0"`, 1),
		"invalid stable map":    strings.Replace(raw, `"executionAuthority": "CALLER_OBSERVED"`, `"executionAuthority": "VERIFIED"`, 1),
		"unknown stable member": strings.Replace(raw, `"spec": "cem/1.0"`, `"spec": "cem/1.0", "extra": true`, 1),
		"case-variant 0.x spec": strings.Replace(raw, `"spec": "cem/1.0"`, `"spec": "cem/1.0", "Spec": "cem/0.3"`, 1),
		"duplicate 0.x spec":    strings.Replace(raw, `"spec": "cem/1.0"`, `"spec": "cem/1.0", "spec": "cem/0.3"`, 1),
		"duplicate 1.0 spec":    `{"spec": "cem/1.0", "spec": "cem/0.3", "baseRevision": "` + strings.Repeat("a", 40) + `"}`,
		"case-variant 1.0 spec": `{"spec": "cem/0.3", "Spec": "cem/1.0", "baseRevision": "` + strings.Repeat("a", 40) + `"}`,
	} {
		if text == raw {
			t.Fatalf("%s: the edit did not apply", name)
		}
		if _, err := decodeSealed(&Blob{Text: text}); err == nil || !strings.HasPrefix(err.Error(), GapUnsupported) {
			t.Fatalf("%s: decodeSealed = %v, want an unsupported refusal", name, err)
		}
	}
	if _, err := decodeSealed(&Blob{Text: raw}); err != nil {
		t.Fatalf("the exact stable map was refused: %v", err)
	}
	for _, spec := range []string{"cem/0.1", "cem/0.2", "cem/0.3"} {
		_, err := decodeSealed(&Blob{Text: `{"spec":"` + spec + `","baseRevision":"` + strings.Repeat("a", 40) + `"}`})
		if err == nil || !strings.HasPrefix(err.Error(), GapUnsupported) || !strings.Contains(err.Error(), "unsupported-spec") ||
			!strings.Contains(err.Error(), wire.RetiredSpecFix) {
			t.Fatalf("a retired %s map was not refused with the recovery line: %v", spec, err)
		}
	}
}

// TestCEMV1084ConsoleReceiptAuthentication proves the chain pane shows a
// dsse-ed25519 receipt's keyid and envelope as references it neither checks
// nor trusts, keeps the authentication axis NOT_OBSERVED, and opens no
// receipt-attestation artifact.
func TestCEMV1084ConsoleReceiptAuthentication(t *testing.T) {
	var m map[string]any
	if err := json.Unmarshal(stablePacketMap(t), &m); err != nil {
		t.Fatal(err)
	}
	keyID, envelope := strings.Repeat("4", 64), strings.Repeat("e", 64)
	m["runnerReceipts"].([]any)[0].(map[string]any)["authentication"] = map[string]any{"method": "dsse-ed25519", "keyid": keyID, "envelopeSha256": envelope}
	m["artifacts"] = append(m["artifacts"].([]any), map[string]any{"kind": "receipt-attestation", "path": "receipt-attestation.json", "sha256": envelope})
	sealed, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wire.ParseStable(sealed); err != nil {
		t.Fatalf("signed packet map does not parse: %v", err)
	}
	page := mainOf(sealStableFixture(t, append(sealed, '\n'), false).page(t, ""))
	for _, want := range []string{
		"authentication dsse-ed25519 keyid " + keyID + " envelope " + envelope + " (signature not checked and keyid not trusted in this pane)",
		"authentication NOT_OBSERVED in this pane: the pane trusts no key",
		"a signature never raises executionAtCommit",
		"artifacts[6]", "receipt-attestation artifact sha256:" + envelope + " (content not opened)",
	} {
		if !strings.Contains(page, want) {
			t.Fatalf("stable chain page lacks %q:\n%s", want, page)
		}
	}
	if strings.Contains(page, "authentication VERIFIED") || strings.Contains(page, "UNTRUSTED_KEY") {
		t.Fatal("the pane reported a receipt signature state it never checks")
	}
}
