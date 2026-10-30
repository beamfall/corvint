// Liquid Optics — Lite — artwork seen through rippling glass.
// Web-heritage "liquid-optics" upgraded: fbm water surface refracts IMG with
// chromatic split over a deep dark pool; a sharp HDR caustic web dances on
// mids, bass ripple bursts expand from the beat, treble glints crest the waves.
// param0 = Refraction ; param1 = Caustics ; param2 = Flow

float waterH(vec2 q, float t) {
    float h1 = fbm(q + vec2(t * 0.35, -t * 0.22));
    float h2 = fbm(q * 1.7 - vec2(t * 0.27, t * 0.31) + 3.1);
    return h1 * 0.62 + h2 * 0.38;
}

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = centered(uv, u.resolution);
    float flow = mix(0.16, 0.9, u.param2);
    float t = u.time * flow;

    float refr = mix(0.010, 0.055, u.param0);
    float causticAmt = mix(0.5, 2.6, u.param1);

    // ---- water surface normal (finite differences of fbm height) ----
    vec2 w = p * 2.4;
    float e = 0.035;
    float h0 = waterH(w, t);
    float gx = (waterH(w + vec2(e, 0.0), t) - h0) / e;
    float gy = (waterH(w + vec2(0.0, e), t) - h0) / e;

    // bass ripple burst expanding from center each beat
    float r = length(p);
    float rippleR = fract(u.beatPhase) * 1.7;
    float rippleEnv = exp(-abs(r - rippleR) * 6.5) * u.bassImpact;
    float ripple = sin((r - rippleR) * 30.0) * rippleEnv;
    vec2 rdir = p / max(r, 1e-3);
    vec2 n = vec2(gx, gy) * (1.0 + 0.6 * u.mid) + rdir * ripple * 3.0;

    // ---- refracted artwork with chromatic split ----
    vec2 off = n * refr * (1.0 + 0.9 * u.bassImpact);
    vec2 base = uv + off;
    float split = 0.0045 + 0.004 * u.treble;
    vec3 art;
    art.r = IMG(base + n * split).r;
    art.g = IMG(base).g;
    art.b = IMG(base - n * split).b;

    // deep pool: artwork sinks into dark water — squared keeps blacks black,
    // troughs go nearly lightless, crests catch what light gets through
    float depthShade = mix(0.14, 1.0, smoothstep(0.12, 0.90, h0));
    vec3 deepTint = vec3(0.50, 0.66, 0.88); // cold water absorbs red first
    vec3 col = art * art * deepTint * depthShade * (0.60 + 0.70 * u.level);

    // ---- HDR caustic web: thin bright filaments, black between ----
    float ca = fbmRidged(w * 0.9 + n * 0.35 + vec2(0.0, t * 0.25));
    float web = pow(saturate(ca * 1.12 - 0.12) / 1.0, 6.0);
    float hue = h0 * 0.35 + 0.08 * u.spectralCentroid + 0.02 * u.time;
    vec3 causticCol = palVisual(hue, u);
    causticCol = accentize(causticCol, u.accent, 0.18);
    col += causticCol * web * causticAmt * (0.5 + 1.1 * u.mid + 1.6 * u.bassImpact);

    // secondary coarser web, counter-drifting (depth parallax)
    float ca2 = fbmRidged(w * 0.55 - n * 0.22 + vec2(t * 0.19, 4.7));
    float web2 = pow(saturate(ca2 * 1.10 - 0.10), 6.0);
    col += palVisual(hue + 0.18, u) * web2 * causticAmt * (0.20 + 0.55 * u.mid);

    // sharp glints only where filaments cross wave crests (HDR sparks)
    float crest = smoothstep(0.60, 0.82, h0);
    float glint = pow(saturate(1.0 - abs(gx * 0.6 + gy * 0.4)), 22.0) * crest * web;
    col += peakWhite(palVisual(hue + 0.3, u), 0.5) * glint
         * (0.3 + 1.6 * u.treble) * causticAmt * 0.7;

    // ripple ring itself glows
    col += palRoleBass(0.06, u) * rippleEnv * rippleEnv * 1.0;

    // pool vignette: dark glass rim closing to black
    col *= 1.0 - smoothstep(0.62, 1.35, r) * 0.96;

    col = feedbackTrail(col, uv, min(u.trailDecay, 0.40), 0.0015, 0.0);
    return col; // LINEAR HDR
}
