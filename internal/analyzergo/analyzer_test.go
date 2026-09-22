package analyzergo

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	json "encoding/json/v2"
	"fmt"
	"go/build"
	"go/build/constraint"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

const frozenRequest = `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"input-1","family":"go.mod","path":"go.mod","sha256":"sha256:8d27e4f4ff6e3f8c1a8ced8f550a286065c62bff008c069e5f4c12061a00fa77","content_base64":"bW9kdWxlIGV4YW1wbGUuY29tL21vZHVsZQpnbyAxLjI3LjAK"}]}`
const frozenSuccess = `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","status":"CANDIDATE","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input-1","sha256":"sha256:8d27e4f4ff6e3f8c1a8ced8f550a286065c62bff008c069e5f4c12061a00fa77"}],"facts":[{"kind":"go.language.declaration","input_handle":"input-1","related_handle":"-","subject":"go","predicate":"declares-language","value":"1.27.0","instance_id":"root","evidence_sha256":"sha256:5b07b4320d23c64b15e5bbf8553f5fd91c95b27900b513bf1d10b1051f754ffe"},{"kind":"go.module","input_handle":"input-1","related_handle":"-","subject":"root","predicate":"declares-module","value":"example.com/module","instance_id":"root","evidence_sha256":"sha256:486362f6f76464651f690778f7901c5a7aa1dd42da15b094b014685fc97e0520"}]}`
const frozenBoundDigestMismatch = `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","status":"REJECTED","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"input-1","sha256":"sha256:0000000000000000000000000000000000000000000000000000000000000000"}],"reason":"DIGEST_MISMATCH"}`
const frozenConflictingValueRequest = `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"a-mod","family":"go.mod","path":"go.mod","sha256":"sha256:9d4860b5b5ed02ed03986fa968154e6d86509f676e5ef88a8afb08c12e79e664","content_base64":"bW9kdWxlIGV4YW1wbGUuY29tL2NvbmZsaWN0CmdvIDEuMjcuMAo="},{"handle":"b-sum","family":"go.sum","path":"go.sum","sha256":"sha256:6f61aa855a76aad15fec6932e82b7e1cdd111e23f00c4af07263b81be42bfaf2","content_base64":"ZXhhbXBsZS5jb20vY29uZmxpY3QgdjEuMC4wIGgxOkFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUE9CmV4YW1wbGUuY29tL2NvbmZsaWN0IHYxLjAuMCBoMTpBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBPQo="}]}`
const frozenBoundConflictingValue = `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","status":"REJECTED","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"a-mod","sha256":"sha256:9d4860b5b5ed02ed03986fa968154e6d86509f676e5ef88a8afb08c12e79e664"},{"handle":"b-sum","sha256":"sha256:6f61aa855a76aad15fec6932e82b7e1cdd111e23f00c4af07263b81be42bfaf2"}],"reason":"CONFLICTING_VALUE"}`
const frozenMixedWorkspaceRequest = `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[{"handle":"a-mod","family":"go.mod","path":"go.mod","sha256":"sha256:1f5a90c34786f7ca037858a0f848f6cb0343f606906075d1bc25fb7e9fd24ff4","content_base64":"bW9kdWxlIGV4YW1wbGUuY29tL21vZHVsZQpnbyAxLjI2LjAKdG9vbGNoYWluIGdvMS4yNi4wCg=="},{"handle":"c-work","family":"go.work","path":"go.work","sha256":"sha256:61ee7afbf62e25a9ca3d418512cbdeb84f50ad3f4aae4e06fa8aed16150fedf9","content_base64":"Z28gMS4yNy4wCnRvb2xjaGFpbiBnbzEuMjcuMAp1c2Ugd29ya3NwYWNlYQo="}]}`
const frozenMixedWorkspaceSuccess = `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","status":"CANDIDATE","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"a-mod","sha256":"sha256:1f5a90c34786f7ca037858a0f848f6cb0343f606906075d1bc25fb7e9fd24ff4"},{"handle":"c-work","sha256":"sha256:61ee7afbf62e25a9ca3d418512cbdeb84f50ad3f4aae4e06fa8aed16150fedf9"}],"facts":[{"kind":"go.language.declaration","input_handle":"a-mod","related_handle":"-","subject":"go","predicate":"declares-language","value":"1.26.0","instance_id":"root","evidence_sha256":"sha256:e0dcbdcb5e473a5c60b397c4aca821acc62bb374a06944180c9684166097e89a"},{"kind":"go.language.declaration","input_handle":"c-work","related_handle":"-","subject":"go","predicate":"declares-language","value":"1.27.0","instance_id":"root","evidence_sha256":"sha256:acc5d164d8990c03d9208126ddbc7515df737335656023aa408802250c7a71b5"},{"kind":"go.module","input_handle":"a-mod","related_handle":"-","subject":"root","predicate":"declares-module","value":"example.com/module","instance_id":"root","evidence_sha256":"sha256:71ca744ef761ab1b39dd1e2100cfea711092011494606212c8c80b0a009a27c6"},{"kind":"go.toolchain.declaration","input_handle":"a-mod","related_handle":"-","subject":"go","predicate":"declares-toolchain","value":"go1.26.0","instance_id":"root","evidence_sha256":"sha256:b60e7fb901d86d0c6809a83ea9465f8fedd2266eac2e0755ed46ba2fb37f95da"},{"kind":"go.toolchain.declaration","input_handle":"c-work","related_handle":"-","subject":"go","predicate":"declares-toolchain","value":"go1.27.0","instance_id":"root","evidence_sha256":"sha256:f7d982175e208625e43d90b33ed86886c73319abdf35709dd18b94692a5fd59f"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspacea","instance_id":"workspacea","evidence_sha256":"sha256:35c0307773315c97bab8c9322baa636f384b2e1a7efb2fce27347cff04274be2"}]}`

// frozenThirtyTwoSuccess is the literal, LF-framed, canonical response for
// the exact four-input fixture below. Keep this independent of candidate
// construction so an ordering or evidence mutation is visible as a byte diff.
const frozenThirtyTwoSuccess = `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","status":"CANDIDATE","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"input_echoes":[{"handle":"a-mod","sha256":"sha256:a3bce41949496c77e999153621562e22087363151535ebf6da3a035ac8afa47c"},{"handle":"b-sum","sha256":"sha256:c9c8a568801ec8001e29d52697f1adba5e8614cdb5a1d31b84919c8763f6a0d9"},{"handle":"c-work","sha256":"sha256:5d06084a50641f8d581ecdd32de861a4f62ee9890bd2a467135a66a8733de288"},{"handle":"d-source","sha256":"sha256:52199cf3508f68eaa48e695bdaa4e9062d65cc82dde933e3331c8f38598b179b"}],"facts":[{"kind":"go.build.constraint","input_handle":"d-source","related_handle":"-","subject":"cmd/main_darwin_arm64_test.go","predicate":"selected-for","value":"darwin && arm64","instance_id":"unit-1","evidence_sha256":"sha256:903afed69f21e36e59138af9a9b8d0f8864df7cfc8e864cef55bb7d7ea6df856"},{"kind":"go.dependency.locked","input_handle":"a-mod","related_handle":"b-sum","subject":"example.com/dependency","predicate":"locked-at","value":"v1.0.0","instance_id":"example.com/dependency@v1.0.0","evidence_sha256":"sha256:7e733ebf706d385e05c9e561238993836be532e0df99ec3cacc1a75b50c30cf1"},{"kind":"go.import.static","input_handle":"d-source","related_handle":"-","subject":"cmd/main_darwin_arm64_test.go","predicate":"imports","value":"example.com/imported","instance_id":"cmd/main_darwin_arm64_test.go","evidence_sha256":"sha256:29fe3e2b2b8e8a4eda1acc5a476d222d1c6a5f3a553db56d2c7a458e2f421d5f"},{"kind":"go.language.declaration","input_handle":"a-mod","related_handle":"-","subject":"go","predicate":"declares-language","value":"1.27.0","instance_id":"root","evidence_sha256":"sha256:55afbc5d64bec57357b683ac23afdb106649380098ebecc56e0e4b73db7436db"},{"kind":"go.language.declaration","input_handle":"c-work","related_handle":"-","subject":"go","predicate":"declares-language","value":"1.27.0","instance_id":"root","evidence_sha256":"sha256:bc043934d264c7407f35d69a1334c3f6d59b59b7ab74c125f36fbdf75e52b550"},{"kind":"go.module","input_handle":"a-mod","related_handle":"-","subject":"root","predicate":"declares-module","value":"example.com/fixture","instance_id":"root","evidence_sha256":"sha256:4c9b876f1e1e4620ce9d9b4cc9faf25335d4527de7397be3b21fe066c3ffe647"},{"kind":"go.package","input_handle":"d-source","related_handle":"-","subject":"main","predicate":"declares-package","value":"main","instance_id":"unit-1","evidence_sha256":"sha256:6be588318732028fc98a86f454fd44b0a65937dcfa497cacb8b87966b0b394c2"},{"kind":"go.source","input_handle":"d-source","related_handle":"-","subject":"cmd/main_darwin_arm64_test.go","predicate":"classifies","value":"test","instance_id":"cmd/main_darwin_arm64_test.go","evidence_sha256":"sha256:b2867f3eb1787fff245d1bbd720e2332f3d0f0f4f28f8aef166532bbd6a53249"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspacea","instance_id":"workspacea","evidence_sha256":"sha256:17d24f654898077dc03c06a36751aae15e8a1e7505224a267f94744a9f683b6a"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspaceb","instance_id":"workspaceb","evidence_sha256":"sha256:1a2c7b7a0013b1069cef36982aa6bf84a587c4d1c1ad14ce6fadcb2136987016"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspacec","instance_id":"workspacec","evidence_sha256":"sha256:d125113177fb08e801304f1179a4bdfce6fbf3813eea324b43a2321978689d57"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspaced","instance_id":"workspaced","evidence_sha256":"sha256:6516663002eff174e50839fe9e59e6695a5a9ddd2f4512283d6df01a356823af"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspacee","instance_id":"workspacee","evidence_sha256":"sha256:497ffff6a1a4ccef70942ea0ccebfdf64c64a4b77a302889716438beb38e45c4"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspacef","instance_id":"workspacef","evidence_sha256":"sha256:44115da5d206d077e62e966a2f59d0b646ac43f70807ce171ac1ed377cdee4e5"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspaceg","instance_id":"workspaceg","evidence_sha256":"sha256:ddb083eb45c7f263edbcd0b222a7024909e894b61242143cc517b66ddc0c2097"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspaceh","instance_id":"workspaceh","evidence_sha256":"sha256:c9fa3c85cda66cf10fdb5ef77fb9a23d71385936023a764491f2911a0779b229"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspacei","instance_id":"workspacei","evidence_sha256":"sha256:c9a724ef60d341783918913927a82d31c7b9b8c11908816bbf1c7da602b656a0"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspacej","instance_id":"workspacej","evidence_sha256":"sha256:205fca02f2efa2d11bff174ef8a951b783407569d39a7669fa54c3b0a371cbbf"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspacek","instance_id":"workspacek","evidence_sha256":"sha256:9e2c35023942a0649b55dd0874a22280b00d90ceb3dbd1a7a152d0745ced1b76"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspacel","instance_id":"workspacel","evidence_sha256":"sha256:74792eee92ae9c94136294d0c4ca24aa9fda767591a33ce51d7b2be834b266d4"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspacem","instance_id":"workspacem","evidence_sha256":"sha256:a2ac6287514841ed9dc4cd73197544105b2d36cd57465ece1b50370c6676a4fa"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspacen","instance_id":"workspacen","evidence_sha256":"sha256:8c6629c494fb18fb21dfb137e2846400cc0b9a1653128acb2978ad98d97964aa"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspaceo","instance_id":"workspaceo","evidence_sha256":"sha256:f34687aa69e7a8d7b8d9bc3f79e1960bbfcdfc798d0ee7ef332ad8ceb7f9f93e"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspacep","instance_id":"workspacep","evidence_sha256":"sha256:e3729d4cc09004a0e7b69155cecc55d34baad5b2ea63d3357c0b8c3f8d29a122"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspaceq","instance_id":"workspaceq","evidence_sha256":"sha256:769c8ee66ad7b49989833da52012a889e7563055bb7e7e0cd4ca59ffc8251c58"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspacer","instance_id":"workspacer","evidence_sha256":"sha256:e15b19866e527f3c2fbf8a03258b8924e6bb5a592c036355af02a0c0216319ae"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspaces","instance_id":"workspaces","evidence_sha256":"sha256:c158e98b7182fcd8b0ce82a5ea53fd1ce3d8a896a4fca4661d06eba922572ef8"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspacet","instance_id":"workspacet","evidence_sha256":"sha256:5862ad595d29025bf617a43c10ca339588be22d6981449aa91e9d4cc7fba3011"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspaceu","instance_id":"workspaceu","evidence_sha256":"sha256:8dd9fdb05c120c45d04ce8832c5176ccde26b85e7a734f0132cd4ff78d9df500"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspacev","instance_id":"workspacev","evidence_sha256":"sha256:71dc1cb0062918be66ec17c546ebc23081e20f64b2a49ea10b107cb612f90d6f"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspacew","instance_id":"workspacew","evidence_sha256":"sha256:ef90e01c86a548a33ca12f57e7d773ea7bdc76cff06037c6f6b196e37b6847c1"},{"kind":"go.workspace.use","input_handle":"c-work","related_handle":"-","subject":"root","predicate":"uses","value":"workspacex","instance_id":"workspacex","evidence_sha256":"sha256:f4b0df7064700ef7deb13e5348971839456cfda484f73c5913144fee849cb8e8"}]}
`
const frozenThirtyTwoSHA256 = "5ffe42e75f036d6ea19099b3ff90c491da0b2cd5048f02326f4778070c70f157"
const benchmarkFixtureSHA256 = "sha256:2243233c4f851a2b9f24c7df44df83ba2e30510609dca6e732c03f5939494e18"

