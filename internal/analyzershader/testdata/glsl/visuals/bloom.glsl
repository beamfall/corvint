// Bloom — Lite / centered organic ink bloom.
// Portable body is authoritative for improvements (hand-sync to Bloom.metal).
// A single luminous organism blooming from the center: domain-warped petals
// breathing with the level, a hot core, bass light-leak ring and electric
// treble veins. Everything outside the bloom stays black.
// param0 = Flow ; param1 = Color Spread ; param2 = Veins

vec3 visual(vec2 uv, VisualUniforms u) {
    float flowSpeed   = mix(0.05, 0.30, u.param0);
    float colorSpread = mix(0.15, 0.90, u.param1);
    float veinAmt     = mix(0.00, 1.00, u.param2);

    vec2 p = motionCentered(uv, u);
    float r = length(p);
    float ang = atan2_(p.y, p.x);

    float t = u.time;
    float hueTemp = u.spectralCentroid;

    // ---- breathing envelope: the bloom's overall radius ----
    float breathe = 0.66
                  + 0.07 * sin(t * 0.45)
                  + 0.22 * u.level
                  + 0.14 * u.bassImpact
                  + 0.05 * sin(u.beatPhase * 6.2831853);

    // ---- organic petals: domain-warped noise sculpting the radius ----
    float warpAmp = 0.30 + 1.10 * u.amplitude;
    vec2 seed = p * 1.45 + vec2(t * flowSpeed * 0.35, -t * flowSpeed * 0.22);
    vec2 w = vec2(fbm(seed), fbm(seed + vec2(5.23, 3.78))) - 0.5;
    float n = fbm(seed * 1.35 + w * warpAmp + vec2(0.0, -t * flowSpeed * 0.5));

    // slow-rotating petal lobes, count breathing with the mid band
    float lobes = 5.0;
    float petal = 0.5 + 0.5 * sin(ang * lobes - t * 0.35 + w.x * 3.2);
    petal = mix(0.72, 1.0, petal);

    // signed field: positive inside the bloom
    float field = breathe * petal * (0.62 + 0.62 * n) - r;

    float gate = smoothstep(-0.03, 0.16, field);
    float bodyShade = smoothstep(-0.03, 0.42, field);

    // ---- color: radius + noise walks the palette ----
    float palT = fract(0.06 * t
                     + (1.0 - saturate(r / (breathe + 0.2))) * colorSpread
                     + n * colorSpread * 0.8
                     + 0.08 * sin(ang + w.x * 4.0)
                     + 0.16 * hueTemp);
    vec3 col = palVisual(palT, u);
    col = accentize(col, u.accent, 0.08);
    col *= gate * (0.22 + 0.70 * bodyShade) * (0.42 + 0.60 * u.level)
         * (0.55 + 0.45 * petal);

    // hot HDR core, kicked by bass
    float core = exp(-r * r * (16.0 - 5.0 * u.bassImpact));
    vec3 coreCol = palVisual(palT + 0.12, u);
    col += peakWhite(coreCol, 0.06 + 0.10 * u.bassImpact)
         * core * (0.22 + 0.80 * u.bassImpact + 0.30 * u.beat);

    // ---- bass light-leak ring around the bloom ----
    float leak = u.bassImpact;
    if (leak > 0.01) {
        float ringR = breathe * (0.92 + 0.18 * leak);
        float ringW = 0.045 + 0.06 * leak;
        float ringMask = exp(-((r - ringR) / max(ringW, 1e-3))*((r - ringR) / max(ringW, 1e-3)) * 3.0);
        vec3 leakCol = accentize(palVisual(0.10 + 0.45 * hueTemp, u), u.accent, 0.2);
        col += leakCol * ringMask * leak * 0.65;
    }

    // ---- electric veins radiating through the bloom (treble) ----
    float veinPower = (0.25 + u.treble) * veinAmt;
    if (veinPower > 0.005) {
        float vein = fbmRidged(seed * 2.1 + w * 1.4 + vec2(4.1, 2.7));
        float veinLine = smoothstep(0.62, 0.86, vein) * gate;
        vec3 veinCol = palVisual(0.72 + 0.18 * hueTemp + 0.05 * sin(t * 1.3), u);
        veinCol = peakWhite(veinCol, veinLine * u.treble * 0.12);
        col += veinCol * veinLine * veinPower * (0.6 + 1.4 * u.treble) * 1.1;
    }

    // onset flash: a bright rim tracing the bloom's silhouette
    if (u.onset > 0.05) {
        float silh = gate * (1.0 - smoothstep(0.0, 0.10, field));
        col += vec3(0.5, 1.1, 1.3) * silh * u.onset * 1.2;
    }

    // feedback trail — slow outward exhale
    float trailZoom = 0.004 + 0.010 * u.amplitude;
    float trailRot  = 0.002 + 0.003 * u.beat;
    col = feedbackTrail(col, uv, min(u.trailDecay, 0.70), trailZoom, trailRot);

    return col; // LINEAR HDR — post chain tonemaps
}
