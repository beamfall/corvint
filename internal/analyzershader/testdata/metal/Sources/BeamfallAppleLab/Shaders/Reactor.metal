#include "VisualShared.h"

// Reactor — central fusion core with orbiting spectrum arcs.
// Analytic polar geometry; no raymarch.
// param0 Core, param1 Orbit Count, param2 Sparks

fragment float4 frag_reactor(VOut in [[stage_in]],
                             constant VisualUniforms& u [[buffer(0)]],
                             texture2d<float> uImage [[texture(0)]],
                             texture2d<float> uPrev [[texture(1)]],
    sampler uSamp [[sampler(0)]]) {
    const float SPOKE_COUNT = 8.0;
    float2 p = motionCentered(in.uv, u);
    float phaseSpin = beatRadians(u, 0.0);
    p = rot2(0.18 + sin(phaseSpin) * 0.035) * p;
    p.y /= 0.86;
    float r = length(p);
    float a = atan2(p.y, p.x);
    float ang = a / 6.2831853 + 0.5;

    float core = mix(0.12, 0.25, u.param0) + u.bassImpact * 0.025 + phasePulse(u, 0.0, 0.20) * 0.012;
    float orbitCount = mix(3.0, 8.0, u.param1);
    float sparkAmt = mix(0.4, 1.7, u.param2);

    float3 glow = float3(0.0);

    float coreFill = exp(-r * r / max(core * core, 1e-4));
    glow += palRoleFog(0.58 + 0.1 * u.spectralCentroid, u) * coreFill * (0.12 + u.level * 0.42 + u.onset * 0.55);
    glow += palRoleBass(0.55, u) * aaLine(r - core, 0.006) * (1.0 + 2.2 * u.bassImpact + 1.8 * u.onset);

    for (int k = 0; k < 8; k++) {
        if (float(k) >= orbitCount) break;
        float fk = float(k);
        float rr = core + 0.08 + fk * 0.085;
        float bandv = bandAtWrapped(u, fk / 8.0 + ang * SPOKE_COUNT + u.beatPhase * 0.08);
        float orbitPhase = beatRadians(u, fk * 0.11);
        float wobble = sin(a * (3.0 + fk) + u.time * (0.8 + fk * 0.1) + orbitPhase * 0.18) * 0.012 * bandv;
        float line = aaLine(r - rr - wobble, 0.0032);
        float cell = fract(ang * (18.0 + fk * 2.0) - u.time * 0.08 - fk * 0.13 - u.beatPhase * (0.45 + fk * 0.05));
        float arc = smoothstep(0.02, 0.10, cell) * smoothstep(0.72, 0.52, cell);
        float phaseSpark = phasePulse(u, fract(fk * 0.17), 0.18);
        float3 c = palRoleTrace(ang + fk * 0.09 + 0.1 * u.spectralCentroid + u.beatPhase * 0.12, u);
        glow += c * line * arc * (0.35 + bandv * 3.2 + phaseSpark * 0.45 + u.beat * 0.6);
    }

    for (int i = 0; i < 34; i++) {
        float fi = float(i);
        float h = hash11(fi * 1.73 + floor(u.beatCount));
        float life = fract(h + u.time * (0.25 + h * 0.7) + u.beatPhase * 0.16);
        float aa = h * 6.2831853 + life * 0.8;
        float sr = mix(core, 1.45, life);
        float2 sp = float2(cos(aa), sin(aa) * 0.86) * sr;
        float d = length(p - sp);
        float spark = min(1.5, 0.00022 / (d * d + 0.00035));
        glow += palRoleTreble(h + 0.2 * u.spectralCentroid, u) * spark * (1.0 - life) * (u.onset * 1.1 + u.flux * 0.4) * sparkAmt;
    }

    glow = accentize(glow, u.accent, 0.12);
    float3 col = feedbackTrail(glow, uPrev, uSamp, in.uv,
                               min(u.trailDecay, 0.90),
                               0.003 + 0.007 * u.bassImpact,
                               0.002 + 0.004 * u.beat);
    return float4(col, 1.0);
}
