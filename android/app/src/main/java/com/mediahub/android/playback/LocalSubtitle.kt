package com.mediahub.android.playback

import java.io.File

data class LocalSubtitleFile(
    val file: File,
    val mimeType: String,
)

internal fun localSubtitleMimeType(contentType: String, fileName: String): String {
    val type = contentType.substringBefore(';').trim().lowercase()
    val name = fileName.lowercase()
    return when {
        type == "text/x-ssa" || name.endsWith(".ass") || name.endsWith(".ssa") -> "text/x-ssa"
        else -> "application/x-subrip"
    }
}

internal fun localSubtitleExtension(mimeType: String, fileName: String): String {
    val name = fileName.lowercase()
    return when {
        mimeType == "text/x-ssa" || name.endsWith(".ass") -> ".ass"
        name.endsWith(".ssa") -> ".ssa"
        else -> ".srt"
    }
}

internal fun writeLocalSubtitleCache(
    cacheDir: File,
    itemId: String,
    bytes: ByteArray,
    contentType: String,
    fileName: String,
): LocalSubtitleFile? {
    if (!itemId.matches(Regex("^[A-Za-z0-9_-]{1,128}$")) || bytes.isEmpty()) return null
    val mimeType = localSubtitleMimeType(contentType, fileName)
    val directory = File(cacheDir, "local-subtitles").apply { mkdirs() }
    val file = File(directory, itemId + localSubtitleExtension(mimeType, fileName))
    file.writeBytes(bytes)
    return LocalSubtitleFile(file, mimeType)
}

internal fun clearLocalSubtitleCache(cacheDir: File, itemId: String) {
    if (!itemId.matches(Regex("^[A-Za-z0-9_-]{1,128}$"))) return
    val directory = File(cacheDir, "local-subtitles")
    listOf(".ass", ".ssa", ".srt").forEach { ext ->
        File(directory, itemId + ext).delete()
    }
}
