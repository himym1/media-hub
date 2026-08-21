package com.mediahub.android.core.designsystem

import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Shape
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
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
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
import com.composables.icons.lucide.X
import top.yukonga.miuix.kmp.basic.Button
import top.yukonga.miuix.kmp.basic.ButtonDefaults
import top.yukonga.miuix.kmp.basic.Icon as MiuixIcon
import top.yukonga.miuix.kmp.basic.IconButton as MiuixIconButton
import top.yukonga.miuix.kmp.basic.InputField
import top.yukonga.miuix.kmp.basic.SearchBar
import top.yukonga.miuix.kmp.basic.Text as MiuixText
import top.yukonga.miuix.kmp.basic.TextButton
import top.yukonga.miuix.kmp.basic.TextField
import top.yukonga.miuix.kmp.overlay.OverlayDialog
import top.yukonga.miuix.kmp.preference.CheckboxPreference
import top.yukonga.miuix.kmp.preference.SwitchPreference
import top.yukonga.miuix.kmp.theme.MiuixTheme

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
    MiuixText(
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
    MiuixIcon(
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
        modifier = modifier,
        enabled = enabled,
        minHeight = 48.dp,
        colors = ButtonDefaults.buttonColorsPrimary(),
    ) {
        if (icon != null) {
            MediaHubIcon(
                imageVector = icon,
                contentDescription = null,
                modifier = Modifier.size(16.dp),
                tint = MiuixTheme.colorScheme.onPrimary,
            )
            Spacer(Modifier.width(8.dp))
        }
        MediaHubText(
            text = label,
            color = MiuixTheme.colorScheme.onPrimary,
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
    Button(
        onClick = onClick,
        modifier = modifier,
        enabled = enabled,
        minHeight = 48.dp,
        colors = ButtonDefaults.buttonColors(),
    ) {
        if (icon != null) {
            MediaHubIcon(
                imageVector = icon,
                contentDescription = null,
                modifier = Modifier.size(16.dp),
                tint = MiuixTheme.colorScheme.onSecondaryVariant,
            )
            Spacer(Modifier.width(8.dp))
        }
        MediaHubText(
            text = label,
            color = MiuixTheme.colorScheme.onSecondaryVariant,
            fontSize = 13.sp,
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
    MiuixIconButton(
        onClick = onClick,
        modifier = modifier.semantics { this.contentDescription = contentDescription },
        enabled = enabled,
        minHeight = 48.dp,
        minWidth = 48.dp,
    ) {
        val iconTint = if (tint != Color.Unspecified) {
            tint
        } else if (enabled) {
            MediaHubColors.TextPrimary
        } else {
            MediaHubColors.TextMuted
        }
        MediaHubIcon(
            imageVector = imageVector,
            contentDescription = contentDescription,
            modifier = Modifier.size(20.dp),
            tint = iconTint,
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
    val content: @Composable () -> Unit = {
        Row(
            modifier = Modifier.fillMaxWidth().selectableGroup(),
            horizontalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            options.forEach { (value, label) ->
                val active = value == selected
                Box(
                    modifier = Modifier
                        .weight(1f)
                        .heightIn(min = 48.dp)
                        .selectable(selected = active, role = role, onClick = { onSelected(value) })
                        .padding(vertical = 10.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    MediaHubText(
                        text = label,
                        color = if (active) MiuixTheme.colorScheme.primary else MediaHubColors.TextMuted,
                        fontSize = 13.sp,
                        fontWeight = if (active) FontWeight.SemiBold else FontWeight.Normal,
                    )
                }
            }
        }
    }
    if (raised) {
        MediaHubCard(modifier = modifier, insideMargin = PaddingValues(4.dp)) { content() }
    } else {
        Box(modifier) { content() }
    }
}

@Composable
private fun MediaHubSearchInput(
    value: String,
    onValueChange: (String) -> Unit,
    onSearch: () -> Unit,
    expanded: Boolean,
    onExpandedChange: (Boolean) -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    placeholder: String,
) {
    InputField(
        query = value,
        onQueryChange = onValueChange,
        onSearch = { onSearch() },
        expanded = expanded,
        onExpandedChange = onExpandedChange,
        modifier = modifier
            .heightIn(min = 48.dp)
            .semantics { contentDescription = placeholder },
        label = placeholder,
        enabled = enabled,
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
    )
}

@Composable
fun MediaHubSearchField(
    value: String,
    onValueChange: (String) -> Unit,
    onSearch: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    placeholder: String = "搜索电影或电视剧",
) {
    MediaHubSearchInput(
        value = value,
        onValueChange = onValueChange,
        onSearch = onSearch,
        expanded = false,
        onExpandedChange = {},
        modifier = modifier,
        enabled = enabled,
        placeholder = placeholder,
    )
}

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
            MediaHubSearchInput(
                value = query,
                onValueChange = onQueryChange,
                onSearch = onSearch,
                expanded = expanded,
                onExpandedChange = onExpandedChange,
                enabled = enabled,
                placeholder = placeholder,
            )
        },
        onExpandedChange = onExpandedChange,
        modifier = modifier,
        expanded = expanded,
        outsideEndAction = {
            TextButton(text = "取消", onClick = { onExpandedChange(false) })
        },
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
    TextField(
        value = value,
        onValueChange = onValueChange,
        modifier = modifier.semantics { contentDescription = placeholder },
        label = placeholder,
        useLabelAsPlaceholder = true,
        enabled = enabled,
        singleLine = true,
        keyboardOptions = KeyboardOptions(keyboardType = keyboardType),
        visualTransformation = if (password) PasswordVisualTransformation() else VisualTransformation.None,
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
    TextField(
        value = value,
        onValueChange = onValueChange,
        modifier = modifier.semantics { contentDescription = placeholder },
        label = placeholder,
        useLabelAsPlaceholder = true,
        enabled = enabled,
        minLines = 5,
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
    TextField(
        value = value,
        onValueChange = onValueChange,
        modifier = modifier,
        label = "密码",
        useLabelAsPlaceholder = true,
        enabled = enabled,
        singleLine = true,
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
        modifier = modifier,
        enabled = enabled,
        minHeight = 48.dp,
        colors = ButtonDefaults.buttonColors(
            color = MediaHubColors.Error,
            contentColor = MediaHubColors.OnAccent,
        ),
    ) {
        if (icon != null) {
            MediaHubIcon(
                imageVector = icon,
                contentDescription = null,
                modifier = Modifier.size(16.dp),
                tint = MediaHubColors.OnAccent,
            )
            Spacer(Modifier.width(8.dp))
        }
        MediaHubText(
            text = label,
            color = MediaHubColors.OnAccent,
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
    onConfirm: () -> Unit,
    onDismiss: () -> Unit,
) {
    OverlayDialog(
        show = visible,
        title = title,
        summary = message,
        titleColor = if (isDestructive) MediaHubColors.Error else MiuixTheme.colorScheme.onSurface,
        onDismissRequest = onDismiss,
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.End,
        ) {
            TextButton(text = cancelLabel, onClick = onDismiss)
            Spacer(Modifier.width(8.dp))
            TextButton(
                text = confirmLabel,
                onClick = onConfirm,
                colors = if (isDestructive) {
                    ButtonDefaults.textButtonColors(textColor = MediaHubColors.Error)
                } else {
                    ButtonDefaults.textButtonColorsPrimary()
                },
            )
        }
    }
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
    MediaHubCard(
        modifier = modifier,
        insideMargin = PaddingValues(horizontal = 14.dp, vertical = 4.dp),
        onClick = if (enabled) onClick else null,
        color = if (selected) MediaHubColors.SurfaceSelected else Color.Unspecified,
    ) {
        Box(
            modifier = Modifier
                .heightIn(min = 48.dp)
                .selectable(selected = selected, enabled = enabled, role = role, onClick = onClick),
            contentAlignment = Alignment.Center,
        ) {
            MediaHubText(
                text = label,
                color = if (selected) MiuixTheme.colorScheme.primary else MediaHubColors.TextSecondary,
                fontSize = 13.sp,
                fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal,
            )
        }
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
    SwitchPreference(
        checked = checked,
        onCheckedChange = onCheckedChange,
        title = title,
        modifier = modifier.heightIn(min = 48.dp),
        summary = summary,
        enabled = enabled,
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
    CheckboxPreference(
        title = title,
        checked = checked,
        onCheckedChange = onCheckedChange,
        modifier = modifier.heightIn(min = 48.dp),
        summary = summary,
        enabled = enabled,
    )
}

@Composable
fun MediaHubEmptyState(
    title: String,
    message: String,
    icon: ImageVector,
    modifier: Modifier = Modifier,
) {
    MediaHubCard(
        modifier = modifier.fillMaxWidth(),
        insideMargin = PaddingValues(vertical = 40.dp, horizontal = 24.dp),
    ) {
        Column(
            modifier = Modifier.fillMaxWidth(),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Box(
                modifier = Modifier
                    .size(56.dp)
                    .background(MediaHubColors.NeutralContainer, CircleShape),
                contentAlignment = Alignment.Center,
            ) {
                MediaHubIcon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = MediaHubColors.TextSecondary,
                    modifier = Modifier.size(26.dp),
                )
            }
            MediaHubText(
                text = title,
                modifier = Modifier.padding(top = 16.dp),
                color = MediaHubColors.TextStrong,
                fontSize = 15.sp,
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
            .background(bgColor, RoundedCornerShape(6.dp))
            .padding(horizontal = 7.dp, vertical = 3.dp),
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
            fontSize = 11.sp,
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
    Box(
        modifier = modifier
            .fillMaxWidth()
            .height(4.dp)
            .clip(RoundedCornerShape(2.dp))
            .background(trackColor),
    ) {
        Box(
            modifier = Modifier
                .fillMaxHeight()
                .fillMaxWidth(progress.coerceIn(0f, 1f))
                .clip(RoundedCornerShape(2.dp))
                .background(color),
        )
    }
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
                        .height(2.dp)
                        .background(
                            when {
                                step.isCompleted || step.isCurrent -> MediaHubColors.Accent
                                step.isFailed -> MediaHubColors.Error
                                else -> MediaHubColors.Border
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
                        .size(20.dp)
                        .background(
                            when {
                                step.isFailed -> MediaHubColors.ErrorContainer
                                step.isCompleted -> MediaHubColors.SuccessContainer
                                step.isCurrent -> MediaHubColors.AccentLight
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
                    fontSize = 11.sp,
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
            MediaHubColors.SurfaceHigh.copy(alpha = 0.9f),
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
    shape: Shape = RoundedCornerShape(8.dp),
) {
    Box(
        modifier = modifier
            .clip(shape)
            .background(mediaHubShimmerBrush()),
    )
}

