package com.mediahub.android.core.image

import android.graphics.BitmapFactory
import android.util.LruCache
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import com.mediahub.android.core.auth.SecureSessionStore
import com.mediahub.android.core.network.MediaHubHttpClient
import java.net.URLEncoder
import kotlinx.coroutines.sync.Semaphore
import kotlinx.coroutines.sync.withPermit

interface PosterLoader {
    fun cached(itemId: String): ImageBitmap?
    suspend fun load(itemId: String): ImageBitmap?
}

class EmbyPosterLoader(
    private val http: MediaHubHttpClient,
    private val sessionStore: SecureSessionStore,
) : PosterLoader {
    private val cache = LruCache<String, ImageBitmap>(20)
    private val requests = Semaphore(4)
    private val idPattern = Regex("^[A-Za-z0-9_-]{1,128}$")

    override fun cached(itemId: String): ImageBitmap? = cache.get(itemId)

    override suspend fun load(itemId: String): ImageBitmap? {
        if (!idPattern.matches(itemId)) return null
        cache.get(itemId)?.let { return it }
        return requests.withPermit {
            cache.get(itemId)?.let { return@withPermit it }
            val token = sessionStore.load() ?: return@withPermit null
            val data = runCatching {
                http.requestBytes(
                    path = "/api/v1/integrations/emby/items/${URLEncoder.encode(itemId, Charsets.UTF_8.name())}/primary-image",
                    token = token,
                )
            }.getOrNull() ?: return@withPermit null
            val bitmap = BitmapFactory.decodeByteArray(data, 0, data.size)?.asImageBitmap() ?: return@withPermit null
            cache.put(itemId, bitmap)
            bitmap
        }
    }
}
