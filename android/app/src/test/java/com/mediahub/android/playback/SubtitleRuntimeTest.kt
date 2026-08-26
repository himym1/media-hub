package com.mediahub.android.playback

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class SubtitleRuntimeTest {
    @Test
    fun readsLastSrtAndAssCue() {
        val srt = """
            1
            00:01:00,000 --> 00:01:02,000
            你好
            2
            00:42:10,500 --> 00:42:12,000
            结束
        """.trimIndent()
        assertEquals(42 * 60_000L + 12_000L, lastSubtitleCueMs(srt, "application/x-subrip"))
        val ass = """
            [Events]
            Dialogue: 0,0:01:00.00,0:01:02.00,Default,,0,0,0,,你好
            Dialogue: 0,0:40:00.00,0:40:03.50,Default,,0,0,0,,结束
        """.trimIndent()
        assertEquals(40 * 60_000L + 3_500L, lastSubtitleCueMs(ass, "text/x-ssa"))
    }

    @Test
    fun warnsOnlyWhenRuntimesDiverge() {
        assertNull(subtitleRuntimeWarning(45 * 60_000L, 45 * 60_000L + 10_000L))
        assertEquals(
            "字幕时长和片源大约差 2 分钟，可能不是这一版",
            subtitleRuntimeWarning(45 * 60_000L, 47 * 60_000L),
        )
    }
}
