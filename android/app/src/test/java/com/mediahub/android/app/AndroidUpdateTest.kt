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
    fun formatsDownloadProgress() {
        assertEquals(0.5f, updateProgressFraction(18L * 1024 * 1024, 36L * 1024 * 1024), 0.0f)
        assertEquals("18.0 MB / 36.0 MB · 50%", formatUpdateProgress(18L * 1024 * 1024, 36L * 1024 * 1024))
        assertEquals("正在下载", formatUpdateProgress(0, 0))
    }
}
