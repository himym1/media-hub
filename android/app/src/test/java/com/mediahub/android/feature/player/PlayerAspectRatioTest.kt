package com.mediahub.android.feature.player

import androidx.media3.ui.AspectRatioFrameLayout
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class PlayerAspectRatioTest {
    @Test
    fun exposesAllMedia3ResizeModes() {
        val modes = PlayerAspectRatio.options.map { it.resizeMode }.toSet()
        assertEquals(
            setOf(
                AspectRatioFrameLayout.RESIZE_MODE_FIT,
                AspectRatioFrameLayout.RESIZE_MODE_ZOOM,
                AspectRatioFrameLayout.RESIZE_MODE_FIXED_WIDTH,
                AspectRatioFrameLayout.RESIZE_MODE_FIXED_HEIGHT,
                AspectRatioFrameLayout.RESIZE_MODE_FILL,
            ),
            modes,
        )
        assertTrue(PlayerAspectRatio.Fit.label.isNotBlank())
        assertTrue(PlayerAspectRatio.Fit.description.isNotBlank())
    }
}
