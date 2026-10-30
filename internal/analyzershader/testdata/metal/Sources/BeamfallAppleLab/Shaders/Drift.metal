#include "VisualShared.h"

// Drift — Lite / procedural liquid-glass caustics.
// Live FLACs often provide only a smooth fallback gradient as uImage, so Drift
// cannot depend on album-art detail. The image becomes a low-frequency tint; the
// subject is generated caustic linework over dark glass.
// param0 Defocus/Flow     -> caustic drift and warp
// param1 Lens Pulse       -> bass lens pulse
// param2 Edge Spectra     -> spectral glints and vignette depth

fragment float4 frag_drift(VOut in [[stage_in]],
                           constant VisualUniforms& u [[buffer(0)]],
                           texture2d<float> uImage [[texture(0)]],
                           texture2d<float> uPrev [[texture(1)]],
                           sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 p = motionCentered(uv, u);
    float ar = u.resolution.x / max(u.resolution.y, 1.0);

    float flowAmt = mix(0.04, 0.22, u.param0);
    float lensPulse = mix(0.00, 0.08, u.param1) * u.bassImpact;
    float spectra = mix(0.15, 1.0, u.param2);

    float2 flow = p;
    flow += 0.10 * float2(
        fbm(p * 1.35 + float2(u.time * flowAmt, -u.time * flowAmt * 0.35)),
        fbm(rot2(0.7) * p * 1.15 + float2(-u.time * flowAmt * 0.42, u.time * flowAmt))
    ) - 0.05;
    flow += normalize(p + 1e-5) * lensPulse * dot(p, p);

    // A smooth image is only tint. It never supplies full-frame brightness.
    float3 tint = uImage.sample(uSamp, uv * 0.22 + 0.39).rgb;
    float3 base = mix(float3(0.006, 0.012, 0.017), tint * 0.09, 0.26);

    float r = length(p);
    float centralShadow = 1.0 - 0.48 * exp(-r * r * 2.7);
    float vignette = 1.0 - mix(0.74, 0.92, u.param2) * smoothstep(0.34, 1.28, r);
    base *= centralShadow * max(vignette, 0.0) * (0.62 + 0.12 * u.level);

    float3 glow = float3(0.0);

    // Layered diagonal caustic arcs. Thin lines carry the light; the field stays dark.
    for (int i = 0; i < 9; i++) {
        float fi = float(i);
        float laneSeed = hash11(fi * 2.17);
        float phase = fi * 1.37 + u.time * (0.035 + 0.010 * hash11(fi * 4.3));
        float2 q = rot2(mix(-0.38, 0.42, hash11(fi * 8.1))) * flow;

        float curve = sin(q.x * (1.2 + fi * 0.13) + phase) * 0.18
                    + sin(q.x * (2.6 + fi * 0.07) - phase * 0.71) * 0.055;
        float lane = q.y - curve - mix(-0.72, 0.72, laneSeed);
        float width = mix(0.0025, 0.0080, hash11(fi * 5.31));
        float line = aaLine(lane, width);

        float window = smoothstep(-0.90, -0.42, q.y) * (1.0 - smoothstep(0.22, 0.78, q.y));
        window *= smoothstep(1.22, 0.18, abs(q.x));

        float energy = bandAt(u, fract(fi / 9.0 + 0.10 * u.beatPhase));
        float3 lineCol = palNight(fract(fi / 9.0 + 0.12 * u.spectralCentroid + 0.015 * u.time));
        lineCol = accentize(lineCol, u.accent, 0.12);
        glow += lineCol * line * window * (0.07 + energy * 0.72) * (0.52 + 0.20 * u.level);
    }

    // Thin lens rim, deliberately not a broad halo.
    float rimR = 0.34 + 0.035 * u.bassImpact;
    float rim = aaLine(r - rimR, 0.0035);
    glow += palNeon(0.55 + 0.12 * u.spectralCentroid) * rim * (0.32 + 0.70 * u.bassImpact);

    // Tiny onset/flux glints on the lens edge.
    for (int i = 0; i < 10; i++) {
        float fi = float(i);
        float a = fi / 10.0 * 6.2831853 + hash11(fi * 0.57) * 0.4;
        float rr = 0.30 + 0.12 * hash11(fi * 1.91 + floor(u.beatCount));
        float2 sp = float2(cos(a) * rr / max(ar, 1e-3), sin(a) * rr);
        float d = length(p - sp);
        float spark = min(1.6, 0.00022 / (d * d + 0.00045));
        glow += palHeatIce(fi / 10.0 + 0.1 * u.spectralCentroid) * spark * u.flux * spectra;
    }

    float3 trailedGlow = feedbackTrail(glow, uPrev, uSamp, uv,
                                       min(u.trailDecay, 0.38),
                                       0.0015 + 0.003 * u.bassImpact,
                                       0.0015 * sin(u.time * 0.2));
    return float4(base + trailedGlow, 1.0);
}
