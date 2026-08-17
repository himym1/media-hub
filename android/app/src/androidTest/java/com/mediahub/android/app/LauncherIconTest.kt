package com.mediahub.android.app

import android.content.Context
import android.graphics.Bitmap
import android.graphics.Color
import android.graphics.drawable.AdaptiveIconDrawable
import android.graphics.drawable.ColorDrawable
import androidx.core.content.ContextCompat
import androidx.core.graphics.drawable.DrawableCompat
import androidx.core.graphics.drawable.toBitmap
import androidx.test.core.app.ApplicationProvider
import androidx.test.core.graphics.writeToTestStorage
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.filters.SdkSuppress
import com.mediahub.android.R
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
        val monochrome = requireNotNull(adaptiveIcon.monochrome)

        val normal = adaptiveIcon.toBitmap(192, 192, Bitmap.Config.ARGB_8888)
        normal.writeToTestStorage("mediahub-launcher-icon")
        assertRenderedIcon(normal)

        val round = requireNotNull(ContextCompat.getDrawable(context, R.mipmap.ic_launcher_round))
            .toBitmap(192, 192, Bitmap.Config.ARGB_8888)
        round.writeToTestStorage("mediahub-launcher-round")
        assertRenderedIcon(round)

        val monochromeBitmap = monochrome.toBitmap(192, 192, Bitmap.Config.ARGB_8888)
        monochromeBitmap.writeToTestStorage("mediahub-launcher-monochrome")
        val monochromePixels = monochromeBitmap.pixels()
        val monochromeVisible = monochromePixels.count { it ushr 24 > 0 }
        assertTrue("monochrome visible pixels=$monochromeVisible", monochromeVisible in monochromePixels.size / 10..monochromePixels.size * 3 / 5)

        val themedForeground = requireNotNull(monochrome.constantState).newDrawable().mutate()
        DrawableCompat.setTint(themedForeground, Color.rgb(34, 83, 58))
        val themed = AdaptiveIconDrawable(ColorDrawable(Color.rgb(213, 232, 219)), themedForeground)
            .toBitmap(192, 192, Bitmap.Config.ARGB_8888)
        themed.writeToTestStorage("mediahub-launcher-themed")
        assertRenderedIcon(themed)
    }

    private fun assertRenderedIcon(bitmap: Bitmap) {
        val pixels = bitmap.pixels()
        val visiblePixels = pixels.count { it ushr 24 > 0 }
        assertTrue("visible launcher pixels=$visiblePixels", visiblePixels > pixels.size * 3 / 5)
        assertTrue("launcher color count=${pixels.toSet().size}", pixels.toSet().size > 16)
    }

    private fun Bitmap.pixels(): IntArray = IntArray(width * height).also { values ->
        getPixels(values, 0, width, 0, 0, width, height)
    }
}
