#include "VisualShared.h"

// Helix — continuous spectral double-helix ribbons.
// Screen-space analytic strand fields; cheap and Apple TV friendly.
// param0 Twist, param1 Depth, param2 Sparkle

fragment float4 frag_helix(VOut in [[stage_in]],
                           constant VisualUniforms& u [[buffer(0)]],
                           texture2d<float> uImage [[texture(0)]],
                           texture2d<float> uPrev [[texture(1)]],
                           sampler uSamp [[sampler(0)]]) {
    float2 p = motionCentered(in.uv, u);
    p = rot2(-0.08) * p;

    float t = clamp((p.x + 1.55) / 3.10, 0.0, 1.0);
    float twist = mix(2.0, 5.2, u.param0);
    float depth = mix(0.28, 0.72, u.param1);
    float sparkle = mix(0.35, 1.5, u.param2);
    float bandv = bandAt(u, t);
    float phase = t * 6.2831853 * twist + u.time * 0.85 + u.beatPhase * 1.2;

    float amp = (0.14 + bandv * depth * 0.28) * smoothstep(0.0, 0.12, t) * (1.0 - smoothstep(0.88, 1.0, t));
    float y1 = sin(phase) * amp;
    float y2 = -y1;
    float z1 = 0.55 + 0.45 * cos(phase);
    float z2 = 1.0 - z1;

    float3 c1 = palRoleTrace(t + 0.12 * u.spectralCentroid, u);
    float3 c2 = palRoleTrace(t + 0.50 + 0.12 * u.spectralCentroid, u);
    float strand1 = aaLine(p.y - y1, 0.006 + bandv * 0.004) * z1;
    float strand2 = aaLine(p.y - y2, 0.006 + bandv * 0.004) * z2;
    float halo1 = aaLine(p.y - y1, 0.028) * 0.18 * z1;
    float halo2 = aaLine(p.y - y2, 0.028) * 0.18 * z2;

    float3 glow = c1 * (strand1 * (0.7 + bandv * 3.4 + (u.onset + u.beat * 0.6 + u.bassImpact * 0.5) * 2.2) + halo1)
                + c2 * (strand2 * (0.7 + bandv * 3.4 + (u.onset + u.beat * 0.6 + u.bassImpact * 0.5) * 2.2) + halo2);

    // Rungs: short segments between strands at discrete x cells.
    float cell = t * 28.0;
    float rungGate = aaLineCyclic(cell, 0.045);
    float between = step(min(y1, y2), p.y) * step(p.y, max(y1, y2));
    glow += palRoleFog(t + 0.2, u) * rungGate * between * (0.12 + bandv * 1.4 + u.beat * 0.8);

    // Spark nodes on beat/flux.
    float sparkCell = floor(t * 28.0);
    float sparkSeed = hash11(sparkCell + floor(u.beatCount));
    float sparkY = mix(y1, y2, step(0.5, sparkSeed));
    float spark = aaLineCyclic(cell, 0.040) * aaLine(p.y - sparkY, 0.025) * step(0.82, sparkSeed);
    glow += palRoleTreble(t + 0.08 * u.spectralCentroid, u) * spark * (u.onset + u.beat * 0.7 + u.bassImpact * 0.5) * sparkle * 4.0;

    float3 col = feedbackTrail(glow, uPrev, uSamp, in.uv,
                               min(u.trailDecay, 0.84),
                               0.002 + 0.004 * u.bassImpact,
                               0.0015 * u.beat);
    return float4(col, 1.0);
}
