// Aurora Photo Veil — the photo glows softly full-frame behind translucent
// aurora ribbons that carry brighter slivers of the same image; ribbons sway
// with the bands and shimmer on onsets. Calm, cinematic, photo-first.
// Portable port of beamfall-apple-ui Shaders/PhotoVisuals.metal frag_photoveil (keep in sync).
// param0 = Ribbons ; param1 = Sway ; param2 = Texture

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    float ribbons = floor(mix(4.0, 7.0, u.param0) + 0.5);
    float swayAmt = mix(0.50, 1.60, u.param1);
    float texMix  = mix(0.25, 0.80, u.param2);

    // full-frame photo backdrop, gently veiled (slightly desaturated + dim)
    vec3 base = max(IMG(clamp(uv, 0.0, 1.0)).rgb, vec3(0.0));
    vec3 col = mix(vec3(luma(base)), base, 0.75) * (0.16 + 0.10 * u.level);

    for (int i = 0; i < 7; i++) {
        float fi = float(i);
        if (fi >= ribbons) { break; }
        float x = p.x;
        float y = p.y - mix(-0.45, 0.35, fi / max(ribbons - 1.0, 1.0));
        float freq = bandAt(u, fi / 6.0);
        float curve = sin(x * (1.2 + fi * 0.18) + u.time * (0.10 + fi * 0.018))
                    * (0.10 + freq * 0.18) * swayAmt
                    + sin(x * 2.7 - u.time * 0.08 + fi) * 0.04 * swayAmt;
        float dy = y - curve;
        float ribbon = aaLine(dy, 0.018 + freq * 0.022);
        float body = smoothstep(0.26 + freq * 0.22, 0.0, abs(dy));

        // the ribbon lifts a brighter, sharper strip of the same photo
        vec2 imgUV = vec2(uv.x, clamp(uv.y + dy * 0.35, 0.0, 1.0));
        vec3 photo = max(IMG(clamp(imgUV, 0.0, 1.0)).rgb, vec3(0.0));
        vec3 c = mix(palVisual(fi / 7.0 + u.time * 0.01, u), photo * 1.15,
                     texMix + luma(photo) * 0.20);
        float shimmer = 0.85 + 0.30 * sin(x * 9.0 + u.time * 2.0 + fi * 2.3) * u.treble;
        col += c * (ribbon * (0.55 + freq * 2.2 + u.onset * 0.8)
                  + body * (0.16 + 0.22 * freq)) * shimmer
             * smoothstep(1.6, 0.30, abs(x));
    }

    // low mist and a slow breathing vignette
    col += palVisual(0.55, u) * exp(-abs(p.y - 0.75) * 8.0) * (0.06 + 0.12 * u.bass);
    col *= 0.40 + 0.60 * smoothstep(1.75, 0.50, length(p));
    col = feedbackTrail(col, uv, min(u.trailDecay, 0.62), 0.002, 0.003 * u.bassImpact);
    return col; // LINEAR HDR — post chain tonemaps
}
