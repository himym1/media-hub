package com.mediahub.android.feature.library

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
    }
}
