#include "VisualShared.h"
// Oscillo Ring — standard — The waveform bent into a closed ring: a live radial trace breathes on bass, echo rings detach and decay outward on every beat, and treble sparks orbit the rim.
// GENERATED from beamfall-visual-shaders/visuals/oscillo-ring.glsl — edit the portable
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

// Oscillo Ring — Standard — the waveform trace wrapped into a closed ring:
// band-synthesized radial trace, beat-timed echo rings detaching outward,
// treble sparks orbiting the circumference, bassImpact breathing the whole ring.
// param0 Trace Amp ; param1 Echo Reach ; param2 Spark Density

static float ringTrace(float ang, float a01, float t, constant VisualUniforms& u) {
    float bA = bandAtWrapped(u, a01 + t * 0.021);
    float bB = bandAtWrapped(u, a01 * 0.5 + 0.31 - t * 0.013);
    float w = sin(ang * 5.0 + t * 1.25) * 0.40
            + sin(ang * 9.0 - t * 2.05 + bB * 2.2) * 0.26
            + sin(ang * 23.0 + t * 5.1) * 0.12 * (0.4 + u.treble)
            + sin(ang * 41.0 - t * 3.4) * 0.07 * (0.3 + u.mid);
    return w * (0.35 + 1.0 * bA);
}

static float3 visualBody(float2 uv, constant VisualUniforms& u, VISUAL_BODY_TEXARGS) {
    float2 p = motionCentered(uv, u);
    float r = length(p);
    float ang = atan2_(p.y, p.x);
    float a01 = ang / 6.2831853 + 0.5;

    float ampScale = mix(0.05, 0.17, u.param0) * (0.5 + 0.9 * u.amplitude);
    float reach = mix(0.28, 0.75, u.param1);

    // whole ring breathes on bass
    float breathe = 1.0 + 0.075 * u.bassImpact + 0.015 * sin(beatRadians(u, 0.0));
    float r0 = 0.50 * breathe;

    float3 col = float3(0.0);

    // ---- background: dark field + faint spectral haze behind the ring ----
    float hazeBand = bandAtWrapped(u, a01 * 2.0);
    float haze = exp(-abs(r - r0) * 7.0) * (0.018 + 0.060 * hazeBand * u.amplitude);
    col += palRoleFog(0.52 + 0.08 * u.spectralCentroid, u) * haze;
    // inner core glow, dim and cool
    col += palRoleFog(0.18, u) * exp(-r * 4.0) * (0.03 + 0.06 * u.level);

    // ---- live master ring ----
    float trace = ringTrace(ang, a01, u.time, u) * ampScale;
    float d = r - (r0 + trace);
    float line = aaLine(d, 0.0036);
    float halo = exp(-abs(d) * 42.0) * 0.20;
    float3 tc = palRoleTrace(fract(a01 + 0.10 * u.spectralCentroid), u);
    tc = accentize(tc, u.accent, 0.10);
    col += tc * (line * (1.9 + 2.6 * bandAtWrapped(u, a01)) + halo * (0.5 + u.amplitude)) * 1.35;

    // beat pulse: a white-hot arc sweeps around the master ring
    float sweep = exp(-pow(sin((a01 - u.beatPhase) * 3.14159265), 2.0) * 160.0);
    col += palRolePeak(0.12, u) * sweep * aaLine(d, 0.012) * u.bassImpact * 2.4;

    // ---- echo rings: detach on beats, decay outward ----
    for (int j = 0; j < 4; ++j) {
        float fj = float(j);
        float age = u.beatPhase + fj;
        float re = r0 + age * 0.24 * reach;
        float shapeAmp = ampScale * exp(-age * 0.55);
        float eTrace = ringTrace(ang, a01, u.time - age * 0.55, u) * shapeAmp;
        float de = r - (re + eTrace);
        float fade = exp(-age * 1.5) * (0.40 + 0.60 * u.level);
        float eLine = aaLine(de, 0.0026 + 0.0028 * age);
        // echoes keep the trace's angular coloring so they read as detached copies
        float3 ec = palRoleTrace(fract(a01 + 0.07 * (u.beatCount - fj)), u);
        col += ec * (eLine * 1.3 + exp(-abs(de) * 60.0) * 0.15) * fade;
    }

    // ---- treble sparks orbiting the circumference ----
    int nSpark = int(mix(10.0, 30.0, u.param2) + 0.5);
    for (int i = 0; i < 30; ++i) {
        if (i >= nSpark) break;
        float fi = float(i);
        float2 h = hash22(float2(fi * 3.13, 17.7));
        float dir = h.x > 0.5 ? 1.0 : -1.0;
        float sa = h.x * 6.2831853 + u.time * (0.35 + 0.9 * h.y) * dir;
        float sa01 = sa / 6.2831853;
        float sr = r0 + ringTrace(sa, fract(sa01 + 0.5), u.time, u) * ampScale
                 + 0.020 * sin(u.time * 3.0 + fi);
        float2 sp = float2(cos(sa), sin(sa)) * sr;
        float2 dv = p - sp;
        float d2 = dot(dv, dv);
        float tw = 0.6 + 0.4 * sin(u.time * 11.0 + fi * 2.3);
        col += palRoleTreble(fract(fi * 0.061), u)
             * (2.6e-5 / (d2 + 3.5e-5)) * (0.25 + 1.9 * u.treble) * tw;
    }

    // outer falloff keeps the frame composed
    col *= 1.0 - smoothstep(1.05, 1.55, r) * 0.85;

    col = feedbackTrail(col, uv, min(u.trailDecay, 0.80),
                        0.0035 + 0.005 * u.bassImpact, 0.0012, uImage, uPrev, uSamp);
    return col; // LINEAR HDR
}


fragment float4 frag_oscillo_ring(VOut in [[stage_in]],
                          constant VisualUniforms& u [[buffer(0)]],
                          texture2d<float> uImage  [[texture(0)]],
                          texture2d<float> uPrev   [[texture(1)]],
                          sampler uSamp [[sampler(0)]]) {
    return float4(sanitizeColor(visualBody(in.uv, u, uImage, uPrev, uSamp)), 1.0);
}
