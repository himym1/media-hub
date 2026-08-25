package com.mediahub.android.feature.player

import androidx.media3.common.C
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

    @Test
    fun hidesReleaseFilenameAsAudioTitle() {
        assertTrue(isReleaseStyleTrackLabel("[UXN] Show.E02.160123.UHDTV"))
        assertEquals("音轨 1", displayTrackTitle("[UXN] Show.E02.160123.UHDTV", "und", C.TRACK_TYPE_AUDIO, 0))
        assertEquals("韩语", displayTrackTitle("[UXN] Show.E02.160123.UHDTV", "kor", C.TRACK_TYPE_AUDIO, 0))
        assertEquals("评论音轨", displayTrackTitle("评论音轨", "kor", C.TRACK_TYPE_AUDIO, 1))
    }

    @Test
    fun defaultAudioChoiceSelectsFirstUnselectedTrack() {
        val first = PlayerTrackOption(C.TRACK_TYPE_AUDIO, 0, 0, "音轨 1", "立体声 · AC3", selected = false, supported = true)
        assertEquals(first, defaultAudioChoice(listOf(first)))
        assertEquals(null, defaultAudioChoice(listOf(first.copy(selected = true))))
        assertEquals(null, defaultAudioChoice(emptyList()))
    }
}
