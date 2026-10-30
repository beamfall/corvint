// Photo Prism — the photo seen through a spinning prism. Full-frame chromatic
// dispersion wedges surround a clean magnified circular waveform lens, so the
// photo stays the hero while the spectrum ring plays the music.
// Portable port of beamfall-apple-ui Shaders/PhotoVisuals.metal frag_photoprism (keep in sync).
// param0 = Shard Spin ; param1 = Refraction ; param2 = Glow

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    float r = length(p);
    float rS = max(r, 1e-3);
    float a = atan2_(p.y, p.x);

    float spinAmt = mix(0.02, 0.22, u.param0);
    float refr    = mix(0.35, 1.30, u.param1);
    float glow    = mix(0.55, 1.60, u.param2);

    // spectrum-driven lens radius (identity: circular waveform lens)
    float t = fract(a / 6.2831853 + 0.5 + u.time * 0.02);
    float b = bandAt(u, t);
    float ringR = 0.46 + b * 0.11 + u.bassImpact * 0.04;

    // ---- prism wedges (outside the lens) ----
    float spin = u.time * spinAmt + u.beatCount * 0.015;
    float segs = 10.0;
    float wedgeF = (a + spin * 6.2831853) * segs / 6.2831853;
    float wid = floor(wedgeF);
    float wfr = fract(wedgeF);
    float wh = hash11(fract(wid / segs) * 37.7 + 5.1);

    // per-wedge dispersion strength; bands + beats push the split harder
    float disp = refr * (0.010 + 0.045 * wh) * (0.55 + 1.3 * b + 0.9 * u.bassImpact);
    vec2 dir = p / rS;
    vec2 baseUV = uv + dir * 0.010 * sin(u.time * 0.30 + wh * 6.2831853);
    float cr = IMG(clamp(baseUV + dir * disp * 1.45, 0.0, 1.0)).r;
    float cg = IMG(clamp(baseUV + dir * disp * 0.85, 0.0, 1.0)).g;
    float cb = IMG(clamp(baseUV + dir * disp * 0.30, 0.0, 1.0)).b;
    vec3 outPhoto = max(vec3(cr, cg, cb), vec3(0.0));
    // facet lighting so each shard reads as cut glass
    float facet = 0.62 + 0.34 * wh + 0.20 * sin(wfr * 3.14159);
    outPhoto *= facet;

    // ---- clean magnified photo inside the lens ----
    vec2 lp = p / max(ringR, 1e-3);
    float lr2 = dot(lp, lp);
    vec2 lensUV = (uv - 0.5) * (0.80 + 0.03 * sin(u.time * 0.05));
    lensUV *= 1.0 + 0.14 * lr2; // gentle barrel bulge
    vec3 inPhoto = max(IMG(clamp(lensUV + 0.5, 0.0, 1.0)).rgb, vec3(0.0));
    inPhoto *= 0.86 + 0.30 * exp(-lr2 * 1.8); // lens hot-center
    inPhoto *= 1.0 - 0.42 * smoothstep(0.55, 1.0, lr2); // glass-edge falloff

    float inside = smoothstep(ringR + 0.012, ringR - 0.012, r);
    vec3 col = mix(outPhoto * (0.30 + 0.28 * b + 0.10 * u.level),
                   inPhoto * 0.62, inside);

    // rainbow fringe seams at wedge boundaries
    float seam = aaLineCyclic(wedgeF, 0.020);
    col += palVisual(t + wh * 0.3, u) * seam * (1.0 - inside) * glow * (0.10 + 0.7 * b);

    // ---- HDR spectrum ring ----
    float ring = aaLine(r - ringR, 0.006 + 0.009 * b);
    vec3 ringCol = peakWhite(palVisual(t, u), 0.20 + 0.50 * u.onset);
    col += ringCol * ring * glow * (0.7 + 1.8 * b + 1.1 * u.onset);
    // dispersive halo hugging the ring
    col += palVisual(t + 0.15, u) * exp(-abs(r - ringR) * 30.0) * glow * (0.06 + 0.30 * b);

    // cinematic vignette, frame stays full
    col *= 0.30 + 0.70 * smoothstep(1.60, 0.45, r);
    col = feedbackTrail(col, uv, min(u.trailDecay, 0.55), 0.002 + 0.004 * u.bassImpact, 0.001);
    return col; // LINEAR HDR — post chain tonemaps
}
