package com.mediahub.android.core.designsystem

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Checkbox
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.IconButtonDefaults
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.SearchBar
import androidx.compose.material3.SearchBarDefaults
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TextField
import androidx.compose.material3.TextFieldDefaults
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.ArrowRight
import com.composables.icons.lucide.Check
import com.composables.icons.lucide.CircleAlert
import com.composables.icons.lucide.Eye
import com.composables.icons.lucide.EyeOff
import com.composables.icons.lucide.Info
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.Search
import com.composables.icons.lucide.X

@Composable
fun MediaHubText(
    text: String,
    modifier: Modifier = Modifier,
    color: Color = MediaHubColors.TextPrimary,
    fontSize: TextUnit = 14.sp,
    fontWeight: FontWeight? = null,
    maxLines: Int = Int.MAX_VALUE,
    overflow: TextOverflow = TextOverflow.Clip,
) {
    Text(
        text = text,
        modifier = modifier,
        color = color,
        fontSize = fontSize,
        fontWeight = fontWeight,
        maxLines = maxLines,
        overflow = overflow,
    )
}

@Composable
fun MediaHubIcon(
    imageVector: ImageVector,
    contentDescription: String?,
    modifier: Modifier = Modifier,
    tint: Color = MediaHubColors.TextSecondary,
) {
    Icon(
        imageVector = imageVector,
        contentDescription = contentDescription,
        modifier = modifier,
        tint = tint,
    )
}

@Composable
fun MediaHubButton(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    icon: ImageVector? = null,
) {
    Button(
        onClick = onClick,
        modifier = modifier.heightIn(min = 48.dp),
        enabled = enabled,
        shape = MaterialTheme.shapes.large,
        colors = ButtonDefaults.buttonColors(
            containerColor = MaterialTheme.colorScheme.primary,
            contentColor = MaterialTheme.colorScheme.onPrimary,
        ),
    ) {
        if (icon != null) {
            MediaHubIcon(
                imageVector = icon,
                contentDescription = null,
                modifier = Modifier.size(16.dp),
                tint = MaterialTheme.colorScheme.onPrimary,
            )
            Spacer(Modifier.width(8.dp))
        }
        Text(
            text = label,
            fontSize = 14.sp,
            fontWeight = FontWeight.SemiBold,
        )
    }
}

@Composable
fun MediaHubSecondaryButton(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    icon: ImageVector? = null,
) {
    FilledTonalButton(
        onClick = onClick,
        modifier = modifier.heightIn(min = 48.dp),
        enabled = enabled,
        shape = MaterialTheme.shapes.large,
    ) {
        if (icon != null) {
            MediaHubIcon(
                imageVector = icon,
                contentDescription = null,
                modifier = Modifier.size(16.dp),
                tint = MaterialTheme.colorScheme.onSecondaryContainer,
            )
            Spacer(Modifier.width(8.dp))
        }
        Text(
            text = label,
            fontSize = 13.sp,
            fontWeight = FontWeight.Medium,
        )
    }
}

@Composable
fun MediaHubTextButton(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    destructive: Boolean = false,
    icon: ImageVector? = null,
) {
    TextButton(
        onClick = onClick,
        modifier = modifier.heightIn(min = 48.dp),
        enabled = enabled,
        colors = ButtonDefaults.textButtonColors(
            contentColor = if (destructive) {
                MaterialTheme.colorScheme.error
            } else {
                MaterialTheme.colorScheme.primary
            },
        ),
    ) {
        if (icon != null) {
            MediaHubIcon(
                imageVector = icon,
                contentDescription = null,
                modifier = Modifier.size(16.dp),
                tint = if (destructive) {
                    MaterialTheme.colorScheme.error
                } else {
                    MaterialTheme.colorScheme.primary
                },
            )
            Spacer(Modifier.width(8.dp))
        }
        Text(
            text = label,
            fontSize = 14.sp,
            fontWeight = FontWeight.Medium,
        )
    }
}

