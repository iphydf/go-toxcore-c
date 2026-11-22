#!/usr/bin/env bash
# Builds toxcore and its libraries for Android, static, into PREFIX, with the
# toxcore.pc that build.sh needs in PREFIX/lib/pkgconfig.
#
#   android/deps.sh ARCH PREFIX TOXCORE_SRC
#
# ARCH is arm64 (phones) or amd64 (the emulator, which also needs nasm).
# TOXCORE_SRC is a c-toxcore checkout with its submodules. It needs
# ANDROID_HOME with an NDK (or ANDROID_NDK_HOME), CMake, make, perl and curl.
set -euo pipefail

arch=$1
prefix=$(mkdir -p "$2" && realpath "$2")
toxcore=$(realpath "$3")
api=26

case $arch in
arm64) triple=aarch64-linux-android abi=arm64-v8a vpx=arm64-android-gcc ;;
amd64) triple=x86_64-linux-android abi=x86_64 vpx=x86_64-android-gcc ;;
*)
  echo "unknown ARCH $arch" >&2
  exit 1
  ;;
esac

ndk=${ANDROID_NDK_HOME:-$(find "$ANDROID_HOME/ndk" -mindepth 1 -maxdepth 1 | sort -V | tail -1)}
bin=$ndk/toolchains/llvm/prebuilt/linux-x86_64/bin
export CC=$bin/$triple$api-clang CXX=$bin/$triple$api-clang++
export AR=$bin/llvm-ar RANLIB=$bin/llvm-ranlib STRIP=$bin/llvm-strip
export PKG_CONFIG_LIBDIR=$prefix/lib/pkgconfig PKG_CONFIG_PATH=

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
jobs=$(nproc)

# fetch URL SHA256 unpacks a release into the current directory.
fetch() {
  curl -sSfL -o "$tmp/src.tar.gz" "$1"
  echo "$2  $tmp/src.tar.gz" | sha256sum -c --quiet
  tar xzf "$tmp/src.tar.gz" --strip-components=1
  rm "$tmp/src.tar.gz"
}

mkdir "$tmp/sodium" && cd "$tmp/sodium"
fetch https://github.com/jedisct1/libsodium/releases/download/1.0.22-RELEASE/libsodium-1.0.22.tar.gz \
  adbdd8f16149e81ac6078a03aca6fc03b592b89ef7b5ed83841c086191be3349
./configure --prefix="$prefix" --host="$triple" --disable-shared --with-pic >/dev/null
make -j"$jobs" install >/dev/null

mkdir "$tmp/opus" && cd "$tmp/opus"
fetch https://github.com/xiph/opus/releases/download/v1.5.2/opus-1.5.2.tar.gz \
  65c1d2f78b9f2fb20082c38cbe47c951ad5839345876e46941612ee87f9a7ce1
./configure --prefix="$prefix" --host="$triple" --disable-shared --with-pic \
  --disable-doc --disable-extra-programs >/dev/null
make -j"$jobs" install >/dev/null

mkdir "$tmp/vpx" && cd "$tmp/vpx"
fetch https://github.com/webmproject/libvpx/archive/refs/tags/v1.17.0.tar.gz \
  1020f184046187baa2985dbde38e0691f49c44088bca7a1842b0236c6081dc0a
# libvpx assembles Arm code with the compiler, not with as.
AS=$([ "$arch" = arm64 ] && echo "$CC -c" || echo nasm) \
  LD=$CC ./configure --prefix="$prefix" --target="$vpx" --enable-pic \
  --disable-examples --disable-tools --disable-docs --disable-unit-tests >/dev/null
make -j"$jobs" install >/dev/null

cmake -S "$toxcore" -B "$tmp/toxcore" -Wno-dev \
  -DCMAKE_TOOLCHAIN_FILE="$ndk/build/cmake/android.toolchain.cmake" \
  -DANDROID_ABI="$abi" -DANDROID_PLATFORM="$api" \
  -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX="$prefix" \
  -DCMAKE_FIND_ROOT_PATH="$prefix" \
  -DENABLE_SHARED=OFF -DEXPERIMENTAL_API=ON -DMUST_BUILD_TOXAV=ON \
  -DBOOTSTRAP_DAEMON=OFF -DDHT_BOOTSTRAP=OFF -DUNITTEST=OFF >/dev/null
cmake --build "$tmp/toxcore" -j"$jobs" >/dev/null
cmake --install "$tmp/toxcore" >/dev/null

# cgo asks pkg-config without --static: toxcore.pc names every library.
cat >"$prefix/lib/pkgconfig/toxcore.pc" <<EOF
Name: toxcore
Description: toxcore and its libraries, static, for Android $arch
Version: 0.2
Cflags: -I$prefix/include
Libs: $prefix/lib/libtoxcore.a $prefix/lib/libsodium.a $prefix/lib/libopus.a $prefix/lib/libvpx.a -lm
EOF
