#include "VisualShared.h"
// Interference — standard — Bass, mid and treble each radiate circular wavefronts shaped by their own trace; where crests align the water burns HDR-hot with moiré shimmer, and every beat re-anchors the emitters with a soft shockwave.
// GENERATED from beamfall-visual-shaders/visuals/interference.glsl — edit the portable
// body and re-run tools/gen.mjs; do not hand-edit below this line.

#define IMG(x) uImage.sample(uSamp,(x))
#define PREV(x) uPrev.sample(uSamp,(x))
#define atan2_ atan2
static inline float mod(float a, float b) { return a - b * floor(a / b); }
static inline float2 mod(float2 a, float2 b) { return a - b * floor(a / b); }
static inline float2 mod(float2 a, float b) { return a - b * floor(a / b); }
static inline float3 mod(float3 a, float3 b) { return a - b * floor(a / b); }
static inline float3 mod(float3 a, float b) { return a - b * floor(a / b); }

#define VISUAL_BODY_TEXARGS texture2d<float> uImage, texture2d<float> uPrev, sampler uSamp
static float3 feedbackTrail(float3 col, float2 uv, float decay, float zoom, float rotAmt,
                            VISUAL_BODY_TEXARGS) {
    return feedbackTrail(col, uPrev, uSamp, uv, decay, zoom, rotAmt);
}

// Interference — Standard — three band-driven wavefront emitters.
// Bass, mid and treble each own a slowly drifting emitter radiating circular
// wavefronts whose radial profile is that band's synthesized trace. Where
// crests align, constructive interference glows HDR-hot with a moiré shimmer.
// Beats re-anchor the emitters and fire a soft shockwave from each.
// param0 = Ring Density ; param1 = Drift ; param2 = Heat

// Emitter position: beat-anchored (re-rolled each beat, eased in) + slow drift.
static float2 emitterPos(float fi, float driftAmt, constant VisualUniforms& u) {
    float bc = floor(u.beatCount);
    float2 aNow = (hash22(float2(fi * 7.31 + 11.0, bc + 23.0)) - 0.5) * float2(1.30, 0.95);
    float2 aPrev = (hash22(float2(fi * 7.31 + 11.0, bc - 1.0 + 23.0)) - 0.5) * float2(1.30, 0.95);
    float2 anchor = mix(aPrev, aNow, smoothstep(0.0, 0.30, u.beatPhase));
    float2 drift = float2(sin(u.time * 0.21 + fi * 2.09), cos(u.time * 0.165 + fi * 1.37));
    // fixed triangulated homes keep the three sources spread apart
    float2 home = float2(cos(fi * 2.0944 + 0.65), sin(fi * 2.0944 + 0.65)) * 0.48;
    return home + anchor * 0.45 * driftAmt + drift * 0.18 * driftAmt;
}

// Band trace as a radial ring profile: crest value in ~[-1, 1] at ring coord q.
static float traceProfile(float q, float fi, constant VisualUniforms& u) {
    float fCenter = fi * 0.36 + 0.08; // 0.08 bass / 0.44 mid / 0.80 treble
    float bandA = bandAt(u, fract(fCenter + q * 0.045));
    float w = sin(q * 6.2831853) * (0.55 + 0.85 * bandA);
    w += sin(q * 6.2831853 * 2.0 + fi * 2.1) * 0.28 * bandA;
    w += sin(q * 6.2831853 * 3.7 - u.time * 0.6 + fi) * 0.12;
    return w;
}

static float3 visualBody(float2 uv, constant VisualUniforms& u, VISUAL_BODY_TEXARGS) {
    float2 p = motionCentered(uv, u);

    float freq = mix(2.6, 7.5, u.param0);
    float driftAmt = mix(0.35, 1.0, u.param1);
    float heat = mix(1.6, 3.4, u.param2);

    float energy0 = 0.25 + 1.5 * u.bass;
    float energy1 = 0.25 + 1.5 * u.mid;
    float energy2 = 0.25 + 1.5 * u.treble;

    float sum = 0.0;      // signed interference field
    float crests = 0.0;   // sharp ring lines
    float3 col = float3(0.0);

    for (int i = 0; i < 3; i++) {
        float fi = float(i);
        float2 pos = emitterPos(fi, driftAmt, u);
        float r = length(p - pos) + 1e-4;
        float speedI = 0.55 + 0.25 * fi + 0.35 * u.level;
        float q = r * freq - u.time * speedI - fi * 0.7;

        float s = traceProfile(q, fi, u);
        float e = (i == 0) ? energy0 : ((i == 1) ? energy1 : energy2);
        float att = 1.0 / (1.0 + r * r * 2.6);
        sum += s * e * att;

        // Sharp crest ring where the trace tops out — per-band tinted.
        float crest = smoothstep(0.55, 0.95, s) * att * e;
        crests += crest;
        float3 tint = (i == 0) ? palRoleBass(0.10, u)
                  : ((i == 1) ? palRoleTrace(0.45 + 0.08 * u.spectralCentroid, u)
                              : palRoleTreble(0.75, u));
        col += tint * crest * 0.85;

        // Emitter core: a small breathing coal.
        float coreGlow = exp(-r * r * 90.0) * (0.6 + 1.8 * e * 0.4);
        col += tint * coreGlow * 1.4;

        // Beat shockwave: soft expanding ring from the fresh anchor.
        float swR = u.beatPhase * 1.25;
        float sw = aaLine(r - swR, 0.020 + 0.015 * u.beatPhase)
                 * exp(-u.beatPhase * 3.2) * (0.4 + 1.2 * u.bassImpact);
        col += palRolePeak(0.2 + fi * 0.1, u) * sw * 1.6;
    }

    // ---- constructive interference: hot zones where crests align ----
    float constructive = max(sum, 0.0);
    float hot = pow(constructive * 0.75, heat);

    // moiré shimmer inside the hot zones
    float shim = 0.78 + 0.22 * sin(sum * 9.0 + u.time * 2.6);

    float hue = fract(0.06 + constructive * 0.28 + 0.10 * u.spectralCentroid);
    float3 zoneCol = palVisual(hue, u);
    zoneCol = accentize(zoneCol, u.accent, 0.08);
    col += zoneCol * hot * shim * (0.8 + 0.7 * u.level);
    col += peakWhite(zoneCol, 0.6) * smoothstep(2.2, 3.6, hot) * u.onset * 1.5;

    // destructive zones sink into a deep cool wash, not flat black
    float destructive = max(-sum, 0.0);
    col += palRoleFog(0.62 + 0.08 * u.spectralCentroid, u)
         * destructive * 0.05 * (0.5 + 0.5 * u.level);

    float3 outc = feedbackTrail(col, uv,
                              min(u.trailDecay, 0.55),
                              0.0018,
                              0.0, uImage, uPrev, uSamp);
    return outc; // LINEAR HDR
}


fragment float4 frag_interference(VOut in [[stage_in]],
                          constant VisualUniforms& u [[buffer(0)]],
                          texture2d<float> uImage  [[texture(0)]],
                          texture2d<float> uPrev   [[texture(1)]],
                          sampler uSamp [[sampler(0)]]) {
    return float4(sanitizeColor(visualBody(in.uv, u, uImage, uPrev, uSamp)), 1.0);
}
