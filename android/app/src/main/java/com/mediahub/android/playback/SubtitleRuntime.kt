package com.mediahub.android.playback

import kotlin.math.abs
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

internal object SubtitleRuntime {
    private val lastCue = MutableStateFlow<Long?>(null)
    val lastCueMs: StateFlow<Long?> = lastCue.asStateFlow()

    fun inspect(subtitle: LocalSubtitleFile?) {
        lastCue.value = subtitle?.let { file ->
            runCatching { lastSubtitleCueMs(file.file.readText(), file.mimeType) }.getOrNull()
        }
    }

    fun reset() {
        lastCue.value = null
    }
}

internal fun lastSubtitleCueMs(text: String, mimeType: String): Long? {
    val pattern = if (mimeType == "text/x-ssa") ASS_TIME else SRT_TIME
    var latest = -1L
    for (match in pattern.findAll(text)) {
        val parsed = parseCueTime(match.groupValues.drop(1)) ?: continue
        if (parsed > latest) latest = parsed
    }
    return latest.takeIf { it >= 0L }
}

internal fun subtitleRuntimeWarning(videoMs: Long, lastCueMs: Long?): String? {
    if (lastCueMs == null || videoMs < 60_000L || lastCueMs < 30_000L) return null
    val delta = abs(videoMs - lastCueMs)
    if (delta < 45_000L) return null
    val minutes = ((delta + 30_000L) / 60_000L).coerceAtLeast(1L)
    return "字幕时长和片源大约差 $minutes 分钟，可能不是这一版"
}

private fun parseCueTime(parts: List<String>): Long? {
    val hours = parts.getOrNull(0)?.toLongOrNull() ?: return null
    val minutes = parts.getOrNull(1)?.toLongOrNull() ?: return null
    val seconds = parts.getOrNull(2)?.toLongOrNull() ?: return null
    val fraction = parts.getOrNull(3).orEmpty().padEnd(3, '0').take(3).toLongOrNull() ?: 0L
    return ((hours * 60 + minutes) * 60 + seconds) * 1000 + fraction
}

private val SRT_TIME = Regex("""(\d{1,2}):(\d{2}):(\d{2})[,.](\d{1,3})""")
private val ASS_TIME = Regex("""(\d{1,2}):(\d{2}):(\d{2})\.(\d{1,2})""")
