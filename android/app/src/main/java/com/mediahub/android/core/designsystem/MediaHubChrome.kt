package com.mediahub.android.core.designsystem

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.displayCutout
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.systemBars
import androidx.compose.foundation.layout.union
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.selection.selectable
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationRail
import androidx.compose.material3.NavigationRailItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.ScrollableTabRow
import androidx.compose.material3.Tab
import androidx.compose.material3.TabRow
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.ChevronRight
import com.composables.icons.lucide.EllipsisVertical
import com.composables.icons.lucide.Lucide

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
        containerColor = MaterialTheme.colorScheme.background,
        contentColor = MaterialTheme.colorScheme.onBackground,
        contentWindowInsets = if (consumeWindowInsets) {
            WindowInsets.systemBars.union(WindowInsets.displayCutout)
        } else {
            WindowInsets(0, 0, 0, 0)
        },
        content = content,
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MediaHubTopAppBar(
    title: String,
    subtitle: String = "",
    navigationIcon: @Composable () -> Unit = {},
    actions: @Composable RowScope.() -> Unit = {},
) {
    TopAppBar(
        title = {
            Column {
                Text(
                    text = title,
                    style = MaterialTheme.typography.titleLarge,
                    color = MaterialTheme.colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                if (subtitle.isNotBlank()) {
                    Text(
                        text = subtitle,
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
        },
        navigationIcon = navigationIcon,
        actions = actions,
        colors = TopAppBarDefaults.topAppBarColors(
            containerColor = MaterialTheme.colorScheme.background,
            titleContentColor = MaterialTheme.colorScheme.onSurface,
            actionIconContentColor = MaterialTheme.colorScheme.onSurface,
            navigationIconContentColor = MaterialTheme.colorScheme.onSurface,
        ),
    )
}

@Composable
fun MediaHubNavigationBar(
    items: List<MediaHubNavItem>,
    selectedKey: String,
    onSelected: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    NavigationBar(
        modifier = modifier,
        containerColor = MaterialTheme.colorScheme.surfaceContainer,
        contentColor = MaterialTheme.colorScheme.onSurfaceVariant,
    ) {
        items.forEach { item ->
            val selected = item.key == selectedKey
            NavigationBarItem(
                selected = selected,
                onClick = { onSelected(item.key) },
                modifier = Modifier.semantics { this.contentDescription = item.label },
                icon = {
                    MediaHubIcon(
                        imageVector = item.icon,
                        contentDescription = item.label,
                        modifier = Modifier.size(22.dp),
                        tint = if (selected) {
                            MaterialTheme.colorScheme.onSecondaryContainer
                        } else {
                            MaterialTheme.colorScheme.onSurfaceVariant
                        },
                    )
                },
                label = {
                    Text(
                        text = item.label,
                        fontSize = 12.sp,
                        fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
                    )
                },
            )
        }
    }
}

@Composable
fun MediaHubNavigationRail(
    items: List<MediaHubNavItem>,
    selectedKey: String,
    onSelected: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    NavigationRail(
        modifier = modifier
            .width(88.dp)
            .fillMaxHeight(),
        containerColor = MaterialTheme.colorScheme.surfaceContainerLow,
    ) {
        items.forEach { item ->
            val selected = item.key == selectedKey
            NavigationRailItem(
                selected = selected,
                onClick = { onSelected(item.key) },
                modifier = Modifier.semantics { this.contentDescription = item.label },
                icon = {
                    MediaHubIcon(
                        imageVector = item.icon,
                        contentDescription = item.label,
                        modifier = Modifier.size(22.dp),
                        tint = if (selected) {
                            MaterialTheme.colorScheme.onSecondaryContainer
                        } else {
                            MaterialTheme.colorScheme.onSurfaceVariant
                        },
                    )
                },
                label = {
                    Text(
                        text = item.label,
                        fontSize = 12.sp,
                        fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
                    )
                },
            )
        }
    }
}

@Composable
fun MediaHubTabRow(
    options: List<Pair<String, String>>,
    selected: String,
    onSelected: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    if (options.isEmpty()) return
    val selectedIndex = options.indexOfFirst { it.first == selected }.coerceAtLeast(0)
    val tabs: @Composable () -> Unit = {
        options.forEach { (value, label) ->
            val active = value == selected
            Tab(
                selected = active,
                onClick = { onSelected(value) },
                modifier = Modifier.heightIn(min = 48.dp),
                text = {
                    Text(
                        text = label,
                        fontSize = 14.sp,
                        fontWeight = if (active) FontWeight.SemiBold else FontWeight.Medium,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                },
                selectedContentColor = MaterialTheme.colorScheme.primary,
                unselectedContentColor = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
    if (options.size <= 4) {
        TabRow(
            selectedTabIndex = selectedIndex,
            modifier = modifier.fillMaxWidth(),
            containerColor = Color.Transparent,
            contentColor = MaterialTheme.colorScheme.primary,
            divider = {},
        ) {
            tabs()
        }
    } else {
        ScrollableTabRow(
            selectedTabIndex = selectedIndex,
            modifier = modifier.fillMaxWidth(),
            containerColor = Color.Transparent,
            contentColor = MaterialTheme.colorScheme.primary,
            edgePadding = 0.dp,
            divider = {},
        ) {
            tabs()
        }
    }
}

@Composable
fun MediaHubCard(
    modifier: Modifier = Modifier,
    insideMargin: PaddingValues = PaddingValues(0.dp),
    onClick: (() -> Unit)? = null,
    color: Color = Color.Unspecified,
    elevated: Boolean = true,
    content: @Composable ColumnScope.() -> Unit,
) {
    val colors = CardDefaults.cardColors(
        containerColor = if (color == Color.Unspecified) {
            if (elevated) MaterialTheme.colorScheme.surfaceContainerHigh else MaterialTheme.colorScheme.surfaceContainer
        } else {
            color
        },
    )
    if (onClick == null) {
        Card(
            modifier = modifier,
            shape = MaterialTheme.shapes.large,
            colors = colors,
            elevation = CardDefaults.cardElevation(defaultElevation = if (elevated) 1.dp else 0.dp),
        ) {
            Column(modifier = Modifier.padding(insideMargin), content = content)
        }
    } else {
        Card(
            onClick = onClick,
            modifier = modifier,
            shape = MaterialTheme.shapes.large,
            colors = colors,
            elevation = CardDefaults.cardElevation(defaultElevation = if (elevated) 1.dp else 0.dp),
        ) {
            Column(modifier = Modifier.padding(insideMargin), content = content)
        }
    }
}

@Composable
fun MediaHubSmallTitle(
    text: String,
    modifier: Modifier = Modifier,
) {
    Text(
        text = text,
        modifier = modifier.padding(horizontal = 4.dp, vertical = 8.dp),
        style = MaterialTheme.typography.titleSmall,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
    )
}

@Composable
fun MediaHubPreferenceRow(
    title: String,
    modifier: Modifier = Modifier,
    summary: String? = null,
    enabled: Boolean = true,
    selected: Boolean = false,
    role: Role? = null,
    contentDescription: String? = null,
    onClick: (() -> Unit)? = null,
    start: (@Composable () -> Unit)? = null,
    end: (@Composable RowScope.() -> Unit)? = null,
) {
    ListItem(
        headlineContent = {
            Text(
                text = title,
                color = if (enabled) MaterialTheme.colorScheme.onSurface else MediaHubColors.TextMuted,
                fontSize = 15.sp,
            )
        },
        modifier = modifier
            .heightIn(min = 48.dp)
            .then(
                if (contentDescription == null) {
                    Modifier
                } else {
                    Modifier.semantics { this.contentDescription = contentDescription }
                },
            )
            .then(
                when {
                    onClick == null -> Modifier
                    role != null -> Modifier.selectable(
                        selected = selected,
                        enabled = enabled,
                        role = role,
                        onClick = onClick,
                    )
                    else -> Modifier.clickable(enabled = enabled, onClick = onClick)
                },
            ),
        supportingContent = summary?.let {
            {
                Text(
                    text = it,
                    color = MediaHubColors.TextMuted,
                    fontSize = 12.sp,
                )
            }
        },
        leadingContent = start,
        trailingContent = {
            if (end != null) {
                androidx.compose.foundation.layout.Row { end() }
            } else if (onClick != null) {
                MediaHubIcon(
                    imageVector = Lucide.ChevronRight,
                    contentDescription = null,
                    tint = MediaHubColors.TextMuted,
                    modifier = Modifier.size(18.dp),
                )
            }
        },
        colors = ListItemDefaults.colors(
            containerColor = if (selected) MediaHubColors.SurfaceSelected else Color.Transparent,
        ),
    )
}

data class MediaHubMenuAction(
    val label: String,
    val enabled: Boolean = true,
    val onClick: () -> Unit,
)

@Composable
fun MediaHubOverflowMenu(
    expanded: Boolean,
    onExpandedChange: (Boolean) -> Unit,
    contentDescription: String,
    actions: List<MediaHubMenuAction>,
    modifier: Modifier = Modifier,
) {
    Box(modifier = modifier) {
        MediaHubIconButton(
            imageVector = Lucide.EllipsisVertical,
            contentDescription = contentDescription,
            onClick = { onExpandedChange(!expanded) },
        )
        DropdownMenu(
            expanded = expanded,
            onDismissRequest = { onExpandedChange(false) },
        ) {
            actions.forEach { action ->
                DropdownMenuItem(
                    text = {
                        Text(
                            text = action.label,
                            fontSize = 15.sp,
                            color = if (action.enabled) {
                                MaterialTheme.colorScheme.onSurface
                            } else {
                                MediaHubColors.TextMuted
                            },
                        )
                    },
                    onClick = {
                        onExpandedChange(false)
                        action.onClick()
                    },
                    enabled = action.enabled,
                )
            }
        }
    }
}

@Composable
fun MediaHubListDivider(modifier: Modifier = Modifier) {
    HorizontalDivider(
        modifier = modifier,
        color = MaterialTheme.colorScheme.outlineVariant,
    )
}
