package com.mediahub.android.feature.search

import com.mediahub.android.core.network.DiscoveryItem
import com.mediahub.android.core.network.ReleaseFacts
import com.mediahub.android.core.network.SearchCandidate
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
    fun discoveryResultsHeadingUsesFocusCopy() {
        assertEquals(
            "正在查找《头号玩家》可转存版本…",
            discoveryResultsHeading(searching = true, focusTitle = "头号玩家", resultCount = 0),
        )
        assertEquals(
            "《头号玩家》可转存版本 (3)",
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