@Composable
fun MediaHubIconButton(
    imageVector: ImageVector,
    contentDescription: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    tint: Color = Color.Unspecified,
) {
    IconButton(
        onClick = onClick,
        modifier = modifier
            .size(48.dp)
            .semantics { this.contentDescription = contentDescription },
        enabled = enabled,
        colors = IconButtonDefaults.iconButtonColors(
            contentColor = if (tint != Color.Unspecified) tint else MaterialTheme.colorScheme.onSurface,
            disabledContentColor = MediaHubColors.TextMuted,
        ),
    ) {
        MediaHubIcon(
            imageVector = imageVector,
            contentDescription = contentDescription,
            modifier = Modifier.size(20.dp),
            tint = if (tint != Color.Unspecified) {
                tint
            } else if (enabled) {
                MaterialTheme.colorScheme.onSurface
            } else {
                MediaHubColors.TextMuted
            },
        )
    }
}

@Composable
fun MediaHubSegmentedControl(
    options: List<Pair<String, String>>,
    selected: String,
    onSelected: (String) -> Unit,
    modifier: Modifier = Modifier,
    role: Role = Role.Tab,
    raised: Boolean = true,
) {
    SingleChoiceSegmentedButtonRow(
        modifier = modifier.fillMaxWidth().then(if (raised) Modifier else Modifier),
    ) {
        options.forEachIndexed { index, (value, label) ->
            val active = value == selected
            SegmentedButton(
                selected = active,
                onClick = { onSelected(value) },
                shape = SegmentedButtonDefaults.itemShape(index = index, count = options.size),
                modifier = Modifier
                    .heightIn(min = 48.dp)
                    .selectable(selected = active, role = role, onClick = { onSelected(value) }),
                label = {
                    Text(
                        text = label,
                        fontSize = 13.sp,
                        fontWeight = if (active) FontWeight.SemiBold else FontWeight.Medium,
                    )
                },
            )
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MediaHubSearchField(
    value: String,
    onValueChange: (String) -> Unit,
    onSearch: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    placeholder: String = "搜索电影或电视剧",
) {
    TextField(
        value = value,
        onValueChange = onValueChange,
        modifier = modifier
            .fillMaxWidth()
            .heightIn(min = 48.dp)
            .semantics { contentDescription = placeholder },
        enabled = enabled,
        singleLine = true,
        placeholder = { Text(placeholder) },
        leadingIcon = {
            MediaHubIcon(Lucide.Search, contentDescription = null, tint = MediaHubColors.TextMuted)
        },
        trailingIcon = {
            Row(verticalAlignment = Alignment.CenterVertically) {
                if (value.isNotEmpty()) {
                    MediaHubIconButton(
                        imageVector = Lucide.X,
                        contentDescription = "清空输入",
                        onClick = { onValueChange("") },
                        enabled = enabled,
                    )
                }
                MediaHubIconButton(
                    imageVector = Lucide.ArrowRight,
                    contentDescription = "提交搜索",
                    onClick = onSearch,
                    enabled = enabled && value.isNotBlank(),
                )
            }
        },
        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
        keyboardActions = KeyboardActions(onSearch = { onSearch() }),
        shape = RoundedCornerShape(28.dp),
        colors = TextFieldDefaults.colors(
            focusedContainerColor = MaterialTheme.colorScheme.surfaceContainerHigh,
            unfocusedContainerColor = MaterialTheme.colorScheme.surfaceContainerHigh,
            disabledContainerColor = MaterialTheme.colorScheme.surfaceContainer,
            focusedIndicatorColor = Color.Transparent,
            unfocusedIndicatorColor = Color.Transparent,
            disabledIndicatorColor = Color.Transparent,
        ),
    )
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MediaHubSearchBar(
    query: String,
    onQueryChange: (String) -> Unit,
    onSearch: () -> Unit,
    expanded: Boolean,
    onExpandedChange: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    placeholder: String = "搜索电影或电视剧",
    content: @Composable ColumnScope.() -> Unit,
) {
    SearchBar(
        inputField = {
            SearchBarDefaults.InputField(
                query = query,
                onQueryChange = onQueryChange,
                onSearch = { onSearch() },
                expanded = expanded,
                onExpandedChange = onExpandedChange,
                enabled = enabled,
                placeholder = { Text(placeholder) },
                leadingIcon = {
                    MediaHubIcon(Lucide.Search, contentDescription = null, tint = MediaHubColors.TextMuted)
                },
                trailingIcon = {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        if (query.isNotEmpty()) {
                            MediaHubIconButton(
                                imageVector = Lucide.X,
                                contentDescription = "清空输入",
                                onClick = { onQueryChange("") },
                                enabled = enabled,
                            )
                        }
                        if (expanded) {
                            TextButton(onClick = { onExpandedChange(false) }) {
                                Text("取消")
                            }
                        } else {
                            MediaHubIconButton(
                                imageVector = Lucide.ArrowRight,
                                contentDescription = "提交搜索",
                                onClick = onSearch,
                                enabled = enabled && query.isNotBlank(),
                            )
                        }
                    }
                },
                modifier = Modifier.semantics { contentDescription = placeholder },
            )
        },
        expanded = expanded,
        onExpandedChange = onExpandedChange,
        modifier = modifier.fillMaxWidth(),
        colors = SearchBarDefaults.colors(
            containerColor = MaterialTheme.colorScheme.surfaceContainerHigh,
        ),
        content = content,
    )
}

@Composable
fun MediaHubTextField(
    value: String,
    onValueChange: (String) -> Unit,
    placeholder: String,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    keyboardType: KeyboardType = KeyboardType.Text,
    password: Boolean = false,
) {
    OutlinedTextField(
        value = value,
        onValueChange = onValueChange,
        modifier = modifier
            .fillMaxWidth()
            .heightIn(min = 48.dp)
            .semantics { contentDescription = placeholder },
        enabled = enabled,
        singleLine = true,
        label = { Text(placeholder) },
        visualTransformation = if (password) PasswordVisualTransformation() else VisualTransformation.None,
        keyboardOptions = KeyboardOptions(keyboardType = keyboardType),
        shape = MaterialTheme.shapes.medium,
    )
}

@Composable
fun MediaHubMultilineField(
    value: String,
    onValueChange: (String) -> Unit,
    placeholder: String,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
) {
    OutlinedTextField(
        value = value,
        onValueChange = onValueChange,
        modifier = modifier
            .fillMaxWidth()
            .semantics { contentDescription = placeholder },
        enabled = enabled,
        minLines = 5,
        label = { Text(placeholder) },
        shape = MaterialTheme.shapes.medium,
    )
}

@Composable
fun MediaHubPasswordField(
    value: String,
    onValueChange: (String) -> Unit,
    visible: Boolean,
    onVisibilityChanged: () -> Unit,
    onSubmit: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
) {
    OutlinedTextField(
        value = value,
        onValueChange = onValueChange,
        modifier = modifier
            .fillMaxWidth()
            .heightIn(min = 48.dp),
        enabled = enabled,
        singleLine = true,
        label = { Text("密码") },
        visualTransformation = if (visible) VisualTransformation.None else PasswordVisualTransformation(),
        keyboardOptions = KeyboardOptions(
            keyboardType = KeyboardType.Password,
            imeAction = ImeAction.Done,
        ),
        keyboardActions = KeyboardActions(onDone = { onSubmit() }),
        trailingIcon = {
            MediaHubIconButton(
                imageVector = if (visible) Lucide.EyeOff else Lucide.Eye,
                contentDescription = if (visible) "隐藏密码" else "显示密码",
                onClick = onVisibilityChanged,
                enabled = enabled,
            )
        },
        shape = MaterialTheme.shapes.medium,
    )
}

@Composable
fun MediaHubDestructiveButton(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    icon: ImageVector? = null,
) {
    Button(
        onClick = onClick,
        modifier = modifier.heightIn(min = 48.dp),
        enabled = enabled,
        shape = MaterialTheme.shapes.large,
        colors = ButtonDefaults.buttonColors(
            containerColor = MaterialTheme.colorScheme.error,
            contentColor = MaterialTheme.colorScheme.onError,
        ),
    ) {
        if (icon != null) {
            MediaHubIcon(
                imageVector = icon,
                contentDescription = null,
                modifier = Modifier.size(16.dp),
                tint = MaterialTheme.colorScheme.onError,
            )
            Spacer(Modifier.width(8.dp))
        }
        Text(
            text = label,
            fontSize = 14.sp,
            fontWeight = FontWeight.SemiBold,
        )
    }
}

enum class ToastType { Info, Success, Error, Warning }

data class ToastMessage(
    val id: Long = System.currentTimeMillis(),
    val message: String,
    val type: ToastType = ToastType.Info,
)

@Composable
fun MediaHubToastHost(
    currentToast: ToastMessage?,
    modifier: Modifier = Modifier,
) {
    AnimatedVisibility(
        visible = currentToast != null,
        enter = fadeIn() + slideInVertically { it },
        exit = fadeOut() + slideOutVertically { it },
        modifier = modifier,
    ) {
        if (currentToast != null) {
            val (icon, tint) = when (currentToast.type) {
                ToastType.Success -> Lucide.Check to MediaHubColors.Accent
                ToastType.Error -> Lucide.CircleAlert to MediaHubColors.Error
                ToastType.Warning -> Lucide.CircleAlert to MediaHubColors.Warning
                ToastType.Info -> Lucide.Info to MediaHubColors.Source
            }
            MediaHubCard(
                modifier = Modifier.padding(horizontal = 18.dp),
                insideMargin = PaddingValues(horizontal = 18.dp, vertical = 10.dp),
            ) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    MediaHubIcon(
                        imageVector = icon,
                        contentDescription = null,
                        tint = tint,
                        modifier = Modifier.size(18.dp),
                    )
                    MediaHubText(
                        text = currentToast.message,
                        color = MediaHubColors.TextPrimary,
                        fontSize = 13.sp,
                        fontWeight = FontWeight.Medium,
                    )
                }
            }
        }
    }
}

@Composable
fun MediaHubConfirmDialog(
    visible: Boolean,
    title: String,
    message: String,
    confirmLabel: String = "确认",
    cancelLabel: String = "取消",
    isDestructive: Boolean = false,
    confirmEnabled: Boolean = true,
    extra: (@Composable ColumnScope.() -> Unit)? = null,
    onConfirm: () -> Unit,
    onDismiss: () -> Unit,
) {
    if (!visible) return
    AlertDialog(
        onDismissRequest = onDismiss,
        title = {
            Text(
                text = title,
                color = if (isDestructive) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.onSurface,
            )
        },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                Text(
                    text = message,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    fontSize = 14.sp,
                )
                extra?.invoke(this)
            }
        },
        confirmButton = {
            TextButton(onClick = onConfirm, enabled = confirmEnabled) {
                Text(
                    text = confirmLabel,
                    color = if (isDestructive) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.primary,
                    fontWeight = FontWeight.SemiBold,
                )
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text(cancelLabel)
            }
        },
        shape = MaterialTheme.shapes.extraLarge,
    )
}

