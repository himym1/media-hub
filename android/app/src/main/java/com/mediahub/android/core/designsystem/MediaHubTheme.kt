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
    val Canvas = Color(0xFF0E1114)
    val Surface = Color(0xFF1B2228)
    val SurfaceHigh = Color(0xFF252E35)
    val SurfaceInput = Color(0xFF252E35)
    val SurfaceSelected = Color(0xFF183B2B)
    val CardBackground = Color(0xFF1B2228)

    val Accent = Color(0xFF34D399)
    val AccentGlow = Color(0xFF6EE7B7)
    val AccentLight = Color(0x2E34D399)
    val OnAccent = Color(0xFF064E3B)

    val TextStrong = Color(0xFFF8FAFC)
    val TextPrimary = Color(0xFFF8FAFC)
    val TextSecondary = Color(0xFFCBD5E1)
    val TextMuted = Color(0xFF94A3B8)
    val TextFaint = Color(0xFF64748B)

    val Source = Color(0xFF60A5FA)
    val SourceContainer = Color(0x2E60A5FA)
    val Success = Color(0xFF34D399)
    val SuccessContainer = Color(0x2634D399)
    val Error = Color(0xFFF87171)
    val ErrorContainer = Color(0x2EF87171)
    val Warning = Color(0xFFFBBF24)
    val WarningContainer = Color(0x26FBBF24)
    val NeutralContainer = Color(0x1AFFFFFF)

    val Border = Color(0xFF2E3B46)
    val BorderSubtle = Color(0x14FFFFFF)
    val BorderStrong = Color(0xFF475569)
    val GlassBackground = Color(0xCC1B2228)
    val GlassBorder = Color(0x33FFFFFF)
    val GlassDark = Color(0xCC0E1114)
    val PosterOverlay = Color(0x99000000)
}

object MediaHubShapes {
    val Card = RoundedCornerShape(14.dp)
    val Control = RoundedCornerShape(10.dp)
    val Nav = RoundedCornerShape(16.dp)
    val Chip = RoundedCornerShape(50)
}

internal val MediaHubDarkColorScheme: ColorScheme = darkColorScheme(
    primary = MediaHubColors.Accent,
    onPrimary = MediaHubColors.OnAccent,
    primaryContainer = Color(0xFF145744),
    onPrimaryContainer = MediaHubColors.AccentGlow,
    secondary = MediaHubColors.Source,
    onSecondary = Color(0xFF0B2545),
    secondaryContainer = Color(0xFF1E3A5F),
    onSecondaryContainer = Color(0xFFDBEAFE),
    tertiary = MediaHubColors.Success,
    onTertiary = MediaHubColors.OnAccent,
    tertiaryContainer = Color(0xFF145744),
    onTertiaryContainer = Color(0xFFA7F3D0),
    error = MediaHubColors.Error,
    onError = Color(0xFF4A0B08),
    errorContainer = Color(0xFF7F1D1D),
    onErrorContainer = Color(0xFFFEE2E2),
    background = MediaHubColors.Canvas,
    onBackground = MediaHubColors.TextPrimary,
    surface = MediaHubColors.Surface,
    onSurface = MediaHubColors.TextPrimary,
    surfaceVariant = MediaHubColors.SurfaceHigh,
    onSurfaceVariant = MediaHubColors.TextSecondary,
    outline = MediaHubColors.BorderStrong,
    outlineVariant = MediaHubColors.Border,
    surfaceContainerLowest = Color(0xFF0A0D10),
    surfaceContainerLow = MediaHubColors.Canvas,
    surfaceContainer = MediaHubColors.Surface,
    surfaceContainerHigh = MediaHubColors.SurfaceHigh,
    surfaceContainerHighest = Color(0xFF2D3741),
    inverseSurface = Color(0xFFF8FAFC),
    inverseOnSurface = Color(0xFF1B2228),
    inversePrimary = Color(0xFF059669),
)

internal val MediaHubMaterialShapes = Shapes(
    extraSmall = RoundedCornerShape(8.dp),
    small = RoundedCornerShape(10.dp),
    medium = RoundedCornerShape(12.dp),
    large = RoundedCornerShape(14.dp),
    extraLarge = RoundedCornerShape(20.dp),
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
