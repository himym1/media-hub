package com.mediahub.android.app

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Test

class NavigationModelTest {
    @Test
    fun primaryNavigationContainsOnlyMediaDestinations() {
        assertEquals(
            listOf(
                MainDestination.Search,
                MainDestination.Transfers,
                MainDestination.Subscriptions,
                MainDestination.Library,
            ),
            primaryDestinations,
        )
        assertFalse(primaryDestinations.contains(MainDestination.Operations))
        assertFalse(primaryDestinations.contains(MainDestination.Services))
    }

    @Test
    fun destinationsExposeStableUserFacingTitles() {
        assertEquals("发现", MainDestination.Search.title)
        assertEquals("任务", MainDestination.Transfers.title)
        assertEquals("订阅", MainDestination.Subscriptions.title)
        assertEquals("媒体库", MainDestination.Library.title)
        assertEquals("运维", MainDestination.Operations.title)
        assertEquals("服务与设置", MainDestination.Services.title)
    }

    @Test
    fun systemLayerReturnsToEachPreviouslySelectedPrimaryDestination() {
        val navigation = MainNavigationHistory()
        for (primary in listOf(MainDestination.Transfers, MainDestination.Subscriptions, MainDestination.Library)) {
            assertEquals(primary, navigation.show(primary))
            assertEquals(MainDestination.Services, navigation.openSystem())
            assertEquals(MainDestination.Operations, navigation.showSystem(MainDestination.Operations))
            assertEquals(primary, navigation.closeSystem())
        }
    }
}
