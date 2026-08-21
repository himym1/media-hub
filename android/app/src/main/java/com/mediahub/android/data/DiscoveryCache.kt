package com.mediahub.android.data

import android.content.Context
import com.mediahub.android.core.network.DiscoveryItem
import org.json.JSONArray
import org.json.JSONObject

/**
 * Disk-backed discovery snapshot for stale-while-revalidate on the search home.
 * Fresh within [TTL_MS] skips the initial loading spinner; pull-to-refresh still forces network.
 */
class DiscoveryCache(context: Context) {
    private val preferences = context.applicationContext.getSharedPreferences(PREFERENCES_NAME, Context.MODE_PRIVATE)

    fun loadTrending(maxAgeMs: Long = TTL_MS): List<DiscoveryItem>? {
        val savedAt = preferences.getLong(KEY_SAVED_AT, 0L)
        if (savedAt <= 0L || System.currentTimeMillis() - savedAt > maxAgeMs) {
            return null
        }
        val raw = preferences.getString(KEY_TRENDING, null) ?: return null
        return runCatching { parseItems(JSONArray(raw)) }.getOrNull()?.takeIf { it.isNotEmpty() }
    }

    fun saveTrending(items: List<DiscoveryItem>) {
        if (items.isEmpty()) return
        preferences.edit()
            .putLong(KEY_SAVED_AT, System.currentTimeMillis())
            .putString(KEY_TRENDING, encodeItems(items).toString())
            .apply()
    }

    fun clear() {
        preferences.edit().clear().apply()
    }

    private fun encodeItems(items: List<DiscoveryItem>): JSONArray {
        val array = JSONArray()
        for (item in items) {
            array.put(
                JSONObject()
                    .put("tmdbId", item.tmdbId)
                    .put("title", item.title)
                    .put("year", item.year)
                    .put("mediaType", item.mediaType)
                    .put("posterUrl", item.posterUrl ?: JSONObject.NULL),
            )
        }
        return array
    }

    private fun parseItems(array: JSONArray): List<DiscoveryItem> {
        val items = ArrayList<DiscoveryItem>(array.length())
        for (index in 0 until array.length()) {
            val value = array.getJSONObject(index)
            val tmdbId = value.optString("tmdbId").trim()
            val title = value.optString("title").trim()
            if (tmdbId.isEmpty() || title.isEmpty()) continue
            items += DiscoveryItem(
                tmdbId = tmdbId,
                title = title,
                year = value.optInt("year", 0),
                mediaType = value.optString("mediaType", "movie"),
                posterUrl = value.optString("posterUrl").takeIf { it.isNotBlank() && it != "null" },
            )
        }
        return items
    }

    companion object {
        private const val PREFERENCES_NAME = "media_hub_discovery_cache"
        private const val KEY_SAVED_AT = "saved_at"
        private const val KEY_TRENDING = "trending"
        const val TTL_MS = 30L * 60L * 1000L
    }
}
