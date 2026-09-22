/*
 * Copyright (C) 2016 The Android Open Source Project
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy at http://www.apache.org/licenses/LICENSE-2.0
 */
/* Test-source copy of the target-codec path from Media3 1.10.1's FFmpeg decoder. */
package androidx.media3.decoder.ffmpeg;

import androidx.annotation.Nullable;
import androidx.media3.common.C;
import androidx.media3.common.Format;
import androidx.media3.common.util.Util;
import androidx.media3.decoder.DecoderInputBuffer;
import androidx.media3.decoder.SimpleDecoder;
import androidx.media3.decoder.SimpleDecoderOutputBuffer;
import java.nio.ByteBuffer;

final class FfmpegAudioDecoder
        extends SimpleDecoder<DecoderInputBuffer, SimpleDecoderOutputBuffer, FfmpegDecoderException> {
    private static final int INITIAL_OUTPUT_BUFFER_SIZE_16BIT = 65535;
    private static final int AUDIO_DECODER_ERROR_INVALID_DATA = -1;
    private static final int AUDIO_DECODER_ERROR_OTHER = -2;

    private final String codecName;
    private final @C.PcmEncoding int encoding;
    private int outputBufferSize;
    private long nativeContext;
    private boolean hasOutputFormat;
    private volatile int channelCount;
    private volatile int sampleRate;

    FfmpegAudioDecoder(
            Format format,
            int numInputBuffers,
            int numOutputBuffers,
            int initialInputBufferSize,
            boolean outputFloat)
            throws FfmpegDecoderException {
        super(new DecoderInputBuffer[numInputBuffers], new SimpleDecoderOutputBuffer[numOutputBuffers]);
        codecName = Util.castNonNull(FfmpegLibrary.getCodecName(Util.castNonNull(format.sampleMimeType)));
        encoding = outputFloat ? C.ENCODING_PCM_FLOAT : C.ENCODING_PCM_16BIT;
        outputBufferSize = outputFloat ? INITIAL_OUTPUT_BUFFER_SIZE_16BIT * 2 : INITIAL_OUTPUT_BUFFER_SIZE_16BIT;
        nativeContext =
                ffmpegInitialize(codecName, null, outputFloat, format.sampleRate, format.channelCount);
        if (nativeContext == 0) {
            throw new FfmpegDecoderException("Initialization failed");
        }
        setInitialInputBufferSize(initialInputBufferSize);
    }

    @Override
    public String getName() {
        return "ffmpeg" + FfmpegLibrary.getVersion() + "-" + codecName;
    }

    @Override
    protected DecoderInputBuffer createInputBuffer() {
        return new DecoderInputBuffer(
                DecoderInputBuffer.BUFFER_REPLACEMENT_MODE_DIRECT,
                FfmpegLibrary.getInputBufferPaddingSize());
    }

    @Override
    protected SimpleDecoderOutputBuffer createOutputBuffer() {
        return new SimpleDecoderOutputBuffer(this::releaseOutputBuffer);
    }

    @Override
    protected FfmpegDecoderException createUnexpectedDecodeException(Throwable error) {
        return new FfmpegDecoderException("Unexpected decode error", error);
    }

    @Override
    @Nullable
    protected FfmpegDecoderException decode(
            DecoderInputBuffer inputBuffer, SimpleDecoderOutputBuffer outputBuffer, boolean reset) {
        if (reset && (nativeContext = ffmpegReset(nativeContext, null)) == 0) {
            return new FfmpegDecoderException("Error resetting");
        }
        ByteBuffer inputData = Util.castNonNull(inputBuffer.data);
        ByteBuffer outputData = outputBuffer.init(inputBuffer.timeUs, outputBufferSize);
        int result =
                ffmpegDecode(
                        nativeContext,
                        inputData,
                        inputData.limit(),
                        outputBuffer,
                        outputData,
                        outputBufferSize);
        if (result == AUDIO_DECODER_ERROR_OTHER) {
            return new FfmpegDecoderException("Error decoding");
        }
        if (result == AUDIO_DECODER_ERROR_INVALID_DATA || result == 0) {
            outputBuffer.shouldBeSkipped = true;
            return null;
        }
        if (!hasOutputFormat) {
            channelCount = ffmpegGetChannelCount(nativeContext);
            sampleRate = ffmpegGetSampleRate(nativeContext);
            hasOutputFormat = true;
        }
        outputData = Util.castNonNull(outputBuffer.data);
        outputData.position(0);
        outputData.limit(result);
        return null;
    }

    @SuppressWarnings("unused")
    private ByteBuffer growOutputBuffer(
            SimpleDecoderOutputBuffer outputBuffer, int requiredSize) {
        outputBufferSize = requiredSize;
        return outputBuffer.grow(requiredSize);
    }

    @Override
    public void release() {
        super.release();
        ffmpegRelease(nativeContext);
        nativeContext = 0;
    }

    int getChannelCount() {
        return channelCount;
    }

    int getSampleRate() {
        return sampleRate;
    }

    @C.PcmEncoding
    int getEncoding() {
        return encoding;
    }

    private native long ffmpegInitialize(
            String codecName,
            @Nullable byte[] extraData,
            boolean outputFloat,
            int rawSampleRate,
            int rawChannelCount);

    private native int ffmpegDecode(
            long context,
            ByteBuffer inputData,
            int inputSize,
            SimpleDecoderOutputBuffer decoderOutputBuffer,
            ByteBuffer outputData,
            int outputSize);

    private native int ffmpegGetChannelCount(long context);

    private native int ffmpegGetSampleRate(long context);

    private native long ffmpegReset(long context, @Nullable byte[] extraData);

    private native void ffmpegRelease(long context);
}
