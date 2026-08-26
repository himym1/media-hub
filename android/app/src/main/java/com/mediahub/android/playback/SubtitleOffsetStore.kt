package com.mediahub.android.playback

import android.content.Context

internal class SubtitleOffsetStore(
    private val values: MutableMap<String, Long>,
) {
    fun get(mediaId: String): Long = offsetKey(mediaId)?.let { values[it] } ?: 0L

    fun put(mediaId: String, offsetMs: Long) {
        val key = offsetKey(mediaId) ?: return
        val clamped = offsetMs.coerceIn(SubtitleTiming.MinMs, SubtitleTiming.MaxMs)
        if (clamped == 0L) {
            values.remove(key)
        } else {
            values[key] = clamped
        }
    }

    fun clearItem(itemId: String) {
        if (!itemId.matches(ITEM_ID)) return
        val suffix = ":emby:$itemId"
        values.keys.filter { it.endsWith(suffix) }.toList().forEach(values::remove)
    }
}

internal fun subtitleOffsetStore(context: Context): SubtitleOffsetStore {
    val prefs = context.applicationContext.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
    val values = object : MutableMap<String, Long> by mutableMapOf() {
        override fun get(key: String): Long? =
            if (prefs.contains(key)) prefs.getLong(key, 0L) else null

        override fun put(key: String, value: Long): Long? {
            val previous = get(key)
            prefs.edit().putLong(key, value).apply()
            return previous
        }

        override fun remove(key: String): Long? {
            val previous = get(key)
            prefs.edit().remove(key).apply()
            return previous
        }

        override val keys: MutableSet<String>
            get() = prefs.all.keys.toMutableSet()
    }
    return SubtitleOffsetStore(values)
}

private fun offsetKey(mediaId: String): String? =
    mediaId.takeIf { it.matches(MEDIA_ID) && it.length <= 256 }

private val ITEM_ID = Regex("^[A-Za-z0-9_-]{1,128}$")
private val MEDIA_ID = Regex("^[A-Za-z0-9_.:-]{8,256}$")
private const val PREFS_NAME = "media-hub-subtitle-offsets"
