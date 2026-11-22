#!/usr/bin/env bash
# Makes emoji.ttf: Noto Color Emoji without the digits and signs it has for
# keycaps, and with a space as wide as the Go fonts' one. The text shaper keeps
# a font while it has the next character, and always for spaces, so a digit
# after an emoji would take the emoji font's digit, and a space its space of
# 1.25 em. Needs curl and fonttools.
set -euo pipefail
version=v2.051
sha256=72a635cb3d2f3524c51620cdde406b217204e8a6a06c6a096ff8ed4b5fd6e27b
cd "$(dirname "$0")"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
url=https://raw.githubusercontent.com/googlefonts/noto-emoji/$version/fonts
curl -sSfL -o "$tmp/NotoColorEmoji.ttf" "$url/NotoColorEmoji.ttf"
echo "$sha256  $tmp/NotoColorEmoji.ttf" | sha256sum -c --quiet
curl -sSfL -o LICENSE "$url/LICENSE"
pyftsubset "$tmp/NotoColorEmoji.ttf" --output-file=emoji.ttf \
  --unicodes='U+0020,U+00A9,U+00AE,U+0100-10FFFF' \
  --layout-features='*' --glyph-names --notdef-outline \
  --name-IDs='*' --name-languages='*'
# Go Regular's space is 569 of 2048 units, as this font's em is.
python3 - <<'END'
from fontTools.ttLib import TTFont
f = TTFont("emoji.ttf")
assert f["head"].unitsPerEm == 2048
f["hmtx"][f.getBestCmap()[0x20]] = (569, 0)
f.save("emoji.ttf")
END
