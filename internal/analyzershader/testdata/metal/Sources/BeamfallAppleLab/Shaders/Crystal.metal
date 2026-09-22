#include "VisualShared.h"

// Crystal — Flagship / frequency + beat + onset.
// A faceted prism refracting a procedural neon event-horizon environment.
// Fractures into spectrum shards on transients. Deep black background.
// param0 Facet Count    → 2..12 cutting planes per ring
// param1 Dispersion     → IOR chromatic spread (0=none, 1=max)
// param2 Fracture       → crack-flash intensity on onset/beat
//
// HDR out — no tonemap. Facet glints / cracks 5-8x; body 1-3x.

// ---------------------------------------------------------------------------
// SDF: faceted crystal (PRESERVED from original)
// ---------------------------------------------------------------------------
static inline float sdOctahedron(float3 p, float s) {
    float3 q = abs(p);
    return (q.x + q.y + q.z - s) * 0.57735027f;   // 1/sqrt(3)
}

static inline float sdPlane(float3 p, float3 n, float d) {
    return dot(p, n) - d;
}

static float crystalSDF(float3 p, float facetSharp, float time) {
    float d = sdOctahedron(p, 0.80f);
    float sphere = length(p) - 0.90f;
    d = max(d, sphere);

    int N = int(clamp(facetSharp, 2.0f, 12.0f));
    float angleStep = 6.28318530f / float(N);

    for (int i = 0; i < N; ++i) {
        float a = float(i) * angleStep + time * 0.07f;
        float3 n1 = normalize(float3(cos(a), 0.55f, sin(a)));
        d = max(d, sdPlane(p, n1, 0.62f));
    }
    for (int i = 0; i < N; ++i) {
        float a = float(i) * angleStep + angleStep * 0.5f + time * 0.07f;
        float3 n2 = normalize(float3(cos(a), -0.55f, sin(a)));
        d = max(d, sdPlane(p, n2, 0.62f));
    }
    return d;
}

// ---------------------------------------------------------------------------
// Normal — tetrahedron method (PRESERVED from original)
// ---------------------------------------------------------------------------
static float3 crystalNormal(float3 p, float facetSharp, float time) {
    const float2 e = float2(0.0015f, -0.0015f);
    float3 n =
        e.xyy * crystalSDF(p + e.xyy, facetSharp, time) +
        e.yyx * crystalSDF(p + e.yyx, facetSharp, time) +
        e.yxy * crystalSDF(p + e.yxy, facetSharp, time) +
        e.xxx * crystalSDF(p + e.xxx, facetSharp, time);
    float len = length(n);
    return len > 1e-6f ? n / len : float3(0.0f, 1.0f, 0.0f);
}

// ---------------------------------------------------------------------------
// Procedural neon environment — polar spectral rings.
// No uImage dependency: gem looks premium even with no artwork loaded.
// ---------------------------------------------------------------------------
static float3 procEnv(float3 dir, float time, float spectralCentroid) {
    // Spherical coords of the direction
    float phi   = atan2(dir.z, dir.x);          // -pi..pi azimuth
    float theta = asin(clamp(dir.y, -1.0f, 1.0f)); // -pi/2..pi/2 elevation
    float u = phi / 6.28318530f + 0.5f;         // 0..1
    float v = theta / 3.14159265f + 0.5f;       // 0..1

    float3 env = float3(0.0f);

    // Several neon polar bands (ring layers at discrete elevation bands)
    for (int b = 0; b < 6; ++b) {
        float fb = float(b);
        float bandCenter = 0.15f + fb * 0.12f;
        float bandV      = abs(v - bandCenter);
        float ringMask   = 1.0f - smoothstep(0.008f, 0.032f, bandV);

        // Hue rotates around azimuth; animated slowly
        float t  = u + fb * 0.17f + time * (0.02f + fb * 0.007f);
        float3 c = palCity(t + 0.12f * spectralCentroid);

        // Brightness varies by azimuth band for a spectrum effect
        float azBand = fract(u * 8.0f + fb * 0.3f);
        float bright = 0.6f + 0.6f * sin(azBand * 6.28318530f);

        env += c * ringMask * bright * 1.8f;
    }

    // Dark baseline — the gem floats in black
    return env;
}

