package com.mediahub.android.playback

import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.Test

class SubtitleTimingTest {
    @Before
    fun resetOffset() {
        SubtitleTiming.reset()
    }

    @Test
    fun formatsAlignedAndShiftedOffsets() {
        assertEquals("已对齐", formatSubtitleOffset(0))
        assertEquals("延后 0.1 秒", formatSubtitleOffset(100))
        assertEquals("提前 1.5 秒", formatSubtitleOffset(-1_500))
        assertEquals("延后 2 秒", formatSubtitleOffset(2_000))
    }

    @Test
    fun clampsOffsetAndExposesMicroseconds() {
        SubtitleTiming.shift(100)
        assertEquals(100L, SubtitleTiming.offsetMs)
        assertEquals(100_000L, SubtitleTiming.offsetUs)
        SubtitleTiming.set(40_000)
        assertEquals(SubtitleTiming.MaxMs, SubtitleTiming.offsetMs)
        SubtitleTiming.set(-40_000)
        assertEquals(SubtitleTiming.MinMs, SubtitleTiming.offsetMs)
        SubtitleTiming.reset()
        assertEquals(0L, SubtitleTiming.offsetMs)
    }
}
