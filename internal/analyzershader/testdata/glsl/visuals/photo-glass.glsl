// Glass Album Monoliths — the photo displayed across a skyline of glass
// towers on a wet black floor: the image spans the whole equalizer skyline so
// it reads as one picture, with true mirrored reflection and floor sheen.
// Portable port of beamfall-apple-ui Shaders/PhotoVisuals.metal frag_photoglass (keep in sync).
// param0 = Towers ; param1 = Height ; param2 = Reflection

float sdRoundRect2(vec2 p, vec2 b, float r) {
    vec2 q = abs(p) - b + r;
    return length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - r;
}

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    p.y = -p.y; // y-up world (display convention: raw +p.y is down-screen)
    float count   = floor(mix(18.0, 34.0, u.param0) + 0.5);
    float hAmt    = mix(0.45, 1.15, u.param1);
    float reflAmt = mix(0.25, 1.00, u.param2);

    float floorY = -0.50;
    vec3 col = vec3(0.003, 0.005, 0.011);

    // wet-floor sheen + horizon glow along the floor line
    float floorGlow = exp(-abs(p.y - floorY) * 9.0);
    col += palVisual(0.62 + 0.06 * u.spectralCentroid, u) * floorGlow
         * (0.05 + 0.14 * u.level);

    float span = 1.62;
    for (int i = 0; i < 34; i++) {
        float fi = float(i);
        if (fi >= count) { break; }
        float x = mix(-span, span, (fi + 0.5) / count);
        float w = span / count * mix(0.62, 0.92, hash11(fi * 9.1));
        float b = bandAt(u, fi / max(count - 1.0, 1.0));
        float h = (0.28 + b * 1.00 + hash11(fi * 2.7) * 0.34) * hAmt
                + u.bassImpact * 0.07 * hash11(fi);
        vec2 halfSz = vec2(w, h * 0.5);
        vec2 cq = vec2(p.x - x, p.y - (floorY + h * 0.5));
        float sd = sdRoundRect2(cq, halfSz, 0.012);
        float rect = aaFill(sd);
        float edge = aaLine(sd, 0.004);

        // photo spans the whole skyline: continuous in x, tower height in y
        float vert = clamp((p.y - floorY) / max(h, 0.05), 0.0, 1.0);
        vec2 imgUV = vec2((x + cq.x + span) / (2.0 * span), 1.0 - vert);
        vec3 photo = max(IMG(clamp(imgUV, 0.001, 0.999)).rgb, vec3(0.0));

        // glass shading: photo carries the tower, lit toward its top
        col = mix(col, photo * (0.40 + 0.40 * b + 0.18 * vert + 0.10 * u.level), rect);
        col += edge * palVisual(fi / count + 0.05 * u.time, u) * (0.25 + 1.1 * b);

        // mirrored photo reflection under the floor, rippled and fading
        vec2 rq = vec2(cq.x, (2.0 * floorY - p.y) - (floorY + h * 0.5));
        float refl = aaFill(sdRoundRect2(rq, halfSz, 0.012));
        vec2 rimg = vec2(imgUV.x + sin(p.y * 60.0 + u.time * 1.4) * 0.004,
                         clamp(1.0 - (floorY - p.y) / max(h, 0.05), 0.0, 1.0));
        vec3 rphoto = max(IMG(clamp(rimg, 0.001, 0.999)).rgb, vec3(0.0));
        float rfade = exp(-max(floorY - p.y, 0.0) * 4.5);
        col += refl * rphoto * reflAmt * 0.30 * rfade * (0.5 + 0.6 * b);
    }

    // below-floor darkening keeps the mirror moody
    col *= 1.0 - 0.45 * (1.0 - smoothstep(floorY - 0.55, floorY, p.y));
    col *= 0.42 + 0.58 * smoothstep(1.70, 0.55, length(p));
    return col; // LINEAR HDR — post chain tonemaps
}
