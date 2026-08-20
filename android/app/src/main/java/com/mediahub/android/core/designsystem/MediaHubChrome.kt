package com.mediahub.android.core.designsystem

import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.displayCutout
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.systemBars
import androidx.compose.foundation.layout.union
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.unit.dp
import top.yukonga.miuix.kmp.basic.Card
import top.yukonga.miuix.kmp.basic.CardDefaults
import top.yukonga.miuix.kmp.basic.HorizontalDivider
import top.yukonga.miuix.kmp.basic.NavigationBar
import top.yukonga.miuix.kmp.basic.NavigationBarItem
import top.yukonga.miuix.kmp.basic.Scaffold
import top.yukonga.miuix.kmp.basic.SmallTitle
import top.yukonga.miuix.kmp.basic.TopAppBar
import top.yukonga.miuix.kmp.preference.ArrowPreference
import top.yukonga.miuix.kmp.utils.PressFeedbackType

data class MediaHubNavItem(
    val key: String,
    val label: String,
    val icon: ImageVector,
)

@Composable
fun MediaHubScaffold(
    modifier: Modifier = Modifier,
    topBar: @Composable () -> Unit = {},
    bottomBar: @Composable () -> Unit = {},
    consumeWindowInsets: Boolean = true,
    content: @Composable (PaddingValues) -> Unit,
) {
    Scaffold(
        modifier = modifier,
        topBar = topBar,
        bottomBar = bottomBar,
        contentWindowInsets = if (consumeWindowInsets) {
            WindowInsets.systemBars.union(WindowInsets.displayCutout)
        } else {
            WindowInsets(0, 0, 0, 0)
        },
        content = content,
    )
}

@Composable
fun MediaHubTopAppBar(
    title: String,
    subtitle: String = "",
    navigationIcon: @Composable () -> Unit = {},
    actions: @Composable RowScope.() -> Unit = {},
) {
    TopAppBar(
        title = title,
        subtitle = subtitle,
        navigationIcon = navigationIcon,
        actions = actions,
    )
}

@Composable
fun MediaHubNavigationBar(
    items: List<MediaHubNavItem>,
    selectedKey: String,
    onSelected: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    NavigationBar(modifier = modifier) {
        items.forEach { item ->
            NavigationBarItem(
                selected = item.key == selectedKey,
                onClick = { onSelected(item.key) },
                icon = item.icon,
                label = item.label,
            )
        }
    }
}

@Composable
fun MediaHubCard(
    modifier: Modifier = Modifier,
    insideMargin: PaddingValues = PaddingValues(0.dp),
    onClick: (() -> Unit)? = null,
    color: Color = Color.Unspecified,
    content: @Composable ColumnScope.() -> Unit,
) {
    val colors = if (color == Color.Unspecified) {
        CardDefaults.defaultColors()
    } else {
        CardDefaults.defaultColors(color = color)
    }
    if (onClick == null) {
        Card(modifier = modifier, insideMargin = insideMargin, colors = colors, content = content)
    } else {
        Card(
            modifier = modifier,
            insideMargin = insideMargin,
            colors = colors,
            pressFeedbackType = PressFeedbackType.Sink,
            onClick = onClick,
            content = content,
        )
    }
}

@Composable
fun MediaHubSmallTitle(
    text: String,
    modifier: Modifier = Modifier,
) {
    SmallTitle(text = text, modifier = modifier)
}

@Composable
fun MediaHubPreferenceRow(
    title: String,
    modifier: Modifier = Modifier,
    summary: String? = null,
    enabled: Boolean = true,
    onClick: (() -> Unit)? = null,
    start: (@Composable () -> Unit)? = null,
    end: (@Composable RowScope.() -> Unit)? = null,
) {
    ArrowPreference(
        title = title,
        modifier = modifier.heightIn(min = 48.dp),
        summary = summary,
        startAction = start,
        endActions = end ?: {},
        onClick = onClick,
        enabled = enabled,
    )
}

@Composable
fun MediaHubListDivider(modifier: Modifier = Modifier) {
    HorizontalDivider(modifier = modifier)
}