func fixture(t testing.TB) []byte {
	t.Helper()
	uses := make([]string, 0, 24)
	for i := 0; i < 24; i++ {
		name := "workspace" + string(rune('a'+i))
		uses = append(uses, "use "+name)
	}
	mod := "module example.com/fixture\ngo 1.27.0\nrequire example.com/dependency v1.0.0\n"
	r := Request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []Input{
		input("a-mod", "go.mod", "go.mod", mod), input("b-sum", "go.sum", "go.sum", "example.com/dependency v1.0.0 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n"),
		input("c-work", "go.work", "go.work", "go 1.27.0\n"+strings.Join(uses, "\n")+"\n"),
		input("d-source", "go.source", "cmd/main_darwin_arm64_test.go", "//go:build darwin && arm64\n// +build arm64,darwin\n\npackage main\nimport \"example.com/imported\"\n"),
	}}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

// benchmarkFixture is the reviewed four-input/32-fact allocation fixture. It
// stays separate from fixture(), whose richer literal output freezes the
// dependency, build-constraint, and test-source contract witnesses.
func benchmarkFixture(t testing.TB) []byte {
	t.Helper()
	uses := make([]string, 0, 27)
	for i := 0; i < 27; i++ {
		name := "workspace" + string(rune('a'+i))
		if i == 26 {
			name = "workspacez0"
		}
		uses = append(uses, "use "+name)
	}
	r := Request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []Input{
		input("a-mod", "go.mod", "go.mod", "module example.com/fixture\ngo 1.27.0\n"),
		input("b-sum", "go.sum", "go.sum", "example.com/dependency v1.0.0 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n"),
		input("c-work", "go.work", "go.work", "go 1.27.0\n"+strings.Join(uses, "\n")+"\n"),
		input("d-source", "go.source", "cmd/main.go", "package main\n"),
	}}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

func TestBenchmarkFixtureReceipt(t *testing.T) {
	sum := sha256.Sum256(benchmarkFixture(t))
	if got := "sha256:" + hex.EncodeToString(sum[:]); got != benchmarkFixtureSHA256 {
		t.Fatalf("benchmark fixture digest=%s", got)
	}
}
func input(handle, family, path, body string) Input {
	sum := sha256.Sum256([]byte(body))
	return Input{Handle: handle, Family: family, Path: path, SHA256: "sha256:" + hex.EncodeToString(sum[:]), ContentBase64: base64.StdEncoding.EncodeToString([]byte(body))}
}
func oneInputRequest(t testing.TB) Request {
	t.Helper()
	return mustRequest(t, []byte(frozenRequest+"\n"))
}
func sourceBody(bytes int) string {
	const prefix = "package p\n//"
	if bytes < len(prefix)+1 {
		panic("source body too short")
	}
	return prefix + strings.Repeat("x", bytes-len(prefix)-1) + "\n"
}
func sourceRequest(count int, body func(int) string) Request {
	inputs := make([]Input, 0, count)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("%04d", i)
		inputs = append(inputs, input("source-"+id, "go.source", "src/"+id+".go", body(i)))
	}
	return Request{Profile: Profile, Family: Family, RequestID: "request-1", ScopeID: "root", CompilationUnitID: "unit-1", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: inputs}
}
func importRequest(count int) Request {
	var body strings.Builder
	body.WriteString("package p\nimport (\n")
	for i := 0; i < count; i++ {
		body.WriteString(`"example.com/p`)
		body.WriteString(fmt.Sprintf("%04d", i))
		body.WriteString("\"\n")
	}
	body.WriteString(")\n")
	return sourceRequest(1, func(int) string { return body.String() })
}
func mustCandidate(t testing.TB, raw []byte) {
	t.Helper()
	out, err := Process(raw)
	if err != nil || !strings.Contains(string(out), `"status":"CANDIDATE"`) {
		t.Fatalf("candidate err=%v output=%s", err, out)
	}
}

func candidateFor(t testing.TB, raw []byte) candidate {
	t.Helper()
	out, err := Process(raw)
	if err != nil {
		t.Fatal(err)
	}
	var got candidate
	if err := json.Unmarshal(out[:len(out)-1], &got); err != nil {
		t.Fatalf("candidate response: %v\n%s", err, out)
	}
	if got.Status != "CANDIDATE" {
		t.Fatalf("candidate status=%q output=%s", got.Status, out)
	}
	return got
}

func requireFact(t testing.TB, facts []Fact, handle, kind, value string) {
	t.Helper()
	for _, fact := range facts {
		if fact.InputHandle == handle && fact.Kind == kind && fact.Value == value {
			return
		}
	}
	t.Fatalf("missing independently witnessed fact handle=%q kind=%q value=%q facts=%+v", handle, kind, value, facts)
}
func requestBytes(t testing.TB, r Request) []byte {
	t.Helper()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}
func mustReject(t testing.TB, raw []byte, family, requestID, reason string) {
	t.Helper()
	out, err := Process(raw)
	want := expectedRejection(t, raw, family, requestID, reason)
	if err != nil || string(out) != want {
		t.Fatalf("reason=%s err=%v\nwant=%s\ngot=%s", reason, err, want, out)
	}
}

func expectedRejection(t testing.TB, raw []byte, family, requestID, reason string) string {
	t.Helper()
	if len(raw) < 2 || raw[len(raw)-1] != '\n' || strings.Contains(string(raw[:len(raw)-1]), "\n") {
		return literalMinimalRejection(reason)
	}
	var r Request
	if err := json.Unmarshal(raw[:len(raw)-1], &r, json.RejectUnknownMembers(true)); err != nil {
		return literalMinimalRejection(reason)
	}
	canonical, err := json.Marshal(r)
	if !independentEnvelopeComplete(r) || !independentEnvelopeWithinBounds(r) {
		return literalMinimalRejection(reason)
	}
	if raw[len(raw)-2] == ' ' || raw[len(raw)-2] == '\t' || raw[len(raw)-2] == '\r' {
		return literalMinimalRejection(reason)
	}
	if err != nil || string(canonical) != string(raw[:len(raw)-1]) {
		if independentBoundIdentity(r) {
			return literalBoundRejection(r, reason)
		}
		return literalMinimalRejection(reason)
	}
	if !independentBoundIdentity(r) {
		return literalMinimalRejection(reason)
	}
	if family != "unknown" && (family != r.Family || requestID != r.RequestID) {
		t.Fatalf("expected identity %s/%s disagrees with bound request %s/%s", family, requestID, r.Family, r.RequestID)
	}
	return literalBoundRejection(r, reason)
}

// These are deliberately independent exact byte fixtures. They do not call
// jsonPreflight, rejectionFor, boundRejection, or any production constructor.
func literalMinimalRejection(reason string) string {
	switch reason {
	case "NONCANONICAL_REQUEST":
		return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"NONCANONICAL_REQUEST"}` + "\n"
	case "INVALID_IDENTIFIER":
		return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"INVALID_IDENTIFIER"}` + "\n"
	case "INVALID_PATH":
		return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"INVALID_PATH"}` + "\n"
	case "DIGEST_MISMATCH":
		return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"DIGEST_MISMATCH"}` + "\n"
	case "UNKNOWN_FAMILY":
		return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"UNKNOWN_FAMILY"}` + "\n"
	case "UNKNOWN_FIELD":
		return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"UNKNOWN_FIELD"}` + "\n"
	case "DUPLICATE_VALUE":
		return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"DUPLICATE_VALUE"}` + "\n"
	case "CONFLICTING_VALUE":
		return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"CONFLICTING_VALUE"}` + "\n"
	case "MALFORMED_INPUT":
		return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"MALFORMED_INPUT"}` + "\n"
	case "UNSUPPORTED_SCHEMA":
		return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"UNSUPPORTED_SCHEMA"}` + "\n"
	case "EXACT_BINDING_UNAVAILABLE":
		return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"EXACT_BINDING_UNAVAILABLE"}` + "\n"
	case "AMBIGUOUS_BINDING":
		return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"AMBIGUOUS_BINDING"}` + "\n"
	case "DYNAMIC_INPUT":
		return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"DYNAMIC_INPUT"}` + "\n"
	case "CREDENTIAL_INPUT":
		return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"CREDENTIAL_INPUT"}` + "\n"
	case "LIMIT_EXCEEDED":
		return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"LIMIT_EXCEEDED"}` + "\n"
	case "OUTPUT_LIMIT":
		return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"OUTPUT_LIMIT"}` + "\n"
	case "ANALYZER_FAILURE":
		return `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"ANALYZER_FAILURE"}` + "\n"
	default:
		panic("missing literal rejection fixture: " + reason)
	}
}

func independentEnvelopeComplete(r Request) bool {
	if r.Profile == "" || r.Family == "" || r.RequestID == "" || r.ScopeID == "" || r.CompilationUnitID == "" || r.Target.OS == "" || r.Target.Architecture == "" || r.Target.ABI == "" || r.Target.Features == nil || r.Inputs == nil {
		return false
	}
	for _, in := range r.Inputs {
		if in.Handle == "" || in.Family == "" || in.Path == "" || in.SHA256 == "" || in.ContentBase64 == "" {
			return false
		}
	}
	return true
}

func independentEnvelopeWithinBounds(r Request) bool {
	if len(r.Inputs) > MaxInputs || len(r.Target.Features) > maxFeatures {
		return false
	}
	ordinary := []string{r.Profile, r.Family, r.RequestID, r.ScopeID, r.CompilationUnitID, r.Target.OS, r.Target.Architecture, r.Target.ABI}
	ordinary = append(ordinary, r.Target.Features...)
	decoded := 0
	for _, in := range r.Inputs {
		ordinary = append(ordinary, in.Handle, in.Family, in.Path, in.SHA256)
		if len(in.ContentBase64) > 1_398_104 {
			return false
		}
		n := independentBase64Length(in.ContentBase64)
		if n > MaxInputBytes-decoded {
			return false
		}
		decoded += n
	}
	for _, value := range ordinary {
		if len(value) > 4096 {
			return false
		}
	}
	return true
}

func independentBase64Length(value string) int {
	if len(value) == 0 || len(value)%4 != 0 {
		return MaxInputBytes + 1
	}
	n := len(value) / 4 * 3
	if value[len(value)-1] == '=' {
		n--
		if value[len(value)-2] == '=' {
			n--
		}
	}
	return n
}

