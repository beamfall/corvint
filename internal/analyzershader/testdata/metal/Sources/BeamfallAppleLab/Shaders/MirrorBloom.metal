#include "VisualShared.h"
// Mirror Bloom — standard — A living Rorschach: band traces folded around the mirror axis shape an ink-blot that breathes with amplitude, feathers its edges with treble, and throws symmetric splatter on flux.
// GENERATED from beamfall-visual-shaders/visuals/mirror-bloom.glsl — edit the portable
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

// Mirror Bloom — Standard — a vertically mirrored Rorschach ink-blot whose
// silhouette is built from layered synthesized band traces folded around the
// center axis. Mass breathes with amplitude, edges feather with treble, flux
// throws symmetric ink splatter, and the palette drifts slowly with the
// spectral centroid. Alive and slightly uncanny.
// param0 Blot Mass ; param1 Feather ; param2 Splatter

static float3 visualBody(float2 uv, constant VisualUniforms& u, VISUAL_BODY_TEXARGS) {
    float2 p = motionCentered(uv, u);
    float2 m = float2(abs(p.x), p.y);              // fold across the vertical axis
    float r = length(m) + 1e-5;
    float a = atan2_(m.y, m.x + 1e-4);         // -pi..pi over the right half

    float massAmt = mix(0.75, 1.35, u.param0);
    float featherAmt = mix(0.010, 0.10, u.param1);
    float splatAmt = mix(0.3, 1.8, u.param2);

    // slow timbral palette drift
    float drift = 0.16 * u.spectralCentroid + 0.05 * sin(u.time * 0.043);

    // ---- silhouette: layered synthesized traces around the fold ----
    float breath = 0.40 * massAmt * (0.70 + 0.45 * u.amplitude + 0.14 * u.bassImpact);
    float R = breath;
    for (int i = 0; i < 5; ++i) {
        float fi = float(i);
        float b = bandAt(u, fract(a * 0.159155 + 0.5 + fi * 0.19));
        R += sin(a * (2.0 + fi) + u.time * (0.17 + 0.11 * fi) + fi * 2.13)
           * (0.030 + 0.115 * b) * breath;
    }
    // Rorschach asymmetry: top and bottom lobes differ (only x is mirrored)
    R *= 1.0 + 0.30 * sin(a * 1.0 - 0.7) + 0.12 * sin(a * 3.0 + u.time * 0.06);
    // organic ink-bleed wobble on the rim
    R += (fbm(m * 3.2 + float2(0.0, u.time * 0.05)) - 0.5) * 0.24 * breath;

    float feather = featherAmt * (0.35 + 1.5 * u.treble) + fwidth(r);
    float mass = 1.0 - smoothstep(R - feather, R + feather, r);

    // ---- interior: pooled ink with mottled depth ----
    float pool = fbm(m * 4.5 - float2(0.0, u.time * 0.03));
    float veins = fbmRidged(m * 6.5 + float2(0.0, u.time * 0.02));
    float depth = saturate(1.0 - r / max(R, 1e-3));          // 1 at core, 0 at rim
    float3 inkDeep = palVisual(drift + 0.06 + veins * 0.10, u) * (0.16 + 0.30 * veins);
    float3 inkBody = palVisual(drift + 0.24 + pool * 0.18, u) * (0.30 + 0.60 * pool);
    float3 ink = mix(inkBody, inkDeep, depth * 0.62);
    ink *= 0.55 + 1.35 * veins * veins;                      // marbled vein contrast
    ink *= (0.85 + 0.55 * u.amplitude);

    // rim glow: the wet edge catches light, feathered by treble
    float rim = exp(-pow((r - R) / max(feather * 1.6, 1e-3), 2.0));
    float3 rimCol = palRoleRim(drift + 0.55, u);
    float3 col = ink * mass + rimCol * rim * (0.25 + 1.0 * u.treble) * mass
             + rimCol * rim * 0.14;                          // faint outer wet halo

    // heartbeat: interior brightens softly on the beat pulse
    col += palVisual(drift + 0.38, u) * mass * depth * phasePulse(u, 0.0, 0.20) * 0.35 * u.beat;

    // ---- symmetric splatter droplets thrown by flux ----
    float seed = floor(u.beatCount) * 2.0 + step(0.5, u.beatPhase);
    for (int i = 0; i < 12; ++i) {
        float fi = float(i);
        float2 h = hash22(float2(seed * 0.317 + fi * 11.7, fi * 3.71 + 1.3));
        float da = mix(-2.6, 2.6, h.x);                       // droplet direction
        float dr = breath * (1.12 + 0.85 * h.y);              // beyond the rim
        float2 c = float2(cos(da), sin(da)) * dr;
        c.x = abs(c.x);                                       // stay on fold side
        float2 dd = m - c;
        float size = 0.00004 + 0.00040 * h.y * h.x;
        float drop = exp(-dot(dd, dd) / (size + 1e-6));
        // trailing tail back toward the blot
        float2 tdir = -c / max(length(c), 1e-3);
        float2 tp = m - (c + tdir * 0.03);
        float tail = exp(-dot(tp, tp) / (size * 3.0 + 1e-6)) * 0.3;
        float3 dc = palVisual(drift + 0.30 + h.y * 0.12, u);
        col += dc * (drop + tail) * saturate(u.flux * 1.6) * splatAmt;
    }

    // dark water backdrop: near-black with the faintest fog so it never flattens
    float bg = fbm(p * 1.6 + u.time * 0.014) * 0.030;
    col += palRoleFog(drift + 0.7, u) * bg * (1.0 - mass) * (0.4 + 0.6 * u.amplitude);

    return col;
}


fragment float4 frag_mirror_bloom(VOut in [[stage_in]],
                          constant VisualUniforms& u [[buffer(0)]],
                          texture2d<float> uImage  [[texture(0)]],
                          texture2d<float> uPrev   [[texture(1)]],
                          sampler uSamp [[sampler(0)]]) {
    return float4(sanitizeColor(visualBody(in.uv, u, uImage, uPrev, uSamp)), 1.0);
}
