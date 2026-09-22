#include "VisualShared.h"

// Circuit — beat pulses through a neon board.
// Dense but cheap grid/trace math, no heavy object loop.
// param0 Scale, param1 Pulse Speed, param2 Trace Brightness

fragment float4 frag_circuit(VOut in [[stage_in]],
                             constant VisualUniforms& u [[buffer(0)]],
                             texture2d<float> uImage [[texture(0)]],
                             texture2d<float> uPrev [[texture(1)]],
                             sampler uSamp [[sampler(0)]]) {
    float2 p = motionCentered(in.uv, u);
    float scale = mix(5.0, 13.0, u.param0);
    float speed = mix(0.35, 1.5, u.param1);
    float bright = mix(0.5, 1.6, u.param2);

    float2 q = p * scale;
    float2 cell = floor(q);
    float2 f = fract(q) - 0.5;
    float h = hash21(cell);
    float orient = step(0.5, h);

    float lineH = aaLine(f.y, 0.025) * step(0.18, abs(f.x));
    float lineV = aaLine(f.x, 0.025) * step(0.18, abs(f.y));
    float trace = mix(lineH, lineV, orient);

    float node = min(1.0, 0.002 / (dot(f, f) + 0.002));
    float wave = fract((cell.x + cell.y) * 0.07 - u.time * speed + u.beatPhase);
    float pulse = exp(-pow(wave - 0.5, 2.0) * 60.0) * (0.45 + u.bassImpact);
    float bandv = bandAt(u, fract(h + 0.08 * u.spectralCentroid));
    float3 c = palRoleTrace(h + 0.15 * u.spectralCentroid, u);
    c = accentize(c, u.accent, 0.12);

    float vign = smoothstep(1.45, 0.25, length(p));
    float3 glow = c * (trace * (0.18 + pulse * bright + bandv * 0.55) + node * (0.15 + bandv * 0.9 + u.onset * 0.8)) * vign;

    // Diagonal bus lanes for a more premium circuit-board read.
    float bus = aaLineCyclic((p.x + p.y) * 3.8 - u.time * 0.12, 0.012);
    glow += palRoleTreble(0.55 + 0.1 * u.spectralCentroid, u) * bus * vign * (u.onset * 0.6 + u.beat * 0.3);

    float3 col = feedbackTrail(glow, uPrev, uSamp, in.uv,
                               min(u.trailDecay, 0.78),
                               0.001 + 0.002 * u.bassImpact,
                               0.0);
    return float4(col, 1.0);
}
