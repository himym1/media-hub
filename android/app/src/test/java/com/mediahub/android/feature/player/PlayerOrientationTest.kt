package com.mediahub.android.feature.player

import android.content.pm.ActivityInfo
import org.junit.Assert.assertEquals
import org.junit.Test

class PlayerOrientationTest {
    @Test
    fun startsInSensorLandscapeAndToggleUnlocksThenRelocks() {
        assertEquals(ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE, PlayerLandscapeOrientation)
        assertEquals(
            ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED,
            nextPlayerOrientation(PlayerLandscapeOrientation),
        )
        assertEquals(
            PlayerLandscapeOrientation,
            nextPlayerOrientation(ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED),
        )
        assertEquals(
            PlayerLandscapeOrientation,
            nextPlayerOrientation(ActivityInfo.SCREEN_ORIENTATION_PORTRAIT),
        )
    }
}
