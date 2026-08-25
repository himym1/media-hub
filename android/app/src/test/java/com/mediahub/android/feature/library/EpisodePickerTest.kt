package com.mediahub.android.feature.library

import com.mediahub.android.core.network.EmbyEpisode
import com.mediahub.android.core.network.EmbyItem
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class EpisodePickerTest {
    @Test
    fun yearLikeSeasonBecomesSeasonOne() {
        assertEquals(1, displaySeason(2016))
        assertEquals(1, displaySeason(1))
        assertEquals(0, displaySeason(0))
        assertEquals(2, displaySeason(2))
    }

    @Test
    fun episodeLabelHidesReleaseFilenameAndGenericNames() {
        assertEquals("第 2 集", episodeLabel(episode(season = 2016, episode = 2, name = "Show 2016 E02 UHDTV HEVC 10bit 60fps DD2.0-Group")))
        assertEquals("第 2 集", episodeLabel(episode(season = 1, episode = 2, name = "第二集")))
        assertEquals("第 4 集", episodeLabel(episode(season = 1, episode = 4, name = "Episode 4")))
        assertEquals("第 1 集 · 连接", episodeLabel(episode(season = 1, episode = 1, name = "连接"), "验收剧集"))
        assertEquals("第 1 集", episodeLabel(episode(season = 1, episode = 1, name = "验收剧集"), "验收剧集"))
        assertEquals("第 3 集 · The Dinner Party", episodeLabel(episode(season = 1, episode = 3, name = "The Dinner Party")))
    }

    @Test
    fun episodeTitleKeepsHumanNamesOnly() {
        assertNull(episodeTitle("Show 2016 E02 UHDTV HEVC 10bit"))
        assertNull(episodeTitle("第二集"))
        assertNull(episodeTitle("验收剧集", "验收剧集"))
        assertEquals("连接", episodeTitle("连接", "验收剧集"))
        assertTrue(looksLikeReleaseName("Show.S01E03.1080p.WEB-DL.x265"))
    }

    private fun episode(season: Int, episode: Int, name: String) = EmbyEpisode(
        item = EmbyItem(id = "episode-$episode", name = name, type = "Episode", year = 2016, tmdbId = null, season = season, episode = episode),
        externalUrl = "https://emby.example/episode-$episode",
        appUrl = "emby://items/server-1/episode-$episode",
    )
}
