package com.mediahub.android.playback

import org.junit.Assert.assertEquals
import org.junit.Test

class SubtitleOffsetStoreTest {
    @Test
    fun persistsAndClearsPerItem() {
        val store = SubtitleOffsetStore(mutableMapOf())
        store.put("abc:emby:item-1", 800)
        store.put("abc:emby:item-2", -200)
        assertEquals(800L, store.get("abc:emby:item-1"))
        store.put("abc:emby:item-1", 0)
        assertEquals(0L, store.get("abc:emby:item-1"))
        store.put("abc:emby:item-1", 400)
        store.clearItem("item-1")
        assertEquals(0L, store.get("abc:emby:item-1"))
        assertEquals(-200L, store.get("abc:emby:item-2"))
    }
}
