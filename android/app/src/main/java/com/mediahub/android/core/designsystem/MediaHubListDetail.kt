package com.mediahub.android.core.designsystem

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.unit.dp
import androidx.compose.material3.adaptive.ExperimentalMaterial3AdaptiveApi
import androidx.compose.material3.adaptive.currentWindowAdaptiveInfo
import androidx.compose.material3.adaptive.layout.AnimatedPane
import androidx.compose.material3.adaptive.layout.ListDetailPaneScaffold
import androidx.compose.material3.adaptive.layout.PaneAdaptedValue
import androidx.compose.material3.adaptive.layout.ThreePaneScaffoldValue
import androidx.compose.material3.adaptive.layout.calculatePaneScaffoldDirective
import com.mediahub.android.app.LocalTwoPane

@OptIn(ExperimentalMaterial3AdaptiveApi::class)
@Composable
fun MediaHubListDetail(
    detailOpen: Boolean,
    emptyTitle: String,
    emptyMessage: String,
    emptyIcon: ImageVector,
    modifier: Modifier = Modifier,
    twoPane: Boolean = LocalTwoPane.current,
    list: @Composable () -> Unit,
    detail: @Composable () -> Unit,
) {
    val directive = calculatePaneScaffoldDirective(currentWindowAdaptiveInfo()).copy(
        maxHorizontalPartitions = if (twoPane) 2 else 1,
        horizontalPartitionSpacerSize = 12.dp,
    )
    val value = when {
        twoPane -> ThreePaneScaffoldValue(
            primary = PaneAdaptedValue.Expanded,
            secondary = PaneAdaptedValue.Expanded,
            tertiary = PaneAdaptedValue.Hidden,
        )
        detailOpen -> ThreePaneScaffoldValue(
            primary = PaneAdaptedValue.Expanded,
            secondary = PaneAdaptedValue.Hidden,
            tertiary = PaneAdaptedValue.Hidden,
        )
        else -> ThreePaneScaffoldValue(
            primary = PaneAdaptedValue.Hidden,
            secondary = PaneAdaptedValue.Expanded,
            tertiary = PaneAdaptedValue.Hidden,
        )
    }
    ListDetailPaneScaffold(
        modifier = modifier.fillMaxSize(),
        directive = directive,
        value = value,
        listPane = { AnimatedPane { list() } },
        detailPane = {
            AnimatedPane {
                if (detailOpen) {
                    detail()
                } else {
                    Box(Modifier.fillMaxSize().padding(24.dp), contentAlignment = Alignment.Center) {
                        MediaHubEmptyState(title = emptyTitle, message = emptyMessage, icon = emptyIcon)
                    }
                }
            }
        },
    )
}
