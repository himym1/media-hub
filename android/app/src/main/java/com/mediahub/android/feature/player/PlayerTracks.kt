@file:androidx.annotation.OptIn(androidx.media3.common.util.UnstableApi::class)

package com.mediahub.android.feature.player

import androidx.media3.common.C
import androidx.media3.common.Format
import androidx.media3.common.Player
import androidx.media3.common.TrackSelectionOverride
import androidx.media3.common.TrackSelectionParameters
import androidx.media3.common.Tracks
import java.util.Locale

internal data class PlayerTrackOption(
    val type: Int,
    val groupIndex: Int,
    val trackIndex: Int,
    val title: String,
    val detail: String,
    val selected: Boolean,
    val supported: Boolean,
)

internal fun playerTrackOptions(tracks: Tracks, type: Int): List<PlayerTrackOption> {
    val options = buildList {
        tracks.groups.forEachIndexed { groupIndex, group ->
            if (group.type != type || group.length == 0) return@forEachIndexed
            if (shouldExpandTrackGroup(group)) {
                for (trackIndex in 0 until group.length) {
                    add(playerTrackOption(group, groupIndex, trackIndex, type))
                }
            } else {
                val selectedIndex = (0 until group.length).firstOrNull { group.isTrackSelected(it) } ?: 0
                add(playerTrackOption(group, groupIndex, selectedIndex, type))
            }
        }
    }
    val supported = options.filter { it.supported }
    return supported.ifEmpty { options }
}

internal fun applyPlayerTrackSelection(
    parameters: TrackSelectionParameters,
    group: Tracks.Group,
    trackIndex: Int,
    type: Int,
): TrackSelectionParameters =
    parameters
        .buildUpon()
        .clearOverridesOfType(type)
        .setTrackTypeDisabled(type, false)
        .setOverrideForType(TrackSelectionOverride(group.mediaTrackGroup, trackIndex))
        .build()

internal fun disablePlayerTrackType(
    parameters: TrackSelectionParameters,
    type: Int,
): TrackSelectionParameters =
    parameters
        .buildUpon()
        .clearOverridesOfType(type)
        .setTrackTypeDisabled(type, true)
        .build()

internal fun Player.applyTrackOption(option: PlayerTrackOption) {
    if (!isCommandAvailable(Player.COMMAND_SET_TRACK_SELECTION_PARAMETERS)) return
    val group = currentTracks.groups.getOrNull(option.groupIndex) ?: return
    if (group.type != option.type || option.trackIndex !in 0 until group.length) return
    trackSelectionParameters = applyPlayerTrackSelection(
        trackSelectionParameters,
        group,
        option.trackIndex,
        option.type,
    )
}

internal fun Player.disableTrackType(type: Int) {
    if (!isCommandAvailable(Player.COMMAND_SET_TRACK_SELECTION_PARAMETERS)) return
    trackSelectionParameters = disablePlayerTrackType(trackSelectionParameters, type)
}

internal fun displayTrackLanguage(code: String?): String? {
    val raw = code?.trim().orEmpty()
    if (raw.isEmpty() || raw.equals("und", ignoreCase = true) || raw.equals("unk", ignoreCase = true)) {
        return null
    }
    val iso = Iso639_2[raw.lowercase(Locale.ROOT)] ?: raw
    val locale = Locale.forLanguageTag(iso.replace('_', '-'))
    val name = locale.getDisplayLanguage(Locale.CHINESE).trim()
    return name.takeIf { it.isNotEmpty() && !it.equals(raw, ignoreCase = true) } ?: raw
}

internal fun audioChannelLabel(channelCount: Int): String = when (channelCount) {
    1 -> "单声道"
    2 -> "立体声"
    6 -> "5.1"
    8 -> "7.1"
    C.INDEX_UNSET, 0 -> ""
    else -> "${channelCount} 声道"
}

internal fun trackDetail(format: Format): String {
    val codec = format.sampleMimeType?.substringAfterLast('/')?.uppercase(Locale.ROOT).orEmpty()
    return listOf(audioChannelLabel(format.channelCount), codec).filter { it.isNotBlank() }.joinToString(" · ")
}

private fun playerTrackOption(
    group: Tracks.Group,
    groupIndex: Int,
    trackIndex: Int,
    type: Int,
): PlayerTrackOption {
    val format = group.getTrackFormat(trackIndex)
    val fallback = if (type == C.TRACK_TYPE_AUDIO) "音轨 ${trackIndex + 1}" else "字幕 ${trackIndex + 1}"
    val title = format.label?.takeIf { it.isNotBlank() }
        ?: displayTrackLanguage(format.language)
        ?: fallback
    val detail = trackDetail(format).ifBlank {
        if (type == C.TRACK_TYPE_TEXT) {
            format.sampleMimeType?.substringAfterLast('/')?.uppercase(Locale.ROOT) ?: "内嵌"
        } else {
            "默认"
        }
    }
    return PlayerTrackOption(
        type = type,
        groupIndex = groupIndex,
        trackIndex = trackIndex,
        title = title,
        detail = detail,
        selected = group.isTrackSelected(trackIndex),
        supported = group.isTrackSupported(trackIndex),
    )
}

private fun shouldExpandTrackGroup(group: Tracks.Group): Boolean {
    if (group.length <= 1) return false
    val languages = (0 until group.length).map { group.getTrackFormat(it).language.orEmpty() }.toSet()
    val labels = (0 until group.length).map { group.getTrackFormat(it).label.orEmpty() }.toSet()
    return languages.size > 1 || labels.size > 1
}

private val Iso639_2 = mapOf(
    "chi" to "zh",
    "zho" to "zh",
    "jpn" to "ja",
    "eng" to "en",
    "kor" to "ko",
    "fre" to "fr",
    "fra" to "fr",
    "ger" to "de",
    "deu" to "de",
    "spa" to "es",
    "ita" to "it",
    "rus" to "ru",
    "tha" to "th",
    "vie" to "vi",
    "por" to "pt",
)
