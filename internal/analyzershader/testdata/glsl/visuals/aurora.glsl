// Aurora — Flagship — frequency + beat.
// Portable body is authoritative for improvements (hand-sync to Aurora.metal).
//
// Full-frame undulating light curtains over a starfield, with a dark ground
// and a dimmed water reflection below the horizon. Each curtain is a bright
// lower rim with rays streaking upward, striated by ridged noise and lit by
// the spectrum across the width of the frame.
// param0 Curtains -> 3..5 curtain layers
// param1 Sway     -> undulation amplitude
// param2 Altitude -> how high the rays reach

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    // y-down coords: p.y = -1 top of screen, +1 bottom.

    int nCurtains = int(mix(3.0, 5.0, u.param0) + 0.5);
    float swayAmt = mix(0.12, 0.34, u.param1);
    float altitude = mix(0.55, 1.25, u.param2);

    float t = u.time;
    float horizonY = 0.70;

    // Mirror below-horizon space back into the sky for a water reflection.
    float ground = smoothstep(horizonY - 0.005, horizonY + 0.03, p.y);
    vec2 q = p;
    q.y = horizonY - abs(p.y - horizonY) * 1.30;
    // ripple the reflection slightly
    q.x += ground * 0.020 * sin(p.y * 34.0 + t * 1.3);
    float mirrorDim = mix(1.0, 0.30, ground);

    vec3 col = vec3(0.0);

    // Deep night-sky gradient — darkest at the top, faint glow near horizon.
    float skyT = saturate((q.y + 1.0) * 0.5);
    col += palVisual(0.60 + 0.10 * u.spectralCentroid, u)
         * (0.006 + 0.040 * skyT * skyT) * (0.55 + 0.75 * u.level) * mirrorDim;

    // Stars (sky only), twinkling with treble.
    vec2 cell = floor(p * vec2(52.0, 30.0));
    vec2 sh = hash22(cell);
    vec2 sPos = (cell + sh) / vec2(52.0, 30.0);
    float sd = length((p - sPos) * vec2(1.0, 1.0));
    float starSeed = hash21(cell + 7.0);
    float twinkle = 0.55 + 0.45 * sin(t * (2.0 + starSeed * 5.0) + starSeed * 40.0);
    float star = exp(-sd * sd * 26000.0) * step(0.93, starSeed) * twinkle;
    col += vec3(0.75, 0.85, 1.0) * star * (0.30 + 0.7 * u.treble) * (1.0 - ground);

    vec3 glow = vec3(0.0);

    for (int ci = 0; ci < 5; ci++) {
        if (ci >= nCurtains) break;
        float fi = float(ci);
        float fr = (fi + 0.5) / float(nCurtains);

        // farther curtains sit higher on screen and render dimmer
        float depth = mix(1.05, 0.45, fr);
        float baseY = mix(0.52, -0.42, fr);

        // spectrum drive across the frame width
        float xb = saturate(q.x * 0.26 + 0.5);
        float energy = bandAt(u, fr * 0.22 + xb * 0.58);

        // undulating lower-edge curve — band-driven sway
        float sway = sin(q.x * 0.65 - t * 0.21 + fi * 4.0) * 0.90
                   + sin(q.x * 1.5 + t * 0.50 + fi * 2.3) * 0.55
                   + sin(q.x * 3.1 - t * 0.33 + fi * 1.1) * 0.28;
        sway += (fbm(vec2(q.x * 0.85 + fi * 7.0, t * 0.11 + fi * 3.3)) - 0.5) * 2.4;
        float yc = baseY + sway * swayAmt * (1.0 + u.bass * 1.3 + u.bassImpact * 0.7);
        // beat wave traveling along the curtain
        yc += 0.045 * u.bassImpact * sin(q.x * 3.4 - u.beatPhase * 6.2831853 + fi * 1.7);

        float d = q.y - yc;              // >0 below the rim, <0 in the rays
        float above = max(-d, 0.0);

        // curtain body: rays fading upward; thin skirt just below the rim
        float body = d > 0.0 ? exp(-d * 34.0) * 0.30
                             : exp(-above / (0.22 * altitude));
        float rim = exp(-d * d * 620.0);

        // curtain presence: hard dark gaps where the curtain fades out
        float pres = smoothstep(0.28, 0.62,
                     fbm(vec2(q.x * 0.42 + fi * 13.7, t * 0.05 + fi)) + energy * 0.22);
        body *= pres; rim *= pres;

        // vertical ray striations, brightened by band energy
        float rays = fbmRidged(vec2(q.x * (4.5 + fi * 0.9) + fi * 11.0 + sway * 0.8,
                                    t * 0.18 + fi * 5.0));
        rays = 0.16 + 0.90 * smoothstep(0.30, 0.95, rays + energy * 0.30);

        float bright = (0.30 + 1.3 * energy + 0.40 * u.beat) * depth;

        // hue drifts with height: hot rim -> cooler high rays
        vec3 cLow = palVisual(fr * 0.45 + 0.06 + 0.10 * u.spectralCentroid + 0.015 * t, u);
        vec3 cHigh = palVisual(fr * 0.45 + 0.38 + 0.10 * u.spectralCentroid + 0.015 * t, u);
        vec3 c = mix(cLow, cHigh, saturate(above * 1.4 / altitude));
        c = accentize(c, u.accent, 0.10);

        col += c * body * rays * bright * 0.36 * mirrorDim;
        glow += peakWhite(c, 0.10 + 0.12 * u.treble) * rim * rays
              * (0.20 + 0.9 * energy + 0.6 * u.bassImpact) * depth * mirrorDim * 0.35;

        // treble sparks racing along the rim
        float sparkle = step(0.80, hash21(vec2(floor(q.x * 26.0 + fi * 9.0),
                                               floor(u.beatCount))));
        glow += palVisual(fr + 0.5, u) * rim * sparkle * u.flux * u.treble * 0.8 * mirrorDim;
    }

    // horizon line + waterline haze
    float hLine = exp(-abs(p.y - horizonY) * 60.0);
    col += palVisual(0.72 + 0.10 * u.spectralCentroid, u) * hLine
         * (0.05 + 0.18 * u.bassImpact + 0.10 * u.amplitude);
    // darken the water body slightly toward the bottom
    col *= 1.0 - ground * smoothstep(horizonY, 1.35, p.y) * 0.45;

    vec3 trailedGlow = feedbackTrail(glow, uv, min(u.trailDecay, 0.50),
                                     0.0015, 0.0005);
    return col + trailedGlow; // LINEAR HDR — post chain tonemaps
}
