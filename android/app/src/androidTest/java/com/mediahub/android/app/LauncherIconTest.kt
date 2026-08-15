package com.mediahub.android.app

import android.content.Context
import android.graphics.Bitmap
import android.graphics.drawable.AdaptiveIconDrawable
import androidx.core.graphics.drawable.toBitmap
import androidx.test.core.app.ApplicationProvider
import androidx.test.core.graphics.writeToTestStorage
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.filters.SdkSuppress
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
@SdkSuppress(minSdkVersion = 33)
class LauncherIconTest {
    @Test
    fun launcherIconIsAdaptiveThemeableAndNonBlank() {
        val context = ApplicationProvider.getApplicationContext<Context>()
        val icon = context.packageManager.getApplicationIcon(context.applicationInfo)

        assertTrue(icon is AdaptiveIconDrawable)
        val adaptiveIcon = icon as AdaptiveIconDrawable
        assertNotNull(adaptiveIcon.background)
        assertNotNull(adaptiveIcon.foreground)
        assertNotNull(adaptiveIcon.monochrome)

        val bitmap = adaptiveIcon.toBitmap(192, 192, Bitmap.Config.ARGB_8888)
        val pixels = IntArray(bitmap.width * bitmap.height)
        bitmap.getPixels(pixels, 0, bitmap.width, 0, 0, bitmap.width, bitmap.height)
        assertTrue(pixels.count { it ushr 24 > 0 } > pixels.size * 9 / 10)
        assertTrue(pixels.toSet().size > 16)
        bitmap.writeToTestStorage("mediahub-launcher-icon")
    }
}
