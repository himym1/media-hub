package com.mediahub.android.app

import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.platform.LocalConfiguration

internal const val TwoPaneMinWidthDp = 600

val LocalTwoPane = staticCompositionLocalOf { false }

fun usesTwoPaneLayout(widthDp: Int): Boolean = widthDp >= TwoPaneMinWidthDp

fun showsWorkspaceTopBar(detailOpen: Boolean, twoPane: Boolean): Boolean = twoPane || !detailOpen

fun showsWorkspaceBottomBar(inSystem: Boolean, detailOpen: Boolean, twoPane: Boolean): Boolean =
    !inSystem && !twoPane && !detailOpen

fun showsWorkspaceNavigationRail(inSystem: Boolean, twoPane: Boolean): Boolean = !inSystem && twoPane

@Composable
fun currentWindowUsesTwoPane(): Boolean = usesTwoPaneLayout(LocalConfiguration.current.screenWidthDp)

@Composable
fun ProvideWindowAdaptive(content: @Composable () -> Unit) {
    CompositionLocalProvider(LocalTwoPane provides currentWindowUsesTwoPane(), content = content)
}
