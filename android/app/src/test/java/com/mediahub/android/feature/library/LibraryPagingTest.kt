package com.mediahub.android.feature.library

import com.mediahub.android.core.network.MediaLibrary
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class LibraryPagingTest {
    @Test
    fun pagingStaysWithinAvailableEmbyItems() {
        assertEquals(1, libraryPageCount(0))
        assertEquals(2, libraryPageCount(25))
        assertEquals(1, nextLibraryPage(current = 0, delta = 1, total = 25))
        assertEquals(0, nextLibraryPage(current = 1, delta = -1, total = 25))
        assertNull(nextLibraryPage(current = 0, delta = -1, total = 25))
        assertNull(nextLibraryPage(current = 1, delta = 1, total = 25))
        assertEquals(true, libraryHasMore(itemCount = 24, total = 25, searching = false))
        assertEquals(false, libraryHasMore(itemCount = 25, total = 25, searching = false))
        assertEquals(false, libraryHasMore(itemCount = 8, total = 25, searching = true))
    }

    @Test
    fun adultGroupsStayOffTheRootTabRow() {
        val libraries = listOf(
            MediaLibrary(id = "movies", name = "电影", collectionType = "movies"),
            MediaLibrary(id = AdultLibraryId, name = "成人影视", collectionType = "movies"),
            MediaLibrary(id = "group-jp", name = "日本", collectionType = "movies", parentId = AdultLibraryId),
        )
        assertEquals(listOf("movies", AdultLibraryId), rootLibraries(libraries).map { it.id })
        assertEquals(listOf("group-jp"), childLibraries(libraries, AdultLibraryId).map { it.id })
        assertEquals(AdultLibraryId, libraryRootId(libraries, "group-jp"))
        assertEquals("movies", libraryRootId(libraries, "movies"))
    }

    @Test
    fun sharedCatalogIdsStayOffTheLocalLibraryList() {
        assertEquals(true, isSharedEmbyId("r_remote"))
        assertEquals(false, isSharedEmbyId("movies"))
        assertEquals("电影", libraryDisplayName("共享/电影"))
    }
}