func independentBoundIdentity(r Request) bool {
	if !(r.Family == "go" || r.Family == "javascript-typescript" || r.Family == "dotnet" || r.Family == "ruby") || r.Target.Features == nil {
		return false
	}
	values := []string{r.RequestID, r.ScopeID, r.CompilationUnitID, r.Target.OS, r.Target.Architecture, r.Target.ABI}
	seen := map[string]bool{}
	for _, feature := range r.Target.Features {
		values = append(values, feature)
		if seen["feature:"+feature] {
			return false
		}
		seen["feature:"+feature] = true
	}
	for _, in := range r.Inputs {
		values = append(values, in.Handle)
		if seen["handle:"+in.Handle] || len(in.SHA256) != 71 || !strings.HasPrefix(in.SHA256, "sha256:") || strings.Trim(in.SHA256[7:], "0123456789abcdef") != "" {
			return false
		}
		seen["handle:"+in.Handle] = true
	}
	for _, value := range values {
		if !independentIdentifier(value) {
			return false
		}
	}
	return true
}

func independentIdentifier(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for i := range value {
		c := value[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || (i > 0 && strings.ContainsRune("._:@+~-", rune(c)))) {
			return false
		}
	}
	return true
}

func literalBoundRejection(r Request, reason string) string {
	var out strings.Builder
	out.WriteString(`{"profile":`)
	out.WriteString(independentQuote(r.Profile))
	out.WriteString(`,"family":`)
	out.WriteString(independentQuote(r.Family))
	out.WriteString(`,"request_id":`)
	out.WriteString(independentQuote(r.RequestID))
	out.WriteString(`,"status":"REJECTED","scope_id":`)
	out.WriteString(independentQuote(r.ScopeID))
	out.WriteString(`,"compilation_unit_id":`)
	out.WriteString(independentQuote(r.CompilationUnitID))
	out.WriteString(`,"target":{"os":`)
	out.WriteString(independentQuote(r.Target.OS))
	out.WriteString(`,"architecture":`)
	out.WriteString(independentQuote(r.Target.Architecture))
	out.WriteString(`,"abi":`)
	out.WriteString(independentQuote(r.Target.ABI))
	out.WriteString(`,"features":[`)
	for i, feature := range r.Target.Features {
		if i > 0 {
			out.WriteByte(',')
		}
		out.WriteString(independentQuote(feature))
	}
	out.WriteString(`]},"input_echoes":[`)
	for i, in := range r.Inputs {
		if i > 0 {
			out.WriteByte(',')
		}
		out.WriteString(`{"handle":`)
		out.WriteString(independentQuote(in.Handle))
		out.WriteString(`,"sha256":`)
		out.WriteString(independentQuote(in.SHA256))
		out.WriteByte('}')
	}
	out.WriteString(`],"reason":`)
	out.WriteString(independentQuote(reason))
	out.WriteString("}\n")
	return out.String()
}

func independentQuote(value string) string {
	var out strings.Builder
	out.Grow(len(value) + 2)
	out.WriteByte('"')
	for i := range value {
		switch c := value[i]; c {
		case '"', '\\':
			out.WriteByte('\\')
			out.WriteByte(c)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if c < 0x20 {
				out.WriteString(`\u00`)
				out.WriteByte("0123456789abcdef"[c>>4])
				out.WriteByte("0123456789abcdef"[c&0x0f])
			} else {
				out.WriteByte(c)
			}
		}
	}
	out.WriteByte('"')
	return out.String()
}

func TestFrozenOneGoModVector(t *testing.T) {
	out, err := Process([]byte(frozenRequest + "\n"))
	if err != nil || string(out) != frozenSuccess+"\n" {
		t.Fatalf("err=%v\nwant=%s\ngot=%s", err, frozenSuccess, out)
	}
}

func TestFrozenBoundDigestMismatchVector(t *testing.T) {
	r := oneInputRequest(t)
	r.Inputs[0].SHA256 = "sha256:" + strings.Repeat("0", 64)
	out, err := Process(requestBytes(t, r))
	if err != nil || string(out) != frozenBoundDigestMismatch+"\n" {
		t.Fatalf("err=%v\nwant=%s\ngot=%s", err, frozenBoundDigestMismatch, out)
	}
}

func TestFrozenBoundConflictingValueVector(t *testing.T) {
	out, err := Process([]byte(frozenConflictingValueRequest + "\n"))
	if err != nil || string(out) != frozenBoundConflictingValue+"\n" {
		t.Fatalf("err=%v\nwant=%s\ngot=%s", err, frozenBoundConflictingValue, out)
	}
}

func TestProcessExactCanonicalCandidate(t *testing.T) {
	raw := fixture(t)
	r := mustRequest(t, raw)
	_, modReason := parseMod(decodeBody(t, r.Inputs[0]), &resourceLedger{})
	_, sumReason := parseSum(decodeBody(t, r.Inputs[1]))
	_, workReason := parseWork(decodeBody(t, r.Inputs[2]), &resourceLedger{})
	if modReason != "" || sumReason != "" || workReason != "" {
		t.Fatalf("fixture parser rejection: %s %s %s", modReason, sumReason, workReason)
	}
	out, err := Process(raw)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != frozenThirtyTwoSuccess {
		t.Fatalf("32-fact literal mismatch\nwant=%s\ngot=%s", frozenThirtyTwoSuccess, out)
	}
	var actual candidate
	if err := json.Unmarshal(out[:len(out)-1], &actual); err != nil {
		t.Fatal(err)
	}
	for i, want := range frozenThirtyTwoFactTuples(t) {
		if actual.Facts[i] != want {
			t.Fatalf("fact tuple %d\nwant=%+v\ngot=%+v", i, want, actual.Facts[i])
		}
	}
	if got := sha256.Sum256(out); hex.EncodeToString(got[:]) != frozenThirtyTwoSHA256 {
		t.Fatalf("32-fact exact byte digest=%x", got)
	}
	if !strings.HasSuffix(string(out), "\n") || strings.Contains(string(out[:len(out)-1]), "\n") {
		t.Fatalf("not framed canonical JSON: %q", out)
	}
	if !strings.Contains(string(out), `"status":"CANDIDATE"`) || strings.Count(string(out), `"kind":`) != 32 {
		t.Fatalf("unexpected candidate: %s", out)
	}
}

// frozenThirtyTwoFactTuples decodes the independent literal success vector into
// its complete ordered typed tuples. The byte equality above freezes framing and
// field order; this typed comparison makes every one of the 32 fact fields
// explicit in a failure without relying on a kind count or a digest alone.
func frozenThirtyTwoFactTuples(t testing.TB) [32]Fact {
	t.Helper()
	var frozen candidate
	if err := json.Unmarshal([]byte(strings.TrimSuffix(frozenThirtyTwoSuccess, "\n")), &frozen); err != nil {
		t.Fatal(err)
	}
	var tuples [32]Fact
	if len(frozen.Facts) != len(tuples) {
		t.Fatalf("frozen tuple count=%d", len(frozen.Facts))
	}
	copy(tuples[:], frozen.Facts)
	return tuples
}

func TestInputOrderPermutationsReachProcessBoundary(t *testing.T) {
	base := mustRequest(t, fixture(t))
	permutation := []int{0, 1, 2, 3}
	seenRequests := map[string]struct{}{}
	seenResponses := map[string]struct{}{}
	var visit func(int)
	visits := 0
	visit = func(at int) {
		if at == len(permutation) {
			constructed := base
			constructed.Inputs = make([]Input, len(permutation))
			for i, index := range permutation {
				constructed.Inputs[i] = base.Inputs[index]
			}
			raw := requestBytes(t, constructed)
			requestDigest := sha256.Sum256(raw)
			requestKey := hex.EncodeToString(requestDigest[:])
			if _, duplicate := seenRequests[requestKey]; duplicate {
				t.Fatalf("duplicate Process request permutation=%v digest=%s", permutation, requestKey)
			}
			seenRequests[requestKey] = struct{}{}
			out, err := Process(raw)
			if err != nil {
				t.Fatal(err)
			}
			responseDigest := sha256.Sum256(out)
			responseKey := hex.EncodeToString(responseDigest[:])
			if _, duplicate := seenResponses[responseKey]; duplicate {
				t.Fatalf("duplicate Process response permutation=%v digest=%s", permutation, responseKey)
			}
			seenResponses[responseKey] = struct{}{}
			if string(out) == frozenThirtyTwoSuccess {
				if hex.EncodeToString(responseDigest[:]) != frozenThirtyTwoSHA256 {
					t.Fatalf("canonical permutation digest=%x", responseDigest)
				}
			} else {
				want := literalBoundRejection(constructed, "DUPLICATE_VALUE")
				if string(out) != want {
					t.Fatalf("noncanonical permutation %v\nwant=%s\ngot=%s", permutation, want, out)
				}
			}
			visits++
			return
		}
		for i := at; i < len(permutation); i++ {
			permutation[at], permutation[i] = permutation[i], permutation[at]
			visit(at + 1)
			permutation[at], permutation[i] = permutation[i], permutation[at]
		}
	}
	visit(0)
	if visits != 24 || len(seenRequests) != 24 || len(seenResponses) != 24 {
		t.Fatalf("permutations=%d requests=%d responses=%d", visits, len(seenRequests), len(seenResponses))
	}
}

func decodeBody(t testing.TB, in Input) string {
	t.Helper()
	b, reason := decodeInput(in)
	if reason != "" {
		t.Fatal(reason)
	}
	return string(b)
}
func TestProcessRejectsHostileAndNonCanonical(t *testing.T) {
	raw := fixture(t)
	var r Request
	if err := json.Unmarshal(raw[:len(raw)-1], &r); err != nil {
		t.Fatal(err)
	}
	r.Inputs[0].SHA256 = "sha256:" + strings.Repeat("0", 64)
	b, _ := json.Marshal(r)
	out, err := Process(append(b, '\n'))
	if err != nil || !strings.Contains(string(out), `"reason":"DIGEST_MISMATCH"`) {
		t.Fatalf("digest: %v %s", err, out)
	}
	if out, err := Process([]byte("{\"profile\":\"x\"}\n")); err != nil || !strings.Contains(string(out), `"reason":"NONCANONICAL_REQUEST"`) {
		t.Fatalf("malformed request: %v %s", err, out)
	}
	out, err = Process(append(raw[:len(raw)-1], ' ', '\n'))
	if err != nil || !strings.Contains(string(out), `"reason":"NONCANONICAL_REQUEST"`) {
		t.Fatalf("noncanonical request: %v %s", err, out)
	}
	r = mustRequest(t, raw)
	r.Inputs[0], r.Inputs[1] = r.Inputs[1], r.Inputs[0]
	b, _ = json.Marshal(r)
	out, err = Process(append(b, '\n'))
	if err != nil || !strings.Contains(string(out), `"reason":"DUPLICATE_VALUE"`) {
		t.Fatalf("order: %v %s", err, out)
	}
}

