#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
mark="$root/docs/design/media-hub-icon-mark.svg"
resource_root="$root/android/app/src/main/res"
colors="$resource_root/values/colors.xml"
mode=${1:-generate}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

[[ "$mode" == "generate" || "$mode" == "--check" ]] || { echo "usage: $0 [--check]" >&2; exit 2; }
command -v qlmanage >/dev/null || { echo "qlmanage is required" >&2; exit 1; }
command -v swift >/dev/null || { echo "swift is required" >&2; exit 1; }
command -v ruby >/dev/null || { echo "ruby is required" >&2; exit 1; }

read -r background primary secondary < <(ruby -e '
  source = File.read(ARGV.fetch(0))
  names = %w[background primary secondary]
  colors = names.map do |name|
    source[/--#{name}:(#[0-9A-Fa-f]{6})/, 1] or abort("missing --#{name} launcher color")
  end
  puts colors.join(" ")
' "$mark")

qlmanage -t -s 1024 -o "$tmp" "$mark" >/dev/null 2>&1
rendered="$tmp/$(basename "$mark").png"
[[ -s "$rendered" ]] || { echo "failed to render canonical launcher mark" >&2; exit 1; }

generated="$tmp/res"
swift "$root/scripts/render-launcher-icons.swift" \
    "$rendered" "$generated" "$background" "$primary" "$secondary"

ruby -e '
  path, background = ARGV
  source = File.read(path)
  replacement = %(<color name="ic_launcher_background">#{background}</color>)
  abort("missing launcher background color") unless source.sub!(/<color name="ic_launcher_background">#[0-9A-Fa-f]{6}<\/color>/, replacement)
  print source
' "$colors" "$background" > "$tmp/colors.xml"

specs=(mdpi:48:108 hdpi:72:162 xhdpi:96:216 xxhdpi:144:324 xxxhdpi:192:432)
expected_count=0
for spec in "${specs[@]}"; do
    IFS=: read -r density legacy adaptive <<< "$spec"
    for entry in "ic_launcher.png:$legacy" "ic_launcher_round.png:$legacy" "ic_launcher_foreground.png:$adaptive" "ic_launcher_monochrome.png:$adaptive"; do
        IFS=: read -r name expected_size <<< "$entry"
        file="$generated/mipmap-$density/$name"
        [[ -s "$file" ]] || { echo "missing generated icon: $file" >&2; exit 1; }
        width=$(sips -g pixelWidth "$file" 2>/dev/null | awk '/pixelWidth/{print $2}')
        height=$(sips -g pixelHeight "$file" 2>/dev/null | awk '/pixelHeight/{print $2}')
        alpha=$(sips -g hasAlpha "$file" 2>/dev/null | awk '/hasAlpha/{print $2}')
        [[ "$width" == "$expected_size" && "$height" == "$expected_size" && "$alpha" == "yes" ]] || {
            echo "invalid generated icon $file: ${width}x${height}, alpha=$alpha" >&2
            exit 1
        }
        expected_count=$((expected_count + 1))
    done
done
actual_count=$(rg --files "$generated" | wc -l | tr -d ' ')
[[ "$actual_count" == "$expected_count" ]] || { echo "expected $expected_count generated icons, found $actual_count" >&2; exit 1; }

if [[ "$mode" == "--check" ]]; then
    for generated_file in $(rg --files "$generated"); do
        relative=${generated_file#"$generated/"}
        cmp -s "$generated_file" "$resource_root/$relative" || { echo "stale launcher asset: $relative" >&2; exit 1; }
    done
    cmp -s "$tmp/colors.xml" "$colors" || { echo "stale launcher background color" >&2; exit 1; }
    echo "android-launcher-icons-current"
    exit 0
fi

for generated_file in $(rg --files "$generated"); do
    relative=${generated_file#"$generated/"}
    install -m 0644 "$generated_file" "$resource_root/$relative"
done
install -m 0644 "$tmp/colors.xml" "$colors"
echo "android-launcher-icons-generated"
