// Hologram gallery drift — photo prints floating in a dark gallery. Cards are
// depth-sorted with true occlusion, soft drop shadows and a moving sheen; the
// card on the beat lifts and glows. Photo fills stay truthful and bright.
// Portable port of beamfall-apple-ui Shaders/PhotoVisuals.metal frag_photogallery (keep in sync).
// param0 = Cards ; param1 = Drift ; param2 = Edges

float sdRoundRect(vec2 p, vec2 b, float r) {
    vec2 q = abs(p) - b + r;
    return length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - r;
}

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    float cardCount = floor(mix(4.0, 7.0, u.param0) + 0.5);
    float driftAmt  = mix(0.40, 1.50, u.param1);
    float edgeAmt   = mix(0.40, 1.60, u.param2);
    float t = u.time;

    // gallery backdrop: deep blue-black with a soft floor wash
    vec3 col = vec3(0.004, 0.006, 0.013);
    col += palVisual(0.58, u) * exp(-abs(p.y - 0.58) * 9.0) * (0.10 + 0.18 * u.level);
    col += palVisual(0.72, u) * (0.020 * (1.0 - uv.y));

    for (int i = 0; i < 7; i++) {
        float fi = float(i);
        if (fi >= cardCount) { break; }
        // back-to-front: i=0 deepest, drawn first, occluded by nearer cards
        float depth = mix(1.50, 0.62, fi / 6.0);
        vec2 h2 = hash22(vec2(fi * 3.7 + 1.3, 9.1));
        vec2 center = vec2(mix(-1.02, 1.02, h2.x), mix(-0.38, 0.38, h2.y));
        center += vec2(sin(t * (0.070 + fi * 0.013) + fi * 2.1),
                       cos(t * (0.055 + fi * 0.011) + fi)) * vec2(0.15, 0.08) * driftAmt;

        vec2 q = (p - center) / depth;
        q = rot2((h2.x - 0.5) * 0.30 + sin(t * 0.05 + fi) * 0.06 * driftAmt) * q;

        vec2 half_ = vec2(0.21, 0.14);
        float sd = sdRoundRect(q, half_, 0.020);
        float fill = aaFill(sd);
        float edge = aaLine(sd, 0.004);

        float b = bandAt(u, fi / 6.0);
        // the beat picks one card to lift
        float pick = 1.0 - min(1.0, abs(mod(u.beatCount, cardCount) - fi));
        float lift = pick * (0.25 + 0.75 * u.beat);

        // soft drop shadow cast down-right onto whatever is behind
        float shSd = sdRoundRect(q - vec2(0.030, 0.040), half_, 0.020);
        float shadow = exp(-max(shSd, 0.0) * 26.0) * (1.0 - fill);
        col *= 1.0 - shadow * 0.45;

        // photo print
        vec2 imgUV = q / (half_ * 2.0) + 0.5;
        vec3 photo = max(IMG(clamp(imgUV, 0.001, 0.999)).rgb, vec3(0.0));
        float fog = mix(1.0, 0.42, smoothstep(0.62, 1.50, depth));
        // gliding sheen across the print
        float s = q.x * 0.8 + q.y * 0.6;
        float sheen = exp(-((s - sin(t * 0.4 + fi * 1.7) * 0.20) * 9.0)*((s - sin(t * 0.4 + fi * 1.7) * 0.20) * 9.0));
        vec3 cardCol = photo * (0.48 + 0.30 * b + 0.32 * lift) * fog
                     + vec3(0.9, 0.88, 0.85) * sheen * 0.14 * fog;

        col = mix(col, cardCol, fill);
        col += palVisual(fi / 7.0 + 0.04 * t, u) * edge * edgeAmt
             * (0.45 + 1.5 * b + 1.6 * lift + 0.6 * u.onset) * fog;
    }

    col *= 0.45 + 0.55 * smoothstep(1.75, 0.55, length(p));
    return col; // LINEAR HDR — post chain tonemaps
}
