#!/usr/bin/env bash
set -euo pipefail

./android/gradlew -p android :app:connectedDebugAndroidTest \
    -Pandroid.testInstrumentationRunnerArguments.additionalTestOutputDir=/sdcard/Android/media/com.mediahub.android.debug/additional_test_output

output_dir="android/app/build/outputs/connected_android_test_additional_output"
shopt -s globstar nullglob
for screenshot in \
    mediahub-segmented-control.png \
    mediahub-large-font.png \
    mediahub-player.png \
    mediahub-player-pip.png \
    mediahub-launcher-icon.png \
    mediahub-launcher-round.png \
    mediahub-launcher-monochrome.png \
    mediahub-launcher-themed.png; do
    matches=("$output_dir"/**/"$screenshot")
    if (( ${#matches[@]} == 0 )) || [[ ! -s "${matches[0]}" ]]; then
        echo "Missing Android UI evidence: $screenshot" >&2
        exit 1
    fi
done
