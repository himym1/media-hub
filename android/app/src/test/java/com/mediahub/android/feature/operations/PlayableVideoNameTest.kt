package com.mediahub.android.feature.operations

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class PlayableVideoNameTest {
    @Test
    fun acceptsOnlySupportedVideoExtensions() {
        assertTrue(isPlayableVideoName("Movie.MKV"))
        assertTrue(isPlayableVideoName("Episode.01.m2ts"))
        assertFalse(isPlayableVideoName("subtitle.ass"))
        assertFalse(isPlayableVideoName("no-extension"))
    }
}
