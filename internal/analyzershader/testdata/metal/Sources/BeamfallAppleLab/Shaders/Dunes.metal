#include "VisualShared.h"

// Dunes — topographic neon wire landscape.
// Perspective screen-space contours, not a raymarched terrain.
// param0 Speed, param1 Height, param2 Grid Density

fragment float4 frag_dunes(VOut in [[stage_in]],
                           constant VisualUniforms& u [[buffer(0)]],
                           texture2d<float> uImage [[texture(0)]],
                           texture2d<float> uPrev [[texture(1)]],
                           sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float horizon = 0.42;
    float3 glow = float3(0.0);

    if (uv.y > horizon) {
        float ft = (uv.y - horizon) / max(1.0 - horizon, 0.001);
        float depth = 1.0 / max(ft * 2.2 + 0.035, 0.035);
        float speed = mix(0.16, 0.75, u.param0);
        float height = mix(0.10, 0.42, u.param1);
        float density = mix(0.50, 1.45, u.param2);
        float phase = beatRadians(u, ft * 0.22);
        float phaseLift = beatWave(u, ft * 0.18);

        float wx = (uv.x - 0.5) * depth * 1.55;
        float wz = depth + u.time * speed + u.beatPhase * (0.38 + u.bass * 0.28);
        float surface = sin(wx * 1.7 + wz * 0.45 + phase * 0.18) * 0.18;
        surface += sin(wx * 3.3 - wz * 0.32 + u.mid * 2.0 - phase * 0.22) * 0.08;
        surface += (fbm(float2(wx * 0.22 + cos(phase) * 0.10, wz * 0.18 + sin(phase) * 0.08)) - 0.5) * 0.22;
        surface *= height * (0.8 + u.bassImpact * 0.45);

        float rows = (wz + surface * 3.0 + u.beatPhase * 0.70) * density;
        float cols = (wx + surface + sin(phase) * 0.035) * density * 0.72;
        float rowLine = aaLineCyclic(rows, 0.010 / max(depth * 0.25, 0.7));
        float colLine = aaLineCyclic(cols, 0.006 / max(depth * 0.25, 0.7)) * 0.45;
        float fade = exp(-ft * 2.2) * smoothstep(0.02, 0.18, ft);
        float hue = uv.x + 0.16 * u.spectralCentroid + surface + u.beatPhase * 0.08;
        glow += palNeon(hue) * max(rowLine, colLine) * fade * (0.55 + 1.4 * bandAt(u, fract(uv.x + u.beatPhase * 0.08)));

        float crest = smoothstep(0.22, 0.38, surface + phaseLift * 0.05) * rowLine;
        glow += palHeatIce(0.08 + uv.x * 0.3) * crest * u.bassImpact * 3.0;
        glow += palVisual(0.15 + uv.x * 0.25 + u.beatPhase * 0.16, u) * rowLine * fade * phasePulse(u, ft * 0.35, 0.16) * (0.10 + u.level * 0.18);
    }

    float horizonLine = aaLine(uv.y - horizon, 0.0016);
    glow += palNight(0.58 + 0.1 * u.spectralCentroid) * horizonLine * 0.22;

    float3 col = feedbackTrail(glow, uPrev, uSamp, uv,
                               min(u.trailDecay, 0.86),
                               0.002 + 0.004 * u.bassImpact,
                               0.0);
    return float4(col, 1.0);
}
