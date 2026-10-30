#include "VisualShared.h"

// Comets — orbital comet trails crossing a dark stage.
// 56 analytic comets with short tails.
// param0 Count, param1 Orbit Spread, param2 Tail Length

fragment float4 frag_comets(VOut in [[stage_in]],
                            constant VisualUniforms& u [[buffer(0)]],
                            texture2d<float> uImage [[texture(0)]],
                            texture2d<float> uPrev [[texture(1)]],
                            sampler uSamp [[sampler(0)]]) {
    float2 p = motionCentered(in.uv, u);
    float count = mix(24.0, 56.0, u.param0);
    float spread = mix(0.42, 1.20, u.param1);
    float tail = mix(0.10, 0.34, u.param2);
    float3 glow = float3(0.0);

    for (int i = 0; i < 56; i++) {
        if (float(i) >= count) break;
        float fi = float(i);
        float2 h = hash22(float2(fi * 1.73, 3.0));
        float bandv = bandAtWrapped(u, h.x + u.beatPhase * 0.10);
        float phasePush = phasePulse(u, h.x, 0.20);
        float life = fract(h.y + u.time * (0.06 + 0.30 * h.x) + u.beatCount * 0.010 + u.beatPhase * (0.08 + h.x * 0.16));
        float a = h.x * 6.2831853 + life * 6.2831853 * (0.35 + h.y) + sin(beatRadians(u, h.y)) * 0.08;
        float rr = mix(0.16, spread, h.y) * (0.75 + bandv * 0.42 + u.bassImpact * 0.08 + phasePush * 0.08);
        float2 pos = float2(cos(a) * rr, sin(a) * rr * 0.62);
        float2 tangent = normalize(float2(-sin(a), cos(a) * 0.62) + 1e-5);

        float2 v = p - pos;
        float along = dot(v, tangent);
        float across = length(v - tangent * along);
        float head = min(1.6, 0.00022 / (dot(v, v) + 0.00022));
        float trail = exp(-across * across * 900.0) * smoothstep(-tail, 0.0, along) * (1.0 - smoothstep(0.0, 0.08, along));
        float3 c = palRoleTrace(h.x + 0.16 * u.spectralCentroid + u.beatPhase * 0.10, u);
        glow += c * (head * (0.40 + bandv * 2.2 + phasePush * 0.28 + u.onset * 0.75) + trail * (0.10 + bandv * 1.25 + phasePush * 0.16 + u.beat * 0.35));
    }

    float core = aaLine(length(p) - (0.22 + u.bassImpact * 0.025), 0.003);
    glow += palRoleFog(0.58 + 0.1 * u.spectralCentroid, u) * core * (0.45 + u.bassImpact * 1.3 + u.onset * 0.5);

    float3 col = feedbackTrail(glow, uPrev, uSamp, in.uv,
                               min(u.trailDecay, 0.90),
                               0.002 + 0.006 * u.bassImpact,
                               0.002 * u.beat);
    return float4(col, 1.0);
}
