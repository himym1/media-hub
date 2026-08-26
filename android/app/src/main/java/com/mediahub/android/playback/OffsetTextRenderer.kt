package com.mediahub.android.playback

import android.os.Looper
import androidx.media3.common.util.UnstableApi
import androidx.media3.exoplayer.Renderer
import androidx.media3.exoplayer.text.TextOutput
import androidx.media3.exoplayer.text.TextRenderer

@UnstableApi
internal class OffsetTextRenderer(
    output: TextOutput,
    outputLooper: Looper?,
    private val delegate: TextRenderer = TextRenderer(output, outputLooper),
) : Renderer by delegate {
    override fun render(positionUs: Long, elapsedRealtimeUs: Long) {
        delegate.render(positionUs - SubtitleTiming.offsetUs, elapsedRealtimeUs)
    }
}