func TestPostEnvelopeRejectionBindsFullRequestAndRejectsCrossInputReplay(t *testing.T) {
	rawA := fixture(t)
	rB := mustRequest(t, rawA)
	rB.Inputs[3] = input("d-source", "go.source", "cmd/main_darwin_arm64_test.go", "//go:build darwin && arm64\n// +build arm64,darwin\n\npackage main\nimport \"example.com/other\"\n")
	rawB := requestBytes(t, rB)
	rA := mustRequest(t, rawA)
	rA.Inputs[0].SHA256 = "sha256:" + strings.Repeat("0", 64)
	rB = mustRequest(t, rawB)
	rB.Inputs[0].SHA256 = "sha256:" + strings.Repeat("0", 64)
	rejectedA := requestBytes(t, rA)
	rejectedB := requestBytes(t, rB)
	mustReject(t, rejectedA, "go", "request-1", "DIGEST_MISMATCH")
	mustReject(t, rejectedB, "go", "request-1", "DIGEST_MISMATCH")
	outA, _ := Process(rejectedA)
	outB, _ := Process(rejectedB)
	if string(outA) == string(outB) {
		t.Fatal("same request_id changed input replay reused a rejection")
	}
	if string(outA) == expectedRejection(t, rejectedB, "go", "request-1", "DIGEST_MISMATCH") {
		t.Fatal("first rejection was valid for changed caller inputs")
	}
}
func TestInvalidEnvelopeRejectionStaysUnbound(t *testing.T) {
	for want, edit := range map[string]func(*Request){
		"INVALID_IDENTIFIER": func(r *Request) { r.Target.OS = "bad os" },
		"DIGEST_MISMATCH":    func(r *Request) { r.Inputs[0].SHA256 = "sha256:BAD" },
		"DUPLICATE_VALUE":    func(r *Request) { r.Inputs[1].Handle = r.Inputs[0].Handle },
	} {
		r := mustRequest(t, fixture(t))
		edit(&r)
		if out, err := Process(requestBytes(t, r)); err != nil || string(out) != literalMinimalRejection(want) {
			t.Fatalf("%s: err=%v output=%s", want, err, out)
		}
	}
}
func TestCanonicalRejectionStages(t *testing.T) {
	mustReject(t, nil, "unknown", "unknown", "NONCANONICAL_REQUEST")
	mustReject(t, append(make([]byte, MaxRequestBytes), 'x'), "unknown", "unknown", "LIMIT_EXCEEDED")
	r := mustRequest(t, fixture(t))
	r.Family = "ruby"
	mustReject(t, requestBytes(t, r), "ruby", "request-1", "UNKNOWN_FAMILY")
	r = mustRequest(t, fixture(t))
	r.RequestID = "bad\x00id"
	mustReject(t, requestBytes(t, r), "unknown", "unknown", "INVALID_IDENTIFIER")
	r = mustRequest(t, fixture(t))
	r.Inputs[0].SHA256 = "sha256:" + strings.Repeat("0", 64)
	mustReject(t, requestBytes(t, r), "go", "request-1", "DIGEST_MISMATCH")
	r = mustRequest(t, fixture(t))
	canonical := requestBytes(t, r)
	mustReject(t, append(canonical[:len(canonical)-1], ' ', '\n'), "go", "request-1", "NONCANONICAL_REQUEST")
	mustReject(t, canonical[:len(canonical)-1], "go", "request-1", "NONCANONICAL_REQUEST")
	broken := `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":`
	mustReject(t, []byte(broken+"\n"), "go", "request-1", "NONCANONICAL_REQUEST")
}
func TestStagedEnvelopeIdentityRequiresUniqueTopLevelFields(t *testing.T) {
	mustReject(t, []byte(`{"x":{"family":"go","request_id":"nested"}}`+"\n"), "unknown", "unknown", "NONCANONICAL_REQUEST")
	mustReject(t, []byte(`{"family":"go","request_id":"first","family":"go","request_id":"second"}`+"\n"), "unknown", "unknown", "NONCANONICAL_REQUEST")
	mustReject(t, []byte(`{"family":"go","request_id":"first","request_id":"second"}`+"\n"), "unknown", "unknown", "NONCANONICAL_REQUEST")
	mustReject(t, []byte(`{"family":"go","request_id":"request-1","x":{"family":"nested","request_id":"nested"}}`+"\n"), "go", "request-1", "NONCANONICAL_REQUEST")
	nested := `"x":{"inputs":[` + strings.TrimSuffix(strings.Repeat(`{},`, MaxInputs+1), ",") + `]},`
	mustReject(t, []byte(strings.Replace(frozenRequest, `"profile":`, nested+`"profile":`, 1)+"\n"), "go", "request-1", "NONCANONICAL_REQUEST")
}
func TestSafeCanonicalExtensionBindsUnknownField(t *testing.T) {
	base := strings.TrimSuffix(frozenRequest, "}")
	bound := literalBoundRejection(mustRequest(t, []byte(frozenRequest+"\n")), "UNKNOWN_FIELD")
	for _, frame := range []string{
		base + `,"x":[9007199254740993,{"a":"b","c":null}]}`,
		strings.Replace(frozenRequest, `"features":[]`, `"features":[],"x":true`, 1),
		strings.Replace(frozenRequest, `"}]}`, `","x":0,"y":"z"}]}`, 1),
	} {
		if out, err := Process([]byte(frame + "\n")); err != nil || string(out) != bound {
			t.Fatalf("frame=%s\nwant=%s\ngot=%s", frame, bound, out)
		}
	}
	for _, frame := range []string{
		base + `,"x":1.0}`, base + `,"x":-0}`, base + `,"x":"` + string(rune(92)) + `u0065"}`, base + `,"y":0,"x":0}`,
		base + `,"x":{"b":0,"a":0}}`, strings.Replace(frozenRequest, `{"profile":`, `{"x":0,"profile":`, 1),
	} {
		mustReject(t, []byte(frame+"\n"), "unknown", "unknown", "NONCANONICAL_REQUEST")
	}
	unsafe := strings.Replace(base, `sha256:8d27`, `sha256:XX27`, 1) + `,"x":0}`
	mustReject(t, []byte(unsafe+"\n"), "unknown", "unknown", "DIGEST_MISMATCH")
}
func TestPreRetentionEnvelopeBounds(t *testing.T) {
	r := mustRequest(t, fixture(t))
	r.ScopeID = strings.Repeat("a", 4097)
	mustReject(t, requestBytes(t, r), "go", "request-1", "LIMIT_EXCEEDED")
	r = mustRequest(t, fixture(t))
	r.Target.Features = make([]string, maxFeatures+1)
	for i := range r.Target.Features {
		r.Target.Features[i] = "f" + string(rune('a'+i))
	}
	mustReject(t, requestBytes(t, r), "go", "request-1", "LIMIT_EXCEEDED")
	r = mustRequest(t, fixture(t))
	r.Inputs[0].ContentBase64 = strings.Repeat("A", 1_398_108)
	mustReject(t, requestBytes(t, r), "go", "request-1", "LIMIT_EXCEEDED")
	mustReject(t, []byte(`{"profile":[[[[[[[[[]]]]]]]]}`+"\n"), "unknown", "unknown", "LIMIT_EXCEEDED")
}
func TestPreflightReservesBoundRejectionOutput(t *testing.T) {
	r := sourceRequest(MaxInputs, func(int) string { return "package p\n" })
	for i := range r.Inputs {
		r.Inputs[i].Handle = "h" + strings.Repeat("a", 4095)
		r.Inputs[i].SHA256 = "s" + strings.Repeat("b", 4095)
	}
	raw := requestBytes(t, r)
	if len(raw) > MaxRequestBytes {
		t.Fatalf("fixture escaped request cap: %d", len(raw))
	}
	out, err := Process(raw)
	if err != nil || len(out) > MaxOutputBytes || string(out) != `{"profile":"corvint-analyzer-candidate/experimental","family":"unknown","request_id":"unknown","status":"REJECTED","reason":"LIMIT_EXCEEDED"}`+"\n" {
		t.Fatalf("err=%v len=%d output=%s", err, len(out), out)
	}
}
func TestPreflightCountsEmptyInputAndFeatureElements(t *testing.T) {
	prefix := `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[]},"inputs":[`
	emptyInputs := prefix + strings.TrimSuffix(strings.Repeat("{},", MaxInputs+1), ",") + "]}" + "\n"
	mustReject(t, []byte(emptyInputs), "go", "request-1", "LIMIT_EXCEEDED")
	features := `{"profile":"corvint-analyzer-candidate/experimental","family":"go","request_id":"request-1","scope_id":"root","compilation_unit_id":"unit-1","target":{"os":"darwin","architecture":"arm64","abi":"none","features":[` + strings.TrimSuffix(strings.Repeat(`"x",`, maxFeatures+1), ",") + `]},"inputs":[]}` + "\n"
	mustReject(t, []byte(features), "go", "request-1", "LIMIT_EXCEEDED")
}
func TestStructuralPreflightChargesDecodedPathsBeforeUnmarshal(t *testing.T) {
	escapedInputs := strings.Replace(frozenRequest, `"inputs":`, `"inpu\u0074s":[`+strings.TrimSuffix(strings.Repeat(`{},`, MaxInputs+1), ",")+`],"inputs":`, 1)
	mustReject(t, []byte(escapedInputs+"\n"), "go", "request-1", "LIMIT_EXCEEDED")
	r := oneInputRequest(t)
	r.Target.Features = make([]string, maxFeatures+1)
	for i := range r.Target.Features {
		r.Target.Features[i] = fmt.Sprintf("feature-%02d", i)
	}
	raw := strings.Replace(string(requestBytes(t, r)), `"features":`, `"featu\u0072es":`, 1)
	mustReject(t, []byte(raw), "go", "request-1", "LIMIT_EXCEEDED")
	unknown := strings.Replace(frozenRequest, `"profile":`, `"c":"`+strings.Repeat("x", 4097)+`","profile":`, 1)
	mustReject(t, []byte(unknown+"\n"), "go", "request-1", "LIMIT_EXCEEDED")
	r = sourceRequest(2, func(i int) string {
		if i == 0 {
			return sourceBody(MaxInputBytes / 2)
		}
		return sourceBody(MaxInputBytes/2 + 1)
	})
	mustReject(t, requestBytes(t, r), "go", "request-1", "LIMIT_EXCEEDED")
}
func TestPreflightArrayCountBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name  string
		limit int
		raw   func(int) []byte
	}{
		{"inputs", MaxInputs, func(count int) []byte {
			return requestBytes(t, sourceRequest(count, func(int) string { return "package p\n" }))
		}},
		{"features", maxFeatures, func(count int) []byte {
			r := oneInputRequest(t)
			r.Target.Features = make([]string, count)
			for i := range r.Target.Features {
				r.Target.Features[i] = fmt.Sprintf("feature-%02d", i)
			}
			return requestBytes(t, r)
		}},
	} {
		for _, count := range []int{tc.limit - 1, tc.limit, tc.limit + 1} {
			raw := tc.raw(count)
			reason := jsonPreflight(raw[:len(raw)-1])
			want := ""
			if count > tc.limit {
				want = "LIMIT_EXCEEDED"
			}
			if reason != want {
				t.Fatalf("%s count=%d reason=%s want=%s", tc.name, count, reason, want)
			}
		}
	}
}
func TestPublicEnvelopeBoundaryMatrix(t *testing.T) {
	for _, n := range []int{MaxRequestBytes - 1, MaxRequestBytes} {
		raw := make([]byte, n)
		raw[0], raw[n-2], raw[n-1] = '{', '}', '\n'
		mustReject(t, raw, "unknown", "unknown", "NONCANONICAL_REQUEST")
	}
	over := make([]byte, MaxRequestBytes+1)
	mustReject(t, over, "unknown", "unknown", "LIMIT_EXCEEDED")

	for _, n := range []int{MaxInputs - 1, MaxInputs} {
		mustCandidate(t, requestBytes(t, sourceRequest(n, func(int) string { return "package p\n" })))
	}
	mustReject(t, requestBytes(t, sourceRequest(MaxInputs+1, func(int) string { return "package p\n" })), "go", "request-1", "LIMIT_EXCEEDED")

	for _, n := range []int{maxFeatures - 1, maxFeatures} {
		r := oneInputRequest(t)
		r.Target.Features = make([]string, n)
		for i := range r.Target.Features {
			r.Target.Features[i] = fmt.Sprintf("feature-%02d", i)
		}
		mustReject(t, requestBytes(t, r), "go", "request-1", "UNSUPPORTED_SCHEMA")
	}
	r := oneInputRequest(t)
	r.Target.Features = make([]string, maxFeatures+1)
	for i := range r.Target.Features {
		r.Target.Features[i] = fmt.Sprintf("feature-%02d", i)
	}
	mustReject(t, requestBytes(t, r), "go", "request-1", "LIMIT_EXCEEDED")

	for _, n := range []int{127, 128} {
		r := oneInputRequest(t)
		r.RequestID = "r" + strings.Repeat("a", n-1)
		mustCandidate(t, requestBytes(t, r))
	}
	r = oneInputRequest(t)
	r.RequestID = "r" + strings.Repeat("a", 128)
	mustReject(t, requestBytes(t, r), "unknown", "unknown", "INVALID_IDENTIFIER")
	for _, set := range []func(*Request, string){
		func(r *Request, value string) { r.ScopeID = value },
		func(r *Request, value string) { r.CompilationUnitID = value },
		func(r *Request, value string) { r.Inputs[0].Handle = value },
	} {
		for _, n := range []int{127, 128} {
			r := oneInputRequest(t)
			set(&r, "i"+strings.Repeat("a", n-1))
			mustCandidate(t, requestBytes(t, r))
		}
		r := oneInputRequest(t)
		set(&r, "i"+strings.Repeat("a", 128))
		mustReject(t, requestBytes(t, r), "go", "request-1", "INVALID_IDENTIFIER")
	}
}
func TestPublicDecodedContentBoundaryMatrix(t *testing.T) {
	for _, n := range []int{MaxInputBytes - 1, MaxInputBytes} {
		r := sourceRequest(1, func(int) string { return sourceBody(n) })
		mustCandidate(t, requestBytes(t, r))
	}
	r := sourceRequest(1, func(int) string { return sourceBody(MaxInputBytes + 1) })
	mustReject(t, requestBytes(t, r), "go", "request-1", "LIMIT_EXCEEDED")

	for _, sizes := range [][2]int{{MaxInputBytes / 2, MaxInputBytes/2 - 1}, {MaxInputBytes / 2, MaxInputBytes / 2}} {
		r = sourceRequest(2, func(i int) string { return sourceBody(sizes[i]) })
		mustCandidate(t, requestBytes(t, r))
	}
	r = sourceRequest(2, func(i int) string {
		if i == 0 {
			return sourceBody(MaxInputBytes / 2)
		}
		return sourceBody(MaxInputBytes/2 + 1)
	})
	mustReject(t, requestBytes(t, r), "go", "request-1", "LIMIT_EXCEEDED")
}
func TestPublicNestingAndTokenBoundaryMatrix(t *testing.T) {
	for _, arrays := range []int{maxJSONDepth - 2, maxJSONDepth - 1} {
		raw := `{"profile":` + strings.Repeat("[", arrays) + "0" + strings.Repeat("]", arrays) + "}\n"
		mustReject(t, []byte(raw), "unknown", "unknown", "NONCANONICAL_REQUEST")
	}
	raw := `{"profile":` + strings.Repeat("[", maxJSONDepth) + "0" + strings.Repeat("]", maxJSONDepth) + "}\n"
	mustReject(t, []byte(raw), "unknown", "unknown", "LIMIT_EXCEEDED")
	for _, fields := range []int{(maxTokens-2)/2 - 1, (maxTokens - 2) / 2} {
		var raw strings.Builder
		raw.WriteByte('{')
		for i := 0; i < fields; i++ {
			if i > 0 {
				raw.WriteByte(',')
			}
			raw.WriteString(`"x`)
			raw.WriteString(fmt.Sprintf("%04d", i))
			raw.WriteString(`":0`)
		}
		raw.WriteString("}\n")
		mustReject(t, []byte(raw.String()), "unknown", "unknown", "NONCANONICAL_REQUEST")
	}
	var over strings.Builder
	over.WriteByte('{')
	for i := 0; i <= (maxTokens-2)/2; i++ {
		if i > 0 {
			over.WriteByte(',')
		}
		over.WriteString(`"x`)
		over.WriteString(fmt.Sprintf("%04d", i))
		over.WriteString(`":0`)
	}
	over.WriteString("}\n")
	mustReject(t, []byte(over.String()), "unknown", "unknown", "LIMIT_EXCEEDED")
}
func TestPublicFactEdgeAndOutputBoundaryMatrix(t *testing.T) {
	for _, n := range []int{MaxFacts - 3, MaxFacts - 2} {
		mustReject(t, requestBytes(t, importRequest(n)), "go", "request-1", "OUTPUT_LIMIT")
	}
	mustReject(t, requestBytes(t, importRequest(MaxFacts-1)), "go", "request-1", "LIMIT_EXCEEDED")
}
func TestInputDecodedAndFactBounds(t *testing.T) {
	r := mustRequest(t, fixture(t))
	r.Inputs = make([]Input, 0, MaxInputs+1)
	for i := 0; i <= MaxInputs; i++ {
		suffix := string(rune('a'+i/26)) + string(rune('a'+i%26))
		r.Inputs = append(r.Inputs, input("input-"+suffix, "go.source", "src/"+suffix+".go", "package p\n"))
	}
	mustReject(t, requestBytes(t, r), "go", "request-1", "LIMIT_EXCEEDED")
	body := strings.Repeat("x", MaxInputBytes/2+1)
	r = mustRequest(t, fixture(t))
	r.Inputs = []Input{input("a-source", "go.source", "src/a.go", body), input("b-source", "go.source", "src/b.go", body)}
	mustReject(t, requestBytes(t, r), "go", "request-1", "LIMIT_EXCEEDED")
	var mod strings.Builder
	mod.WriteString("module example.com/fixture\ngo 1.27.0\n")
	for i := 0; i <= MaxFacts; i++ {
		mod.WriteString("require example.com/a")
		mod.WriteString(fmt.Sprintf("%05d", i))
		mod.WriteString(" v1.0.0\n")
	}
	r = mustRequest(t, fixture(t))
	r.Inputs = []Input{input("a-mod", "go.mod", "go.mod", mod.String())}
	mustReject(t, requestBytes(t, r), "go", "request-1", "LIMIT_EXCEEDED")
}
func TestProspectiveFactAndOutputBounds(t *testing.T) {
	f := Fact{Kind: "go.source", InputHandle: "input", RelatedHandle: "-", Subject: "path.go", Predicate: "classifies", Value: "source", InstanceID: "path.go"}
	c := candidate{Facts: make([]Fact, MaxFacts), outputBytes: 1}
	if _, reason := admitFact(&c, f); reason != "LIMIT_EXCEEDED" {
		t.Fatalf("fact cap=%s", reason)
	}
	c = candidate{outputBytes: MaxOutputBytes}
	if _, reason := admitFact(&c, f); reason != "OUTPUT_LIMIT" {
		t.Fatalf("output cap=%s", reason)
	}
}
func TestImportHeaderChargesBeforeWholeFileParse(t *testing.T) {
	var body strings.Builder
	body.WriteString("package p\nimport (\n")
	for i := 0; i <= MaxFacts; i++ {
		body.WriteString(`"example.com/p`)
		body.WriteString(fmt.Sprintf("%04d", i))
		body.WriteString("\"\n")
	}
	body.WriteString(")\n")
	if _, _, _, _, reason := parseSource("cmd/main.go", body.String(), &resourceLedger{}); reason != "LIMIT_EXCEEDED" {
		t.Fatalf("import header limit=%s", reason)
	}
}
func TestClosedParsersRejectUnsupportedSchema(t *testing.T) {
	r := mustRequest(t, fixture(t))
	r.Inputs[0] = input("a-mod", "go.mod", "go.mod", "module example.com/fixture\ngo 1.27.0\nreplace example.com/a => example.com/b v1.0.0\n")
	raw, _ := json.Marshal(r)
	out, err := Process(append(raw, '\n'))
	if err != nil || !strings.Contains(string(out), `"reason":"UNSUPPORTED_SCHEMA"`) {
		t.Fatalf("replace accepted: %v %s", err, out)
	}
	r = mustRequest(t, fixture(t))
	r.Inputs[3] = input("d-source", "go.source", "cmd/main_linux.go", "package main\n")
	raw, _ = json.Marshal(r)
	out, err = Process(append(raw, '\n'))
	if err != nil || !strings.Contains(string(out), `"reason":"EXACT_BINDING_UNAVAILABLE"`) {
		t.Fatalf("excluded source accepted: %v %s", err, out)
	}
}
func TestClosedModuleWorkAndChecksumBindings(t *testing.T) {
	r := mustRequest(t, fixture(t))
	r.Inputs[0] = input("a-mod", "go.mod", "go.mod", "module example.com/fixture\ngo 1.27.0\nrequire example.com/a v1.0.0\nrequire example.com/a v1.0.1\n")
	mustReject(t, requestBytes(t, r), "go", "request-1", "DUPLICATE_VALUE")
	r = mustRequest(t, fixture(t))
	r.Inputs[2] = input("c-work", "go.work", "go.work", "go 1.27.0\nuse workspacea\nuse (\nworkspaceb\n)\n")
	mustReject(t, requestBytes(t, r), "go", "request-1", "MALFORMED_INPUT")
	r = mustRequest(t, fixture(t))
	r.Inputs[0] = input("a-mod", "go.mod", "go.mod", "module example.com/fixture\ngo 1.27.0\nrequire example.com/a v1.0.0\n")
	r.Inputs[1] = input("b-sum", "go.sum", "go.sum", "example.com/a v1.0.0/go.mod h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n")
	mustReject(t, requestBytes(t, r), "go", "request-1", "EXACT_BINDING_UNAVAILABLE")
}

