package com.mediahub.android.feature.subscriptions

import org.junit.Assert.assertEquals
import org.junit.Test

class SubscriptionQualityPresetTest {
    @Test
    fun presetsProduceStableBackendRules() {
        val editor = SubscriptionEditorState(
            resolutions = "custom",
            videoCodecs = "custom",
            dynamicRanges = "custom",
            audioContains = "custom",
            preferredSources = "custom",
            minSizeGiB = "1",
            maxSizeGiB = "2",
            allowUnknownSize = true,
            preferSmaller = true,
        )

        val balanced = applyQualityPreset(editor, "balanced")
        assertEquals("2160p, 1080p", balanced.resolutions)
        assertEquals("HEVC, AVC", balanced.videoCodecs)
        assertEquals("40", balanced.maxSizeGiB)
        assertEquals(false, balanced.allowUnknownSize)
        assertEquals(true, balanced.preferSmaller)
        assertEquals("balanced", inferQualityPreset(balanced))

        val custom = balanced.copy(audioContains = "Atmos")
        assertEquals("custom", inferQualityPreset(custom))
    }
}
