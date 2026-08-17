package com.mediahub.android.feature.library

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class EmbyOpenTargetTest {
    @Test
    fun acceptsOnlyOfficialItemDeepLinks() {
        assertEquals(
            "emby://items/server-1/item_1",
            validatedEmbyAppUrl("emby://items/server-1/item_1"),
        )
        assertNull(validatedEmbyAppUrl("https://emby.example/item-1"))
        assertNull(validatedEmbyAppUrl("emby://items/server-1"))
        assertNull(validatedEmbyAppUrl("emby://items/server-1/../item-1"))
        assertNull(validatedEmbyAppUrl("emby://items/server-1/item-1?token=secret"))
        assertNull(validatedEmbyAppUrl("emby://other/server-1/item-1"))
    }

    @Test
    fun labelsMatchTheResolvedDestination() {
        assertEquals("使用 Emby 播放", embyPlayLabel(appAvailable = true))
        assertEquals("在 Emby 网页中播放", embyPlayLabel(appAvailable = false))
    }
}
