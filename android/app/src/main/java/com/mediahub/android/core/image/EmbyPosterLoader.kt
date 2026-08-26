package com.mediahub.android.core.image

import android.content.Context
import android.graphics.BitmapFactory
import android.util.LruCache
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import com.mediahub.android.core.auth.SecureSessionStore
import com.mediahub.android.core.network.MediaHubHttpClient
import java.io.File
import java.net.URLEncoder
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.sync.Semaphore
import kotlinx.coroutines.sync.withPermit
import kotlinx.coroutines.withContext

interface PosterLoader {
    fun cached(itemId: String): ImageBitmap?
    suspend fun load(itemId: String): ImageBitmap?
}

class EmbyPosterLoader(
    context: Context,
    private val http: MediaHubHttpClient,
    private val sessionStore: SecureSessionStore,
) : PosterLoader {
    private val cache = LruCache<String, ImageBitmap>(48)
    private val requests = Semaphore(4)
    private val idPattern = Regex("^[A-Za-z0-9_-]{1,128}$")
    private val diskDir = File(context.applicationContext.cacheDir, "emby-posters").apply { mkdirs() }

    override fun cached(itemId: String): ImageBitmap? = cache.get(itemId)

    fun evict(itemId: String) {
        if (!idPattern.matches(itemId)) return
        cache.remove(itemId)
        diskFile(itemId).delete()
    }

    override suspend fun load(itemId: String): ImageBitmap? {
        if (!idPattern.matches(itemId)) return null
        cache.get(itemId)?.let { return it }
        val fromDisk = withContext(Dispatchers.IO) { readDisk(itemId) }
        if (fromDisk != null) {
            cache.put(itemId, fromDisk)
            return fromDisk
        }
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
            withContext(Dispatchers.IO) { writeDisk(itemId, data) }
            bitmap
        }
    }

    private fun diskFile(itemId: String): File = File(diskDir, "$itemId.bin")

    private fun readDisk(itemId: String): ImageBitmap? {
        val file = diskFile(itemId)
        if (!file.isFile || file.length() == 0L) return null
        if (System.currentTimeMillis() - file.lastModified() > DISK_TTL_MS) {
            file.delete()
            return null
        }
        val data = runCatching { file.readBytes() }.getOrNull() ?: return null
        return BitmapFactory.decodeByteArray(data, 0, data.size)?.asImageBitmap()
    }

    private fun writeDisk(itemId: String, data: ByteArray) {
        runCatching {
            val target = diskFile(itemId)
            val temp = File(diskDir, "$itemId.tmp")
            temp.writeBytes(data)
            if (!temp.renameTo(target)) {
                target.writeBytes(data)
                temp.delete()
            }
            trimDisk()
        }
    }

    private fun trimDisk() {
        val files = diskDir.listFiles()?.filter { it.isFile && it.name.endsWith(".bin") }.orEmpty()
        if (files.size <= MAX_DISK_FILES) return
        files.sortedBy { it.lastModified() }
            .take(files.size - MAX_DISK_FILES)
            .forEach { it.delete() }
    }

    companion object {
        private const val DISK_TTL_MS = 7L * 24L * 60L * 60L * 1000L
        private const val MAX_DISK_FILES = 200
    }
}
