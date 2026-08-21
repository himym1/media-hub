package com.mediahub.android.core.designsystem

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.graphics.Color
import top.yukonga.miuix.kmp.theme.ColorSchemeMode
import top.yukonga.miuix.kmp.theme.MiuixTheme
import top.yukonga.miuix.kmp.theme.ThemeController

object MediaHubColors {
    val Canvas = Color(0xFFF7F8FA)
    val Surface = Color(0xFFFFFFFF)
    val SurfaceHigh = Color(0xFFF0F3F7)
    val SurfaceInput = Color(0xFFEFF2F6)
    val SurfaceSelected = Color(0xFFEAF2FF)
    val CardBackground = Color(0xFFFFFFFF)

    // Brand and Accents
    val Accent = Color(0xFF2563EB)
    val AccentGlow = Color(0xFF3B82F6)
    val AccentLight = Color(0xFFEFF6FF)
    val OnAccent = Color(0xFFFFFFFF)

    // Text Hierarchy
    val TextStrong = Color(0xFF0F172A)
    val TextPrimary = Color(0xFF1E293B)
    val TextSecondary = Color(0xFF64748B)
    val TextMuted = Color(0xFF94A3B8)
    val TextFaint = Color(0xFFCBD5E1)

    // Semantic Status Tokens
    val Source = Color(0xFF4F46E5)
    val SourceContainer = Color(0x144F46E5)
    val Success = Color(0xFF10B981)
    val SuccessContainer = Color(0x1A10B981)
    val Error = Color(0xFFEF4444)
    val ErrorContainer = Color(0x1AEF4444)
    val Warning = Color(0xFFF59E0B)
    val WarningContainer = Color(0x1AF59E0B)
    val NeutralContainer = Color(0x0A0F172A)

    // Borders & Glass
    val Border = Color(0x140F172A)
    val BorderSubtle = Color(0x0A0F172A)
    val BorderStrong = Color(0x260F172A)
    val GlassBackground = Color(0xD9FFFFFF)
    val GlassBorder = Color(0x26FFFFFF)
    val GlassDark = Color(0xB30F172A)
    val PosterOverlay = Color(0x8A000000)
}

@Composable
fun MediaHubTheme(content: @Composable () -> Unit) {
    val controller = remember {
        ThemeController(colorSchemeMode = ColorSchemeMode.Light)
    }

    MiuixTheme(controller = controller) {
        content()
    }
}

