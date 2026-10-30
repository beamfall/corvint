// Liquid Photo Bloom — the photo rests under a living liquid surface:
// wave-refracted, lit by specular skylight and caustics, with neon ink veins
// tracing the music. Photo stays full-frame and recognizable.
// Portable port of beamfall-apple-ui Shaders/PhotoVisuals.metal frag_photoliquid (keep in sync).
// param0 = Flow ; param1 = Veins ; param2 = Bloom

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    float flowAmt  = mix(0.45, 1.40, u.param0);
    float veinAmt  = mix(0.40, 1.50, u.param1);
    float bloomAmt = mix(0.45, 1.35, u.param2);

    float t = u.time;
    float rr = length(p);

    // liquid surface heightfield + bass ripple ring from the center
    vec2 q = p * 1.6 + vec2(t * 0.060 * flowAmt, -t * 0.045 * flowAmt);
    float h0 = fbm(q);
    float ripple = sin(rr * 22.0 - t * 3.0) * exp(-rr * 2.2) * (0.12 + 0.9 * u.bassImpact);
    float e = 0.09;
    vec2 grad = vec2(fbm(q + vec2(e, 0.0)) - h0, fbm(q + vec2(0.0, e)) - h0) / e;
    grad += (p / max(rr, 1e-3)) * ripple * 0.6;

    // refract the photo through the surface
    vec2 refUV = uv + grad * 0.045 * flowAmt * (0.55 + 0.6 * u.amplitude);
    vec3 photo = max(IMG(clamp(refUV, 0.0, 1.0)).rgb, vec3(0.0));

    // underwater exposure — photo carries the frame
    vec3 col = photo * (0.34 + 0.22 * u.level + 0.10 * bloomAmt);

    // specular skylight glints off the wave crests
    vec3 n = normalize(vec3(-grad * 0.55, 1.0));
    vec3 l = normalize(vec3(0.35, 0.55, 0.75));
    float spec = pow(saturate(dot(n, l)), 22.0);
    col += vec3(1.9, 1.85, 1.7) * spec * spec * (0.20 + 0.55 * u.treble) * bloomAmt;

    // caustic web focuses light back into the photo
    float caustic = smoothstep(0.58, 0.98, fbmRidged(q * 1.2 + vec2(0.0, t * 0.08)));
    col += photo * caustic * (0.35 + 0.8 * u.mid) * 0.55;

    // glowing ink veins (identity), band-lit
    float veins = 0.0;
    for (int i = 0; i < 4; i++) {
        float fi = float(i);
        float n2 = fbmRidged(p * (1.6 + fi * 0.5) + vec2(t * (0.06 + fi * 0.012), -t * 0.04));
        veins += smoothstep(0.80, 0.96, n2) * (0.30 + bandAt(u, fi / 4.0) * 0.9);
    }
    col += palVisual(luma(photo) * 0.6 + u.spectralCentroid * 0.1, u)
         * veins * veinAmt * (0.35 + u.onset * 1.2);

    // soft central bloom lifts the subject
    col += photo * exp(-rr * rr * 1.4) * bloomAmt * 0.16;

    col *= 0.32 + 0.68 * smoothstep(1.65, 0.50, rr);
    col = feedbackTrail(col, uv, min(u.trailDecay, 0.55), 0.003, 0.0015);
    return col; // LINEAR HDR — post chain tonemaps
}