@Composable
fun MediaHubDialog(
    title: String,
    onDismiss: () -> Unit,
    confirmLabel: String,
    onConfirm: () -> Unit,
    dismissLabel: String = "关闭",
    content: @Composable ColumnScope.() -> Unit,
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = {
            Text(
                text = title,
                color = MaterialTheme.colorScheme.onSurface,
                fontSize = 18.sp,
                fontWeight = FontWeight.SemiBold,
            )
        },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(4.dp), content = content)
        },
        confirmButton = {
            TextButton(onClick = onConfirm) {
                Text(text = confirmLabel, fontWeight = FontWeight.SemiBold)
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text(dismissLabel)
            }
        },
        shape = MaterialTheme.shapes.extraLarge,
    )
}

@Composable
fun MediaHubFilterChip(
    label: String,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    role: Role = Role.RadioButton,
) {
    Box(
        modifier = modifier
            .heightIn(min = 48.dp)
            .selectable(selected = selected, enabled = enabled, role = role, onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        FilterChip(
            selected = selected,
            onClick = onClick,
            enabled = enabled,
            label = {
                Text(
                    text = label,
                    fontSize = 12.sp,
                    fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Medium,
                )
            },
        )
    }
}

@Composable
fun MediaHubSwitchRow(
    title: String,
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
    summary: String? = null,
    enabled: Boolean = true,
) {
    ListItem(
        headlineContent = { Text(title, fontSize = 15.sp) },
        modifier = modifier.heightIn(min = 48.dp),
        supportingContent = summary?.let { { Text(it, fontSize = 12.sp, color = MediaHubColors.TextMuted) } },
        trailingContent = {
            Switch(
                checked = checked,
                onCheckedChange = onCheckedChange,
                enabled = enabled,
            )
        },
        colors = ListItemDefaults.colors(containerColor = Color.Transparent),
    )
}

@Composable
fun MediaHubCheckboxRow(
    title: String,
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
    summary: String? = null,
    enabled: Boolean = true,
) {
    ListItem(
        headlineContent = { Text(title, fontSize = 15.sp) },
        modifier = modifier.heightIn(min = 48.dp),
        supportingContent = summary?.let { { Text(it, fontSize = 12.sp, color = MediaHubColors.TextMuted) } },
        trailingContent = {
            Checkbox(
                checked = checked,
                onCheckedChange = onCheckedChange,
                enabled = enabled,
            )
        },
        colors = ListItemDefaults.colors(containerColor = Color.Transparent),
    )
}

@Composable
fun MediaHubEmptyState(
    title: String,
    message: String,
    icon: ImageVector,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier
            .fillMaxWidth()
            .padding(vertical = 40.dp, horizontal = 24.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Box(
            modifier = Modifier
                .size(64.dp)
                .background(MaterialTheme.colorScheme.primaryContainer, CircleShape),
            contentAlignment = Alignment.Center,
        ) {
            MediaHubIcon(
                imageVector = icon,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onPrimaryContainer,
                modifier = Modifier.size(28.dp),
            )
        }
        MediaHubText(
            text = title,
            modifier = Modifier.padding(top = 16.dp),
            color = MediaHubColors.TextStrong,
            fontSize = 16.sp,
            fontWeight = FontWeight.SemiBold,
        )
        MediaHubText(
            text = message,
            modifier = Modifier.padding(top = 6.dp),
            color = MediaHubColors.TextMuted,
            fontSize = 12.sp,
        )
    }
}

enum class BadgeVariant { Primary, Success, Warning, Error, Source, Info, Neutral }

@Composable
fun MediaHubBadge(
    text: String,
    modifier: Modifier = Modifier,
    variant: BadgeVariant = BadgeVariant.Neutral,
    icon: ImageVector? = null,
) {
    val (bgColor, contentColor) = when (variant) {
        BadgeVariant.Primary -> MediaHubColors.AccentLight to MediaHubColors.Accent
        BadgeVariant.Success -> MediaHubColors.SuccessContainer to MediaHubColors.Success
        BadgeVariant.Warning -> MediaHubColors.WarningContainer to MediaHubColors.Warning
        BadgeVariant.Error -> MediaHubColors.ErrorContainer to MediaHubColors.Error
        BadgeVariant.Source -> MediaHubColors.SourceContainer to MediaHubColors.Source
        BadgeVariant.Info -> MediaHubColors.AccentLight to MediaHubColors.Accent
        BadgeVariant.Neutral -> MediaHubColors.NeutralContainer to MediaHubColors.TextSecondary
    }

    Row(
        modifier = modifier
            .background(bgColor, MediaHubShapes.Chip)
            .padding(horizontal = 8.dp, vertical = 3.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        if (icon != null) {
            MediaHubIcon(
                imageVector = icon,
                contentDescription = null,
                modifier = Modifier.size(11.dp),
                tint = contentColor,
            )
        }
        MediaHubText(
            text = text,
            color = contentColor,
            fontSize = 12.sp,
            fontWeight = FontWeight.SemiBold,
        )
    }
}

@Composable
fun MediaHubLinearProgress(
    progress: Float,
    modifier: Modifier = Modifier,
    color: Color = MediaHubColors.Accent,
    trackColor: Color = MediaHubColors.NeutralContainer,
) {
    LinearProgressIndicator(
        progress = { progress.coerceIn(0f, 1f) },
        modifier = modifier
            .fillMaxWidth()
            .height(6.dp)
            .clip(RoundedCornerShape(3.dp)),
        color = color,
        trackColor = trackColor,
    )
}

data class PipelineStepItem(
    val key: String,
    val label: String,
    val isCompleted: Boolean,
    val isCurrent: Boolean,
    val isFailed: Boolean = false,
)

@Composable
fun MediaHubPipelineStepper(
    steps: List<PipelineStepItem>,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier
            .fillMaxWidth()
            .padding(vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        steps.forEachIndexed { index, step ->
            if (index > 0) {
                Box(
                    modifier = Modifier
                        .weight(1f)
                        .height(3.dp)
                        .clip(RoundedCornerShape(2.dp))
                        .background(
                            when {
                                step.isCompleted || step.isCurrent -> MaterialTheme.colorScheme.primary
                                step.isFailed -> MaterialTheme.colorScheme.error
                                else -> MaterialTheme.colorScheme.outlineVariant
                            },
                        ),
                )
            }
            Column(
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                Box(
                    modifier = Modifier
                        .size(22.dp)
                        .background(
                            when {
                                step.isFailed -> MediaHubColors.ErrorContainer
                                step.isCompleted -> MediaHubColors.SuccessContainer
                                step.isCurrent -> MaterialTheme.colorScheme.primaryContainer
                                else -> MediaHubColors.NeutralContainer
                            },
                            CircleShape,
                        ),
                    contentAlignment = Alignment.Center,
                ) {
                    val dotColor = when {
                        step.isFailed -> MediaHubColors.Error
                        step.isCompleted -> MediaHubColors.Success
                        step.isCurrent -> MediaHubColors.Accent
                        else -> MediaHubColors.TextMuted
                    }
                    Box(
                        modifier = Modifier
                            .size(8.dp)
                            .background(dotColor, CircleShape),
                    )
                }
                MediaHubText(
                    text = step.label,
                    fontSize = 12.sp,
                    color = when {
                        step.isFailed -> MediaHubColors.Error
                        step.isCurrent -> MediaHubColors.Accent
                        step.isCompleted -> MediaHubColors.TextPrimary
                        else -> MediaHubColors.TextMuted
                    },
                    fontWeight = if (step.isCurrent) FontWeight.SemiBold else FontWeight.Normal,
                )
            }
        }
    }
}

@Composable
fun mediaHubShimmerBrush(showShimmer: Boolean = true, targetValue: Float = 1200f): Brush {
    return if (showShimmer) {
        val shimmerColors = listOf(
            MediaHubColors.CardBackground.copy(alpha = 0.6f),
            MediaHubColors.SurfaceHigh.copy(alpha = 0.95f),
            MediaHubColors.CardBackground.copy(alpha = 0.6f),
        )
        val transition = rememberInfiniteTransition(label = "shimmerTransition")
        val translateAnimation = transition.animateFloat(
            initialValue = 0f,
            targetValue = targetValue,
            animationSpec = infiniteRepeatable(
                animation = tween(durationMillis = 1200, easing = FastOutSlowInEasing),
                repeatMode = RepeatMode.Restart,
            ),
            label = "shimmerTranslate",
        )
        Brush.linearGradient(
            colors = shimmerColors,
            start = Offset.Zero,
            end = Offset(x = translateAnimation.value, y = translateAnimation.value),
        )
    } else {
        Brush.linearGradient(
            colors = listOf(Color.Transparent, Color.Transparent),
            start = Offset.Zero,
            end = Offset.Zero,
        )
    }
}

@Composable
fun MediaHubShimmerBox(
    modifier: Modifier = Modifier,
    shape: Shape = MaterialTheme.shapes.medium,
) {
    Box(
        modifier = modifier
            .clip(shape)
            .background(mediaHubShimmerBrush()),
    )
}
