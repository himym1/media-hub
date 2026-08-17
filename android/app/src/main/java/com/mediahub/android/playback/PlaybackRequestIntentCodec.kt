package com.mediahub.android.playback

import android.content.Intent

object PlaybackRequestIntentCodec {
    private const val EXTRA_PARENT_ID = "parent_id"
    private const val EXTRA_FILE_ID = "file_id"
    private const val EXTRA_TITLE = "title"
    private const val EXTRA_SERVER_IDENTITY = "server_identity"
    private val serverIdentityPattern = Regex("^[a-f0-9]{64}$")

    fun put(intent: Intent, request: PlaybackRequest): Intent = intent
        .putExtra(EXTRA_PARENT_ID, request.parentId)
        .putExtra(EXTRA_FILE_ID, request.fileId)
        .putExtra(EXTRA_TITLE, request.title.take(255))
        .putExtra(EXTRA_SERVER_IDENTITY, request.serverIdentity)

    fun read(intent: Intent): PlaybackRequest? {
        val parentId = intent.getStringExtra(EXTRA_PARENT_ID)?.takeIf(::numericId) ?: return null
        val fileId = intent.getStringExtra(EXTRA_FILE_ID)?.takeIf(::numericId) ?: return null
        val title = intent.getStringExtra(EXTRA_TITLE)?.trim()?.take(255).orEmpty().ifEmpty { "115 视频" }
        val serverIdentity = intent.getStringExtra(EXTRA_SERVER_IDENTITY)?.takeIf(serverIdentityPattern::matches) ?: return null
        return PlaybackRequest(parentId, fileId, title, serverIdentity)
    }

    fun valid(request: PlaybackRequest): Boolean =
        numericId(request.parentId) && numericId(request.fileId) && serverIdentityPattern.matches(request.serverIdentity)

    private fun numericId(value: String): Boolean =
        value.isNotEmpty() && value.length <= 32 && value.all(Char::isDigit)
}
