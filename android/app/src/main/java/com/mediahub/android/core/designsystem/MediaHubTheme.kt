package com.mediahub.android.core.designsystem

import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.ColorScheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

object MediaHubColors {
    val Canvas = Color(0xFF101318)
    val Surface = Color(0xFF1A1F27)
    val SurfaceHigh = Color(0xFF232A34)
    val SurfaceInput = Color(0xFF1E242D)
    val SurfaceSelected = Color(0x335C9BFF)
    val CardBackground = Color(0xFF1A1F27)

    val Accent = Color(0xFF8AB4FF)
    val AccentGlow = Color(0xFFB4CCFF)
    val AccentLight = Color(0x335C9BFF)
    val OnAccent = Color(0xFF06244F)

    val TextStrong = Color(0xFFF4F7FB)
    val TextPrimary = Color(0xFFE6EDF5)
    val TextSecondary = Color(0xFFB4BECB)
    val TextMuted = Color(0xFF8B99A8)
    val TextFaint = Color(0xFF667484)

    val Source = Color(0xFFB4C4FF)
    val SourceContainer = Color(0x338B9CFF)
    val Success = Color(0xFF5EE0B5)
    val SuccessContainer = Color(0x265EE0B5)
    val Error = Color(0xFFFF8A80)
    val ErrorContainer = Color(0x33FF8A80)
    val Warning = Color(0xFFFFC247)
    val WarningContainer = Color(0x26FFC247)
    val NeutralContainer = Color(0x1AFFFFFF)

    val Border = Color(0x22FFFFFF)
    val BorderSubtle = Color(0x14FFFFFF)
    val BorderStrong = Color(0x38FFFFFF)
    val GlassBackground = Color(0xCC1A1F27)
    val GlassBorder = Color(0x33FFFFFF)
    val GlassDark = Color(0xCC101318)
    val PosterOverlay = Color(0x99000000)
}

object MediaHubShapes {
    val Card = RoundedCornerShape(20.dp)
    val Control = RoundedCornerShape(16.dp)
    val Nav = RoundedCornerShape(28.dp)
    val Chip = RoundedCornerShape(50)
}

internal val MediaHubDarkColorScheme: ColorScheme = darkColorScheme(
    primary = MediaHubColors.Accent,
    onPrimary = MediaHubColors.OnAccent,
    primaryContainer = Color(0xFF274A86),
    onPrimaryContainer = MediaHubColors.AccentGlow,
    secondary = MediaHubColors.Source,
    onSecondary = Color(0xFF1A2554),
    secondaryContainer = Color(0xFF2C3A6E),
    onSecondaryContainer = Color(0xFFD7DEFF),
    tertiary = MediaHubColors.Success,
    onTertiary = Color(0xFF00382A),
    tertiaryContainer = Color(0xFF145744),
    onTertiaryContainer = Color(0xFFB6F5DD),
    error = MediaHubColors.Error,
    onError = Color(0xFF4A0B08),
    errorContainer = Color(0xFF8A2B27),
    onErrorContainer = Color(0xFFFFDAD6),
    background = MediaHubColors.Canvas,
    onBackground = MediaHubColors.TextPrimary,
    surface = MediaHubColors.Surface,
    onSurface = MediaHubColors.TextPrimary,
    surfaceVariant = MediaHubColors.SurfaceHigh,
    onSurfaceVariant = MediaHubColors.TextSecondary,
    outline = MediaHubColors.BorderStrong,
    outlineVariant = MediaHubColors.Border,
    surfaceContainerLowest = Color(0xFF0C0F13),
    surfaceContainerLow = MediaHubColors.Canvas,
    surfaceContainer = MediaHubColors.Surface,
    surfaceContainerHigh = MediaHubColors.SurfaceHigh,
    surfaceContainerHighest = Color(0xFF2C3440),
    inverseSurface = Color(0xFFE6EDF5),
    inverseOnSurface = Color(0xFF1A1F27),
    inversePrimary = Color(0xFF2F5DA8),
)

internal val MediaHubMaterialShapes = Shapes(
    extraSmall = RoundedCornerShape(10.dp),
    small = RoundedCornerShape(14.dp),
    medium = RoundedCornerShape(18.dp),
    large = RoundedCornerShape(24.dp),
    extraLarge = RoundedCornerShape(30.dp),
)

internal val MediaHubTypography = Typography().let { base ->
    base.copy(
        displaySmall = base.displaySmall.copy(fontWeight = FontWeight.SemiBold),
        headlineMedium = base.headlineMedium.copy(fontWeight = FontWeight.SemiBold),
        titleLarge = base.titleLarge.copy(fontWeight = FontWeight.SemiBold),
        titleMedium = base.titleMedium.copy(fontWeight = FontWeight.SemiBold),
        bodySmall = TextStyle(
            fontSize = 12.sp,
            lineHeight = 16.sp,
            fontWeight = FontWeight.Normal,
            color = MediaHubColors.TextMuted,
        ),
        labelLarge = base.labelLarge.copy(fontWeight = FontWeight.SemiBold),
        labelSmall = TextStyle(
            fontSize = 12.sp,
            lineHeight = 16.sp,
            fontWeight = FontWeight.Medium,
        ),
    )
}

@Composable
fun MediaHubTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = MediaHubDarkColorScheme,
        shapes = MediaHubMaterialShapes,
        typography = MediaHubTypography,
        content = content,
    )
}
