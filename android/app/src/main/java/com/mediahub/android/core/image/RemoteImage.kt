package com.mediahub.android.core.image

import android.graphics.BitmapFactory
import android.util.LruCache
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import com.composables.icons.lucide.Film
import com.composables.icons.lucide.Lucide
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import java.net.HttpURLConnection
import java.net.URI
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.sync.Semaphore
import kotlinx.coroutines.sync.withPermit
import kotlinx.coroutines.withContext

object RemoteImageCache {
    private val cache = LruCache<String, ImageBitmap>(48)
    private val requests = Semaphore(6)

    fun cached(url: String): ImageBitmap? = cache.get(url)

    suspend fun load(url: String): ImageBitmap? {
        if (!url.startsWith("https://")) return null
        cache.get(url)?.let { return it }
        return requests.withPermit {
            cache.get(url)?.let { return@withPermit it }
            val bytes = withContext(Dispatchers.IO) {
                runCatching { fetchHttps(url) }.getOrNull()
            } ?: return@withPermit null
            val bitmap = BitmapFactory.decodeByteArray(bytes, 0, bytes.size)?.asImageBitmap() ?: return@withPermit null
            cache.put(url, bitmap)
            bitmap
        }
    }

    private fun fetchHttps(url: String): ByteArray? {
        val uri = URI(url)
        if (uri.scheme != "https") return null
        val connection = uri.toURL().openConnection() as HttpURLConnection
        connection.connectTimeout = 4_000
        connection.readTimeout = 4_000
        connection.instanceFollowRedirects = true
        return try {
            if (connection.responseCode !in 200..299) null
            else connection.inputStream.use { it.readBytes() }
        } finally {
            connection.disconnect()
        }
    }
}

@Composable
fun RemotePoster(
    url: String?,
    contentDescription: String,
    modifier: Modifier = Modifier,
) {
    val image by produceState(initialValue = url?.let(RemoteImageCache::cached), url) {
        value = url?.let { RemoteImageCache.load(it) }
    }
    if (image != null) {
        Image(
            bitmap = requireNotNull(image),
            contentDescription = contentDescription,
            modifier = modifier,
            contentScale = ContentScale.Crop,
        )
    } else {
        Box(
            modifier = modifier.background(MediaHubColors.SurfaceInput),
            contentAlignment = Alignment.Center,
        ) {
            MediaHubIcon(
                imageVector = Lucide.Film,
                contentDescription = null,
                tint = MediaHubColors.TextFaint,
                modifier = Modifier.fillMaxSize(0.34f),
            )
        }
    }
}
