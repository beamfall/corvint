package analyzershell

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"strings"
	"testing"
)

const beamfallContextPacketGzipBase64 = `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`
const beamfallRelayE2EGzipBase64 = `H4sICFRZjmoCA2JlYW1mYWxsLXJlbGF5LWNvcmUtZTJlLnNoAG1SXU/bQBB831+xvUYJebi45DFRKjnGLZFSXJkgtQJqXew1OWH7rPOZQiH/vXch5KPlxbpbz87Mzt7HD17baG8pK4+qB1yKZgUNGeTUKqxlTbmQBUAcRYsJ65ykGdpvJnUlSrLH56l/eZ5cRldxEF5/ul2zvjcYMOx2sf6d9RkEURwmcfg9mjhs6H/74s/nya464qzjqG2TtyRR5qIo2JqBzPEa+R88bAqHYTKdXSRns3jE1wxvx2hWVAHitur8lfeGyhq500Y0WtTY0yVynVuuLY71MPwxWwAVDR11vyfFIJcA1owbfOfbquPLCz4jpSuFLFWaUFOtsFIGc9VW2QgP0Z+7wzHSozR4OsY1lPc2QeT1gScAeqyVNvg1CvzgPHR5bY/7kFKRrsi7U3zZyiJzQcGdws0FudqzeZoK8ZQx3DamZfZWghM7cno8jK28T7NbCQ42HG936APsworDuf/zdaU2t8kp3vzz78xlOfnf3AFu032M2mvfuCENNfZRpjZaYyW4bivs/VrYYuzYAruAmEQxlZXQT9ZGp2cty8qQfaiFR0NC/gB/AVoa+0vsAgAA`
const beamfallSDKGateGzipBase64 = `H4sICFRZjmoCA2JlYW1mYWxsLXNkay1nYXRlLnNoAK1YbXPbNhL+zl+xYeVSsk3Jcu7aOdl0xnFkR3N+ydhqrx3b0dAUJPFEgQoBWsnJ+u+3C4AUSclt5q75YIHAYrH77OLZRX5400pF0noKeYvxZ3jyxcQSTILL0hjm4ZyN/DCygiHYtfowTLg/Yzg8sButZtO2rCjkchD4wYR5dm15cXN5en1x1htc9q77g7PTs4/djltb9q8+fejddtyWnM1XrSfmz0Z+FLnzKB2H3BXDqTuOI5+Pg9BVqla21hvFwXQwCiOle33Sqjn2JXNp1QjGC86SsmS+d9VUq0WVMpyxOJUk+757enV+enmpLb68OfvnoN+76t780h/cdc9urj/cddyfDkoGyXjKuGfbVhA/s8Qfs8EoiuP18bnKs5tfu7enF93B+eXNze3gvHeJaEgmZMjHrWyzqzY3/y1ijqdYgS8I3w1bbQi5BWDbL7v3bw7cfzzuNvATgAWTGOyQP/tROITv8Aa26T758VBr+xpK0MOjI4sJP7CsDFsWDesNWOIi7vUjUDPob9vGKX8xBffcA/c5n1e/Nji1NnienoWlSJ/qrc/3n73HXa+1j+40jmCeoEFH6uyVk/m+jqgNhyetIXtu8TSK4EEZ9wIyAXcIzkPywJ18Mkgxb4O22/7pIJsjyZRZK8tKWMQQ3MFavXHnHlxehpwCbMMj7U6YTBMOB0oOr0ABDVByDRu879mdzMAdbfGO8NxMLbI35drWlA9RmNZL+PN4gWP869HNxAsBezuisVVfrdasLVFyRasK7hE483Do7YgHLqSfSKbHwUL/6n00ooDUbPyD2+nn078+2NvcPdnqmUz8OWwCD93fev1s2dlc18kA7bcHDmAif4fk39460O/eXlmUw/M4kbCFijxjoyIRMi9nHZoGBByayGpNnS2kZbBxV6ohAO0wgmlGBk7zhRfcjBDaV8KVa/Aq+YVTBYEsThUhM10QpCBWhHCqIIBGeXbKp2gMp1nDOaVjDN+UGOflXfHfboM4gkQMFRXV1+p1Asct297IBO/z41DYBhdROtA3Zq3iwMiqQxQVZWxXDhtFB0x0OrDwQwKpWATM2koUkK4t83HHNVCsivYv83G+j3DN5nFc2EfHeLVy2dGkWrzHi1BONq9xQn7NplhawZ1DNUOpEu8xHIQjCOLZzOdD4ljSMoICLR6e/Ng+AjlhOmh63UVOFuDK7fXE3jCYyjq4ClVXkY4qsUphEni1dxSAqGzISKH/uiF63e3Cz38Hd/GXGiJYoQCWU0KwJMR6+B9fhjFH/vmShgkTBpU40Vatqx7qpMEoNIC7BnDK0iRAS9gXsv+x4Ndr9GAWFeu3M53mWynLMmLuB1NqHbI2oJQTeWMxT2JCZD2RxAsBZm8+ANVCgB/I1I/oPle2ExvMppLNKL++oxnL9jd/U//shr1mYFPFqkfYToXTX5N7ldr/bEPO8EUHCQ7yTiDhurwLjsDm4r4jEBjWeXzctev3n+3HvYZdnOwUP+r3RG179fvmox413jX239VaD214OGzNnaJJ6zZPQ2Iah5I1pvAvTV7yODdWR0nAKMaURnaFrYopKTOXj2BF9dpEm/wcx1jchdRlCm3A5cWEEiRhPiKQlDPiCIaxSsis5GcFPVNpY880xlTG1u3L1/W8rTww9Kv9KDsB9AoQYPgvP3S9P7tZpqNsqw9yZhhzBsfHx1XUXvUk90EdTBhUnanA/6J70edMgVdwy/Simb1LpQpqhytHxTOPqDqrEslZKAT27ZVwYjSTst/l4FGPAdTyg75Vs3jIPF/GszAwM+s7upH6hYgoVfqCmzyQcWyIAi9OyoOtCjQYTuuzjKUfdVqmAd+pqd57H2pvs/4bh1UU9HEVGII4xX6CxxKGTKI/IWcFSP4MjNdDg7O5e9nBur6oqJuIOO+7F73rPDeRo+taGPawgTg2McFxI5fJs98umbkjIBSw02yPdnb24YlF2K3oL63kgSM8xrp9Y9q+0X8Ctip4QmJpSuz8oEKug3F45ZSSPr94VuFFsBG2jBRdxalEk5r5sHSYqrRsd9yVenhsVEm7UKe2PCEsYyYWUaxN2/TlD9OYR9+q2qplK1N3UNKWP0MvTvtdfHleng/63bt+x8WHNLzxoF3QqtMKbyW96TGVeJAmCePBNxojDIHEK6taoSAJ57JFYgO6UU0x+Z8P/QH6E4ZxWeDrBwln5o58jtUbUyWOR5QY5DsssMRPkJZ8Sfu0gQlzEQfRhEs2kkB7cAOuKqUksQ+hFCgWMLQWfEmmCrytQEmAbRMlKz7Xsc6r5gR108KEWI96qsDndLnQEMkCqbTS2ca+HJJREs/wECRvZCVUgzLoxwhvZBN6ElKB9Kz3TRklExfqUSRS7I73lVYRo6HAGRsKvM44kOjuVNmAn2toIEjYkHGJDZVoFuOFnQLZI9IZUlDVvlLIdE+hJQ3SpQjmKs37DkJ1oPym1GQEqmoeEPrg9HHiVgv/yhJCoH6GR7Ov8qXHn33s/rhs1Byi2JRLr23p2tI7v/PyAkOQD6Q/NhXGkF42a2iPHAp5SuG96PU//vJ+cNvFvPr9U9dDoeLc9ekVPTDX+x9K9P/H1hvjixZr0oDj+hjDRGe57jzGuyxcTJmP3dMPqkFGzc7zrtMogDiORzMdAPwd6P/mqutJNwLqGtSN0YVOS9gb91HJd4DoSKgkIdKcYTZjCezYuViuwMrZrxRQ9P6ZySyMNNQP7JKEwmcj0v8nm8wx1bI6nxO/zsqtPFYxu/iaoF2bj7hMHA0DMuwOLm673Wvb+i/rS6i7URUAAA==`

