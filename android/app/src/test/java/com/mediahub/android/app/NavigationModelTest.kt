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
        assertFalse(primaryDestinations.contains(MainDestination.Services))
    }

    @Test
    fun destinationsExposeStableUserFacingTitles() {
        assertEquals("发现", MainDestination.Search.title)
        assertEquals("任务", MainDestination.Transfers.title)
        assertEquals("订阅", MainDestination.Subscriptions.title)
        assertEquals("媒体库", MainDestination.Library.title)
        assertEquals("服务与设置", MainDestination.Services.title)
    }

    @Test
    fun systemLayerReturnsToEachPreviouslySelectedPrimaryDestination() {
        val navigation = MainNavigationHistory()
        for (primary in listOf(MainDestination.Transfers, MainDestination.Subscriptions, MainDestination.Library)) {
            assertEquals(primary, navigation.show(primary).destination)
            assertEquals(MainDestination.Services, navigation.openSystem().destination)
            assertEquals(MainDestination.Services, navigation.showSystem(MainDestination.Search).destination)
            assertEquals(primary, navigation.closeSystem().destination)
        }
    }

    @Test
    fun typedDetailsOnlyOpenOnTheirOwningDestinationAndPrimaryNavigationClearsThem() {
        val navigation = MainNavigationHistory()
        navigation.show(MainDestination.Search)
        assertEquals(null, navigation.openDetail(WorkspaceDetail.SubscriptionEditor(null)).detail)

        navigation.show(MainDestination.Subscriptions)
        val editor = WorkspaceDetail.SubscriptionEditor("subscription-1")
        assertEquals(editor, navigation.openDetail(editor).detail)
        assertEquals(null, navigation.show(MainDestination.Transfers).detail)

        val transfer = WorkspaceDetail.Transfer("transfer-1")
        assertEquals(transfer, navigation.openDetail(transfer).detail)
        assertEquals(null, navigation.closeDetail().detail)
    }
}
