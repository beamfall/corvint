// Vinyl — luminous record grooves with beat needle sparks.
// Portable body is authoritative for improvements (hand-sync to Vinyl.metal).
// A spinning record under a studio spot: dark platter, spectrum-lit groove
// rings, a static anisotropic light sheen the grooves spin beneath,
// colored label + spindle, playing-position hot groove, needle arm + spark.
// param0 Groove Count ; param1 Spin ; param2 Needle

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p0 = motionCentered(uv, u);
    float spin = u.time * mix(0.06, 0.45, u.param1);

    // slight perspective ellipse: squash is applied in SCREEN space (static),
    // then the groove texture spins within the disc plane.
    vec2 pd = vec2(p0.x, p0.y / 0.86);   // disc-plane coords, static ellipse
    vec2 pr = rot2(spin) * pd;           // spinning texture coords
    float r = length(pd);
    float a = atan2_(pr.y, pr.x);        // spinning angle (texture)
    float sa = atan2_(pd.y, pd.x);       // static angle (lighting)
    float ang = a / 6.2831853 + 0.5;

    float grooveCount = mix(20.0, 52.0, u.param0);
    float needle = mix(0.3, 1.3, u.param2);

    float discR = 0.92;
    float labelR = 0.30;
    float disc = 1.0 - smoothstep(discR - 0.008, discR + 0.008, r);
    float label = 1.0 - smoothstep(labelR - 0.006, labelR + 0.006, r);
    float grooveZone = disc * (1.0 - label) * smoothstep(labelR + 0.01, labelR + 0.05, r);

    vec3 col = vec3(0.0);

    // ---- studio spot falling on the platter (static) ----
    vec2 lightDir = normalize(vec2(-0.55, -0.85));
    float spot = 0.5 + 0.5 * dot(normalize(pd + 1e-5), lightDir);
    float spotFall = exp(-r * r * 0.55) * (0.5 + 0.5 * spot);

    // ---- dark platter base ----
    vec3 platter = vec3(0.014, 0.016, 0.022) * (0.6 + 1.2 * spotFall);
    col += platter * disc;

    // ---- groove micro-rings ----
    float wobble = 0.012 * sin(a * 3.0 + u.time * 0.7) * (0.5 + u.mid);
    float gPhase = (r + wobble) * grooveCount * 6.2831853;
    float micro = 0.5 + 0.5 * sin(gPhase);
    float grooveLine = pow(micro, 6.0); // thin bright ridge per groove

    // spectrum lights the rings: radius maps to frequency (outer = low)
    float fBin = saturate(1.0 - (r - labelR) / max(discR - labelR, 1e-3));
    float energy = bandAt(u, fBin * 0.85);
    float angEnergy = bandAtWrapped(u, ang + 0.15 * u.beatPhase);

    vec3 grooveCol = palVisual(fBin * 0.40 + 0.10 * u.spectralCentroid + 0.01 * u.time, u);
    grooveCol = accentize(grooveCol, u.accent, 0.12);
    col += grooveCol * grooveLine * grooveZone
         * (0.08 + 1.1 * energy * (0.4 + 0.6 * angEnergy)) * (0.5 + 0.7 * u.level);

    // ---- anisotropic sheen: static light streak the grooves spin beneath ----
    float sheenA = -0.62 + 0.35 * sin(u.time * 0.23) + u.beatPhase * 0.30;
    float lobe = cos(2.0 * (sa - sheenA));
    float sheen = pow(max(lobe, 0.0), 9.0);
    vec3 sheenCol = palVisual(0.58 + 0.12 * u.spectralCentroid, u);
    sheenCol = peakWhite(sheenCol, 0.22 + 0.18 * u.treble);
    col += sheenCol * sheen * grooveZone * (0.30 + 0.85 * grooveLine)
         * (0.45 + 1.2 * u.amplitude);

    // ---- playing-position hot groove tracking the beat ----
    float playR = 0.58 + 0.05 * sin(u.beatPhase * 6.2831853);
    float hotGroove = aaLine(r - playR, 0.005);
    float hotHalo = exp(-abs(r - playR) * 55.0) * 0.5;
    col += palRoleTrace(ang + 0.12 * u.spectralCentroid, u) * (hotGroove + hotHalo)
         * grooveZone * (0.8 + 2.2 * u.bassImpact) * (0.5 + 0.8 * angEnergy);

    // ---- label: saturated color with printed arcs + spindle ----
    vec3 labelCol = palVisual(0.08 + 0.06 * sin(u.time * 0.11), u);
    labelCol = accentize(labelCol, u.accent, 0.18);
    float print = 0.75 + 0.25 * sin(a * 2.0 + r * 40.0);
    float ringPrint = aaLine(r - labelR * 0.82, 0.004) + aaLine(r - labelR * 0.55, 0.003);
    col += labelCol * label * (0.30 + 0.55 * spotFall) * print * (0.8 + 0.5 * u.bass);
    col += peakWhite(labelCol, 0.15) * ringPrint * label * (0.4 + 0.8 * u.mid);
    float spindle = 1.0 - smoothstep(0.030, 0.042, r);
    col = mix(col, vec3(0.004), spindle);
    col += vec3(1.6, 1.5, 1.3) * exp(-r * r * 900.0) * (0.15 + 0.35 * u.beat); // spindle glint

    // ---- outer rim edge highlight ----
    float rimEdge = aaLine(r - discR, 0.006);
    col += palVisual(0.72 + 0.1 * u.spectralCentroid, u) * rimEdge
         * (0.35 + 0.65 * spot) * (0.5 + 0.8 * u.amplitude);

    // ---- needle arm: pivot near the corner, stylus on the playing groove ----
    vec2 sty = vec2(cos(-0.50), sin(-0.50) * 0.86) * playR; // screen pos on groove
    vec2 piv = vec2(1.16, -0.86);
    vec2 ab = sty - piv;
    float tt = clamp(dot(p0 - piv, ab) / max(dot(ab, ab), 1e-4), 0.0, 1.0);
    float armD = length(p0 - (piv + ab * tt));
    col += palRolePeak(0.70, u) * (aaLine(armD, 0.004) + exp(-armD * 110.0) * 0.35) * needle * 0.6;
    float cd2 = dot(p0 - sty, p0 - sty);
    float contact = min(2.5, 0.00045 / (cd2 + 0.00030));
    col += palRoleBass(0.08, u) * contact * needle * (0.5 + 1.5 * u.flux + 1.6 * u.bassImpact);

    // faint ambient behind the record so it doesn't float in void
    float bg = (1.0 - disc) * exp(-r * r * 0.7);
    col += palRoleFog(0.62, u) * bg * (0.020 + 0.05 * u.level);

    col = feedbackTrail(col, uv,
                        min(u.trailDecay, 0.60),
                        0.0008 + 0.002 * u.bassImpact,
                        0.0);
    return col; // LINEAR HDR — post chain tonemaps
}
