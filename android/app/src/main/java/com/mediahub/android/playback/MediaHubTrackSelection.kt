package com.mediahub.android.playback

import android.content.Context
import androidx.media3.common.C
import androidx.media3.common.Player
import androidx.media3.common.TrackSelectionOverride
import androidx.media3.common.TrackSelectionParameters
import androidx.media3.common.Tracks
import androidx.media3.exoplayer.trackselection.DefaultTrackSelector

@androidx.annotation.OptIn(androidx.media3.common.util.UnstableApi::class)
internal fun mediaHubTrackSelector(context: Context): DefaultTrackSelector {
    val selector = DefaultTrackSelector(context)
    selector.parameters = selector.buildUponParameters()
        .setExceedRendererCapabilitiesIfNecessary(true)
        .setConstrainAudioChannelCountToDeviceCapabilities(false)
        .setAllowAudioMixedMimeTypeAdaptiveness(true)
        .setAllowAudioMixedChannelCountAdaptiveness(true)
        .setAllowAudioMixedSampleRateAdaptiveness(true)
        .setTrackTypeDisabled(C.TRACK_TYPE_AUDIO, false)
        .setAudioOffloadPreferences(
            TrackSelectionParameters.AudioOffloadPreferences.Builder()
                .setAudioOffloadMode(TrackSelectionParameters.AudioOffloadPreferences.AUDIO_OFFLOAD_MODE_DISABLED)
                .build(),
        )
        .build()
    return selector
}

internal fun defaultAudioOverride(tracks: Tracks): TrackSelectionOverride? {
    val group = tracks.groups.firstOrNull { it.type == C.TRACK_TYPE_AUDIO && it.length > 0 } ?: return null
    if ((0 until group.length).any { group.isTrackSelected(it) }) return null
    val trackIndex = (0 until group.length).firstOrNull { group.isTrackSupported(it) } ?: 0
    return TrackSelectionOverride(group.mediaTrackGroup, trackIndex)
}

internal fun Player.ensureDefaultAudioTrack() {
    if (!isCommandAvailable(Player.COMMAND_SET_TRACK_SELECTION_PARAMETERS)) return
    val override = defaultAudioOverride(currentTracks) ?: return
    trackSelectionParameters = trackSelectionParameters
        .buildUpon()
        .clearOverridesOfType(C.TRACK_TYPE_AUDIO)
        .setTrackTypeDisabled(C.TRACK_TYPE_AUDIO, false)
        .setOverrideForType(override)
        .build()
}
