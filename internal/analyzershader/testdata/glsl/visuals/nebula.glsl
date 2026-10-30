// Nebula — Flagship — frequency + beat.
// Portable port of beamfall-apple-ui Shaders/Nebula.metal (keep in sync).
//
// Volumetric smoke threaded with spectrum lightning around a dark core.
// Performance path: layered analytic shells, not a full raymarch (12 shells).
// DARK CORE enforced: radial carve + density clamp keeps the center black.
// Lightning filaments driven by treble; swirl driven by bassImpact.
//
// param0  Density    (0..1) — cloud thickness/opacity scale
// param1  Swirl      (0..1) — rotational domain-warp strength
// param2  Emission   (0..1) — ridge/lightning emission multiplier

vec3 visual(vec2 uv, VisualUniforms u) {
    // ---- coordinate setup --------------------------------------------------
    vec2  p0 = motionCentered(uv, u);   // aspect-correct ~[-1,1]
    float t  = u.time * 0.14;

    // ---- param mapping -----------------------------------------------------
    float densScale = mix(0.15, 0.70, u.param0);   // keep upper end < 1 to stay wispy
    float swirlAmt  = mix(0.4,  3.2,  u.param1);
    float emitScale = mix(0.8,  3.5,  u.param2);

    // ---- audio features ----------------------------------------------------
    float bassE   = u.bass;
    float midE    = u.mid;
    float trebleE = u.treble;
    float impact  = u.bassImpact;   // gated kick, snaps high on hit
    float phaseBreath = beatWave(u, 0.0);
    float specCentroid = u.spectralCentroid;
    float fluxV    = u.flux;

    float midBand    = bandRange(u, 10, 36);
    float trebleBand = bandRange(u, 40, 64);

    // ---- dark core carve ---------------------------------------------------
    float coreRadius = 0.22 + impact * 0.12;

    // ---- layered volume ----------------------------------------------------
    vec3  col      = vec3(0.0);
    float transmit = 1.0;

    const int   LAYERS     = 12;
    const float INV_LAYERS = 1.0 / float(LAYERS);
    float radius = length(p0);
    float outerMask = 1.0 - smoothstep(1.05, 1.48, radius);
    float coreMask = smoothstep(coreRadius, coreRadius + 0.28, radius);

    for (int i = 0; i < LAYERS; ++i) {
        float fi = (float(i) + 0.5) * INV_LAYERS;
        float depth = 1.0 - fi;
        float sliceAngle = t * 0.55 + swirlAmt * fi * 1.65 + impact * fi * 2.0 + sin(beatRadians(u, fi * 0.23)) * 0.16;
        float shell = mix(1.28, 0.72, fi) * (1.0 + (phaseBreath - 0.5) * 0.045 * (1.0 - fi));
        vec2 sp = rot2(sliceAngle) * (p0 * shell);

        vec2 warp = vec2(
            fbm(sp * 1.05 + vec2(1.7, 9.2) + t * 0.9 + midBand * 0.7 + vec2(u.beatPhase * 0.18, 0.0)),
            fbm(sp * 1.05 + vec2(8.3, 2.8) - t * 0.7 + midBand * 0.7 + vec2(0.0, u.beatPhase * 0.16))
        ) - 0.5;
        sp += warp * (0.28 + midE * 0.36 + depth * 0.10);

        float ridgeA = fbmRidged(sp * (1.55 + fi * 0.75) + vec2(fi * 2.1, t * 0.8));
        float ridgeB = fbmRidged(sp * 2.85 + vec2(t * 0.45, fi * 3.7));
        float rawDens = ridgeA * 0.70 + ridgeB * 0.30;
        float dens = smoothstep(0.63 - trebleBand * 0.025, 0.98, rawDens);
        dens *= (0.30 + 0.70 * smoothstep(0.40, 0.92, ridgeB));
        dens *= coreMask * outerMask * densScale * (0.34 + bassE * 0.24);
        dens = min(dens, 0.20);

        float ridgeLine = smoothstep(0.76, 0.97, ridgeA) * (0.22 + trebleBand * 1.8);
        float sparkBirth = step(0.82 - fluxV * 0.30, hash21(sp * 10.7 + fi * 3.9 + t + u.beatPhase * 0.75)) * fluxV;
        float lightning = ridgeLine + sparkBirth * 1.5;

        vec3 smokeCol = accentize(palVisual(fi * 0.55 + specCentroid * 0.22 + t * 0.04 + u.beatPhase * 0.05, u), u.accent, 0.10);
        vec3 litCol = peakWhite(palVisual(0.75 + specCentroid * 0.2 - fi * 0.15, u),
                                saturate(lightning * 0.20));
        float beatFlash = impact * depth * 1.3;
        vec3 beatCol = palVisual(0.08 + specCentroid * 0.1, u);

        float alpha = min(dens * 0.16, 0.075);
        vec3 emissive = smokeCol * (dens * 0.85 * emitScale + beatFlash * dens)
                      + litCol * (lightning * (1.8 + trebleE * 1.6) * emitScale + beatFlash * 0.18)
                      + beatCol * (beatFlash * 0.45 * dens);
        col += transmit * alpha * emissive;
        transmit *= (1.0 - alpha * 0.45);
    }

    // ---- spectrum lightning filaments (screen-space, on top of volume) -----
    {
        for (int j = 0; j < 5; ++j) {
            float fj = float(j);
            vec2 h   = hash22(vec2(fj * 3.1, 7.3 + u.beatCount * 0.1));
            float fAngle = h.x * 6.2831853 + t * 0.5 + trebleBand * 1.2;
            vec2 fDir = vec2(cos(fAngle), sin(fAngle));
            float proj = dot(p0, fDir);
            float dist = length(p0 - fDir * proj);
            float along = smoothstep(1.3, coreRadius + 0.1, proj);
            along *= smoothstep(coreRadius - 0.05, coreRadius + 0.25, length(p0));
            float bolt = exp(-dist * dist * 620.0) * along;
            bolt *= (trebleBand * 1.2 + fluxV * 0.75) * (0.35 + h.y * 0.45);
            float tBolt = h.x + specCentroid * 0.3;
            vec3 boltCol = peakWhite(palVisual(tBolt, u), saturate(bolt * 0.22));
            col += boltCol * bolt * (1.9 + trebleE * 1.4);
        }
    }

    // ---- background star field (visible only through volume gaps) ----------
    {
        vec2 sv  = uv * vec2(90.0, 50.0);
        vec2 sci = floor(sv);
        vec2 scf = fract(sv);
        float rng  = hash21(sci);
        float rng2 = hash21(sci + 17.3);
        float starR = 0.04 + rng2 * 0.05;
        float star  = smoothstep(starR, 0.0, length(scf - 0.5));
        star *= step(0.92, rng) * transmit;
        vec3 starCol = palVisual(rng + specCentroid * 0.2, u);
        col += starCol * star * (0.35 + trebleE * 0.9);
    }

    // ---- feedback trails (curl-ish UV offset to simulate smoke advection) --
    {
        float swirlFeedback = 0.004 + swirlAmt * 0.003 + impact * 0.008;
        float zoomFeedback  = 0.003 + impact * 0.010;
        col = feedbackTrail(col, uv, u.trailDecay, zoomFeedback, swirlFeedback);
    }

    return col; // LINEAR HDR — post chain tonemaps
}
