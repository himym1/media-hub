package com.mediahub.android.playback

import java.util.Locale
import kotlin.math.abs

internal object SubtitleTiming {
    const val StepMs = 100L
    const val MinMs = -30_000L
    const val MaxMs = 30_000L

    @Volatile
    var offsetMs: Long = 0L
        private set

    val offsetUs: Long
        get() = offsetMs * 1_000L

    fun set(offsetMs: Long) {
        this.offsetMs = offsetMs.coerceIn(MinMs, MaxMs)
    }

    fun shift(deltaMs: Long) {
        set(offsetMs + deltaMs)
    }

    fun reset() {
        offsetMs = 0L
    }
}

internal fun formatSubtitleOffset(offsetMs: Long): String {
    if (offsetMs == 0L) return "已对齐"
    val seconds = abs(offsetMs) / 1000.0
    val value = String.format(Locale.ROOT, "%.1f", seconds)
        .trimEnd('0')
        .trimEnd('.')
    return if (offsetMs > 0L) "延后 $value 秒" else "提前 $value 秒"
}
