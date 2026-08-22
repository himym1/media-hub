package com.mediahub.android.feature.player

import android.content.pm.ActivityInfo
import org.junit.Assert.assertEquals
import org.junit.Test

class PlayerOrientationTest {
    @Test
    fun orientationToggleStaysInLandscapeFamily() {
        assertEquals(ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE, PlayerLandscapeOrientation)
        assertEquals(
            ActivityInfo.SCREEN_ORIENTATION_REVERSE_LANDSCAPE,
            nextPlayerOrientation(PlayerLandscapeOrientation),
        )
        assertEquals(
            PlayerLandscapeOrientation,
            nextPlayerOrientation(ActivityInfo.SCREEN_ORIENTATION_REVERSE_LANDSCAPE),
        )
        assertEquals(
            ActivityInfo.SCREEN_ORIENTATION_REVERSE_LANDSCAPE,
            nextPlayerOrientation(ActivityInfo.SCREEN_ORIENTATION_PORTRAIT),
        )
    }
}
