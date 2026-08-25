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
}
