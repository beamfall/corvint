#include "VisualShared.h"

// Shared post-processing chain: bright-pass -> separable gaussian blur -> bloom
// composite (with chromatic aberration, vignette, grain, filmic tonemap).
// All passes reuse `fullscreen_vertex` (Common.metal) and `VOut`.

struct PostUniforms {
    float2 resolution;
    float2 direction;      // blur direction in pixels (e.g. (1,0) or (0,1))
    float  bloomThreshold;
    float  bloomStrength;
    float  chroma;
    float  vignette;
    float  grain;
    float  time;
    float  rotationAngle;
    float4 presentation; // x=intensity, y=max luma cap, z=safe-zone dim, w=black floor
    float4 safeRect;     // normalized rect reserved for overlaid UI/content
    float  renderScale;
    float  antiAlias;
    float  reserved0;
    float  reserved1;
};

struct VisualTransitionUniforms {
    float2 resolution;
    float  progress;
    float  phase;
    float  reducedMotion;
};

// Extract HDR highlights above a soft threshold.
fragment float4 post_bright(VOut in [[stage_in]],
                            texture2d<float> src [[texture(0)]],
                            constant PostUniforms& p [[buffer(0)]],
                            sampler s [[sampler(0)]]) {
    float3 c = suppressGreenCast(src.sample(s, in.uv).rgb);
    float l = luma(c);
    float k = max(0.0, l - p.bloomThreshold);
    float3 bright = c * (k / max(l, 1e-4));
    // soft knee boost so strong cores bloom hard
    return float4(bright * (1.0 + k), 1.0);
}

// 9-tap separable gaussian (call twice: horizontal then vertical).
fragment float4 post_blur(VOut in [[stage_in]],
                          texture2d<float> src [[texture(0)]],
                          constant PostUniforms& p [[buffer(0)]],
                          sampler s [[sampler(0)]]) {
    float2 texel = p.direction / max(p.resolution, float2(1.0));
    const float w0 = 0.227027;
    const float w[4] = { 0.194594, 0.121622, 0.054054, 0.016216 };
    float3 c = src.sample(s, in.uv).rgb * w0;
    for (int i = 0; i < 4; ++i) {
        float2 off = texel * (float(i) + 1.0);
        c += src.sample(s, in.uv + off).rgb * w[i];
        c += src.sample(s, in.uv - off).rgb * w[i];
    }
    return float4(suppressGreenCast(c), 1.0);
}

