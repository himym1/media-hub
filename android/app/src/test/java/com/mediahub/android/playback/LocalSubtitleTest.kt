package com.mediahub.android.playback

import com.mediahub.android.core.network.contentDispositionFileName
import java.io.File
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class LocalSubtitleTest {
    @Test
    fun mapsAssMimeAndExtension() {
        assertEquals("text/x-ssa", localSubtitleMimeType("text/x-ssa", "chi.ass"))
        assertEquals(".ass", localSubtitleExtension("text/x-ssa", "chi.ass"))
        assertEquals("application/x-subrip", localSubtitleMimeType("application/x-subrip", "chi.srt"))
        assertEquals(".srt", localSubtitleExtension("application/x-subrip", "chi.srt"))
    }

    @Test
    fun writesSidecarIntoAppCache() {
        val cache = File(requireNotNull(System.getProperty("java.io.tmpdir")), "media-hub-sub-${System.nanoTime()}")
        val written = writeLocalSubtitleCache(
            cache,
            "item-1",
            "1\n00:00:01,000 --> 00:00:02,000\n你好\n".toByteArray(),
            "application/x-subrip",
            "chi.srt",
        )
        assertEquals("application/x-subrip", written?.mimeType)
        assertEquals("item-1.srt", written?.file?.name)
        assertEquals(true, written?.file?.readText()?.contains("你好"))
        written?.file?.delete()
        cache.resolve("local-subtitles").delete()
        cache.delete()
    }

    @Test
    fun rejectsUnsafeItemId() {
        val cache = File(requireNotNull(System.getProperty("java.io.tmpdir")))
        assertNull(writeLocalSubtitleCache(cache, "../x", byteArrayOf(1), "application/x-subrip", "chi.srt"))
    }

    @Test
    fun parsesSafeContentDispositionName() {
        assertEquals("chi.srt", contentDispositionFileName("attachment; filename=\"chi.srt\""))
        assertEquals("", contentDispositionFileName("attachment; filename=\"../secret.srt\""))
    }
}
