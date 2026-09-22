package com.beamfall.kit.test.entitlement;

import java.nio.charset.StandardCharsets;
import java.security.KeyFactory;
import java.security.PrivateKey;
import java.security.Signature;
import java.security.spec.PKCS8EncodedKeySpec;
import java.util.Base64;

public final class TestModeEntitlementMint {
    public static final String TEST_KEY_CANARY = "beamfall-test-entitlement-mint-private-key";

    private static final String KID = "test-entitlements-2026-07";
    private static final String PRIVATE_KEY_PKCS8_BASE64 =
        "MC4CAQAwBQYDK2VwBCIEIP8I8lrKUSqfH6m3bjZ89bBtrlNDlCxBI0lkq99g3ec1";
    private static final String PUBLIC_KEY_RAW_BASE64URL =
        "37YmsmnAw17ya4tO3EMisG1EvHZtiWud6TR9mpJg608";

    public String keysetJson() {
        return "{\"version\":\"1\",\"active_kid\":\"" + KID + "\",\"keys\":{\"" + KID + "\":\"" +
            PUBLIC_KEY_RAW_BASE64URL + "\"}}";
    }

    public String mintActive(String entitlementId) {
        return mint(entitlementId, "subscription", "2026-07-01T00:00:00Z", "2026-07-16T00:00:00Z");
    }

    public String mintExpired(String entitlementId) {
        return mint(entitlementId, "subscription", "2026-01-01T00:00:00Z", "2026-01-16T00:00:00Z");
    }

    public String mintLifetime(String entitlementId) {
        return mint(entitlementId, "lifetime", "2026-07-01T00:00:00Z", null);
    }

    public String mint(String entitlementId, String shape, String issuedAt, String expiresAt) {
        try {
            String expiryClaim = expiresAt == null ? "" : "\"expires_at\":" + json(expiresAt) + ",";
            String headerJson = "{\"alg\":\"EdDSA\",\"kid\":\"" + KID + "\",\"typ\":\"JWT\"}";
            String payloadJson = "{" +
                "\"account_id\":\"test-account\"," +
                "\"app_family\":\"android\"," +
                "\"channel_summary\":\"TestMode\"," +
                "\"class\":\"premium\"," +
                "\"entitlement_id\":" + json(entitlementId) + "," +
                expiryClaim +
                "\"issued_at\":" + json(issuedAt) + "," +
                "\"kid\":\"" + KID + "\"," +
                "\"shape\":" + json(shape) + "," +
                "\"typ\":\"beamfall.entitlement.v1\"" +
                "}";
            String header = base64url(headerJson.getBytes(StandardCharsets.UTF_8));
            String payload = base64url(payloadJson.getBytes(StandardCharsets.UTF_8));
            String signingInput = header + "." + payload;

            Signature sig = Signature.getInstance("Ed25519");
            sig.initSign(privateKey());
            sig.update(signingInput.getBytes(StandardCharsets.US_ASCII));
            return signingInput + "." + base64url(sig.sign());
        } catch (Exception e) {
            throw new IllegalStateException("failed to mint test entitlement token", e);
        }
    }

    private static PrivateKey privateKey() throws Exception {
        byte[] bytes = Base64.getDecoder().decode(PRIVATE_KEY_PKCS8_BASE64);
        return KeyFactory.getInstance("Ed25519").generatePrivate(new PKCS8EncodedKeySpec(bytes));
    }

    private static String base64url(byte[] bytes) {
        return Base64.getUrlEncoder().withoutPadding().encodeToString(bytes);
    }

    private static String json(String value) {
        return "\"" + value.replace("\\", "\\\\").replace("\"", "\\\"") + "\"";
    }
}
