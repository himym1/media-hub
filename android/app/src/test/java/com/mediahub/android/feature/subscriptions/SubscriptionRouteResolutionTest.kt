package com.mediahub.android.feature.subscriptions

import org.junit.Assert.assertEquals
import org.junit.Test

class SubscriptionRouteResolutionTest {
    @Test
    fun failedOrStaleMutationsKeepTheCurrentEditor() {
        assertEquals(
            SubscriptionRouteResolution.Keep,
            resolveSubscriptionRoute(true, "current", 20L, true, true, saved = null, deleted = null),
        )
        assertEquals(
            SubscriptionRouteResolution.Keep,
            resolveSubscriptionRoute(
                true,
                "current",
                20L,
                true,
                true,
                saved = SubscriptionMutationResult(19L, "old"),
                deleted = SubscriptionMutationResult(19L, "old"),
            ),
        )
    }

    @Test
    fun newEditorSaveBindsIdentityAndMatchingDeleteClosesIt() {
        val saved = SubscriptionMutationResult(20L, "created")
        val replacement = resolveSubscriptionRoute(
            true,
            null,
            20L,
            initialized = true,
            itemExists = true,
            saved = saved,
            deleted = null,
        )
        assertEquals(SubscriptionRouteResolution.Replace("created", 20L), replacement)

        val deleted = SubscriptionMutationResult(20L, "created")
        assertEquals(
            SubscriptionRouteResolution.Close,
            resolveSubscriptionRoute(true, "created", 20L, true, true, saved = null, deleted = deleted),
        )
    }

    @Test
    fun initializedMissingSubscriptionClosesButInitialEmptyStateDoesNot() {
        assertEquals(
            SubscriptionRouteResolution.Keep,
            resolveSubscriptionRoute(true, "item", 20L, false, false, saved = null, deleted = null),
        )
        assertEquals(
            SubscriptionRouteResolution.Close,
            resolveSubscriptionRoute(true, "item", 20L, true, false, saved = null, deleted = null),
        )
    }
}
