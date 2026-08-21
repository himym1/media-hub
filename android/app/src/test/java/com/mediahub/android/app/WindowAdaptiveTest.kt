package com.mediahub.android.app

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class WindowAdaptiveTest {
    @Test
    fun twoPaneStartsAtMediumWidth() {
        assertFalse(usesTwoPaneLayout(390))
        assertFalse(usesTwoPaneLayout(599))
        assertTrue(usesTwoPaneLayout(600))
        assertTrue(usesTwoPaneLayout(840))
    }

    @Test
    fun compactDetailHidesWorkspaceChrome() {
        assertFalse(showsWorkspaceTopBar(detailOpen = true, twoPane = false))
        assertFalse(showsWorkspaceBottomBar(inSystem = false, detailOpen = true, twoPane = false))
        assertFalse(showsWorkspaceNavigationRail(inSystem = false, twoPane = false))
    }

    @Test
    fun expandedDetailKeepsRailAndTopBar() {
        assertTrue(showsWorkspaceTopBar(detailOpen = true, twoPane = true))
        assertFalse(showsWorkspaceBottomBar(inSystem = false, detailOpen = true, twoPane = true))
        assertTrue(showsWorkspaceNavigationRail(inSystem = false, twoPane = true))
    }

    @Test
    fun systemLayerHidesPrimaryNavigation() {
        assertFalse(showsWorkspaceBottomBar(inSystem = true, detailOpen = false, twoPane = false))
        assertFalse(showsWorkspaceNavigationRail(inSystem = true, twoPane = true))
        assertTrue(showsWorkspaceTopBar(detailOpen = false, twoPane = true))
    }
}
