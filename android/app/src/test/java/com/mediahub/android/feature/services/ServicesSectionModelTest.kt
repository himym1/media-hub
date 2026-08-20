package com.mediahub.android.feature.services

import org.junit.Assert.assertEquals
import org.junit.Test

class ServicesSectionModelTest {
    @Test
    fun settingsKeepOverviewProvidersAndAccountAsSeparateSections() {
        assertEquals(
            listOf(
                "overview" to "状态概览",
                "providers" to "服务配置",
                "account" to "账户与更新",
            ),
            serviceSectionOptions,
        )
    }
}
