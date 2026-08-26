package com.mediahub.android.app

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import java.io.File

class AndroidUpdateNotifier(context: Context?) {
    private val app = context?.applicationContext

    fun showProgress(versionName: String, downloadedBytes: Long, totalBytes: Long) {
        val context = app ?: return
        val percent = if (totalBytes > 0L) ((downloadedBytes * 100) / totalBytes).toInt().coerceIn(0, 100) else 0
        notify(
            context,
            NotificationCompat.Builder(context, CHANNEL_ID)
                .setSmallIcon(android.R.drawable.stat_sys_download)
                .setContentTitle("正在下载 Media Hub $versionName")
                .setContentText(formatUpdateProgress(downloadedBytes, totalBytes))
                .setOngoing(true)
                .setOnlyAlertOnce(true)
                .setProgress(100, percent, totalBytes <= 0L)
                .setCategory(NotificationCompat.CATEGORY_PROGRESS),
        )
    }

    fun showReady(versionName: String, file: File) {
        val context = app ?: return
        val install = PendingIntent.getActivity(
            context,
            REQUEST_INSTALL,
            androidUpdateInstallIntent(context, file),
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
        notify(
            context,
            NotificationCompat.Builder(context, CHANNEL_ID)
                .setSmallIcon(android.R.drawable.stat_sys_download_done)
                .setContentTitle("Media Hub $versionName 已下载")
                .setContentText("点击安装")
                .setAutoCancel(true)
                .setOngoing(false)
                .setProgress(0, 0, false)
                .setContentIntent(install)
                .setCategory(NotificationCompat.CATEGORY_STATUS),
        )
    }

    fun showFailed(message: String) {
        val context = app ?: return
        notify(
            context,
            NotificationCompat.Builder(context, CHANNEL_ID)
                .setSmallIcon(android.R.drawable.stat_notify_error)
                .setContentTitle("更新下载失败")
                .setContentText(message)
                .setAutoCancel(true)
                .setOngoing(false)
                .setProgress(0, 0, false)
                .setCategory(NotificationCompat.CATEGORY_ERROR),
        )
    }

    fun cancel() {
        val context = app ?: return
        NotificationManagerCompat.from(context).cancel(NOTIFICATION_ID)
    }

    private fun notify(context: Context, builder: NotificationCompat.Builder) {
        ensureChannel(context)
        val manager = NotificationManagerCompat.from(context)
        if (!manager.areNotificationsEnabled()) return
        if (Build.VERSION.SDK_INT >= 33 &&
            ContextCompat.checkSelfPermission(context, android.Manifest.permission.POST_NOTIFICATIONS) !=
            PackageManager.PERMISSION_GRANTED
        ) {
            return
        }
        try {
            manager.notify(NOTIFICATION_ID, builder.build())
        } catch (_: SecurityException) {
            return
        }
    }

    private fun ensureChannel(context: Context) {
        val manager = context.getSystemService(NotificationManager::class.java) ?: return
        if (manager.getNotificationChannel(CHANNEL_ID) != null) return
        manager.createNotificationChannel(
            NotificationChannel(CHANNEL_ID, "应用更新", NotificationManager.IMPORTANCE_DEFAULT).apply {
                description = "下载 Media Hub 新版本时显示进度"
                setShowBadge(false)
            },
        )
    }

    companion object {
        val None = AndroidUpdateNotifier(null)
        private const val CHANNEL_ID = "media-hub-updates"
        private const val NOTIFICATION_ID = 20016
        private const val REQUEST_INSTALL = 20017
    }
}