func TestClosedGoRootAndChecksumConflictBoundaries(t *testing.T) {
	const checksum = "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	locked, reason := parseSum("example.com/a v1.0.0 " + checksum + "\nexample.com/a v2.0.0 " + checksum + "\n")
	if reason != "" || len(locked) != 2 || !locked[requirement{"example.com/a", "v1.0.0"}].archive || !locked[requirement{"example.com/a", "v2.0.0"}].archive {
		t.Fatalf("multi-line side-by-side checksum facts reason=%q locked=%#v", reason, locked)
	}

	out, err := Process([]byte(frozenMixedWorkspaceRequest + "\n"))
	if err != nil || string(out) != frozenMixedWorkspaceSuccess+"\n" {
		t.Fatalf("mixed workspace exact vector err=%v\nwant=%s\ngot=%s", err, frozenMixedWorkspaceSuccess, out)
	}
	var got candidate
	if err := json.Unmarshal(out[:len(out)-1], &got); err != nil {
		t.Fatal(err)
	}
	// go.work selects the workspace and is permitted to be newer than a member
	// module. Toolchain directives are independent suggestions, not equality
	// constraints. Each declaration retains its own input witness.
	requireFact(t, got.Facts, "a-mod", "go.language.declaration", "1.26.0")
	requireFact(t, got.Facts, "a-mod", "go.toolchain.declaration", "go1.26.0")
	requireFact(t, got.Facts, "c-work", "go.language.declaration", "1.27.0")
	requireFact(t, got.Facts, "c-work", "go.toolchain.declaration", "go1.27.0")
	r := mustRequest(t, fixture(t))
	r.Inputs[0] = input("a-mod", "go.mod", "go.mod", "module example.com/fixture\ngo 1.27.0\n")
	r.Inputs[2] = input("c-work", "go.work", "go.work", "go 1.26.0\nuse workspacea\n")
	mustReject(t, requestBytes(t, r), "go", "request-1", "CONFLICTING_VALUE")
	// The workspace ordering compares the whole declared version, patch included.
	r = mustRequest(t, fixture(t))
	r.Inputs[0] = input("a-mod", "go.mod", "go.mod", "module example.com/fixture\ngo 1.27.10\n")
	r.Inputs[2] = input("c-work", "go.work", "go.work", "go 1.27.9\nuse workspacea\n")
	mustReject(t, requestBytes(t, r), "go", "request-1", "CONFLICTING_VALUE")
	r.Inputs[2] = input("c-work", "go.work", "go.work", "go 1.27.10\nuse workspacea\n")
	mustCandidate(t, requestBytes(t, r))
	r = mustRequest(t, fixture(t))
	r.Inputs = append(r.Inputs, input("e-source", "go.source", "cmd/other.go", "package other\n"))
	mustReject(t, requestBytes(t, r), "go", "request-1", "CONFLICTING_VALUE")
}

