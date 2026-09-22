/*
 * Copyright (C) 2016 The Android Open Source Project
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy at http://www.apache.org/licenses/LICENSE-2.0
 */
/* Test-source copy of Media3 1.10.1's production FFmpeg renderer. */
package androidx.media3.decoder.ffmpeg;

import static androidx.media3.exoplayer.audio.AudioSink.SINK_FORMAT_SUPPORTED_DIRECTLY;
import static androidx.media3.exoplayer.audio.AudioSink.SINK_FORMAT_SUPPORTED_WITH_TRANSCODING;
import static androidx.media3.exoplayer.audio.AudioSink.SINK_FORMAT_UNSUPPORTED;

import android.os.Handler;
import androidx.annotation.Nullable;
import androidx.media3.common.C;
import androidx.media3.common.Format;
import androidx.media3.common.MimeTypes;
import androidx.media3.common.audio.AudioProcessor;
import androidx.media3.common.util.TraceUtil;
import androidx.media3.common.util.Util;
import androidx.media3.decoder.CryptoConfig;
import androidx.media3.exoplayer.audio.AudioRendererEventListener;
import androidx.media3.exoplayer.audio.AudioSink;
import androidx.media3.exoplayer.audio.DecoderAudioRenderer;
import androidx.media3.exoplayer.audio.DefaultAudioSink;

public final class FfmpegAudioRenderer extends DecoderAudioRenderer<FfmpegAudioDecoder> {
    private static final int NUM_BUFFERS = 16;
    private static final int DEFAULT_INPUT_BUFFER_SIZE = 960 * 6;

    public FfmpegAudioRenderer() {
        this(null, null);
    }

    public FfmpegAudioRenderer(
            @Nullable Handler eventHandler,
            @Nullable AudioRendererEventListener eventListener,
            AudioProcessor... audioProcessors) {
        this(
                eventHandler,
                eventListener,
                new DefaultAudioSink.Builder().setAudioProcessors(audioProcessors).build());
    }

    public FfmpegAudioRenderer(
            @Nullable Handler eventHandler,
            @Nullable AudioRendererEventListener eventListener,
            AudioSink audioSink) {
        super(eventHandler, eventListener, audioSink);
    }

    @Override
    public String getName() {
        return "FfmpegAudioRenderer";
    }

    @Override
    protected @C.FormatSupport int supportsFormatInternal(Format format) {
        String mimeType = Util.castNonNull(format.sampleMimeType);
        if (!MimeTypes.isAudio(mimeType)) {
            return C.FORMAT_UNSUPPORTED_TYPE;
        }
        if (!FfmpegLibrary.supportsFormat(mimeType)
                || (!sinkSupportsFormat(format, C.ENCODING_PCM_16BIT)
                        && !sinkSupportsFormat(format, C.ENCODING_PCM_FLOAT))) {
            return C.FORMAT_UNSUPPORTED_SUBTYPE;
        }
        return format.cryptoType == C.CRYPTO_TYPE_NONE
                ? C.FORMAT_HANDLED
                : C.FORMAT_UNSUPPORTED_DRM;
    }

    @Override
    public @AdaptiveSupport int supportsMixedMimeTypeAdaptation() {
        return ADAPTIVE_NOT_SEAMLESS;
    }

    @Override
    protected FfmpegAudioDecoder createDecoder(Format format, @Nullable CryptoConfig cryptoConfig)
            throws FfmpegDecoderException {
        TraceUtil.beginSection("createFfmpegAudioDecoder");
        try {
            int inputSize =
                    format.maxInputSize != Format.NO_VALUE
                            ? format.maxInputSize
                            : DEFAULT_INPUT_BUFFER_SIZE;
            return new FfmpegAudioDecoder(
                    format, NUM_BUFFERS, NUM_BUFFERS, inputSize, shouldOutputFloat(format));
        } finally {
            TraceUtil.endSection();
        }
    }

    @Override
    protected Format getOutputFormat(FfmpegAudioDecoder decoder) {
        return new Format.Builder()
                .setSampleMimeType(MimeTypes.AUDIO_RAW)
                .setChannelCount(decoder.getChannelCount())
                .setSampleRate(decoder.getSampleRate())
                .setPcmEncoding(decoder.getEncoding())
                .build();
    }

    private boolean sinkSupportsFormat(Format inputFormat, @C.PcmEncoding int pcmEncoding) {
        return sinkSupportsFormat(
                Util.getPcmFormat(pcmEncoding, inputFormat.channelCount, inputFormat.sampleRate));
    }

    private boolean shouldOutputFloat(Format inputFormat) {
        if (!sinkSupportsFormat(inputFormat, C.ENCODING_PCM_16BIT)) {
            return true;
        }
        int support =
                getSinkFormatSupport(
                        Util.getPcmFormat(
                                C.ENCODING_PCM_FLOAT,
                                inputFormat.channelCount,
                                inputFormat.sampleRate));
        switch (support) {
            case SINK_FORMAT_SUPPORTED_DIRECTLY:
                return !MimeTypes.AUDIO_AC3.equals(inputFormat.sampleMimeType);
            case SINK_FORMAT_UNSUPPORTED:
            case SINK_FORMAT_SUPPORTED_WITH_TRANSCODING:
            default:
                return false;
        }
    }
}
