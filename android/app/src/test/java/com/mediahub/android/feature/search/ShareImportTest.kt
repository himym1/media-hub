package com.mediahub.android.feature.search

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class ShareImportTest {
    @Test
    fun accepts115ShareUrls() {
        assertTrue(canSubmitShareImport("https://115.com/s/shareABC123?password=ab12"))
        assertTrue(canSubmitShareImport("anxia.com/s/shareABC123", "xy9z"))
    }

    @Test
    fun rejectsOtherClouds() {
        assertFalse(canSubmitShareImport("https://pan.quark.cn/s/nope", "abcd"))
    }
}
