package com.mediahub.android.feature.player

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class PlayerTracksTest {
    @Test
    fun mapsKnownLanguageCodes() {
        val japanese = displayTrackLanguage("jpn")
        val chinese = displayTrackLanguage("chi")
        val english = displayTrackLanguage("en")
        assertTrue(japanese != null && japanese != "jpn" && japanese.contains("日"))
        assertTrue(chinese != null && chinese != "chi" && chinese.contains("中"))
        assertTrue(english != null && english != "en")
    }

    @Test
    fun ignoresUndefinedLanguageCodes() {
        assertNull(displayTrackLanguage(null))
        assertNull(displayTrackLanguage("und"))
        assertNull(displayTrackLanguage(" "))
    }

    @Test
    fun labelsCommonAudioLayouts() {
        assertEquals("立体声", audioChannelLabel(2))
        assertEquals("5.1", audioChannelLabel(6))
        assertEquals("7.1", audioChannelLabel(8))
        assertEquals("", audioChannelLabel(0))
    }
}
