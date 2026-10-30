// Spectral Photo Curtain — the photo woven into a swaying silk curtain of
// vertical panels. Folds catch the light, bands lift panels, a beat-driven
// sweep of light glides across; a dim parallax layer hangs behind the seams.
// Portable port of beamfall-apple-ui Shaders/PhotoVisuals.metal frag_photocurtain (keep in sync).
// param0 = Columns ; param1 = Sway ; param2 = Depth

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    float cols     = floor(mix(16.0, 42.0, u.param0));
    float swayAmt  = mix(0.45, 1.50, u.param1);
    float depthAmt = mix(0.30, 1.00, u.param2);

    float t = u.time;
    vec3 col = vec3(0.0);
    float frontAlpha = 0.0;
    vec3 frontCol = vec3(0.0);

    for (int layer = 0; layer < 2; layer++) {
        float lf = float(layer); // 0 = back curtain, 1 = front curtain
        float sway = sin(t * (0.35 + 0.20 * lf) + uv.y * 3.0 + lf * 1.7)
                   * 0.022 * swayAmt;
        float xs = fract(uv.x + sway + lf * 0.012);
        float cN = max(cols - lf * 0.0, 8.0);
        float strip = floor(xs * cN);
        float sfr = fract(xs * cN);
        float sh = hash11(strip * 7.31 + lf * 41.0);
        float b = bandAt(u, (strip + 0.5) / cN);

        // per-strip vertical slide — the curtain ripples with the bands
        float slide = sin(t * (0.5 + sh * 0.8) + strip * 0.7 + lf * 2.3)
                    * (0.015 + 0.055 * b) * swayAmt;
        vec2 imgUV = vec2(xs, clamp(uv.y + slide, 0.0, 1.0));
        vec3 photo = max(IMG(clamp(imgUV, 0.0, 1.0)).rgb, vec3(0.0));

        // cylindrical fold shading across each panel
        float fold = 0.42 + 0.58 * sin(sfr * 3.14159);
        fold = pow(fold, 1.25) * (0.86 + 0.28 * sh);

        // beat sweep of light gliding across the curtain (graceful when quiet)
        float sd = abs(fract(xs - u.beatPhase + 0.5) - 0.5);
        float sweep = exp(-sd * sd * 60.0) * (0.20 + 0.80 * u.beat);

        vec3 lit = photo * fold * (0.30 + 0.42 * b + 0.30 * sweep + 0.12 * u.level);
        // spectral rim on the panel edges
        float seamGlow = (1.0 - smoothstep(0.0, 0.14, sfr))
                       + (1.0 - smoothstep(1.0, 0.86, sfr));
        lit += palVisual(xs + u.spectralCentroid * 0.10, u)
             * seamGlow * (0.05 + 0.32 * b + 0.22 * sweep);

        if (layer == 0) {
            // back curtain: dim, cool, hangs deeper
            col = lit * 0.28 * depthAmt + vec3(0.003, 0.005, 0.010);
        } else {
            frontCol = lit;
            // narrow seam gap lets the back layer peek through
            float gap = smoothstep(0.985, 0.94, sfr) * smoothstep(0.015, 0.06, sfr);
            frontAlpha = mix(1.0, gap, depthAmt * 0.9);
        }
    }
    col = mix(col, frontCol, clamp(frontAlpha, 0.0, 1.0));

    // gentle top light + floor falloff, cinematic vignette
    col *= 0.85 + 0.30 * (1.0 - smoothstep(-0.9, 1.1, p.y)); // light falls from the rod above
    col *= 0.32 + 0.68 * smoothstep(1.70, 0.55, length(p));
    col = feedbackTrail(col, uv, min(u.trailDecay, 0.50), 0.0015, 0.0008);
    return col; // LINEAR HDR — post chain tonemaps
}
