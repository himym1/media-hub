package com.mediahub.android.feature.search

import com.mediahub.android.core.network.DiscoveryItem
import com.mediahub.android.core.network.ReleaseFacts
import com.mediahub.android.core.network.SearchCandidate
import com.mediahub.android.core.network.SearchIdentity
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class DiscoverySearchTest {
    @Test
    fun discoverySearchQueryIncludesYearWhenPresent() {
        assertEquals(
            "头号玩家 2018",
            discoverySearchQuery(DiscoveryItem("333339", "头号玩家", 2018, "movie", null)),
        )
        assertEquals(
            "范海辛",
            discoverySearchQuery(DiscoveryItem("7131", "范海辛", 0, "movie", null)),
        )
    }

    @Test
    fun prioritizeDiscoveryResultsPrefersTmdbYearAndTransferable() {
        val focus = DiscoveryItem("333339", "头号玩家", 2018, "movie", null)
        val ranked = prioritizeDiscoveryResults(
            listOf(
                candidate("a", "Wrong", year = 2018, mediaType = "movie", tmdbId = null, token = null),
                candidate("b", "Ready Player One", year = 2018, mediaType = "movie", tmdbId = "333339", token = "tok"),
                candidate("c", "Series Hit", year = 2018, mediaType = "series", tmdbId = "333339", token = "tok"),
            ),
            focus,
        )
        assertEquals("b", ranked.first().id)
        assertTrue(ranked.indexOfFirst { it.id == "b" } < ranked.indexOfFirst { it.id == "c" })
    }

    @Test
    fun pipelineHelpersDistinguishTransferAndDownload() {
        val transfer = candidate("a", "A", year = 2004, mediaType = "movie", tmdbId = null, token = "tok")
        val download = transfer.copy(id = "b", sourceId = "moviepilot", transferState = "downloadable")
        assertEquals("115 转存", pipelineLabel(transfer))
        assertEquals("PT 下载", pipelineLabel(download))
        assertEquals(false, isDownloadable(transfer))
        assertEquals(true, isDownloadable(download))
        assertEquals(true, matchesPipeline(transfer, "transfer"))
        assertEquals(false, matchesPipeline(transfer, "download"))
    }

    @Test
    fun pickSearchIdentityPrefersSelectedTmdb() {
        val first = SearchIdentity("1", "First", year = 2003, mediaType = "movie")
        val second = SearchIdentity("2", "Second", year = 2008, mediaType = "movie")
        val selected = candidate("b", "B", year = 2008, mediaType = "movie", tmdbId = "2", token = "tok")
        assertEquals("Second", pickSearchIdentity(listOf(first, second), selected)?.title)
        assertEquals("First", pickSearchIdentity(listOf(first, second), null)?.title)
    }

    @Test
    fun discoveryResultsHeadingUsesFocusCopy() {
        assertEquals(
            "正在查找《头号玩家》可获取版本…",
            discoveryResultsHeading(searching = true, focusTitle = "头号玩家", resultCount = 0),
        )
        assertEquals(
            "《头号玩家》可获取版本 (3)",
            discoveryResultsHeading(searching = false, focusTitle = "头号玩家", resultCount = 3),
        )
        assertEquals(
            "搜索结果 2",
            discoveryResultsHeading(searching = false, focusTitle = "", resultCount = 2),
        )
    }

    private fun candidate(
        id: String,
        title: String,
        year: Int,
        mediaType: String,
        tmdbId: String?,
        token: String?,
    ) = SearchCandidate(
        id = id,
        title = title,
        year = year,
        season = 0,
        episodeStart = 0,
        episodeEnd = 0,
        mediaType = mediaType,
        tmdbId = tmdbId,
        source = "juying",
        provider = null,
        posterUrl = null,
        release = ReleaseFacts(
            resolution = "1080p",
            videoCodec = "x264",
            dynamicRange = null,
            audio = null,
            sizeBytes = 0L,
        ),
        transferState = if (token != null) "available" else "unavailable",
        transferToken = token,
    )
}