// ---------------------------------------------------------------------------
// Chromatic dispersion refraction (3 separate IOR channels → prism effect)
// ---------------------------------------------------------------------------
static float3 envRefractChromatic(float3 rd, float3 n, float iorBase, float dispersion,
                                   float time, float spectralCentroid) {
    float dsp = dispersion * 0.12f;
    float iorR = iorBase - dsp;
    float iorG = iorBase;
    float iorB = iorBase + dsp;

    float3 rr = refract(rd, n, 1.0f / iorR);
    float3 rg = refract(rd, n, 1.0f / iorG);
    float3 rb = refract(rd, n, 1.0f / iorB);

    // Total internal reflection fallback
    if (length(rr) < 0.01f) rr = reflect(rd, n);
    if (length(rg) < 0.01f) rg = reflect(rd, n);
    if (length(rb) < 0.01f) rb = reflect(rd, n);

    return float3(
        procEnv(rr, time, spectralCentroid).r,
        procEnv(rg, time, spectralCentroid).g,
        procEnv(rb, time, spectralCentroid).b
    );
}

// ---------------------------------------------------------------------------
// Crack-flash pattern: high-freq sinusoidal thresholded to thin lines.
// Shard count driven by treble / onset — more treble = more fracture lines.
// ---------------------------------------------------------------------------
static float crackPattern(float3 p, float shardFreq) {
    float v = 0.0f;
    v += abs(sin(p.x * 14.3f + p.y * 9.7f));
    v += abs(sin(p.y * 11.1f + p.z * 17.3f));
    v += abs(sin(p.z * 13.7f + p.x * 8.1f));
    // Second harmonic for denser shards at high treble
    v += 0.4f * abs(sin(p.x * shardFreq + p.z * 21.1f));
    v /= 3.4f;
    return 1.0f - smoothstep(0.0f, 0.20f, v);
}

