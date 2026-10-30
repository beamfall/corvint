// Laser Tunnel — Flagship — hard-vector wireframe flythrough.
// Web-heritage "laser-tunnel" upgraded. Identity: polygonal laser frames and
// corner rails rushing past — pure vector lines with HDR cores, faint
// band-lit wall fill, a rotating sweep beam. Deliberately distinct from
// Tunnel/Event Horizon: no volumetric disc, no circular equalizer.
// param0 = Speed ; param1 = Sides ; param2 = Rail Glow

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    float speed = mix(0.5, 2.4, u.param0);
    float sidesF = floor(mix(4.0, 8.0, u.param1) + 0.5);
    float railGlow = mix(0.8, 3.0, u.param2);

    // gentle flight-path wobble
    p += vec2(sin(u.time * 0.37), cos(u.time * 0.29)) * 0.07;
    p = rot2(0.06 * sin(u.time * 0.22)) * p;

    float a = atan2_(p.y, p.x) + u.time * 0.05;
    float sector = 6.2831853 / sidesF;
    float faceId = floor((a + 3.14159265) / sector);
    float am = a + 3.14159265 - (faceId + 0.5) * sector; // wedge-local angle

    // polygon-metric radius (flat walls)
    float rPoly = length(p) * cos(am);
    float rp = max(rPoly, 0.035);

    float z = 1.0 / rp;                 // depth down the tunnel
    float zt = z + u.time * speed * 2.2 + u.bassImpact * 0.35;
    float fog = exp(-z * 0.11);         // distance haze -> deep black throat

    vec3 col = vec3(0.0);

    // ---- polygonal laser frames ----
    float cell = fract(zt);
    float seg = floor(zt);
    float dz = cell - 0.5;
    float frameLine = aaLine(dz * rp * 2.2, 0.0035);         // crisp core
    float frameHalo = exp(-abs(dz) * 9.0) * 0.35;            // soft spill
    float hueF = fract(seg * 0.061 + 0.02 * u.time + 0.08 * u.spectralCentroid);
    vec3 fc = palVisual(hueF, u);
    fc = accentize(fc, u.accent, 0.12);
    // every 4th frame is a "checkpoint" that slams on the beat
    float chk = step(0.5, 1.0 - abs(mod(seg, 4.0))) * u.bassImpact;
    float frameE = (2.0 + 6.0 * chk + 1.2 * u.amplitude) * fog;
    col += fc * (frameLine + frameHalo) * frameE;
    col += peakWhite(fc, 0.5) * frameLine * chk * fog * 3.0;

    // ---- corner rails running to the vanishing point ----
    float dCorner = (abs(am) - sector * 0.5) * length(p);   // signed screen distance
    float rail = aaLine(dCorner, 0.0026);
    float railHalo = exp(-abs(dCorner) * 55.0) * 0.5;
    int rb = int(mod(faceId, sidesF));
    float railBand = band(u, (rb * 7 + 8) % 64);
    vec3 rc = palRoleTreble(fract(faceId * 0.13 + 0.03 * u.time), u);
    col += rc * (rail + railHalo) * railGlow * (1.2 + 3.0 * railBand) * fog;

    // ---- faint band-lit wall fill (gives the frames a room to live in) ----
    float wallBand = band(u, (rb * 9 + int(mod(seg, 5.0)) * 3 + 4) % 64);
    float wallShade = 0.06 + 0.16 * wallBand * wallBand;
    col += palVisual(hueF + 0.35, u) * wallShade * fog * smoothstep(0.9, 0.35, cell);

    // ---- rotating sweep laser (radar arm) ----
    float sweepA = a - u.time * 0.8;
    float sweep = exp(-(sin(sweepA * 0.5))*(sin(sweepA * 0.5)) * 260.0);
    col += palRolePeak(0.65, u) * sweep * fog * (0.5 + 2.2 * u.treble) * smoothstep(0.05, 0.4, rp);

    // ---- beat shock frame racing down the tunnel ----
    float waveZ = fract(zt * 0.25 - u.beatPhase);
    float shock = exp(-waveZ * waveZ * 90.0);
    col += palRoleBass(0.1, u) * shock * fog * u.bassImpact * 2.6;

    // vanishing-point black core + outer falloff
    col *= smoothstep(0.015, 0.06, rp);
    col *= 1.0 - smoothstep(1.15, 1.7, length(p)) * 0.9;

    // warp streaks
    col = feedbackTrail(col, uv, min(u.trailDecay, 0.86),
                        0.009 + 0.012 * u.bassImpact, 0.0025);
    return col; // LINEAR HDR
}
