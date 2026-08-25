package com.mediahub.android.app

import android.content.Context
import android.content.Intent
import android.provider.Settings
import androidx.core.content.FileProvider
import com.mediahub.android.core.network.AndroidRelease
import java.io.File

internal fun newerAndroidRelease(currentVersionCode: Int, latest: AndroidRelease): AndroidRelease? =
    latest.takeIf { it.versionCode > currentVersionCode }

internal fun androidUpdateFile(context: Context, versionCode: Int): File {
    val directory = File(context.cacheDir, "updates").apply { mkdirs() }
    return File(directory, "media-hub-$versionCode.apk")
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
    val uri = FileProvider.getUriForFile(context, "${context.packageName}.files", file)
    context.startActivity(
        Intent(Intent.ACTION_VIEW).apply {
            setDataAndType(uri, "application/vnd.android.package-archive")
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_ACTIVITY_NEW_TASK)
        },
    )
    return true
}
