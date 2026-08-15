#!/usr/bin/env bash
set -euo pipefail

./android/gradlew -p android :app:connectedDebugAndroidTest

report_dir="android/app/build/reports/androidTests/connected/screenshots"
mkdir -p "$report_dir"
adb pull /sdcard/Android/data/com.mediahub.android/files/mediahub-segmented-control.png "$report_dir/mediahub-segmented-control.png"
adb pull /sdcard/Android/data/com.mediahub.android/files/mediahub-large-font.png "$report_dir/mediahub-large-font.png"
