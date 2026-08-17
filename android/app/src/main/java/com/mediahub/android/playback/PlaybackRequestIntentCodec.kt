package com.mediahub.android.playback

import android.content.Intent
import java.net.URI

object PlaybackRequestIntentCodec {
    private const val EXTRA_TARGET_TYPE = "target_type"
    private const val EXTRA_PARENT_ID = "parent_id"
    private const val EXTRA_FILE_ID = "file_id"
    private const val EXTRA_EMBY_ITEM_ID = "emby_item_id"
    private const val EXTRA_TITLE = "title"
    private const val EXTRA_SERVER_IDENTITY = "server_identity"
    private const val EXTRA_FALLBACK_APP_URL = "fallback_app_url"
    private const val EXTRA_FALLBACK_WEB_URL = "fallback_web_url"
    private const val TARGET_DRIVE115 = "drive115"
    private const val TARGET_EMBY = "emby"
    private val serverIdentityPattern = Regex("^[a-f0-9]{64}$")
    private val embyIdPattern = Regex("^[A-Za-z0-9_-]{1,128}$")

    fun put(intent: Intent, request: PlaybackRequest): Intent {
        intent.putExtra(EXTRA_TITLE, request.title.take(255))
            .putExtra(EXTRA_SERVER_IDENTITY, request.serverIdentity)
        request.fallback?.let { fallback ->
            intent.putExtra(EXTRA_FALLBACK_APP_URL, fallback.appUrl)
                .putExtra(EXTRA_FALLBACK_WEB_URL, fallback.webUrl)
        }
        when (val target = request.target) {
            is Drive115Target -> intent
                .putExtra(EXTRA_TARGET_TYPE, TARGET_DRIVE115)
                .putExtra(EXTRA_PARENT_ID, target.parentId)
                .putExtra(EXTRA_FILE_ID, target.fileId)
            is EmbyItemTarget -> intent
                .putExtra(EXTRA_TARGET_TYPE, TARGET_EMBY)
                .putExtra(EXTRA_EMBY_ITEM_ID, target.itemId)
        }
        return intent
    }

    fun read(intent: Intent): PlaybackRequest? {
        val target = when (intent.getStringExtra(EXTRA_TARGET_TYPE)) {
            TARGET_DRIVE115 -> {
                val parentId = intent.getStringExtra(EXTRA_PARENT_ID)?.takeIf(::numericId) ?: return null
                val fileId = intent.getStringExtra(EXTRA_FILE_ID)?.takeIf(::numericId) ?: return null
                Drive115Target(parentId, fileId)
            }
            TARGET_EMBY -> {
                val itemId = intent.getStringExtra(EXTRA_EMBY_ITEM_ID)?.takeIf(embyIdPattern::matches) ?: return null
                EmbyItemTarget(itemId)
            }
            else -> return null
        }
        val title = intent.getStringExtra(EXTRA_TITLE)?.trim()?.take(255).orEmpty().ifEmpty { "视频" }
        val serverIdentity = intent.getStringExtra(EXTRA_SERVER_IDENTITY)?.takeIf(serverIdentityPattern::matches) ?: return null
        val fallback = playbackFallback(
            intent.getStringExtra(EXTRA_FALLBACK_APP_URL),
            intent.getStringExtra(EXTRA_FALLBACK_WEB_URL),
        )
        return PlaybackRequest(target, title, serverIdentity, fallback)
    }

    fun valid(request: PlaybackRequest): Boolean =
        serverIdentityPattern.matches(request.serverIdentity) &&
            (request.fallback == null || playbackFallback(request.fallback.appUrl, request.fallback.webUrl) == request.fallback) &&
            when (val target = request.target) {
                is Drive115Target -> numericId(target.parentId) && numericId(target.fileId)
                is EmbyItemTarget -> embyIdPattern.matches(target.itemId)
            }

    private fun playbackFallback(appUrl: String?, webUrl: String?): PlaybackFallback? {
        val app = appUrl?.trim()?.takeIf(::validEmbyAppUrl)
        val web = webUrl?.trim()?.takeIf(::validWebUrl)
        return if (app == null && web == null) null else PlaybackFallback(app, web)
    }

    private fun validEmbyAppUrl(value: String): Boolean {
        val parsed = runCatching { URI(value) }.getOrNull() ?: return false
        val segments = parsed.path.orEmpty().split('/').filter(String::isNotEmpty)
        return parsed.scheme.equals("emby", true) && parsed.host.equals("items", true) &&
            parsed.userInfo == null && parsed.port == -1 && parsed.query == null && parsed.fragment == null &&
            segments.size == 2 && segments.all(embyIdPattern::matches)
    }

    private fun validWebUrl(value: String): Boolean {
        val parsed = runCatching { URI(value) }.getOrNull() ?: return false
        return parsed.scheme in setOf("http", "https") && !parsed.host.isNullOrBlank() && parsed.userInfo == null
    }

    private fun numericId(value: String): Boolean =
        value.isNotEmpty() && value.length <= 32 && value.all(Char::isDigit)
}
