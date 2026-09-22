#include "VisualShared.h"
// Strata Ridge — standard — Unknown Pleasures, elevated: a stack of waveform traces recedes upward in perspective, near ridges occluding the old; bass lifts the live peaks, onsets race a shimmer along the front ridge.
// GENERATED from beamfall-visual-shaders/visuals/strata-ridge.glsl — edit the portable
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

// Strata Ridge — Standard — stacked ridgeline history (Unknown Pleasures, elevated).
// A stack of synthesized waveform traces recedes upward with perspective
// compression: the nearest ridge is live and HDR-bright, older ridges dim and
// desaturate, and each ridge occludes the ones behind it. Bass lifts the peaks;
// onsets send a white-hot shimmer racing along the front ridge.
// param0 = Ridge Count ; param1 = Peak Drama ; param2 = Afterglow

// One synthesized trace height for ridge `id` at horizontal position x01.
// Central envelope concentrates energy mid-frame like the album plate.
static float ridgeTrace(float2 xAndId, constant VisualUniforms& u) {
    float x = xAndId.x;
    float id = xAndId.y;
    float env = exp(-pow((x - 0.5) * 3.1, 2.0));
    float edge = smoothstep(0.0, 0.06, x) * smoothstep(1.0, 0.94, x);
    float bandA = bandAt(u, fract(x * 0.82 + id * 0.043));
    float bandB = bandAt(u, fract(x * 0.31 + 0.27 + id * 0.061));
    float w = sin(x * 11.0 + id * 1.93 + u.time * 0.55) * 0.42;
    w += sin(x * 23.0 - id * 3.11 - u.time * 0.9 + bandB * 2.4) * 0.26;
    w += sin(x * 53.0 + id * 7.7 + u.time * 2.1) * 0.09 * (0.3 + u.treble);
    float jag = (vnoise(float2(x * 14.0 + id * 13.7, id * 3.3)) - 0.5) * 0.30;
    float h = env * (0.55 + 0.9 * bandA) * (0.6 + w + jag) + edge * 0.02 * bandB;
    return max(h, 0.0) * edge;
}

static float3 visualBody(float2 uv, constant VisualUniforms& u, VISUAL_BODY_TEXARGS) {
    const int MAXR = 26;
    int ridges = int(mix(14.0, 26.0, u.param0) + 0.5);
    float drama = mix(0.55, 1.35, u.param1);

    float x = uv.x;
    float3 glow = float3(0.0);
    float maxCrest = -1.0; // occlusion silhouette from nearer ridges

    for (int i = 0; i < MAXR; i++) {
        if (i >= ridges) break;
        float fi = float(i);
        float t = fi / max(float(ridges - 1), 1.0); // 0 = front, 1 = oldest

        // Perspective compression: wide spacing up front, rows crushing
        // together as they recede toward the top of the frame.
        float baseline = 0.14 + 0.70 * (1.0 - pow(1.0 - t, 1.65));

        // Older traces flatten and calm down; bass lifts the near peaks.
        float amp = drama * mix(0.26, 0.055, t)
                  * (1.0 + 0.9 * u.bass * (1.0 - t) + 0.35 * u.bassImpact * (1.0 - t));
        float crest = baseline + ridgeTrace(float2(x, fi), u) * amp;

        // Occlusion: a nearer ridge's peak hides the trace lines behind it.
        float vis = smoothstep(0.0, 0.014, crest - maxCrest);
        maxCrest = max(maxCrest, crest + 0.002);

        float d = uv.y - crest;
        float w = mix(0.0032, 0.0011, t);
        float line = aaLine(d, w);
        float under = exp(-max(-d, 0.0) * mix(40.0, 160.0, t)) * step(d, 0.0);

        float hue = fract(0.08 + x * 0.22 + t * 0.30 + 0.10 * u.spectralCentroid);
        float3 c = palRoleTrace(hue, u);
        float grey = luma(c);
        c = mix(c, float3(grey), t * 0.72); // age desaturates
        c = accentize(c, u.accent, 0.06);

        float bright = mix(2.2, 0.10, pow(t, 0.42)); // front = HDR hero
        float liveBand = bandAt(u, fract(x * 0.82 + fi * 0.043));
        glow += c * line * vis * bright * (0.55 + 0.9 * liveBand);
        glow += c * under * vis * bright * 0.10; // faint glow skirt under each crest

        // Onset shimmer: a hot packet racing along the live front ridge.
        if (i == 0) {
            float sx = fract(u.beatPhase * 1.0);
            float packet = exp(-pow((x - sx) * 10.0, 2.0));
            float3 hotC = peakWhite(palRoleTreble(hue, u), 0.5 + 0.4 * u.onset);
            glow += hotC * line * packet * (u.onset * 3.2 + u.flux * 0.8);
            // steady hot core so the hero ridge always reads
            glow += peakWhite(c, 0.12) * line * 0.45 * (0.4 + 0.6 * u.level);
        }
    }

    // Whisper of paper grain in the void so black regions aren't dead.
    float dust = vnoise(uv * float2(180.0, 120.0) + u.time * 0.15);
    glow += palRoleFog(0.6, u) * dust * 0.006;

    float3 col = feedbackTrail(glow, uv,
                             clamp(mix(0.25, 0.72, u.param2), 0.0, 0.86),
                             0.0012 + 0.002 * u.bassImpact,
                             0.0, uImage, uPrev, uSamp);
    return col; // LINEAR HDR
}


fragment float4 frag_strata_ridge(VOut in [[stage_in]],
                          constant VisualUniforms& u [[buffer(0)]],
                          texture2d<float> uImage  [[texture(0)]],
                          texture2d<float> uPrev   [[texture(1)]],
                          sampler uSamp [[sampler(0)]]) {
    return float4(sanitizeColor(visualBody(in.uv, u, uImage, uPrev, uSamp)), 1.0);
}
