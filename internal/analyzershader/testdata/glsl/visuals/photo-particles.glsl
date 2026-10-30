// Chromatic Portrait Particles — the photo rendered as a living halftone
// mosaic: dot size follows photo luminance so the image reads at a glance,
// bands make cells breathe, bass bursts scatter them outward from center.
// Portable port of beamfall-apple-ui Shaders/PhotoVisuals.metal frag_photoparticles (keep in sync).
// param0 = Particle Size ; param1 = Scatter ; param2 = Color

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    float sizeAmt  = mix(1.55, 0.70, u.param0); // bigger param -> chunkier dots
    float scatter  = mix(0.30, 1.40, u.param1);
    float colorAmt = u.param2;

    vec2 gridN = vec2(96.0, 54.0) * sizeAmt;
    vec2 g = uv * gridN;
    vec2 grid = floor(g);
    vec2 cell = fract(g) - 0.5;
    float h = hash21(grid);
    float freq = bandAt(u, h);

    // scatter: jitter per beat + radial burst from center on bass hits
    vec2 cellUV = (grid + 0.5) / gridN;
    vec2 dirC = cellUV - 0.5;
    vec2 jitter = (hash22(grid + floor(u.beatCount)) - 0.5)
                * (0.010 + freq * 0.030) * scatter
                + dirC * u.bassImpact * 0.045 * scatter * (0.5 + h);

    vec3 photo = max(IMG(clamp(cellUV + jitter, 0.0, 1.0)).rgb, vec3(0.0));
    float lum = luma(photo);

    // halftone: dot radius follows luminance so the photo reads at distance
    float dotR = 0.16 + lum * 0.34 + freq * 0.10;
    float dotMask = smoothstep(dotR, dotR * 0.55, length(cell));

    vec3 tint = palVisual(h * 0.18 + lum * 0.5 + u.time * 0.012, u);
    vec3 dotCol = mix(photo * (0.85 + freq * 0.8), tint * (0.35 + lum * 0.6),
                      colorAmt * 0.45);
    vec3 col = dotCol * dotMask * (0.55 + 0.45 * freq + 0.30 * u.onset);

    // dim continuous photo underneath anchors legibility between dots
    vec3 under = max(IMG(clamp(uv, 0.0, 1.0)).rgb, vec3(0.0));
    col += under * 0.08;

    // treble sparkle rides the brightest dots
    col += vec3(1.6, 1.55, 1.45) * dotMask * smoothstep(0.72, 0.95, lum)
         * u.treble * 0.55;

    col *= 0.35 + 0.65 * smoothstep(1.60, 0.45, length(p));
    col = feedbackTrail(col, uv, min(u.trailDecay, 0.62), 0.002, 0.003 * u.onset);
    return col; // LINEAR HDR — post chain tonemaps
}
