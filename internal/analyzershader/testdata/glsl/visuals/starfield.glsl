// Starfield — Flagship "Warp Drive"
// Portable port of beamfall-apple-ui Shaders/Starfield.metal (keep in sync).
// param0 = Warp Speed ; param1 = Density ; param2 = Streak Length

vec3 visual(vec2 uv, VisualUniforms u) {
    // ── aspect-correct centred coords ────────────────────────────────────────
    vec2 p = motionCentered(uv, u);   // ~[-1,1] x-scaled by aspect

    // ── params ───────────────────────────────────────────────────────────────
    float baseSpeed  = mix(0.30, 5.0, u.param0);
    int   starCount  = int(mix(20.0, 120.0, clamp(u.param1, 0.0, 1.0)));
    float streakMult = mix(0.10, 0.80, u.param2);

    // ── warp speed: locked to beat tempo + bass surge ────────────────────────
    float beatSurge  = u.bassImpact * 3.5 + u.beat * 1.5;
    float warpSpeed  = baseSpeed + beatSurge + u.beatPhase * baseSpeed * 0.45;

    // Monotonic warp position (drives star depth cycling)
    float warpPos = u.time * warpSpeed;

    // ── accumulate stars ─────────────────────────────────────────────────────
    vec3 col = vec3(0.0);

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
        float ang01     = h1;                          // same as angle/2π, in [0,1)
        float specBend  = bandAt(u, ang01);            // 0..1
        float bendAmt   = specBend * u.mid * 0.55;    // mids amplify bending
        float bentAngle = baseAngle + bendAmt * sin(depth * 6.2832 + u.time * 0.8);

        // Radial distance: non-linear growth for perspective acceleration
        float radius = depth * depth * 1.6;            // 0..1.6

        // 2D star position
        vec2 starPos = vec2(cos(bentAngle), sin(bentAngle)) * radius;

        // ── streak segment toward centre (tail direction) ────────────────────
        float streakLen = streakMult * depth * depth
                          * (1.0 + warpSpeed * 0.12)
                          * (1.0 + u.bassImpact * 1.8); // bass makes tails longer

        // Guard near-zero for normalize
        float starDist = length(starPos);
        vec2 tailDir = starDist > 1e-5 ? (-starPos / starDist) : vec2(0.0, -1.0);

        // Project pixel onto streak segment, find closest point
        vec2  toPixel = p - starPos;
        float tProj   = clamp(dot(toPixel, tailDir),
                              0.0, max(streakLen, 1e-5));
        vec2  closest = starPos + tailDir * tProj;
        float dist    = length(p - closest);

        // Star dot size: tiny near origin, blooms toward viewer
        float dotSize = (0.0005 + h3 * 0.0025) * (0.2 + depth * 2.0);

        // Brightness envelope: fade in from singularity, fade out past edge
        float bright = smoothstep(0.0, 0.30, depth)
                     * smoothstep(1.0, 0.72, depth)
                     * (0.55 + h3 * 0.45) * (0.35 + specBend * 0.9 + u.onset * 0.6 + u.beat * 0.35);

        // Glow falloff along the streak — quadratic so halos stay tight and
        // space between stars stays black; cores still reach ~5x HDR
        float glow = (dotSize * dotSize) / (dist * dist + dotSize * dotSize * 0.2);
        glow = min(glow, 5.0) * bright;

        // ── color ────────────────────────────────────────────────────────────
        float tailFrac = tProj / max(streakLen, 1e-5); // 0=head, 1=tail

        // Base hue follows the active semantic palette across star index + depth.
        vec3 coreCol = palRoleTrace(h4 + depth * 0.15 + u.spectralCentroid * 0.12, u);

        // Star cores: saturated palette color, only a hint of white-hot
        vec3 headCol = mix(coreCol * 1.15, vec3(1.1, 1.15, 1.25),
                           0.18 + 0.25 * smoothstep(0.9, 1.0, depth));

        // Tail: semantic bass color surges on beat.
        vec3 tailCol = palRoleBass(h4 * 0.6 + u.bassImpact * 0.3, u);
        tailCol = mix(tailCol, palRolePeak(0.08 + h4 * 0.05, u), u.beat * 0.8);

        vec3 starCol = mix(headCol, tailCol, tailFrac * 0.7);
        starCol = accentize(starCol, u.accent, 0.10);

        // treble: tiny bright white sparks among the stars
        float whiteSpark = smoothstep(0.86, 1.0, h3) * saturate(u.treble * 1.4);
        starCol = mix(starCol, vec3(1.3, 1.3, 1.5), whiteSpark * 0.7);

        col += starCol * glow;
    }

    // ── beat eruption: radial orange streak burst from center ─────────────────
    {
        float r = length(p);
        float a = atan2_(p.y, p.x);
        // Radial ripple ring on beat: bright expanding ring
        float beatRing = u.beat * u.beat * u.beat * 1.6
                       * aaLine(r - (0.35 + u.beatPhase * 0.6), 0.008)
                       * (1.0 - smoothstep(0.3, 1.5, r));
        vec3 burstCol = palNeon(0.08 + a / 6.28318530718 * 0.25); // orange-gold
        col += burstCol * beatRing;

        // flux spark eruption: random radial filaments
        for (int j = 0; j < 12; ++j) {
            float fj   = float(j);
            float ha   = hash11(fj * 0.411 + u.beatCount * 0.1);
            float sa   = ha * 6.28318530718;
            // Each filament aligns radially outward
            vec2 dir = vec2(cos(sa), sin(sa));
            // Project pixel onto this ray from origin
            float t   = max(dot(p, dir), 0.0);
            vec2  cl  = dir * t;
            float dd  = length(p - cl);
            float filament = 0.0004 / max(dd, 0.001);
            filament *= smoothstep(0.8, 0.0, t) * u.flux * u.flux * 3.0;
            col += palNeon(ha + 0.15) * filament;
        }
    }

    // ── center singularity: fade to BLACK (not white) ─────────────────────────
    float cr = length(p);
    col *= smoothstep(0.0, 0.12, cr);      // fully black at the singularity

    // ── feedback warp trails (zoom-OUT = warp-speed effect) ──────────────────
    float trailZoom = 0.006 + 0.018 * u.bassImpact + 0.008 * u.beat;
    float trailRot  = 0.002 * u.beat;
    col = feedbackTrail(col, uv, min(u.trailDecay, 0.74), trailZoom, trailRot);

    return col; // LINEAR HDR — post chain tonemaps
}
