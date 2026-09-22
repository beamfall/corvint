#include "VisualShared.h"

// Starfield — Flagship "Warp Drive". Warp-speed star streaks bend around polar
// spectrum rings, with beat radial jumps and orange burst eruptions.
//
// param0  Warp Speed      0..1 → base warp rate (slow cruise → hyperspace)
// param1  Density         0..1 → star sample count 20..120
// param2  Streak Length   0..1 → motion-blur tail multiplier
//
// HDR out — DO NOT tonemap. Near streaks 4–7x, beat eruptions 8x.
// Background must stay BLACK. feedbackTrail with zoom-OUT for warp streaks.
fragment float4 frag_starfield(VOut in [[stage_in]],
                               constant VisualUniforms& u [[buffer(0)]],
                               texture2d<float> uImage [[texture(0)]],
                               texture2d<float> uPrev  [[texture(1)]],
                               sampler uSamp [[sampler(0)]]) {

    // ── aspect-correct centred coords ────────────────────────────────────────
    float2 p = motionCentered(in.uv, u);   // ~[-1,1] x-scaled by aspect

    // ── params ───────────────────────────────────────────────────────────────
    float baseSpeed  = mix(0.30, 5.0, u.param0);
    int   starCount  = int(mix(20.0, 120.0, clamp(u.param1, 0.0, 1.0)));
    float streakMult = mix(0.10, 0.80, u.param2);

    // ── warp speed: locked to beat tempo + bass surge ────────────────────────
    // beatPhase (0→1 sawtooth) adds continuous forward push between beats.
    // bassImpact fires a fast surge on kick; beat adds a gentler pulse.
    float beatSurge  = u.bassImpact * 3.5 + u.beat * 1.5;
    float warpSpeed  = baseSpeed + beatSurge + u.beatPhase * baseSpeed * 0.45;

    // Monotonic warp position (drives star depth cycling)
    float warpPos = u.time * warpSpeed;

    // ── accumulate stars ─────────────────────────────────────────────────────
    float3 col = float3(0.0);

    for (int i = 0; i < 120; ++i) {
        if (i >= starCount) break;

        float fi = float(i);

        // Per-star deterministic seeds
        float h1 = hash11(fi * 0.1731 + 1.73);   // base angle [0,1)
        float h2 = hash11(fi * 0.3741 + 3.17);   // depth offset [0,1)
        float h3 = hash11(fi * 0.6191 + 7.37);   // brightness / size seed
        float h4 = hash11(fi * 0.2513 + 5.09);   // color selector

        // Star angle in [-π, π]
        float baseAngle = h1 * 6.28318530718;

        // Depth in [0,1): 0=center singularity, 1=viewer; advances with warpPos
        float depth = fract(h2 + warpPos * 0.065);

        // ── bending: mids drive path curvature around spectrum rings ─────────
        // Sample the band at this star's angular position
        float ang01     = h1;                          // same as angle/2π, in [0,1)
        float specBend  = bandAt(u, ang01);            // 0..1
        float bendAmt   = specBend * u.mid * 0.55;    // mids amplify bending
        float bentAngle = baseAngle + bendAmt * sin(depth * 6.2832 + u.time * 0.8);

        // Radial distance: non-linear growth for perspective acceleration
        float radius = depth * depth * 1.6;            // 0..1.6

        // 2D star position
        float2 starPos = float2(cos(bentAngle), sin(bentAngle)) * radius;

        // ── streak segment toward centre (tail direction) ────────────────────
        float streakLen = streakMult * depth * depth
                          * (1.0 + warpSpeed * 0.12)
                          * (1.0 + u.bassImpact * 1.8); // bass makes tails longer

        // Guard near-zero for normalize
        float starDist = length(starPos);
        float2 tailDir = starDist > 1e-5 ? (-starPos / starDist) : float2(0.0, -1.0);

        // Project pixel onto streak segment, find closest point
        float2 toPixel = p - starPos;
        float  tProj   = clamp(dot(toPixel, tailDir),
                               0.0, max(streakLen, 1e-5));
        float2 closest = starPos + tailDir * tProj;
        float  dist    = length(p - closest);

        // Star dot size: tiny near origin, blooms toward viewer
        float dotSize = (0.0005 + h3 * 0.0025) * (0.2 + depth * 2.0);

        // Brightness envelope: fade in from singularity, fade out past edge
        float bright = smoothstep(0.0, 0.30, depth)
                     * smoothstep(1.0, 0.72, depth)
                     * (0.55 + h3 * 0.45) * (0.35 + specBend * 0.9 + u.onset * 0.6 + u.beat * 0.35);

        // Glow falloff along the streak
        float glow = dotSize / max(dist, dotSize * 0.4);
        glow = min(glow, 6.0) * bright;   // allow up to 6x for HDR bloom

        // ── color ────────────────────────────────────────────────────────────
        // Head (tProj≈0) → cyan/white; tail → magenta arc or orange beat streak
        float tailFrac = tProj / max(streakLen, 1e-5); // 0=head, 1=tail

        // Base hue follows the active semantic palette across star index + depth.
        float3 coreCol = palRoleTrace(h4 + depth * 0.15 + u.spectralCentroid * 0.12, u);

        // Star cores: shift toward white/cyan
        float3 headCol = mix(float3(0.85, 0.95, 1.0),
                             coreCol * float3(0.6, 0.85, 1.1), 0.35);

        // Tail: semantic bass color surges on beat.
        float3 tailCol = palRoleBass(h4 * 0.6 + u.bassImpact * 0.3, u);
        tailCol = mix(tailCol, palRolePeak(0.08 + h4 * 0.05, u), u.beat * 0.8);

        float3 starCol = mix(headCol, tailCol, tailFrac * 0.7);
        starCol = accentize(starCol, u.accent, 0.10);

        // treble: tiny bright white sparks among the stars
        float whiteSpark = smoothstep(0.80, 1.0, h3) * u.treble * 3.0;
        starCol = mix(starCol, float3(1.2, 1.2, 1.4), whiteSpark);

        col += starCol * glow;
    }

    // ── beat eruption: radial orange streak burst from center ─────────────────
    // flux drives additional spark births in a radial fan
    {
        float r = length(p);
        float a = atan2(p.y, p.x);
        // Radial ripple ring on beat: bright expanding ring
        float beatRing = u.beat * 8.0                             // 8x HDR
                       * aaLine(r - (0.35 + u.beatPhase * 0.6), 0.025)
                       * (1.0 - smoothstep(0.3, 1.5, r));
        float3 burstCol = palNeon(0.08 + a / 6.28318530718 * 0.25); // orange-gold
        col += burstCol * beatRing;

        // flux spark eruption: random radial filaments
        float fluxSparks = 0.0;
        for (int j = 0; j < 12; ++j) {
            float fj   = float(j);
            float ha   = hash11(fj * 0.411 + u.beatCount * 0.1);
            float sa   = ha * 6.28318530718;
            // Each filament aligns radially outward
            float2 dir = float2(cos(sa), sin(sa));
            // Project pixel onto this ray from origin
            float  t   = max(dot(p, dir), 0.0);
            float2 cl  = dir * t;
            float  dd  = length(p - cl);
            float  filament = 0.0006 / max(dd, 0.001);
            filament *= smoothstep(0.8, 0.0, t) * u.flux * 5.0;
            col += palNeon(ha + 0.15) * filament;
        }
        (void)fluxSparks; // suppresses unused warning
    }

    // ── center singularity: fade to BLACK (not white) ─────────────────────────
    float cr = length(p);
    col *= smoothstep(0.0, 0.12, cr);      // fully black at the singularity

    // ── feedback warp trails (zoom-OUT = warp-speed effect) ──────────────────
    // Small zoom outward; beat briefly increases both zoom and slight rotation
    float trailZoom = 0.006 + 0.018 * u.bassImpact + 0.008 * u.beat;
    float trailRot  = 0.002 * u.beat;
    col = feedbackTrail(col, uPrev, uSamp, in.uv,
                        u.trailDecay, trailZoom, trailRot);

    // LINEAR HDR output — no tonemap here
    return float4(col, 1.0);
}
