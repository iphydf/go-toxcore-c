#!/usr/bin/env bash
# Builds Go Tox's APK. gogio builds the Go code and Gio's Java into an AAR; this
# adds the app's manifest, Java and resources, the way Android's own build
# would, without Gradle.
#
#   android/build.sh ARCH OUT.apk
#
# ARCH is a gogio architecture (arm64, amd64, ...). It needs Go, a JDK, zip,
# ANDROID_HOME with platform 35, build-tools 35.0.0 and an NDK, and
# PKG_CONFIG_PATH with a toxcore.pc for ARCH that links toxcore statically,
# and curl and fonttools for the emoji font the first time.
# It signs with KEYSTORE (password KEYSTORE_PASS), by default the Android debug
# key, as gogio does.
set -euo pipefail

arch=$1
out=$(realpath "$2")
here=$(cd "$(dirname "$0")" && pwd)
app=$(dirname "$here")
versionName=0.0.1
versionCode=2
sdk=35
platform=$ANDROID_HOME/platforms/android-$sdk
tools=$ANDROID_HOME/build-tools/35.0.0

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# The emoji font isn't in the repository.
[ -f "$app/fonts/emoji.ttf" ] || "$app/fonts/emoji.sh"

(cd "$app" && go tool gogio -target android -buildmode archive \
  -arch "$arch" -minsdk "$sdk" -appid chat.tox.gotox \
  -ldflags "-X main.version=$versionName" -o "$tmp/gio.aar" .)
mkdir "$tmp/aar"
(cd "$tmp/aar" && unzip -q ../gio.aar)

# Resources: the app's, Gio's theme, and the icon.
cp -r "$here/res" "$tmp/res"
cp -r "$tmp/aar/res/." "$tmp/res/"
mkdir -p "$tmp/res/drawable-nodpi"
cp "$app/appicon.png" "$tmp/res/drawable-nodpi/ic_launcher_layer.png"
"$tools/aapt2" compile --dir "$tmp/res" -o "$tmp/res.zip"
"$tools/aapt2" link -I "$platform/android.jar" \
  --manifest "$here/AndroidManifest.xml" \
  --min-sdk-version "$sdk" --target-sdk-version "$sdk" \
  --version-code "$versionCode" --version-name "$versionName" \
  --java "$tmp/gen" -o "$tmp/app.apk" "$tmp/res.zip"

# Java: the app's, Gio's, and the jars of the Go packages that come with
# some (gogio leaves those out of an AAR).
mapfile -t sources < <(find "$here/java" "$tmp/gen" -name '*.java')
javac -source 17 -target 17 -classpath "$platform/android.jar:$tmp/aar/classes.jar" \
  -d "$tmp/classes" "${sources[@]}"
mapfile -t jars < <(cd "$app" && GOOS=android GOARCH="$arch" CGO_ENABLED=1 go list -deps -f '{{.Dir}}' . |
  while read -r dir; do find "$dir" -maxdepth 1 -name '*.jar'; done)
mapfile -t classes < <(find "$tmp/classes" -name '*.class')
# This d8 knows API levels up to 34: 26 has all the Java the code uses.
"$tools/d8" --lib "$platform/android.jar" --min-api 26 --output "$tmp" \
  "${classes[@]}" "$tmp/aar/classes.jar" "${jars[@]}"

# The Go code, as a native library.
mv "$tmp/aar/jni" "$tmp/lib"
(cd "$tmp" && zip -q app.apk classes.dex && zip -qr app.apk lib)

"$tools/zipalign" -f 4 "$tmp/app.apk" "$tmp/aligned.apk"
"$tools/apksigner" sign \
  --ks "${KEYSTORE:-$HOME/.android/debug.keystore}" \
  --ks-pass "pass:${KEYSTORE_PASS:-android}" \
  --out "$out" "$tmp/aligned.apk"