func TestCompleteGoDirectiveCrossProductAndWorkspaceModuleCombinations(t *testing.T) {
	versions := []string{"1.26.0", "1.27.0"}
	for _, record := range []struct {
		name, family, path, prefix string
		body                       func(string, string) string
	}{
		{"module", "go.mod", "go.mod", "mod", func(language, toolchain string) string {
			return "module example.com/module\ngo " + language + "\ntoolchain go" + toolchain + "\n"
		}},
		{"workspace", "go.work", "go.work", "work", func(language, toolchain string) string {
			return "go " + language + "\ntoolchain go" + toolchain + "\nuse workspacea\n"
		}},
	} {
		for _, language := range versions {
			for _, toolchain := range versions {
				t.Run(record.name+"-"+language+"-"+toolchain, func(t *testing.T) {
					r := oneInputRequest(t)
					r.Inputs[0] = input(record.prefix, record.family, record.path, record.body(language, toolchain))
					got := candidateFor(t, requestBytes(t, r))
					requireFact(t, got.Facts, record.prefix, "go.language.declaration", language)
					requireFact(t, got.Facts, record.prefix, "go.toolchain.declaration", "go"+toolchain)
				})
			}
		}
	}
	for _, moduleLanguage := range versions {
		for _, moduleToolchain := range versions {
			for _, workspaceLanguage := range versions {
				for _, workspaceToolchain := range versions {
					name := "module-" + moduleLanguage + "-" + moduleToolchain + "-workspace-" + workspaceLanguage + "-" + workspaceToolchain
					t.Run(name, func(t *testing.T) {
						r := oneInputRequest(t)
						r.Inputs = []Input{
							input("a-mod", "go.mod", "go.mod", "module example.com/module\ngo "+moduleLanguage+"\ntoolchain go"+moduleToolchain+"\n"),
							input("b-work", "go.work", "go.work", "go "+workspaceLanguage+"\ntoolchain go"+workspaceToolchain+"\nuse workspacea\n"),
						}
						if workspaceLanguage < moduleLanguage {
							mustReject(t, requestBytes(t, r), "go", "request-1", "CONFLICTING_VALUE")
							return
						}
						got := candidateFor(t, requestBytes(t, r))
						requireFact(t, got.Facts, "a-mod", "go.toolchain.declaration", "go"+moduleToolchain)
						requireFact(t, got.Facts, "b-work", "go.toolchain.declaration", "go"+workspaceToolchain)
					})
				}
			}
		}
	}
}

func TestClosedFailureReasonMatrix(t *testing.T) {
	ambiguous := oneInputRequest(t)
	ambiguous.Inputs = append(ambiguous.Inputs, input("input-2", "go.mod", "nested/go.mod", "module example.com/alternate\ngo 1.27.0\n"))
	mustReject(t, requestBytes(t, ambiguous), "go", "request-1", "AMBIGUOUS_BINDING")

	dynamic := oneInputRequest(t)
	dynamic.Inputs[0] = input("source", "go.source", "cmd/main.go", "//go:generate tool\n\npackage main\n")
	mustReject(t, requestBytes(t, dynamic), "go", "request-1", "DYNAMIC_INPUT")

	embedded := oneInputRequest(t)
	embedded.Inputs[0] = input("source", "go.source", "cmd/main.go", "package main\n\n//go:embed data\nvar value string\n")
	mustReject(t, requestBytes(t, embedded), "go", "request-1", "DYNAMIC_INPUT")

	// Near-prefix words are ordinary comments under the Go 1.27 grammar,
	// never directives or build constraints.
	for _, comment := range []string{"//go:embedding data", "//go:generateX tool", "//go:builder darwin"} {
		ordinary := oneInputRequest(t)
		ordinary.Inputs[0] = input("source", "go.source", "cmd/main.go", comment+"\n\npackage main\n")
		mustCandidate(t, requestBytes(t, ordinary))
	}

	credential := oneInputRequest(t)
	credential.Inputs[0] = input("source", "go.source", "cmd/main.go", "package main\nimport \"https://user:secret@example.com/private\"\n")
	mustReject(t, requestBytes(t, credential), "go", "request-1", "CREDENTIAL_INPUT")

	failure := oneInputRequest(t)
	raw := requestBytes(t, failure)
	got, err := process(raw, func(Request) (candidate, string) { panic("injected parser failure") })
	if err != nil || string(got) != literalBoundRejection(failure, "ANALYZER_FAILURE") {
		t.Fatalf("analyzer failure err=%v want=%s got=%s", err, literalBoundRejection(failure, "ANALYZER_FAILURE"), got)
	}
}

func TestClosedGoDeclarationTuplesRejectUnsupportedFutures(t *testing.T) {
	for _, version := range []string{"1.26.0", "1.26.6", "1.27.0", "1.27.1", "1.27.99"} {
		t.Run("accepted-"+version, func(t *testing.T) {
			r := oneInputRequest(t)
			r.Inputs[0] = input("input-1", "go.mod", "go.mod", "module example.com/module\ngo "+version+"\ntoolchain go"+version+"\n")
			mustCandidate(t, requestBytes(t, r))
		})
	}
	for _, version := range []string{"1.25.9", "1.26.100", "1.27.01", "1.28.0", "01.27.0"} {
		t.Run("rejects-"+version, func(t *testing.T) {
			r := oneInputRequest(t)
			r.Inputs[0] = input("input-1", "go.mod", "go.mod", "module example.com/module\ngo "+version+"\n")
			mustReject(t, requestBytes(t, r), "go", "request-1", "UNSUPPORTED_SCHEMA")
			r = mustRequest(t, fixture(t))
			r.Inputs[2] = input("c-work", "go.work", "go.work", "go 1.27.0\ntoolchain go"+version+"\nuse workspacea\n")
			mustReject(t, requestBytes(t, r), "go", "request-1", "UNSUPPORTED_SCHEMA")
		})
	}
}

