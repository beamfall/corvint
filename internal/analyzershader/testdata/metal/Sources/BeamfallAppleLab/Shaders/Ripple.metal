#include "VisualShared.h"

// Ripple — Lite / beat + spectrum. Beat impacts throw luminous, frequency-warped
// cymatics ripples across a black liquid surface. Multiple rings spawn at center
// and off-axis seed points; each ring's radius is distorted by the spectrum so the
// contours read as cymatic standing-wave patterns rather than perfect circles.
// Older rings cool to cyan/violet; fresh beat rings are gold/white HDR.
//
// param0 = Ring Width      (0..1  →  thin shard .. wide pulse)
// param1 = Expansion       (0..1  →  slow drift .. rapid blaze)
// param2 = Persistence     (0..1  →  short-lived .. long decay)
//
// HDR out — DO NOT tonemap here. Ring leading edges target 4–6x.
fragment float4 frag_ripple(VOut in [[stage_in]],
                             constant VisualUniforms& u [[buffer(0)]],
                             texture2d<float> uImage    [[texture(0)]],
                             texture2d<float> uPrev     [[texture(1)]],
                             sampler uSamp              [[sampler(0)]]) {
    const float TEXTURE_REPEATS = 6.0;

    // -------------------------------------------------------------------------
    // Param mapping
    // -------------------------------------------------------------------------
    float ringHalfW   = mix(0.003, 0.022, u.param0);   // aaLine half-width
    float expansion   = mix(0.28,  1.40,  u.param1);    // max expansion radius
    float persistence = mix(0.40,  0.92,  u.param2);    // feedback zoom amount

    // -------------------------------------------------------------------------
    // Coordinate setup — aspect-correct centered space
    // -------------------------------------------------------------------------
    float2 p = motionCentered(in.uv, u);
    float  a = atan2(p.y, p.x);                         // angle -π..π
    float  ang01 = a / 6.2831853 + 0.5;                 // 0..1 around circle

    // -------------------------------------------------------------------------
    // Per-angle spectrum read — used to distort ring radii (cymatics contours).
    // We blend two adjacent bands so the warp is smooth around the ring.
    // -------------------------------------------------------------------------
    // Map angle to a low-end or mid bin range so bass/mid drive contour shape
    float specBin  = bandAt(u, ang01 * 0.55 + 0.05);   // 0..1 band value at angle
    float specBin2 = bandAt(u, fract(ang01 * 0.55 + 0.30)); // second harmonic
    // Combine: fundamental shape + half-amplitude second harmonic
    float warp     = specBin * 0.18 + specBin2 * 0.09;  // radius perturbation

    // Additional fbm-domain angular warp so contours look organic (tiny nudge)
    float fbmAngle = fbm(float2(ang01 * 3.2 + u.time * 0.07,
                                u.beatCount * 0.17));
    warp += (fbmAngle - 0.5) * 0.06 * (0.3 + u.mid * 0.7);

    // -------------------------------------------------------------------------
    // Ring train — 7 rings cycling in phase, seeded by beatCount for variation
    // -------------------------------------------------------------------------
    const int NRINGS = 7;
    float3 col = float3(0.0);

    for (int k = 0; k < NRINGS; ++k) {
        float fk = float(k);

        // Stagger phase so rings are spread across the cycle; beatCount shifts
        // the pattern so each beat "set" has a different starting point.
        float phaseOffset = fract(fk / float(NRINGS) +
                                  u.beatCount * (0.13 + fk * 0.019));
        // Speed scales with param1 + bassImpact for hard reactivity
        float speed = 0.12 * (1.0 + u.amplitude * 0.5)
                    * (0.65 + 0.35 * u.bassImpact + 0.15 * u.beat);
        float phase = fract(u.beatPhase * 0.5 + phaseOffset + u.time * speed);

        // Warped ring radius — the cymatic distortion is added here
        float ringR   = phase * expansion;
        float ringRW  = ringR + warp * ringR;   // distort proportional to radius

        // Radial distance from this ring (guard: max(r,0))
        float r = length(p);
        float dist = abs(r - ringRW);

        // AA line with fwidth — correct anti-aliasing regardless of ring size
        float lineMask = aaLine(dist, ringHalfW);

        // Fade with age: ring dims as it expands. Capped at 1.5× expansion.
        float age   = clamp(phase, 0.0, 1.0);
        float ageFade = pow(1.0 - age, 1.6 + u.param2 * 0.8);

        // Beat-fresh boost: rings born just after a beat impact are gold/white hot
        // phase near 0 = newborn. bassImpact snaps high on a kick.
        float birthAge    = smoothstep(0.18, 0.0, phase);
        float beatBoost   = 1.0 + birthAge * (u.bassImpact * 5.0 + u.beat * 2.5);

        // Brightness = line × age × beat boost × global audio gate
        float brightness = lineMask * ageFade * beatBoost *
                           (0.6 + u.amplitude * 1.4);

        // -------------------------------------------------------------------
        // Color: new ring = gold/white hot; older rings cool to cyan/violet
        // palNeon maps t→ magenta/orange/green/cyan/violet
        // We drive t with phase so rings drift from warm (low t) to cool (high t)
        // -------------------------------------------------------------------
        float palT   = fract(phase * 0.55 + fk * 0.13 + u.spectralCentroid * 0.12
                             + u.time * 0.04);
        float3 ringCol = palVisual(palT, u);

        // New birth rings: hot orange, with white only on true beat peaks.
        float3 hotBirth = float3(1.7, 0.28, 0.03);
        ringCol = mix(ringCol, hotBirth, birthAge * saturate(u.bassImpact + u.beat));
        ringCol = peakWhite(ringCol, birthAge * u.bassImpact * 0.32);

        // Accent weave — subtle
        ringCol = accentize(ringCol, u.accent, 0.10);

        // HDR scale: leading edge brightness 4–6× on a beat
        float hdrScale = mix(1.8, 5.0, birthAge * saturate(u.bassImpact + u.beat * 0.6));
        col += ringCol * brightness * hdrScale;
    }

    // -------------------------------------------------------------------------
    // Onset / treble: thin secondary caustic shards — sharp bright threads that
    // flash briefly on transients, adding cymatics-shard texture.
    // -------------------------------------------------------------------------
    if (u.onset > 0.05 || u.treble > 0.30) {
        float r = length(p);
        // Thin rings at 3–5 fixed fractional radii, modulated by treble
        for (int s = 0; s < 4; s++) {
            float shardR = (0.15 + float(s) * 0.11) * expansion;
            float shardR2 = shardR + warp * shardR * 0.6;  // lighter warp
            float shardDist = abs(r - shardR2);
            float shardMask = aaLine(shardDist, ringHalfW * 0.35);
            // Modulate shard brightness sharply by angle (gives shard/arc look)
            float angMod = 0.5 + 0.5 * sin(ang01 * 6.2831853 * TEXTURE_REPEATS
                                            + u.time * 1.8);
            float3 shardCol = palVisual(ang01 + float(s) * 0.22, u);
            shardCol = peakWhite(shardCol, saturate(u.treble * u.onset * 0.35));
            col += shardCol * shardMask * angMod
                   * (u.onset * 3.5 + u.treble * 1.2)
                   * 2.5;
        }
    }

    // -------------------------------------------------------------------------
    // Feedback trails: small outward zoom → persistent expanding rings.
    // zoom = small constant + bassImpact boost; rotation = near-zero (ripples
    // don't spin). We use persistence param to scale the effective trailDecay.
    // -------------------------------------------------------------------------
    float zoom    = 0.004 + 0.010 * u.bassImpact * u.param1; // slight beat pulse
    float rotAmt  = 0.0015 * u.mid;                           // barely perceptible
    // Scale trailDecay by persistence param so param2 drives ring lifetime
    float effDecay = u.trailDecay * (0.75 + 0.25 * persistence);
    col = feedbackTrail(col, uPrev, uSamp, in.uv, effDecay, zoom, rotAmt);

    // Background must stay black — no additive bg fill.
    return float4(col, 1.0);   // LINEAR HDR — post chain tonemaps
}