fragment float4 post_visual_transition(VOut in [[stage_in]],
                                       texture2d<float> fromScene [[texture(0)]],
                                       texture2d<float> toScene [[texture(1)]],
                                       constant VisualTransitionUniforms& p [[buffer(0)]],
                                       sampler s [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 q = uv - 0.5;
    float aspect = p.resolution.x / max(p.resolution.y, 1.0);
    q.x *= aspect;

    float t = clamp(p.progress, 0.0, 1.0);
    float eased = t * t * (3.0 - 2.0 * t);
    float motion = 1.0 - clamp(p.reducedMotion, 0.0, 1.0);
    float energy = 1.0 - abs(eased * 2.0 - 1.0);
    float ripple = sin(length(q) * 18.0 - p.phase * 6.2831853) * 0.018 * energy * motion;
    float2 dir = normalize(q + 1e-4);

    float2 fromUV = clamp(uv - dir * ripple * (1.0 - eased), float2(0.0), float2(1.0));
    float2 toUV = clamp(uv + dir * ripple * eased, float2(0.0), float2(1.0));
    float3 fromCol = suppressGreenCast(fromScene.sample(s, fromUV).rgb);
    float3 toCol = suppressGreenCast(toScene.sample(s, toUV).rgb);

    float radial = smoothstep(-0.20, 0.80, eased - length(q) * 0.42);
    float scan = smoothstep(-0.08, 0.08, uv.x - (1.0 - eased));
    float wipe = mix(eased, max(radial, scan * 0.72), 0.38 * motion);
    float edge = smoothstep(0.020, 0.0, abs(wipe - eased)) * motion;

    float3 col = suppressGreenCast(mix(fromCol, toCol, wipe));
    col += (fromCol + toCol) * edge * 0.025;
    return float4(suppressGreenCast(col), 1.0);
}

// Final composite: scene + bloom, chromatic aberration, tonemap, vignette, grain.
fragment float4 post_composite(VOut in [[stage_in]],
                               texture2d<float> scene [[texture(0)]],
                               texture2d<float> bloom [[texture(1)]],
                               constant PostUniforms& p [[buffer(0)]],
                               sampler s [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 dir = uv - 0.5;
    float2 sampleUV = uv;

    if (abs(p.rotationAngle) > 0.0001) {
        float aspect = p.resolution.x / max(p.resolution.y, 1.0);
        float2 q = dir;
        q.x *= aspect;
        q = rot2(p.rotationAngle) * q;
        q.x /= aspect;
        sampleUV = q + 0.5;
    }
    float edge = min(min(sampleUV.x, 1.0 - sampleUV.x),
                     min(sampleUV.y, 1.0 - sampleUV.y));
    float inside = smoothstep(-0.08, 0.025, edge);
    float2 sUV = clamp(sampleUV, float2(0.0), float2(1.0));
    float2 sampleDir = sUV - 0.5;

    // chromatic aberration grows toward the edges
    float ca = p.chroma * (0.4 + dot(sampleDir, sampleDir) * 2.0);
    float aa = clamp(p.antiAlias, 0.0, 1.0);
    float2 aaTexel = (0.45 + aa * 1.35) / max(p.resolution, float2(1.0));
    float2 uvR = clamp(sUV + sampleDir * ca, float2(0.0), float2(1.0));
    float2 uvB = clamp(sUV - sampleDir * ca, float2(0.0), float2(1.0));

    float3 center;
    center.r = scene.sample(s, uvR).r;
    center.g = scene.sample(s, sUV).g;
    center.b = scene.sample(s, uvB).b;

    float3 filtered = center;
    if (aa > 0.001) {
        float3 taps = float3(0.0);
        float2 offsets[4] = {
            float2( aaTexel.x, 0.0),
            float2(-aaTexel.x, 0.0),
            float2(0.0,  aaTexel.y),
            float2(0.0, -aaTexel.y)
        };
        for (int i = 0; i < 4; i++) {
            float2 o = offsets[i];
            taps.r += scene.sample(s, clamp(uvR + o, float2(0.0), float2(1.0))).r;
            taps.g += scene.sample(s, clamp(sUV + o, float2(0.0), float2(1.0))).g;
            taps.b += scene.sample(s, clamp(uvB + o, float2(0.0), float2(1.0))).b;
        }
        filtered = mix(center, center * 0.50 + taps * 0.125, aa);
    }

    float3 col = suppressGreenCast(filtered);
    col *= inside;

    // additive bloom (bloom buffer is lower-res; linear sampler upscales)
    if (p.bloomStrength > 0.0001) {
        col += suppressGreenCast(bloom.sample(s, sUV).rgb) * p.bloomStrength * inside;
    }

    // filmic tonemap to LDR
    float3 outc = tonemap(col);

    float visualIntensity = clamp(p.presentation.x, 0.0, 1.5);
    float maxLuma = clamp(p.presentation.y, 0.05, 1.0);
    float safeDim = clamp(p.presentation.z, 0.0, 1.0);
    float blackFloor = clamp(p.presentation.w, 0.0, 0.08);

    outc *= visualIntensity;

    // gentle contrast/saturation lift, keep blacks black
    float l = luma(outc);
    outc = mix(float3(l), outc, 1.12);                 // saturation
    outc = clamp((outc - 0.5) * 1.06 + 0.5, 0.0, 1.0); // contrast
    outc = suppressGreenCast(outc);

    float safe = safeRectMask(uv, p.safeRect, 0.035);
    outc *= mix(1.0, 1.0 - safeDim, safe);

    float postL = max(luma(outc), 1e-4);
    if (postL > maxLuma) {
        outc *= maxLuma / postL;
    }
    outc = max(outc, float3(blackFloor));

    // vignette
    float vig = 1.0 - p.vignette * dot(dir, dir) * 1.6;
    outc *= clamp(vig, 0.0, 1.0);

    // film grain
    float g = (hash21(uv * p.resolution + p.time * 60.0) - 0.5) * p.grain;
    g *= smoothstep(0.03, 0.25, l);
    outc = suppressGreenCast(clamp(outc + g, 0.0, 1.0));

    return float4(outc, 1.0);
}
