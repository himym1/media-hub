package com.mediahub.android.feature.services

import org.junit.Assert.assertEquals
import org.junit.Test

class ServicesSectionModelTest {
    @Test
    fun settingsKeepOverviewProvidersAndAccountAsSeparateSections() {
        assertEquals(
            listOf(
                "overview" to "概览",
                "providers" to "Provider",
                "account" to "账户",
            ),
            serviceSectionOptions,
        )
    }
}
