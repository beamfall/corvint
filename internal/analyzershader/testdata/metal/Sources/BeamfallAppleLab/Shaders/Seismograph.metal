#include "VisualShared.h"
// Seismograph — lite — Four recording pens (bass, mid, treble, flux) draw scrolling traces on ruled paper; onsets jolt the needles, beats leave tick-marks, and heavy flux bleeds the ink.
// GENERATED from beamfall-visual-shaders/visuals/seismograph.glsl — edit the portable
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

// Seismograph — Lite — four recording pens (bass/mid/treble/flux) drawing
// scrolling traces on a paper field; needle overshoot on onset, beat tick-marks
// scrolling past, ink-bleed halo on high flux. Calm-capable instrument look.
// param0 Scroll Speed ; param1 Pen Gain ; param2 Ink Bleed

static float laneEnergy(int i, constant VisualUniforms& u) {
    if (i == 0) return u.bass;
    if (i == 1) return u.mid;
    if (i == 2) return u.treble;
    return saturate(u.flux);
}

static float laneWiggle(float xs, float fi, constant VisualUniforms& u) {
    float n = vnoise(float2(xs * 21.0, fi * 13.7)) - 0.5;
    float w = sin(xs * 34.0 + fi * 3.1) * 0.42
            + sin(xs * 71.0 - fi * 1.7) * 0.20
            + n * 1.1;
    // recorded-history envelope so the paper shows past loud/quiet passages
    float env = 0.20 + 0.80 * vnoise(float2(xs * 2.6, fi * 7.7 + 40.0));
    return w * env;
}

static float3 visualBody(float2 uv, constant VisualUniforms& u, VISUAL_BODY_TEXARGS) {
    float scroll = mix(0.05, 0.28, u.param0);
    float gain = mix(0.5, 1.6, u.param1);
    float bleed = u.param2;
    float xs = uv.x + u.time * scroll;
    float penX = 0.84;

    // ---- paper field ----
    float3 paper = palRoleFog(0.78, u);
    float fiber = vnoise(uv * float2(210.0, 160.0)) * 0.25 + 0.75;
    float3 col = paper * 0.055 * fiber;
    // ruled feed lines
    float rule = aaLine(abs(fract(uv.y * 10.0 + 0.5) - 0.5) * 0.1, 0.0008);
    col += paper * rule * 0.045;
    // right margin: pen carriage bar
    col += paper * aaLine(uv.x - penX, 0.0016) * 0.12;

    // ---- beat tick-marks scrolling past (top strip) ----
    float strip = smoothstep(0.905, 0.915, uv.y) * (1.0 - smoothstep(0.955, 0.965, uv.y));
    for (int j = 0; j < 6; ++j) {
        float age = u.beatPhase + float(j);
        float tx = penX - age * scroll * 1.9 - 0.002;
        float tick = 1.0 - smoothstep(0.0012, 0.0042, abs(uv.x - tx));
        col += palRoleTreble(0.15 + float(j) * 0.04, u) * tick * strip * exp(-age * 0.45) * 0.9;
    }

    // ---- four recording pens ----
    for (int i = 0; i < 4; ++i) {
        float fi = float(i);
        float laneY = 0.775 - fi * 0.185;
        float e = laneEnergy(i, u);

        // Live audio changes most strongly near the pen, but the visible paper
        // feed should still carry the current waveform shape across the frame.
        float nearPen = smoothstep(0.10, penX, uv.x);
        float drive = saturate(e * 1.20 + u.amplitude * 0.35 + u.level * 0.20);
        float amp = 0.052 * gain * (0.36 + 1.45 * drive + 1.00 * drive * nearPen);
        float v = laneWiggle(xs, fi, u) * amp;

        // needle overshoot jitter right at the pen on onsets
        float penZone = exp(-pow((uv.x - penX) * 22.0, 2.0));
        v += u.onset * penZone * sin(u.time * 46.0 + fi * 2.1) * 0.020;

        float d = uv.y - (laneY + v);
        float line = aaLine(d, 0.0018 + 0.0012 * drive);
        float3 ink = palRoleTrace(0.08 + fi * 0.21, u);
        ink = accentize(ink, u.accent, 0.06);
        col += ink * line * (0.45 + 2.70 * drive);

        // ink-bleed halo when flux runs hot
        float halo = exp(-abs(d) * mix(220.0, 55.0, bleed * saturate(u.flux)));
        col += ink * halo * bleed * saturate(u.flux) * (0.22 + 0.28 * e);

        // pen head: small hot dot riding the live trace at penX
        float xp = penX + u.time * scroll;
        float vp = laneWiggle(xp, fi, u) * 0.052 * gain * (0.35 + 1.75 * e)
                 + u.onset * sin(u.time * 46.0 + fi * 2.1) * 0.020;
        float2 dp = uv - float2(penX, laneY + vp);
        dp.x *= u.resolution.x / max(u.resolution.y, 1.0);
        float d2 = dot(dp, dp);
        col += palRolePeak(0.08 + fi * 0.21, u) * (1.1e-5 / (d2 + 1.6e-5)) * (0.35 + 1.45 * e + 0.55 * u.bassImpact);

        // lane label stub: short baseline dash at the left edge
        float stub = aaLine(uv.y - laneY, 0.0012) * (1.0 - smoothstep(0.02, 0.05, uv.x));
        col += ink * stub * 0.5;
    }

    // soft vignette keeps the instrument framed
    float2 c = uv - 0.5;
    col *= 1.0 - dot(c, c) * 0.55;
    return col; // LINEAR HDR
}


fragment float4 frag_seismograph(VOut in [[stage_in]],
                          constant VisualUniforms& u [[buffer(0)]],
                          texture2d<float> uImage  [[texture(0)]],
                          texture2d<float> uPrev   [[texture(1)]],
                          sampler uSamp [[sampler(0)]]) {
    return float4(sanitizeColor(visualBody(in.uv, u, uImage, uPrev, uSamp)), 1.0);
}
