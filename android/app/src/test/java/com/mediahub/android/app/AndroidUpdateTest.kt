package com.mediahub.android.app

import com.mediahub.android.core.network.AndroidRelease
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class AndroidUpdateTest {
    @Test
    fun promptsOnlyWhenServerVersionIsNewer() {
        val latest = AndroidRelease(
            versionCode = 20010,
            versionName = "0.20.10",
            minimumSupportedVersionCode = 1,
            sha256 = "abc",
            sizeBytes = 12,
            publishedAt = "2026-08-25T04:00:00Z",
            notes = "Media Hub 0.20.10",
            downloadPath = "/api/v1/client/android/releases/20010/apk",
        )
        assertEquals(latest, newerAndroidRelease(20009, latest))
        assertNull(newerAndroidRelease(20010, latest))
        assertNull(newerAndroidRelease(20011, latest))
    }

    @Test
    fun hidesBoilerplateNotesFromUpdateCopy() {
        val latest = AndroidRelease(
            versionCode = 21007,
            versionName = "0.21.7",
            minimumSupportedVersionCode = 1,
            sha256 = "abc",
            sizeBytes = 12L * 1024 * 1024,
            publishedAt = "2026-09-15T04:00:00Z",
            notes = "Media Hub 0.21.7",
            downloadPath = "/api/v1/client/android/releases/21007/apk",
        )
        assertNull(androidReleaseBlurb(latest.notes, latest.versionName))
        assertEquals("发现 0.21.7", androidUpdateCardSummary(latest))
        assertEquals("发现 0.21.7 · 修复安装后仍提示更新", androidUpdateCardSummary(latest.copy(notes = "修复安装后仍提示更新")))
        assertEquals(
            "安装包 12.0 MB",
            androidUpdatePromptBody(AppUpdatePrompt(latest)),
        )
        assertEquals(
            "下载完成，安装后即可使用",
            androidUpdatePromptBody(AppUpdatePrompt(latest, downloadedPath = "/tmp/update.apk")),
        )
    }

    @Test
    fun formatsDownloadProgress() {
        assertEquals(0.5f, updateProgressFraction(18L * 1024 * 1024, 36L * 1024 * 1024), 0.0f)
        assertEquals("18.0 MB / 36.0 MB · 50%", formatUpdateProgress(18L * 1024 * 1024, 36L * 1024 * 1024))
        assertEquals("正在下载", formatUpdateProgress(0, 0))
    }
}
