package com.mediahub.android.app

import android.content.Context
import android.content.Intent
import android.provider.Settings
import androidx.core.content.FileProvider
import com.mediahub.android.core.network.AndroidRelease
import java.io.File
import java.util.Locale

internal fun newerAndroidRelease(currentVersionCode: Int, latest: AndroidRelease): AndroidRelease? =
    latest.takeIf { it.versionCode > currentVersionCode }

internal fun androidReleaseBlurb(notes: String, versionName: String): String? {
    val trimmed = notes.trim()
    if (trimmed.isEmpty() || trimmed == "Media Hub $versionName") return null
    return trimmed
}

internal fun androidUpdateCardSummary(release: AndroidRelease): String {
    val blurb = androidReleaseBlurb(release.notes, release.versionName)
    return if (blurb == null) "发现 ${release.versionName}" else "发现 ${release.versionName} · $blurb"
}

internal fun androidUpdatePromptBody(prompt: AppUpdatePrompt): String {
    prompt.errorMessage?.takeIf { it.isNotBlank() }?.let { return it }
    val release = prompt.release
    return when {
        prompt.downloading -> formatUpdateProgress(prompt.downloadedBytes, release.sizeBytes)
        prompt.downloadedPath != null -> "下载完成，安装后即可使用"
        else -> "安装包 ${formatUpdateMegabytes(release.sizeBytes)}"
    }
}

internal fun androidUpdateFile(context: Context, versionCode: Int): File {
    val directory = File(context.cacheDir, "updates").apply { mkdirs() }
    return File(directory, "media-hub-$versionCode.apk")
}

internal fun updateProgressFraction(downloadedBytes: Long, totalBytes: Long): Float {
    if (totalBytes <= 0L) return 0f
    return (downloadedBytes.toFloat() / totalBytes.toFloat()).coerceIn(0f, 1f)
}

internal fun formatUpdateProgress(downloadedBytes: Long, totalBytes: Long): String {
    if (totalBytes <= 0L) return "正在下载"
    val percent = ((downloadedBytes * 100) / totalBytes).toInt().coerceIn(0, 100)
    return "${formatUpdateMegabytes(downloadedBytes)} / ${formatUpdateMegabytes(totalBytes)} · $percent%"
}

internal fun formatUpdateMegabytes(bytes: Long): String =
    String.format(Locale.US, "%.1f MB", bytes / (1024.0 * 1024.0))

internal fun androidUpdateInstallIntent(context: Context, file: File): Intent {
    val uri = FileProvider.getUriForFile(context, "${context.packageName}.files", file)
    return Intent(Intent.ACTION_VIEW).apply {
        setDataAndType(uri, "application/vnd.android.package-archive")
        clipData = android.content.ClipData.newRawUri("update", uri)
        addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_ACTIVITY_NEW_TASK)
    }
}

internal fun installAndroidUpdate(context: Context, file: File): Boolean {
    if (!context.packageManager.canRequestPackageInstalls()) {
        context.startActivity(
            Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES).apply {
                data = android.net.Uri.parse("package:${context.packageName}")
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            },
        )
        return false
    }
    context.startActivity(androidUpdateInstallIntent(context, file))
    return true
}
