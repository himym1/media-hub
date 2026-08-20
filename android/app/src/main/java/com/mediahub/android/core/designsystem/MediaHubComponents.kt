package com.mediahub.android.core.designsystem

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.foundation.layout.Column
import androidx.compose.ui.draw.clip
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import com.composables.icons.lucide.ArrowRight
import com.composables.icons.lucide.Check
import com.composables.icons.lucide.CircleAlert
import com.composables.icons.lucide.Eye
import com.composables.icons.lucide.EyeOff
import com.composables.icons.lucide.Info
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.Search
import com.composables.icons.lucide.X
import top.yukonga.miuix.kmp.basic.Icon as MiuixIcon
import top.yukonga.miuix.kmp.basic.Text as MiuixText

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
    Row(
        modifier = modifier
            .heightIn(min = 48.dp)
            .alpha(if (enabled) 1f else 0.45f)
            .background(MediaHubColors.Accent, RoundedCornerShape(8.dp))
            .clickable(enabled = enabled, role = Role.Button, onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 12.dp),
        horizontalArrangement = Arrangement.Center,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (icon != null) {
            MediaHubIcon(
                imageVector = icon,
                contentDescription = null,
                modifier = Modifier.size(16.dp),
                tint = MediaHubColors.Canvas,
            )
            Spacer(Modifier.width(8.dp))
        }
        MediaHubText(
            text = label,
            color = MediaHubColors.Canvas,
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
    Row(
        modifier = modifier
            .heightIn(min = 48.dp)
            .alpha(if (enabled) 1f else 0.45f)
            .background(MediaHubColors.Surface, RoundedCornerShape(8.dp))
            .border(width = 1.dp, color = MediaHubColors.Border, shape = RoundedCornerShape(8.dp))
            .clickable(enabled = enabled, role = Role.Button, onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 12.dp),
        horizontalArrangement = Arrangement.Center,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (icon != null) {
            MediaHubIcon(
                imageVector = icon,
                contentDescription = null,
                modifier = Modifier.size(16.dp),
                tint = MediaHubColors.TextPrimary,
            )
            Spacer(Modifier.width(8.dp))
        }
        MediaHubText(
            text = label,
            color = MediaHubColors.TextPrimary,
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
) {
    val interactionSource = remember { MutableInteractionSource() }
    Box(
        modifier = modifier
            .size(48.dp)
            .alpha(if (enabled) 1f else 0.45f)
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                enabled = enabled,
                role = Role.Button,
                onClickLabel = contentDescription,
                onClick = onClick,
            ),
        contentAlignment = Alignment.Center,
    ) {
        MediaHubIcon(
            imageVector = imageVector,
            contentDescription = contentDescription,
            modifier = Modifier.size(20.dp),
            tint = if (enabled) MediaHubColors.TextPrimary else MediaHubColors.TextMuted,
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
) {
    Row(
        modifier = modifier
            .fillMaxWidth()
            .selectableGroup()
            .background(MediaHubColors.Surface, RoundedCornerShape(8.dp))
            .border(width = 1.dp, color = MediaHubColors.Border, shape = RoundedCornerShape(8.dp))
            .padding(4.dp),
        horizontalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        options.forEach { (value, label) ->
            val active = value == selected
            val pillBackground = if (active) MediaHubColors.SurfaceSelected else Color.Transparent
            val pillBorder = if (active) MediaHubColors.Accent else Color.Transparent
            Box(
                modifier = Modifier
                    .weight(1f)
                    .heightIn(min = 44.dp)
                    .background(pillBackground, RoundedCornerShape(6.dp))
                    .border(width = if (active) 1.dp else 0.dp, color = pillBorder, shape = RoundedCornerShape(6.dp))
                    .selectable(selected = active, role = role, onClick = { onSelected(value) })
                    .padding(vertical = 10.dp),
                contentAlignment = Alignment.Center,
            ) {
                MediaHubText(
                    text = label,
                    color = if (active) MediaHubColors.TextPrimary else MediaHubColors.TextMuted,
                    fontSize = 13.sp,
                    fontWeight = if (active) FontWeight.SemiBold else FontWeight.Normal,
                )
            }
        }
    }
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
    Row(
        modifier = modifier
            .background(MediaHubColors.SurfaceInput, RoundedCornerShape(8.dp))
            .border(width = 1.dp, color = MediaHubColors.Border, shape = RoundedCornerShape(8.dp))
            .padding(start = 14.dp, end = 4.dp, top = 2.dp, bottom = 2.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MediaHubIcon(
            imageVector = Lucide.Search,
            contentDescription = null,
            tint = MediaHubColors.TextMuted,
            modifier = Modifier.size(19.dp),
        )
        BasicTextField(
            value = value,
            onValueChange = onValueChange,
            enabled = enabled,
            singleLine = true,
            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
            keyboardActions = KeyboardActions(onSearch = { onSearch() }),
            textStyle = TextStyle(color = MediaHubColors.TextPrimary, fontSize = 14.sp),
            cursorBrush = SolidColor(MediaHubColors.Accent),
            decorationBox = { innerTextField ->
                Box(Modifier.padding(start = 10.dp)) {
                    if (value.isEmpty()) {
                        MediaHubText(text = placeholder, color = MediaHubColors.TextMuted, fontSize = 13.sp)
                    }
                    innerTextField()
                }
            },
            modifier = Modifier
                .weight(1f)
                .semantics { contentDescription = placeholder },
        )
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
    BasicTextField(
        value = value,
        onValueChange = onValueChange,
        enabled = enabled,
        singleLine = true,
        keyboardOptions = KeyboardOptions(keyboardType = keyboardType),
        visualTransformation = if (password) PasswordVisualTransformation() else VisualTransformation.None,
        textStyle = TextStyle(color = MediaHubColors.TextPrimary, fontSize = 14.sp),
        cursorBrush = SolidColor(MediaHubColors.Accent),
        decorationBox = { innerTextField ->
            Box(
                modifier = Modifier
                    .heightIn(min = 48.dp)
                    .background(MediaHubColors.SurfaceInput, RoundedCornerShape(8.dp))
                    .border(width = 1.dp, color = MediaHubColors.Border, shape = RoundedCornerShape(8.dp))
                    .padding(horizontal = 14.dp, vertical = 12.dp),
            ) {
                if (value.isEmpty()) {
                    MediaHubText(text = placeholder, color = MediaHubColors.TextMuted, fontSize = 13.sp)
                }
                innerTextField()
            }
        },
        modifier = modifier.semantics { contentDescription = placeholder },
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
    BasicTextField(
        value = value,
        onValueChange = onValueChange,
        enabled = enabled,
        minLines = 5,
        textStyle = TextStyle(color = MediaHubColors.TextPrimary, fontSize = 13.sp),
        cursorBrush = SolidColor(MediaHubColors.Accent),
        decorationBox = { innerTextField ->
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .background(MediaHubColors.SurfaceInput, RoundedCornerShape(8.dp))
                    .border(width = 1.dp, color = MediaHubColors.Border, shape = RoundedCornerShape(8.dp))
                    .padding(horizontal = 14.dp, vertical = 12.dp),
            ) {
                if (value.isEmpty()) {
                    MediaHubText(text = placeholder, color = MediaHubColors.TextMuted, fontSize = 12.sp)
                }
                innerTextField()
            }
        },
        modifier = modifier.semantics { contentDescription = placeholder },
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
    Row(
        modifier = modifier
            .background(MediaHubColors.SurfaceInput, RoundedCornerShape(8.dp))
            .border(width = 1.dp, color = MediaHubColors.Border, shape = RoundedCornerShape(8.dp))
            .padding(start = 14.dp, end = 4.dp, top = 2.dp, bottom = 2.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        BasicTextField(
            value = value,
            onValueChange = onValueChange,
            enabled = enabled,
            singleLine = true,
            visualTransformation = if (visible) VisualTransformation.None else PasswordVisualTransformation(),
            keyboardOptions = KeyboardOptions(
                keyboardType = KeyboardType.Password,
                imeAction = ImeAction.Done,
            ),
            keyboardActions = KeyboardActions(onDone = { onSubmit() }),
            textStyle = TextStyle(color = MediaHubColors.TextPrimary, fontSize = 14.sp),
            cursorBrush = SolidColor(MediaHubColors.Accent),
            modifier = Modifier.weight(1f),
        )
        MediaHubIconButton(
            imageVector = if (visible) Lucide.EyeOff else Lucide.Eye,
            contentDescription = if (visible) "隐藏密码" else "显示密码",
            onClick = onVisibilityChanged,
            enabled = enabled,
        )
    }
}

@Composable
fun MediaHubDestructiveButton(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
    icon: ImageVector? = null,
) {
    Row(
        modifier = modifier
            .heightIn(min = 48.dp)
            .alpha(if (enabled) 1f else 0.45f)
            .background(MediaHubColors.Error, RoundedCornerShape(8.dp))
            .clickable(enabled = enabled, role = Role.Button, onClick = onClick)
            .padding(horizontal = 16.dp, vertical = 12.dp),
        horizontalArrangement = Arrangement.Center,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (icon != null) {
            MediaHubIcon(
                imageVector = icon,
                contentDescription = null,
                modifier = Modifier.size(16.dp),
                tint = MediaHubColors.Canvas,
            )
            Spacer(Modifier.width(8.dp))
        }
        MediaHubText(
            text = label,
            color = MediaHubColors.Canvas,
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
            Row(
                modifier = Modifier
                    .background(MediaHubColors.Surface, RoundedCornerShape(24.dp))
                    .border(width = 1.dp, color = tint.copy(alpha = 0.6f), shape = RoundedCornerShape(24.dp))
                    .padding(horizontal = 18.dp, vertical = 10.dp),
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
    if (visible) {
        Dialog(
            onDismissRequest = onDismiss,
            properties = DialogProperties(usePlatformDefaultWidth = true, dismissOnBackPress = true, dismissOnClickOutside = true),
        ) {
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(16.dp))
                    .background(MediaHubColors.Surface)
                    .border(
                        width = 1.dp,
                        color = if (isDestructive) MediaHubColors.Error.copy(alpha = 0.6f) else MediaHubColors.Border,
                        shape = RoundedCornerShape(16.dp),
                    )
                    .padding(22.dp),
            ) {
                Column(
                    modifier = Modifier.fillMaxWidth(),
                    verticalArrangement = Arrangement.spacedBy(14.dp),
                ) {
                    MediaHubText(
                        text = title,
                        fontSize = 17.sp,
                        fontWeight = FontWeight.Bold,
                        color = if (isDestructive) MediaHubColors.Error else MediaHubColors.TextPrimary,
                    )
                    MediaHubText(
                        text = message,
                        color = MediaHubColors.TextSecondary,
                        fontSize = 13.sp,
                    )
                    Row(
                        modifier = Modifier.fillMaxWidth().padding(top = 8.dp),
                        horizontalArrangement = Arrangement.spacedBy(10.dp),
                    ) {
                        MediaHubSecondaryButton(
                            label = cancelLabel,
                            onClick = onDismiss,
                            modifier = Modifier.weight(1f),
                        )
                        if (isDestructive) {
                            MediaHubDestructiveButton(
                                label = confirmLabel,
                                onClick = onConfirm,
                                modifier = Modifier.weight(1f),
                            )
                        } else {
                            MediaHubButton(
                                label = confirmLabel,
                                onClick = onConfirm,
                                modifier = Modifier.weight(1f),
                            )
                        }
                    }
                }
            }
        }
    }
}
