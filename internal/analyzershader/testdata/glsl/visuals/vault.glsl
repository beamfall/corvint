// Vault — Flagship — "The Song Is Architecture"
// Raymarched generative halls: rooms keyed to a section index
// (floor(beatCount/16)), chorus sections open into taller vaults. Beat-pulsed
// emissive ribs, treble clerestory shafts, bass torchlight, fog to black.
// param0 = Grandeur ; param1 = Ornament ; param2 = Walk Speed

// room descriptor: x=halfWidth y=height z=ornament w=seed
vec4 roomAt(float idx) {
    float s = hash11(idx * 7.31 + 2.7);
    float chorus = step(2.0, mod(idx, 4.0));
    float w = mix(1.35 + s * 0.5, 2.60 + s * 1.2, chorus);
    float h = mix(1.80 + s * 0.6, 3.40 + s * 1.5, chorus);
    return vec4(w, h, mix(0.45, 1.0, chorus), s);
}

// returns (distance, ribEmissive)
vec2 mapVault(vec3 p, vec4 r, float orn) {
    float dWall = r.x - abs(p.x);
    float ceil0 = r.y - abs(p.y + (r.y - 1.4) * 0.5);
    float arch = r.y - length(vec2(p.x * (1.6 / max(r.x, 0.3)), max(p.y, 0.0)));
    float dIn = min(dWall, min(ceil0, arch));

    // columns every 3m
    vec2 cz = vec2(abs(p.x) - (r.x - 0.35), mod(p.z, 3.0) - 1.5);
    float dCol = length(cz) - (0.14 + 0.10 * r.z * orn);
    dCol = max(dCol, p.y - 1.6);

    // emissive ribs across the vault
    float rib = abs(mod(p.z, 3.0) - 1.5);
    float onRib = smoothstep(0.12 + 0.05 * orn, 0.04, rib) * step(1.0, p.y);

    float d = min(dIn, dCol);
    d = min(d, p.y + 1.0); // floor
    return vec2(d, onRib);
}

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 sp = centered(uv, u.resolution);
    float grand = mix(0.85, 1.45, u.param0);
    float orn = mix(0.4, 1.4, u.param1);
    float speed = mix(0.9, 2.6, u.param2);

    float zc = u.time * speed;
    float section = floor(u.beatCount / 16.0); // song section => room grammar
    float ridx = floor(zc / 24.0);
    float zin = mod(zc, 24.0);

    vec4 r0 = roomAt(ridx + section * 3.0);
    vec4 r1 = roomAt(ridx + 1.0 + section * 3.0);
    float blend = smoothstep(20.0, 24.0, zin); // doorway morph
    vec4 r = mix(r0, r1, blend);
    r.x *= grand; r.y *= grand;

    // camera: slow sway, gentle bob, y-up world looking +z
    vec3 ro = vec3(sin(u.time * 0.2) * 0.25, -0.10 + sin(u.time * 0.16) * 0.10, zc);
    vec3 rd = normalize(vec3(sp.x * 0.62, -sp.y * 0.62, 1.0));
    float rr = sin(u.time * 0.05) * 0.04;
    rd.xy = rot2(rr) * rd.xy;

    float beatE = (0.25 + 1.5 * exp(-u.beatPhase * 4.0)) * (0.5 + u.beat * 1.6);

    float tt = 0.0, emis = 0.0, glow = 0.0;
    for (int i = 0; i < 80; ++i) {
        vec3 pos = ro + rd * tt;
        vec2 dm = mapVault(pos, r, orn);
        glow += dm.y * beatE * 0.05 * exp(-dm.x * dm.x * 30.0);
        if (dm.x < 0.002 || tt > 60.0) { emis = dm.y; break; }
        tt += dm.x * 0.85;
        emis = dm.y;
    }

    vec3 pos = ro + rd * tt;
    // normal by central differences
    vec2 e = vec2(0.004, 0.0);
    float nx = mapVault(pos + e.xyy, r, orn).x - mapVault(pos - e.xyy, r, orn).x;
    float ny = mapVault(pos + e.yxy, r, orn).x - mapVault(pos - e.yxy, r, orn).x;
    float nz = mapVault(pos + e.yyx, r, orn).x - mapVault(pos - e.yyx, r, orn).x;
    vec3 n = normalize(vec3(nx, ny, nz) + 1e-5);

    // materials: dark stone <-> cold glass by centroid (albedo stays low so
    // the hall reads as shadow; light comes from torches + ribs)
    float matT = u.spectralCentroid;
    vec3 base = mix(vec3(0.13, 0.115, 0.10), vec3(0.075, 0.10, 0.15), matT);
    base *= 0.45 + 0.55 * fbm(pos.xy * 3.0 + pos.z);

    float diff = max(0.0, dot(n, normalize(vec3(0.3, 0.8, -0.4))));

    vec3 accentA = palVisual(0.08 + u.spectralCentroid * 0.45, u);
    vec3 accentB = palVisual(0.58 + u.spectralCentroid * 0.30, u); // chorus emissive
    float chorusNow = step(2.0, mod(ridx + section * 3.0, 4.0));
    vec3 ribCol = mix(accentA, accentB, chorusNow * 0.65);
    ribCol = accentize(ribCol, u.accent, 0.12);

    // torchlight: warm, hugs the low walls, breathes with bass
    float torchH = exp(-abs(pos.y + 0.10) * 2.4); // sconce height; floor falls dark
    // near surfaces fall to silhouette — depth reads as light further in
    float torch = exp(-tt * 0.20) * smoothstep(0.4, 3.2, tt)
                * torchH * (0.40 + u.bass * 1.3)
                * (0.85 + 0.15 * sin(u.time * 7.0 + pos.z * 2.0));
    vec3 torchCol = palRoleBass(0.06, u);

    // cool skylight from the vault crown
    float crown = smoothstep(-0.2, 1.6, pos.y) * 0.22;
    vec3 crownCol = palVisual(0.55 + 0.1 * u.spectralCentroid, u);

    vec3 col = base * (diff * 0.14
                       + torch * 2.0 * torchCol / max(luma(torchCol), 0.3)
                       + crown * crownCol * (0.4 + 0.7 * u.mid));

    // beat-pulsed emissive ribs (HDR cores lighting the vault); near ribs
    // dim to silhouette so the eye is pulled down the hall
    float ribGate = smoothstep(0.3, 2.4, tt) * (0.35 + 0.65 * exp(-tt * 0.07));
    col += ribCol * emis * min(beatE, 2.2) * 2.6 * ribGate;
    col += ribCol * glow * 1.6;

    // treble clerestory light shafts
    float shaft = smoothstep(0.2, 1.0, pos.y)
                * pow(saturate(fbm(vec2(pos.x * 2.0, pos.z * 0.5))), 2.0)
                * u.treble * 0.8;
    col += palRoleTreble(0.3, u) * shaft * exp(-tt * 0.16);

    // fog to black: the far hall vanishes into darkness
    col *= exp(-tt * 0.13);

    // impact dust motes kicked off the floor
    col += torchCol * u.bassImpact * u.bassImpact * 0.06 * fbm(sp * 8.0 + u.time);

    // onset flash breathes down the corridor
    col += ribCol * u.onset * 0.10 * exp(-tt * 0.12);

    return col; // LINEAR HDR
}