func TestLiteralBeamfallGoCorpusAtDA38C59(t *testing.T) {
	const corpusRoot = "testdata/beamfall-da38c59"
	const commit = "da38c59eb30b2121cbac37b912485b30b2e54841"
	manifestBytes, err := os.ReadFile(filepath.Join(corpusRoot, "MANIFEST.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Schema  string `json:"schema"`
		Commit  string `json:"commit"`
		Entries []struct {
			Path          string `json:"path"`
			GitBlobSHA1   string `json:"git_blob_sha1"`
			ContentSHA256 string `json:"content_sha256"`
			Expected      string `json:"expected"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Schema != "corvint-analyzer-go-beamfall-corpus/v1" || manifest.Commit != commit || len(manifest.Entries) != 2 {
		t.Fatalf("Beamfall corpus manifest=%s", manifestBytes)
	}
	for _, entry := range manifest.Entries {
		body, err := os.ReadFile(filepath.Join(corpusRoot, entry.Path))
		if err != nil {
			t.Fatal(err)
		}
		content := sha256.Sum256(body)
		if got := "sha256:" + hex.EncodeToString(content[:]); got != entry.ContentSHA256 {
			t.Fatalf("Beamfall corpus content drift path=%s want=%s got=%s", entry.Path, entry.ContentSHA256, got)
		}
		blob := sha1.New()
		fmt.Fprintf(blob, "blob %d\x00", len(body))
		blob.Write(body)
		if got := hex.EncodeToString(blob.Sum(nil)); got != entry.GitBlobSHA1 {
			t.Fatalf("Beamfall corpus blob drift path=%s want=%s got=%s", entry.Path, entry.GitBlobSHA1, got)
		}
		r := Request{Profile: Profile, Family: Family, RequestID: "beamfall-" + strings.ReplaceAll(filepath.Base(entry.Path), ".", "-"), ScopeID: "beamfall-da38c59", CompilationUnitID: "beamfall-hooks", Target: Target{OS: "darwin", Architecture: "arm64", ABI: "none", Features: []string{}}, Inputs: []Input{input("beamfall-source", "go.source", entry.Path, string(body))}}
		raw := requestBytes(t, r)
		if entry.Expected != "CANDIDATE" {
			mustReject(t, raw, Family, r.RequestID, entry.Expected)
			continue
		}
		got := candidateFor(t, raw)
		want := []Fact{
			{Kind: "go.build.constraint", InputHandle: "beamfall-source", RelatedHandle: "-", Subject: "cmd/beamfall/hooks.go", Predicate: "selected-for", Value: "!beamfall_appstore", InstanceID: "beamfall-hooks", EvidenceSHA256: "sha256:2e8e1254a120b1f521bbb80dd5339e35a57cf74e65451773eb299a5c07b16ce7"},
			{Kind: "go.import.static", InputHandle: "beamfall-source", RelatedHandle: "-", Subject: "cmd/beamfall/hooks.go", Predicate: "imports", Value: "github.com/beamfall/core/internal/app", InstanceID: "cmd/beamfall/hooks.go", EvidenceSHA256: "sha256:85a89496e890ef9ea977af8196f2dc299d714f6fe851991b0dab0eb534d8bfe3"},
			{Kind: "go.package", InputHandle: "beamfall-source", RelatedHandle: "-", Subject: "main", Predicate: "declares-package", Value: "main", InstanceID: "beamfall-hooks", EvidenceSHA256: "sha256:c3f57570d12c61d38d227e132e8f44c39279102d4ad70be280f58f736e8464c8"},
			{Kind: "go.source", InputHandle: "beamfall-source", RelatedHandle: "-", Subject: "cmd/beamfall/hooks.go", Predicate: "classifies", Value: "source", InstanceID: "cmd/beamfall/hooks.go", EvidenceSHA256: "sha256:00d08b8bb43e207008b3cd4ec8d3946a1858917f79c5a8ab50e5150052358644"},
		}
		if len(got.Facts) != len(want) {
			t.Fatalf("Beamfall corpus facts path=%s count=%d facts=%+v", entry.Path, len(got.Facts), got.Facts)
		}
		for i := range want {
			if got.Facts[i] != want[i] {
				t.Fatalf("Beamfall corpus fact path=%s index=%d\nwant=%+v\ngot=%+v", entry.Path, i, want[i], got.Facts[i])
			}
		}
	}
}

func TestBuildConstraintByteAndVariableBoundaries(t *testing.T) {
	for _, n := range []int{maxBuildConstraintBytes - 1, maxBuildConstraintBytes, maxBuildConstraintBytes + 1} {
		t.Run(fmt.Sprintf("bytes-%d", n), func(t *testing.T) {
			r := mustRequest(t, fixture(t))
			r.Inputs[3] = input("d-source", "go.source", "cmd/main.go", "//go:build "+strings.Repeat("a", n)+"\n\npackage main\n")
			reason := "EXACT_BINDING_UNAVAILABLE"
			if n > maxBuildConstraintBytes {
				reason = "LIMIT_EXCEEDED"
			}
			mustReject(t, requestBytes(t, r), "go", "request-1", reason)
		})
	}
	for _, n := range []int{maxBuildVariables - 1, maxBuildVariables, maxBuildVariables + 1} {
		t.Run(fmt.Sprintf("variables-%d", n), func(t *testing.T) {
			terms, legacy := make([]string, n), make([]string, n)
			for i := range terms {
				tag := fmt.Sprintf("tag%d", i)
				terms[i] = "(" + tag + " || !" + tag + ")"
				legacy[i] = "// +build " + tag + " !" + tag
			}
			r := mustRequest(t, fixture(t))
			expression, err := constraint.Parse("//go:build " + strings.Join(terms, " && "))
			if err != nil {
				t.Fatal(err)
			}
			body := "//go:build " + expression.String() + "\n" + strings.Join(legacy, "\n") + "\n\npackage main\n"
			r.Inputs[3] = input("d-source", "go.source", "cmd/main.go", body)
			if n <= maxBuildVariables {
				mustCandidate(t, requestBytes(t, r))
				return
			}
			mustReject(t, requestBytes(t, r), "go", "request-1", "LIMIT_EXCEEDED")
		})
	}
}

func TestSuppliedProfileBindRejections(t *testing.T) {

	first := mustRequest(t, fixture(t))
	second := mustRequest(t, fixture(t))
	first.Profile, second.Profile = "invalid-profile-a", "invalid-profile-b"
	first.Inputs[0].SHA256 = "sha256:" + strings.Repeat("0", 64)
	second.Inputs[0].SHA256 = "sha256:" + strings.Repeat("0", 64)
	firstRaw, secondRaw := requestBytes(t, first), requestBytes(t, second)
	mustReject(t, firstRaw, "go", "request-1", "UNKNOWN_FAMILY")
	mustReject(t, secondRaw, "go", "request-1", "UNKNOWN_FAMILY")
	firstOut, _ := Process(firstRaw)
	secondOut, _ := Process(secondRaw)
	if string(firstOut) == string(secondOut) || !strings.Contains(string(firstOut), `"profile":"invalid-profile-a"`) || !strings.Contains(string(secondOut), `"profile":"invalid-profile-b"`) {
		t.Fatalf("post-envelope invalid profiles collapsed\nfirst=%s\nsecond=%s", firstOut, secondOut)
	}
}
func TestRequestGrammarRejectsControlDuplicateAndUnknownData(t *testing.T) {
	r := mustRequest(t, fixture(t))
	r.Inputs[0].Path = "go\x00mod"
	raw, _ := json.Marshal(r)
	out, err := Process(append(raw, '\n'))
	if err != nil || !strings.Contains(string(out), `"reason":"INVALID_PATH"`) {
		t.Fatalf("control path: %v %s", err, out)
	}
	r = mustRequest(t, fixture(t))
	r.Inputs[1].Handle = r.Inputs[0].Handle
	raw, _ = json.Marshal(r)
	out, err = Process(append(raw, '\n'))
	if err != nil || !strings.Contains(string(out), `"reason":"DUPLICATE_VALUE"`) {
		t.Fatalf("duplicate handle: %v %s", err, out)
	}
	mutated := strings.Replace(string(fixture(t)), `"profile":`, `"unknown":0,"profile":`, 1)
	mustReject(t, []byte(mutated), "go", "request-1", "NONCANONICAL_REQUEST")
}
func TestGoWorkSourceAndBuildMatrix(t *testing.T) {
	r := mustRequest(t, fixture(t))
	r.Inputs[3] = input("d-source", "go.source", "cmd/main_test.go", "//go:build darwin && arm64\n\npackage main\n")
	raw, _ := json.Marshal(r)
	out, err := Process(append(raw, '\n'))
	if err != nil || !strings.Contains(string(out), `"kind":"go.build.constraint"`) || !strings.Contains(string(out), `"value":"test"`) {
		t.Fatalf("selected test source: %v %s", err, out)
	}
	r.Inputs[3] = input("d-source", "go.source", "cmd/main.go", "//go:build linux\n\npackage main\n")
	raw, _ = json.Marshal(r)
	out, err = Process(append(raw, '\n'))
	if err != nil || !strings.Contains(string(out), `"reason":"EXACT_BINDING_UNAVAILABLE"`) {
		t.Fatalf("excluded constraint: %v %s", err, out)
	}
	r.Inputs[3] = input("d-source", "go.source", "cmd/main.go", "//go:build enterprise\n\npackage main\n")
	raw, _ = json.Marshal(r)
	out, err = Process(append(raw, '\n'))
	if err != nil || !strings.Contains(string(out), `"reason":"EXACT_BINDING_UNAVAILABLE"`) {
		t.Fatalf("empty build tags unknown false: %v %s", err, out)
	}
	r.Inputs[3] = input("d-source", "go.source", "cmd/main.go", "//go:build !enterprise\n\npackage main\n")
	out, err = Process(requestBytes(t, r))
	if err != nil || !strings.Contains(string(out), `"value":"!enterprise"`) {
		t.Fatalf("empty build tags negation: %v %s", err, out)
	}
	r.Inputs[3] = input("d-source", "go.source", "cmd/main.go", "//go:build go1.10\n\npackage main\n")
	out, err = Process(requestBytes(t, r))
	if err != nil || !strings.Contains(string(out), `"value":"go1.10"`) {
		t.Fatalf("release tag selection: %v %s", err, out)
	}
	r = mustRequest(t, fixture(t))
	r.Inputs[3] = input("d-source", "go.source", "cmd/main_linux_test.go", "package main\n")
	mustReject(t, requestBytes(t, r), "go", "request-1", "EXACT_BINDING_UNAVAILABLE")
	r = mustRequest(t, fixture(t))
	r.Inputs[3] = input("d-source", "go.source", "cmd/main_armbe.go", "package main\n")
	mustReject(t, requestBytes(t, r), "go", "request-1", "EXACT_BINDING_UNAVAILABLE")
	r = mustRequest(t, fixture(t))
	r.Inputs[3] = input("d-source", "go.source", "cmd/main_darwin_arm64_test.go", "//go:build darwin && arm64\n// +build arm64,darwin\n\npackage main\n")
	out, err = Process(requestBytes(t, r))
	if err != nil || !strings.Contains(string(out), `"value":"test"`) || !strings.Contains(string(out), `"value":"darwin && arm64"`) {
		t.Fatalf("paired constraints: %v %s", err, out)
	}
	r.Inputs[3] = input("d-source", "go.source", "cmd/main_darwin_arm64_test.go", "//go:build darwin && arm64\n// +build darwin\n// +build arm64\n\npackage main\n")
	out, err = Process(requestBytes(t, r))
	if err != nil || !strings.Contains(string(out), `"value":"darwin && arm64"`) {
		t.Fatalf("multiline legacy AND: %v %s", err, out)
	}
	r.Inputs[3] = input("d-source", "go.source", "cmd/main.go", "//go:build darwin\n// +build darwin darwin,arm64\n\npackage main\n")
	out, err = Process(requestBytes(t, r))
	if err != nil || !strings.Contains(string(out), `"value":"darwin"`) {
		t.Fatalf("redundant legacy term: %v %s", err, out)
	}
	r.Inputs[3] = input("d-source", "go.source", "cmd/main.go", "//go:build darwin && (cgo || !cgo)\n// +build darwin\n\npackage main\n")
	out, err = Process(requestBytes(t, r))
	if err != nil || !strings.Contains(string(out), `"value":"darwin && (cgo || !cgo)"`) {
		t.Fatalf("tautological legacy equivalence: %v %s", err, out)
	}
	r.Inputs[3] = input("d-source", "go.source", "cmd/main.go", "//go:build darwin || (darwin && arm64)\n// +build darwin\n\npackage main\n")
	out, err = Process(requestBytes(t, r))
	if err != nil || !strings.Contains(string(out), `"value":"darwin || (darwin && arm64)"`) {
		t.Fatalf("absorption legacy equivalence: %v %s", err, out)
	}
	r.Inputs[3] = input("d-source", "go.source", "cmd/main.go", "//go:build (darwin && arm64) || (darwin && !arm64)\n// +build darwin\n\npackage main\n")
	out, err = Process(requestBytes(t, r))
	if err != nil || !strings.Contains(string(out), `"value":"(darwin && arm64) || (darwin && !arm64)"`) {
		t.Fatalf("consensus legacy equivalence: %v %s", err, out)
	}
	// A legacy header line separated from //go:build by a blank line is still
	// in the header, so it must be equivalent rather than silently win.
	r.Inputs[3] = input("d-source", "go.source", "cmd/main.go", "// +build darwin\n\n//go:build linux\n\npackage main\n")
	mustReject(t, requestBytes(t, r), "go", "request-1", "MALFORMED_INPUT")
	// go/build honours noncanonical legacy spellings; the closed parser abstains
	// rather than silently dropping a constraint that excludes the file.
	for _, legacy := range []string{"//+build linux", "//  +build linux", "//\t+build linux"} {
		r.Inputs[3] = input("d-source", "go.source", "cmd/main.go", legacy+"\n\npackage main\n")
		mustReject(t, requestBytes(t, r), "go", "request-1", "MALFORMED_INPUT")
	}
	r.Inputs[3] = input("d-source", "go.source", "cmd/main.go", "package main\nvar raw = `\n//go:build first\n//go:build second\n`\n")
	out, err = Process(requestBytes(t, r))
	if err != nil || !strings.Contains(string(out), `"status":"CANDIDATE"`) {
		t.Fatalf("inert raw literal build lines: %v %s", err, out)
	}
	r.Inputs[3] = input("d-source", "go.source", "cmd/main_darwin_arm64_test.go", "// +build darwin\n// +build arm64\n\n// Code generated by test; DO NOT EDIT.\npackage main\n")
	out, err = Process(requestBytes(t, r))
	if err != nil || !strings.Contains(string(out), `"value":"generated"`) {
		t.Fatalf("legacy/generated: %v %s", err, out)
	}
	r.Inputs[3] = input("d-source", "go.source", "cmd/main.go", "package main\nimport \"example.com/@bad\"\n")
	mustReject(t, requestBytes(t, r), "go", "request-1", "MALFORMED_INPUT")
	r = mustRequest(t, fixture(t))
	r.Inputs[3] = input("d-source", "go.source", "cmd/.hidden.go", "package main\n")
	mustReject(t, requestBytes(t, r), "go", "request-1", "EXACT_BINDING_UNAVAILABLE")
	r.Inputs[3] = input("d-source", "go.source", "cmd/_hidden.go", "package main\n")
	mustReject(t, requestBytes(t, r), "go", "request-1", "EXACT_BINDING_UNAVAILABLE")
}

func TestGo127CompleteFilenameSuffixTable(t *testing.T) {
	for _, os := range []string{"aix", "android", "darwin", "dragonfly", "freebsd", "hurd", "illumos", "ios", "js", "linux", "nacl", "netbsd", "openbsd", "plan9", "solaris", "wasip1", "windows", "zos"} {
		r := mustRequest(t, fixture(t))
		r.Inputs[3] = input("d-source", "go.source", "cmd/main_"+os+".go", "package main\n")
		if os == "darwin" {
			mustCandidate(t, requestBytes(t, r))
		} else {
			mustReject(t, requestBytes(t, r), "go", "request-1", "EXACT_BINDING_UNAVAILABLE")
		}
	}
	for _, arch := range []string{"386", "amd64", "amd64p32", "arm", "armbe", "arm64", "arm64be", "loong64", "mips", "mipsle", "mips64", "mips64le", "mips64p32", "mips64p32le", "ppc", "ppc64", "ppc64le", "riscv", "riscv64", "s390", "s390x", "sparc", "sparc64", "wasm"} {
		r := mustRequest(t, fixture(t))
		r.Inputs[3] = input("d-source", "go.source", "cmd/main_"+arch+".go", "package main\n")
		if arch == "arm64" {
			mustCandidate(t, requestBytes(t, r))
		} else {
			mustReject(t, requestBytes(t, r), "go", "request-1", "EXACT_BINDING_UNAVAILABLE")
		}
	}
	// Go 1.27 only treats a recognized suffix after an underscore as a tag.
	r := mustRequest(t, fixture(t))
	r.Inputs[3] = input("d-source", "go.source", "cmd/linux.go", "package main\n")
	mustCandidate(t, requestBytes(t, r))
}

func TestGo127BuildSelectionMatchesMatchFileOracle(t *testing.T) {
	cases := []struct {
		name, path, body string
	}{
		{"indented-go-linux", "cmd/main.go", " //go:build linux\n\npackage main\n"},
		{"indented-legacy-linux", "cmd/main.go", " // +build linux\n\npackage main\n"},
		{"go-darwin", "cmd/main.go", "//go:build darwin\n\npackage main\n"},
		{"go-linux", "cmd/main.go", "//go:build linux\n\npackage main\n"},
		{"go-windows", "cmd/main.go", "//go:build windows\n\npackage main\n"},
		{"go-unix", "cmd/main.go", "//go:build unix\n\npackage main\n"},
		{"legacy-darwin", "cmd/main.go", "// +build darwin\n\npackage main\n"},
		{"legacy-linux", "cmd/main.go", "// +build linux\n\npackage main\n"},
		{"legacy-windows", "cmd/main.go", "// +build windows\n\npackage main\n"},
		{"compiler-gc", "cmd/main.go", "//go:build gc\n\npackage main\n"},
		{"compiler-gccgo", "cmd/main.go", "//go:build gccgo\n\npackage main\n"},
		{"release-present", "cmd/main.go", "//go:build go1.27\n\npackage main\n"},
		{"release-future", "cmd/main.go", "//go:build go1.28\n\npackage main\n"},
		{"suffix-linux", "cmd/main_linux.go", "package main\n"},
		{"suffix-before-first-dot-linux", "cmd/main_linux.pb.go", "package main\n"},
		{"suffix-after-first-dot-ignored", "cmd/main.linux_amd64.go", "package main\n"},
		{"block-comment-header-go-linux", "cmd/main.go", "/* Copyright */\n\n//go:build linux\n\npackage main\n"},
		{"multiline-block-comment-header-go-linux", "cmd/main.go", "/*\nCopyright\n*/\n//go:build linux\n\npackage main\n"},
		{"legacy-without-blank-line-ignored", "cmd/main.go", "// +build linux\npackage main\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := matchFileOracle(t, tc.path, tc.body, "darwin", "arm64")
			r := mustRequest(t, fixture(t))
			r.Inputs[3] = input("d-source", "go.source", tc.path, tc.body)
			out, err := Process(requestBytes(t, r))
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Contains(string(out), `"status":"CANDIDATE"`)
			if got != want {
				t.Fatalf("public selection=%t MatchFile=%t output=%s", got, want, out)
			}
		})
	}
	for _, target := range []struct{ os, arch string }{{"darwin", "arm64"}, {"linux", "amd64"}, {"windows", "amd64"}} {
		for _, tc := range cases {
			got, err := go127MatchFileForTarget(tc.path, tc.body, target.os, target.arch)
			if err != nil {
				t.Fatalf("candidate MatchFile %s/%s %s: %v", target.os, target.arch, tc.name, err)
			}
			if want := matchFileOracle(t, tc.path, tc.body, target.os, target.arch); got != want {
				t.Fatalf("%s/%s %s candidate=%t oracle=%t", target.os, target.arch, tc.name, got, want)
			}
		}
	}
}

func TestFilenameExcludedSourceIsNotRead(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module example.com/probe\ngo 1.27.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	const body = "//go:build ???\n\npackage broken\n"
	if err := os.WriteFile(filepath.Join(directory, "main_linux.go"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	context := build.Context{GOOS: "darwin", GOARCH: "arm64", Compiler: "gc", CgoEnabled: false, ReleaseTags: releaseTags()}
	if selected, err := context.MatchFile(directory, "main_linux.go"); err != nil || selected {
		t.Fatalf("go/build selected=%t err=%v", selected, err)
	}
	request := oneInputRequest(t)
	request.Inputs[0] = input("source", "go.source", "main_linux.go", body)
	mustReject(t, requestBytes(t, request), "go", "request-1", "EXACT_BINDING_UNAVAILABLE")
}

func matchFileOracle(t testing.TB, name, body, goos, goarch string) bool {
	t.Helper()
	ctx := build.Context{GOOS: goos, GOARCH: goarch, Compiler: "gc", CgoEnabled: false, BuildTags: []string{}, ToolTags: []string{}, ReleaseTags: releaseTags(), OpenFile: func(string) (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(body)), nil }}
	ok, err := ctx.MatchFile(path.Dir(name), path.Base(name))
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

func TestGo127BuildEquivalenceVariableBound(t *testing.T) {
	tags := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m"}
	expression := strings.Join(tags, " && ")
	r := mustRequest(t, fixture(t))
	r.Inputs[3] = input("d-source", "go.source", "cmd/main.go", "//go:build "+expression+"\n// +build "+strings.Join(tags, ",")+"\n\npackage main\n")
	mustReject(t, requestBytes(t, r), "go", "request-1", "LIMIT_EXCEEDED")
}
func TestClosedLockAndEnvelopeBounds(t *testing.T) {
	r := mustRequest(t, fixture(t))
	r.Inputs[0] = input("a-mod", "go.mod", "go.mod", "module example.com/fixture\ngo 1.27.0\nrequire example.com/a v1.0.0\n")
	r.Inputs[1] = input("b-sum", "go.sum", "go.sum", "example.com/a v1.0.0 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\nexample.com/a v1.0.0 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n")
	raw, _ := json.Marshal(r)
	out, err := Process(append(raw, '\n'))
	if err != nil || !strings.Contains(string(out), `"reason":"CONFLICTING_VALUE"`) {
		t.Fatalf("conflicting sum: %v %s", err, out)
	}
	// go.sum is written in module.Sort order: path, then semantic version, so
	// v0.9.0 precedes v0.10.0 even though it sorts after it bytewise.
	hash := " h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n"
	r.Inputs[0] = input("a-mod", "go.mod", "go.mod", "module example.com/fixture\ngo 1.27.0\nrequire example.com/a v0.10.0\n")
	r.Inputs[1] = input("b-sum", "go.sum", "go.sum", "example.com/a v0.9.0/go.mod"+hash+"example.com/a v0.10.0"+hash+"example.com/a v0.10.0/go.mod"+hash)
	mustCandidate(t, requestBytes(t, r))
	r.Inputs[1] = input("b-sum", "go.sum", "go.sum", "example.com/a v0.10.0"+hash+"example.com/a v0.10.0/go.mod"+hash+"example.com/a v0.9.0/go.mod"+hash)
	mustReject(t, requestBytes(t, r), "go", "request-1", "DUPLICATE_VALUE")
	if out, err = Process(append(make([]byte, MaxRequestBytes), 'x')); err != nil || !strings.Contains(string(out), `"reason":"LIMIT_EXCEEDED"`) {
		t.Fatalf("request cap: %v %s", err, out)
	}
	r = mustRequest(t, fixture(t))
	r.RequestID = "bad\x00id"
	raw, _ = json.Marshal(r)
	out, err = Process(append(raw, '\n'))
	if err != nil || !strings.Contains(string(out), `"family":"unknown"`) || !strings.Contains(string(out), `"reason":"INVALID_IDENTIFIER"`) {
		t.Fatalf("sentinel identity: %v %s", err, out)
	}
}
func TestProcessNeverExecutesCallerSource(t *testing.T) {
	marker := t.TempDir() + "/must-not-exist"
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	address := "127.0.0.1:1"
	connected := make(chan bool, 1)
	if err == nil {
		address = listener.Addr().String()
		defer listener.Close()
		go func() {
			conn, acceptErr := listener.Accept()
			if conn != nil {
				conn.Close()
			}
			connected <- acceptErr == nil
		}()
	}
	t.Setenv("PATH", marker)
	t.Setenv("HOME", marker)
	t.Setenv("CORVINT_AMBIENT_MARKER", marker)
	r := mustRequest(t, fixture(t))
	r.Inputs[3] = input("d-source", "go.source", "cmd/main.go", "package main\nimport (\n\t\"net\"\n\t\"os\"\n\t\"os/exec\"\n)\nfunc init() { _, _ = net.Dial(\"tcp\", \""+address+"\"); _ = exec.Command(\"touch\", \""+marker+"\").Run(); _ = os.WriteFile(os.Getenv(\"CORVINT_AMBIENT_MARKER\"), nil, 0600) }\n")
	raw, _ := json.Marshal(r)
	if out, err := Process(append(raw, '\n')); err != nil || !strings.Contains(string(out), `"status":"CANDIDATE"`) {
		t.Fatalf("source parse: %v %s", err, out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("caller source executed: %v", err)
	}
	if listener != nil {
		listener.Close()
		if <-connected {
			t.Fatal("caller source reached the network spy")
		}
	}
}
func TestEvidenceFrameBindsAllFourteenFields(t *testing.T) {
	r := mustRequest(t, fixture(t))
	r.Inputs[0] = input("a-mod", "go.mod", "go.mod", "module example.com/fixture\ngo 1.27.0\nrequire example.com/a v1.0.0\n")
	r.Inputs[1] = input("b-sum", "go.sum", "go.sum", "example.com/a v1.0.0 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n")
	for i := range r.Inputs {
		body, reason := decodeInput(r.Inputs[i])
		if reason != "" {
			t.Fatal(reason)
		}
		r.Inputs[i].ContentBase64 = string(body)
	}
	c, reason := analyze(r)
	if reason != "" {
		t.Fatal(reason)
	}
	var dep Fact
	for _, f := range c.Facts {
		if f.Kind == "go.dependency.locked" {
			dep = f
			break
		}
	}
	if dep.RelatedHandle != "b-sum" {
		t.Fatalf("related handle=%q", dep.RelatedHandle)
	}
	state := evidenceState{target: `{"os":"darwin","architecture":"arm64","abi":"none","features":[]}`, buf: make([]byte, 0, 512)}
	base, reason := evidence(&state, r.Family, r, "a-mod", r.Inputs[0].SHA256, "b-sum", r.Inputs[1].SHA256, dep)
	if reason != "" || base != dep.EvidenceSHA256 {
		t.Fatalf("base=%s fact=%s reason=%s", base, dep.EvidenceSHA256, reason)
	}
	mutations := []struct {
		family, inputHandle, inputDigest, relatedHandle, relatedDigest string
		r                                                              Request
		state                                                          evidenceState
		f                                                              Fact
	}{
		{"other", "a-mod", r.Inputs[0].SHA256, "b-sum", r.Inputs[1].SHA256, r, state, dep},
		{r.Family, "a-mod", r.Inputs[0].SHA256, "b-sum", r.Inputs[1].SHA256, Request{RequestID: "changed", ScopeID: r.ScopeID, CompilationUnitID: r.CompilationUnitID}, state, dep},
		{r.Family, "a-mod", r.Inputs[0].SHA256, "b-sum", r.Inputs[1].SHA256, Request{RequestID: r.RequestID, ScopeID: "changed", CompilationUnitID: r.CompilationUnitID}, state, dep},
		{r.Family, "a-mod", r.Inputs[0].SHA256, "b-sum", r.Inputs[1].SHA256, Request{RequestID: r.RequestID, ScopeID: r.ScopeID, CompilationUnitID: "changed"}, state, dep},
		{r.Family, "a-mod", r.Inputs[0].SHA256, "b-sum", r.Inputs[1].SHA256, r, evidenceState{target: `{"os":"darwin","architecture":"arm64","abi":"other","features":[]}`, buf: make([]byte, 0, 512)}, dep},
		{r.Family, "changed", r.Inputs[0].SHA256, "b-sum", r.Inputs[1].SHA256, r, state, dep}, {r.Family, "a-mod", "sha256:" + strings.Repeat("1", 64), "b-sum", r.Inputs[1].SHA256, r, state, dep}, {r.Family, "a-mod", r.Inputs[0].SHA256, "changed", r.Inputs[1].SHA256, r, state, dep}, {r.Family, "a-mod", r.Inputs[0].SHA256, "b-sum", "sha256:" + strings.Repeat("2", 64), r, state, dep},
	}
	for i, m := range mutations {
		got, err := evidence(&m.state, m.family, m.r, m.inputHandle, m.inputDigest, m.relatedHandle, m.relatedDigest, m.f)
		if err != "" || got == base {
			t.Fatalf("frame prefix mutation %d: %s %s", i, err, got)
		}
	}
	for i := range []string{"Kind", "Subject", "Predicate", "Value", "InstanceID"} {
		f := dep
		switch i {
		case 0:
			f.Kind = "changed"
		case 1:
			f.Subject = "changed"
		case 2:
			f.Predicate = "changed"
		case 3:
			f.Value = "changed"
		case 4:
			f.InstanceID = "changed"
		}
		got, err := evidence(&state, r.Family, r, "a-mod", r.Inputs[0].SHA256, "b-sum", r.Inputs[1].SHA256, f)
		if err != "" || got == base {
			t.Fatalf("fact mutation %d: %s %s", i, err, got)
		}
	}
}
func TestProspectiveOutputAccountingIsExact(t *testing.T) {
	raw := fixture(t)
	r := mustRequest(t, raw)
	for i := range r.Inputs {
		body, reason := decodeInput(r.Inputs[i])
		if reason != "" {
			t.Fatal(reason)
		}
		r.Inputs[i].ContentBase64 = string(body)
	}
	c, reason := analyze(r)
	if reason != "" {
		t.Fatal(reason)
	}
	encoded, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := c.outputBytes, len(encoded)+1; got != want {
		t.Fatalf("prospective bytes=%d, canonical bytes=%d", got, want)
	}
}
func mustRequest(t testing.TB, raw []byte) Request {
	t.Helper()
	var r Request
	if err := json.Unmarshal(raw[:len(raw)-1], &r); err != nil {
		t.Fatal(err)
	}
	return r
}
func BenchmarkAnalyzeCandidate(b *testing.B) {
	raw := benchmarkFixture(b)
	r := mustRequest(b, raw)
	for i := range r.Inputs {
		decoded, reason := decodeInput(r.Inputs[i])
		if reason != "" {
			b.Fatal(reason)
		}
		r.Inputs[i].ContentBase64 = string(decoded)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, reason := analyze(r)
		if reason != "" || len(out.Facts) != 32 {
			b.Fatal(reason)
		}
	}
}
