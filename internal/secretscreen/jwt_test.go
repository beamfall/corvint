package secretscreen

import (
	"strings"
	"testing"
)

// LTA-V0-016 (V1-1704): the writer's JWT branch requires a compact-JWS
// header, a base64url JSON object, so real JWT shapes are still detected and
// redacted while dotted identifiers pass. StoredV1Pattern is unchanged.
func TestLTAV0016JWTBranchRequiresHeaderStructure(t *testing.T) {
	const (
		header  = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
		payload = "eyJzdWIiOiJzeW50aGV0aWMiLCJpYXQiOjE3MDAwMDAwMDB9"
		sig     = "c3ludGhldGljLXNpZ25hdHVyZS1ub3QtcmVhbA"
		jwt     = header + "." + payload + "." + sig
	)
	for name, text := range map[string]string{
		"bare":           jwt,
		"spaced header":  "eyAiYWxnIjogIm5vbmUiIH0." + payload + "." + sig,
		"newline header": "ewoiYWxnIjoiUlMyNTYifQ." + payload + "." + sig,
		"json property":  `{"id_token":"` + jwt + `"}`,
		"url fragment":   "https://example.invalid/cb#access_token=" + jwt + "&state=x",
		"prose":          "the session jwt " + jwt + " expired",
		"after hyphen":   "x-" + jwt,
	} {
		redacted, hit := Screen(text)
		if !hit || strings.Contains(redacted, header) || strings.Contains(redacted, payload) || strings.Contains(redacted, sig) {
			t.Errorf("%s: Screen(%q) = (%q, %v), want the whole JWT redacted", name, text, redacted, hit)
		}
	}
	// A five-part JWE shape is detected; as before, only its first three
	// segments are redacted.
	if redacted, hit := Screen(jwt + "." + sig + "." + sig); !hit || strings.Contains(redacted, header) || strings.Contains(redacted, payload) {
		t.Errorf("five-part JWE shape: Screen = (%q, %v)", redacted, hit)
	}
	for name, text := range map[string]string{
		"adopter alias":     "CHECKOUT-REGRESSION.PAYMENT-FLOW.SAVED-CARD-3DS-CHALLENGE",
		"adopter alias row": "CHECKOUT-REGRESSION-PAYMENT.FLOW-SAVED-CARD-3DS.CHALLENGE-RETRY-PATH-V2",
		"lowercase dotted":  "abcdefghij.abcdefghij.abcdefghij",
		"embedded ey":       "monkey-business.abcdefghij.abcdefghij",
		"uppercased header": strings.ToUpper(header) + "." + payload + "." + sig,
		"module-like path":  "github_com.beamfall_corvint.internal_secretscreen",
		"two segments only": header + "." + payload,
		"short header":      "eyJhbGci." + payload + "." + sig,
	} {
		if MatchString(text) {
			t.Errorf("%s: writer screen matched %q", name, text)
		}
	}
	// The immutable schema-v1 reader keeps the original any-dotted-triple shape.
	if !MatchStoredV1String("CHECKOUT-REGRESSION.PAYMENT-FLOW.SAVED-CARD-3DS-CHALLENGE") {
		t.Fatal("StoredV1Pattern changed")
	}
}