func TestPinnedShellCorpusIsLiteralAndBound(t *testing.T) {
	entries := []struct {
		name, commit, path, sha, encoded, response string
		bytes                                      int
	}{
		{"beamfall", "da38c59eb30b2121cbac37b912485b30b2e54841", "script/context-packet.sh", "84108938b18b12a8a6a67990d565f66cfc7170683fef099fb0cf7bc2fe886150", beamfallContextPacketGzipBase64, `{"profile":"corvint-analyzer-candidate/experimental","family":"shell","request_id":"req-1","status":"REJECTED","scope_id":"scope","compilation_unit_id":"unit","target":{"os":"linux","architecture":"amd64","abi":"none","features":["bash-5.2"]},"input_echoes":[{"handle":"pinned-beamfall","sha256":"sha256:84108938b18b12a8a6a67990d565f66cfc7170683fef099fb0cf7bc2fe886150"}],"reason":"DYNAMIC_INPUT"}` + "\n", 14391},
		{"relay", "9723152fdd4ead36b32553a171d98749e4fc23e9", "script/relay-core-e2e.sh", "66b02253abc5faa372441776a06820caecf8a8831c075e672ee120958c4868b1", beamfallRelayE2EGzipBase64, `{"profile":"corvint-analyzer-candidate/experimental","family":"shell","request_id":"req-1","status":"REJECTED","scope_id":"scope","compilation_unit_id":"unit","target":{"os":"linux","architecture":"amd64","abi":"none","features":["bash-5.2"]},"input_echoes":[{"handle":"pinned-relay","sha256":"sha256:66b02253abc5faa372441776a06820caecf8a8831c075e672ee120958c4868b1"}],"reason":"DYNAMIC_INPUT"}` + "\n", 748},
		{"sdk", "5536ecde6d12214d6786a6832e537bd5d4feca02", "script/gate.sh", "4a21753f97f725d2f2b656b8befa1bcf292c630c629f17982c7b1609212aaef4", beamfallSDKGateGzipBase64, `{"profile":"corvint-analyzer-candidate/experimental","family":"shell","request_id":"req-1","status":"REJECTED","scope_id":"scope","compilation_unit_id":"unit","target":{"os":"linux","architecture":"amd64","abi":"none","features":["bash-5.2"]},"input_echoes":[{"handle":"pinned-sdk","sha256":"sha256:4a21753f97f725d2f2b656b8befa1bcf292c630c629f17982c7b1609212aaef4"}],"reason":"DYNAMIC_INPUT"}` + "\n", 5457},
	}
	total := 0
	for _, entry := range entries {
		t.Run(entry.name, func(t *testing.T) {
			if len(entry.commit) != 40 || !pathOK(entry.path) || len(entry.sha) != 64 {
				t.Fatalf("invalid immutable coordinate: %+v", entry)
			}
			body := decodePinnedShell(t, entry.encoded, entry.bytes)
			sum := sha256.Sum256(body)
			if got := hex.EncodeToString(sum[:]); got != entry.sha {
				t.Fatalf("literal blob digest=%s want=%s", got, entry.sha)
			}
			if len(body) > MaxInputBytes || len(body) > MaxRequestBytes || entry.bytes > MaxInputBytes {
				t.Fatalf("literal input bound body=%d declared=%d", len(body), entry.bytes)
			}
			r := fixture()
			r.Target = Target{OS: "linux", Architecture: "amd64", ABI: "none", Features: []string{"bash-5.2"}}
			r.Inputs = []Input{{Handle: "pinned-" + entry.name, Family: "shell.posix-bash", Path: entry.path, SHA256: "sha256:" + entry.sha, ContentBase64: base64.StdEncoding.EncodeToString(body)}}
			raw := wire(t, r)
			if len(raw) > MaxRequestBytes {
				t.Fatalf("request bound bytes=%d", len(raw))
			}
			got := AnalyzeCanonical(raw)
			if len(got) > MaxOutputBytes || !bytes.Equal(got, []byte(entry.response)) {
				t.Fatalf("pinned response bytes=%d\ngot:  %q\nwant: %q", len(got), got, entry.response)
			}
			assertCompleteBoundFailure(t, got, r, "DYNAMIC_INPUT")
			total += len(body)
		})
	}
	if total != 20596 {
		t.Fatalf("corpus bytes=%d", total)
	}
}

func decodePinnedShell(t *testing.T, encoded string, expected int) []byte {
	t.Helper()
	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(io.LimitReader(reader, int64(expected)+1))
	closeErr := reader.Close()
	if err != nil || closeErr != nil || len(body) != expected {
		t.Fatalf("literal decode bytes=%d err=%v close=%v", len(body), err, closeErr)
	}
	if strings.ContainsRune(string(body), 0) {
		t.Fatal("literal corpus contains NUL")
	}
	return body
}
