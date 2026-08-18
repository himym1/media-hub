package com.mediahub.android.core.designsystem

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.selectableGroup
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
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.ArrowRight
import com.composables.icons.lucide.Eye
import com.composables.icons.lucide.EyeOff
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.Search
import top.yukonga.miuix.kmp.basic.Button as MiuixButton
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
    MiuixButton(
        modifier = modifier.heightIn(min = 48.dp),
        enabled = enabled,
        onClick = onClick,
    ) {
        if (icon != null) {
            MediaHubIcon(
                imageVector = icon,
                contentDescription = null,
                modifier = Modifier.size(15.dp),
                tint = MediaHubColors.Canvas,
            )
            Box(Modifier.width(7.dp))
        }
        MiuixText(label, fontSize = 13.sp)
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
            .background(MediaHubColors.SurfaceInput, RoundedCornerShape(8.dp))
            .clickable(enabled = enabled, role = Role.Button, onClick = onClick)
            .padding(horizontal = 16.dp),
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
            Box(Modifier.width(8.dp))
        }
        MediaHubText(text = label, fontSize = 13.sp, fontWeight = FontWeight.Medium)
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
            .background(MediaHubColors.SurfaceInput, RoundedCornerShape(8.dp))
            .padding(3.dp),
    ) {
        options.forEach { (value, label) ->
            val active = value == selected
            Box(
                modifier = Modifier
                    .weight(1f)
                    .heightIn(min = 48.dp)
                    .background(if (active) MediaHubColors.SurfaceSelected else Color.Transparent, RoundedCornerShape(6.dp))
                    .selectable(selected = active, role = role, onClick = { onSelected(value) })
                    .padding(vertical = 11.dp),
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
            .padding(start = 15.dp, end = 4.dp, top = 3.dp, bottom = 3.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MediaHubIcon(
            imageVector = Lucide.Search,
            contentDescription = null,
            modifier = Modifier.size(21.dp),
        )
        BasicTextField(
            value = value,
            onValueChange = onValueChange,
            enabled = enabled,
            singleLine = true,
            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
            keyboardActions = KeyboardActions(onSearch = { onSearch() }),
            textStyle = TextStyle(color = MediaHubColors.TextPrimary, fontSize = 15.sp),
            cursorBrush = SolidColor(MediaHubColors.Accent),
            decorationBox = { innerTextField ->
                Box {
                    if (value.isEmpty()) {
                        MediaHubText(text = placeholder, color = MediaHubColors.TextMuted, fontSize = 13.sp)
                    }
                    innerTextField()
                }
            },
            modifier = Modifier
                .padding(start = 12.dp)
                .weight(1f)
                .semantics { contentDescription = placeholder },
        )
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
                    .background(MediaHubColors.SurfaceInput, RoundedCornerShape(7.dp))
                    .padding(horizontal = 12.dp, vertical = 12.dp),
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
					.background(MediaHubColors.SurfaceInput, RoundedCornerShape(7.dp))
					.padding(horizontal = 12.dp, vertical = 12.dp),
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
            .padding(start = 15.dp, end = 4.dp, top = 3.dp, bottom = 3.dp),
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
            textStyle = TextStyle(color = MediaHubColors.TextPrimary, fontSize = 15.sp),
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
