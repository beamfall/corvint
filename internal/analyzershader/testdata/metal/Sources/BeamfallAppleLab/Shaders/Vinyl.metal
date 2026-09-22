#include "VisualShared.h"

// Vinyl — luminous record grooves with beat needle sparks.
// Calm overlay candidate: circular grooves and subtle spectrum etching.
// param0 Groove Count, param1 Spin, param2 Needle

fragment float4 frag_vinyl(VOut in [[stage_in]],
                           constant VisualUniforms& u [[buffer(0)]],
                           texture2d<float> uImage [[texture(0)]],
                           texture2d<float> uPrev [[texture(1)]],
                           sampler uSamp [[sampler(0)]]) {
    float2 p = motionCentered(in.uv, u);
    p = rot2(u.time * mix(0.02, 0.22, u.param1)) * p;
    p.y /= 0.82;
    float r = length(p);
    float a = atan2(p.y, p.x);
    float ang = a / 6.2831853 + 0.5;

    float grooveCount = mix(14.0, 40.0, u.param0);
    float needle = mix(0.2, 1.2, u.param2);
    float3 glow = float3(0.0);

    float ringBand = fract(r * grooveCount + 0.02 * sin(a * 9.0 + u.time));
    float grooves = aaLine(ringBand - 0.5, 0.035) * smoothstep(0.18, 0.28, r) * (1.0 - smoothstep(0.94, 1.10, r));
    float energy = bandAt(u, ang);
    float3 c = palRoleFog(ang * 0.35 + 0.12 * u.spectralCentroid, u);
    glow += c * grooves * (0.18 + energy * 0.75);

    float hotGroove = aaLine(r - (0.46 + 0.04 * sin(u.beatPhase * 6.2831853)), 0.004);
    glow += palRoleTrace(ang + 0.12 * u.spectralCentroid, u) * hotGroove * (0.65 + u.bassImpact * 1.4);

    // Needle arm: diagonal line with a tiny hot contact point.
    float2 a0 = float2(0.15, -0.72);
    float2 a1 = float2(0.52, -0.12 + 0.03 * sin(u.time * 0.6));
    float2 ab = a1 - a0;
    float t = clamp(dot(p - a0, ab) / max(dot(ab, ab), 1e-4), 0.0, 1.0);
    float armD = length(p - (a0 + ab * t));
    glow += palRolePeak(0.70, u) * aaLine(armD, 0.003) * needle * 0.55;
    float contact = min(1.5, 0.00025 / (length(p - a1) * length(p - a1) + 0.00025));
    glow += palRoleBass(0.08, u) * contact * needle * (0.35 + u.flux + u.bassImpact);

    float3 col = feedbackTrail(glow, uPrev, uSamp, in.uv,
                               min(u.trailDecay, 0.72),
                               0.001 + 0.002 * u.bassImpact,
                               0.0);
    return float4(col, 1.0);
}
