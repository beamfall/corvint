#include "VisualShared.h"

// Aurora — Flagship / sparse knife-edge curtains.
// Black sky, separated spectral ribbons, hard dark gaps. No broad stars, rings,
// shimmer, or horizon fog in the default render.
// param0 Curtain Count  -> 3..5
// param1 Sway           -> centerline motion
// param2 Vein Detail    -> ribbon narrowness + crest sharpness

fragment float4 frag_aurora(VOut in [[stage_in]],
                            constant VisualUniforms& u [[buffer(0)]],
                            texture2d<float> uImage [[texture(0)]],
                            texture2d<float> uPrev [[texture(1)]],
                            sampler uSamp [[sampler(0)]]) {
    float2 p = motionCentered(in.uv, u);

    int nCurtains = int(mix(3.0, 5.0, u.param0) + 0.5);
    float swayAmt = mix(0.025, 0.12, u.param1);
    float detail = mix(0.0, 1.0, u.param2);

    float horizonY = 0.58 - u.bassImpact * 0.035;
    float3 body = float3(0.0);
    float3 glow = float3(0.0);

    for (int ci = 0; ci < 5; ci++) {
        if (ci >= nCurtains) break;

        float fi = float(ci);
        float frac = (fi + 0.5) / float(nCurtains);
        float x0 = mix(-0.96, 0.96, frac);
        float lowBand = bandAt(u, frac * 0.42);
        float highBand = bandAt(u, 0.28 + frac * 0.52);

        float centerline = x0
            + sin(u.time * 0.18 + fi * 1.7 + p.y * 1.9) * swayAmt
            + (fbm(float2(p.y * 1.15 + fi, u.time * 0.055 + fi * 2.1)) - 0.5) * swayAmt * 0.9;
        float dx = p.x - centerline;
        float width = mix(0.052, 0.019, detail) * (0.85 + 0.25 * hash11(fi * 3.7));
        float ribbon = exp(-dx * dx / max(width * width, 1e-5));

        float bottom = horizonY - 0.02 - lowBand * 0.05;
        float top = mix(-0.58, -0.08, highBand) - u.bassImpact * 0.05;
        float ywin = smoothstep(top - 0.16, top + 0.04, p.y)
                   * (1.0 - smoothstep(bottom, bottom + 0.06, p.y));

        // Ridged crests create internal black gaps instead of fog sheets.
        float ridge = fbmRidged(float2(dx * mix(22.0, 42.0, detail) + fi * 4.0,
                                       p.y * mix(2.0, 4.3, detail) - u.time * 0.13));
        float sheet = ribbon * ywin * smoothstep(0.54, 0.84, ridge);
        float crest = ribbon * ywin * smoothstep(0.78, 0.95, ridge);

        float hue = frac + 0.12 * u.spectralCentroid + 0.018 * u.time;
        float3 c = palVisual(hue, u);
        c = accentize(c, u.accent, 0.10);

        body += c * sheet * (0.44 + 0.52 * highBand);
        glow += peakWhite(c, crest * u.treble * 0.18) * crest * (1.05 + 3.2 * u.treble);

        // Beat pulse: a short segment traveling up the ribbon, never a full-frame ring.
        float pulseY = mix(bottom, top, fract(u.beatPhase + fi * 0.19));
        float pulse = exp(-pow(p.y - pulseY, 2.0) * 260.0) * ribbon * ywin;
        pulse *= smoothstep(0.18, 0.72, ribbon);
        glow += mix(float3(1.65, 0.30, 0.03), c, 0.40) * pulse * u.bassImpact * 1.25;

        // Tiny crest sparks from flux.
        float sparkleGate = step(0.82, hash11(fi * 7.1 + floor(u.beatCount)));
        glow += palVisual(hue + 0.35, u) * crest * sparkleGate * u.flux * 1.1;
    }

    // A dim horizon reference only where curtains emerge; not a full white band.
    float horizonLine = aaLine(p.y - horizonY, 0.0015);
    glow += palVisual(0.78 + 0.12 * u.spectralCentroid, u) * horizonLine * (0.05 + 0.13 * u.bassImpact)
          * smoothstep(1.05, 0.25, abs(p.x));

    float3 trailedGlow = feedbackTrail(glow, uPrev, uSamp, in.uv,
                                       min(u.trailDecay, 0.58),
                                       0.0015,
                                       0.0005);
    return float4(body + trailedGlow, 1.0);
}