// ---------------------------------------------------------------------------
// Fragment shader
// ---------------------------------------------------------------------------
fragment float4 frag_crystal(VOut in [[stage_in]],
                              constant VisualUniforms& u [[buffer(0)]],
                              texture2d<float> uImage [[texture(0)]],
                              texture2d<float> uPrev  [[texture(1)]],
                              sampler uSamp [[sampler(0)]]) {

    // ------------------------------------------------------------------
    // Params
    // ------------------------------------------------------------------
    float facetSharp  = mix(2.0f, 12.0f, u.param0);          // cutting planes
    float dispersion  = mix(0.05f, 1.0f, u.param1);          // IOR spread
    float fracturePow = u.param2;                              // crack intensity

    // Bass drives gem scale and camera push
    float gemScale    = 1.0f + 0.18f * u.bassImpact;
    float camPush     = 0.22f * u.bassImpact;

    // Mid drives internal wobble on the SDF cutplanes (via time offset)
    float wobbleTime  = u.time + u.mid * 0.8f;

    // Treble + onset -> fracture brightness and shard count
    float shardFreq   = mix(14.0f, 34.0f, u.treble);         // more treble = denser cracks
    float transient   = saturate(u.onset * 3.0f + u.beat * 0.5f);
    float fractureAmt = fracturePow * transient;

    // beatPhase -> slow base rotation on top of the normal yaw
    float beatRot     = u.beatPhase * 0.4f;

    // spectralCentroid -> tint hue bias
    float tintT       = u.spectralCentroid;

    // ------------------------------------------------------------------
    // Ray setup — PRESERVED orbiting camera
    // ------------------------------------------------------------------
    float2 uv = motionCentered(in.uv, u);
    float3 ro = float3(0.0f, 0.0f, 3.2f - camPush);
    float3 rd = normalize(float3(uv, -1.6f));

    float yaw   = u.time * 0.18f + u.bass * 0.4f + beatRot;
    float pitch = u.time * 0.09f + u.mid  * 0.2f;
    float2x2 Ry = rot2(yaw);
    float2x2 Rx = rot2(pitch);
    ro.xz = Ry * ro.xz;
    ro.yz = Rx * ro.yz;
    rd.xz = Ry * rd.xz;
    rd.yz = Rx * rd.yz;

    // ------------------------------------------------------------------
    // Raymarch (PRESERVED, scale gem with bassImpact)
    // ------------------------------------------------------------------
    float t   = 0.0f;
    float hit = 0.0f;
    const float TMAX = 12.0f;
    const float EPS  = 0.001f;

    for (int i = 0; i < 72; ++i) {
        float3 p = ro + rd * t;
        // Scale the SDF query point inversely -> gem grows on bassImpact
        float d = crystalSDF(p / gemScale, facetSharp, wobbleTime) * gemScale;
        if (d < EPS) { hit = 1.0f; break; }
        t += d * 0.85f;
        if (t > TMAX) break;
    }

    // ------------------------------------------------------------------
    // Background — deep black with faint accent vignette
    // ------------------------------------------------------------------
    float vig = 1.0f - smoothstep(0.5f, 1.8f, length(uv));
    float3 col = float3(0.0f) + u.accent.rgb * 0.012f * vig;

    // ------------------------------------------------------------------
    // Gem shading
    // ------------------------------------------------------------------
    if (hit > 0.5f) {
        float3 p = ro + rd * t;
        float3 pLocal = p / gemScale;
        float3 n = crystalNormal(pLocal, facetSharp, wobbleTime);

        // Fresnel (Schlick)
        float cosI = max(-dot(rd, n), 0.0f);
        float F0   = 0.05f;   // crystal base reflectance
        float fres = F0 + (1.0f - F0) * pow(1.0f - cosI, 5.0f);

        // --- Reflection: procedural env ---
        float3 refl     = reflect(rd, n);
        float3 reflCol  = procEnv(refl, u.time, tintT);

        // --- Refraction: chromatic dispersion through procedural env ---
        float ior       = 1.52f;   // crystal-ish
        float3 refrCol  = envRefractChromatic(rd, n, ior, dispersion, u.time, tintT);

        // --- Mix refract/reflect by Fresnel (grazing -> more reflect) ---
        // Body range: 1-3x per spec (reflCol and refrCol are already ~1-2x from procEnv)
        float3 gemCol = mix(refrCol, reflCol, fres);

        // spectralCentroid tint: bias hue toward neon palette
        float3 tintCol = palVisual(tintT + 0.3f, u);
        gemCol = mix(gemCol, gemCol * (0.7f + 0.5f * tintCol), 0.18f);

        // Internal bass/amplitude glow — warm body emission, keep below 3x
        float glow = u.bassImpact * 0.8f + u.amplitude * 0.3f;
        glow *= (1.0f - fres);   // strongest on the interior
        float3 glowCol = palVisual(0.1f + tintT * 0.3f, u);
        gemCol += glowCol * glow * 1.4f;

        // Specular glints on facet edges: 5-8x for bloom to catch
        float spec = pow(max(dot(refl, normalize(float3(0.4f, 0.8f, 0.5f))), 0.0f), 64.0f);
        spec += 0.5f * pow(max(dot(refl, normalize(float3(-0.6f, 0.5f, 0.3f))), 0.0f), 48.0f);
        gemCol += float3(spec) * (5.0f + 3.0f * u.treble);

        // Rim/edge brightening from Fresnel: facet outlines pop
        float rim = pow(1.0f - cosI, 3.5f);
        float3 rimCol = palVisual(tintT + 0.5f + u.beatPhase * 0.2f, u);
        gemCol += rimCol * rim * (1.5f + u.bassImpact * 2.0f);

        // Beat brightening (overall pulse, kept moderate — bloom does the heavy lift)
        gemCol += u.accent.rgb * u.beat * 0.4f;

        // --- Crack-flash on transients ---
        if (fractureAmt > 0.001f) {
            float crack = crackPattern(pLocal * (4.0f + u.param0 * 6.0f), shardFreq);
            // Crack lines: spectrum-colored shards, 5-8x HDR
            float3 crackCol = palVisual(tintT + crack * 0.4f + u.onset * 0.2f, u);
            crackCol = peakWhite(crackCol, 0.22f * u.onset);
            gemCol += crackCol * crack * fractureAmt * 7.0f;
        }

        col = gemCol;
    }

    // ------------------------------------------------------------------
    // Feedback trails — light decay 0.72ish per spec
    // ------------------------------------------------------------------
    col = feedbackTrail(col, uPrev, uSamp, in.uv,
                        u.trailDecay,                              // set to ~0.72 in registry
                        0.002f + 0.006f * u.bassImpact,           // subtle zoom
                        0.001f * sin(u.time * 0.5f));             // very slow swirl

    return float4(col, 1.0f);
}
