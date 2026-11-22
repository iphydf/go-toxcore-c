# Go Tox

A Tox chat client for desktops and phones, built with
[Gio](https://gioui.org) on the binding in this repository. A wide window shows
the chat list and a chat side by side; a narrow one, like a phone's, shows one
page at a time.

It has chats with friends, files and avatars, groups (NGC) and old-style
conferences, and calls with friends, with video from a camera.

It keeps its profiles in the app's data directory (on Linux,
`~/.config/go-tox/profiles`), or one profile in the directory `-profile` gives.
Every profile without a password is online at once, and the menu switches
between them; one with a password comes online when it's opened with it.
A profile is the Tox's `profile.tox`, encrypted if the user gives it a password,
and its chats in `chats.db`, which SQLCipher encrypts with a key from the
profile's secret key: the chats are as safe as the profile.

## Desktop

The binding needs c-toxcore installed with its experimental headers
(`-DEXPERIMENTAL_API=ON`), and Gio needs its
[Linux dependencies](https://gioui.org/doc/install/linux). Calls need
PulseAudio's library (or PipeWire's PulseAudio server), and video a webcam that
Video4Linux gives in YUYV. Notifications go to the desktop's notification
server, and a click on one opens its chat; received files open with
`xdg-open`.

    go run . -profile /tmp/alice

## Android

`android/build.sh` builds the APK. gogio builds the Go code into an AAR, and the
script adds the app's manifest, Java and resources: a service that keeps Tox
connected while the app is closed, notifications for messages, friend requests
and calls, and the sound and camera for calls. It needs Go, a JDK, zip, `ANDROID_HOME` with platform 35, build-tools
35.0.0 and an NDK, and `PKG_CONFIG_PATH` pointing at a `toxcore.pc` for the
target that links toxcore and its libraries (libsodium, libopus, libvpx)
statically. `android/deps.sh` builds those from a c-toxcore checkout:

    android/deps.sh arm64 /tmp/android-arm64 path/to/c-toxcore
    PKG_CONFIG_PATH=/tmp/android-arm64/lib/pkgconfig android/build.sh arm64 go-tox.apk

It signs with the Android debug key, or with `KEYSTORE` and `KEYSTORE_PASS`.
CI (`.github/workflows/android.yml`) builds the APK, signed with a debug key of
its own.

It takes `appicon.png` as the app's icon: `logo.svg` at 512×512, full-bleed for
Android's adaptive icons.

## Emoji

The app draws emoji with `fonts/emoji.ttf`, which `fonts/emoji.sh` fetches and
makes from Noto Color Emoji (with its licence in `fonts/LICENSE`): systems' own
emoji fonts may be in a format Gio can't draw, as Android's are. It needs curl
and fonttools. `android/build.sh` runs it when the font isn't there; for a
desktop build, run it first, or the app has no emoji of its own.

## Bootstrap nodes

The client starts from the nodes in `nodes.go`. Run `tools/update-nodes` before
a release to write the list again from nodes.tox.chat.
