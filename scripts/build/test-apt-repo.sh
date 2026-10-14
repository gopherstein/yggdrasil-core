#!/usr/bin/env bash
# Tests Toskar's apt repository the way people add it (#599): a fresh Ubuntu
# container adds it with its key, never with [trusted=yes], runs apt-get
# update with no warnings, and installs from it.
#
#   test-apt-repo.sh            builds a small package, signs a repository
#                               with a key made for the test, and installs it
#   APT_TEST_DEBS=dist test-apt-repo.sh
#                               the same with the toskar packages in dist/
#   test-apt-repo.sh <dir>      tests a repository already built and signed,
#                               such as the release's dist/apt, installing toskar
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
IMAGE="${APT_TEST_IMAGE:-ubuntu:24.04}"
work="$(mktemp -d)"
server=""
cleanup() {
  [ -n "$server" ] && kill "$server" 2>/dev/null || true
  rm -rf "$work"
}
trap cleanup EXIT

if [ $# -ge 1 ]; then
  repo="$(cd "$1" && pwd)"
  package="toskar"
else
  mkdir -p "$work/debs"
  if [ -n "${APT_TEST_DEBS:-}" ]; then
    package="toskar"
    cp "$APT_TEST_DEBS"/*.deb "$work/debs/"
  else
    # A package with nothing in it.
    package="toskar-apt-test"
    mkdir -p "$work/pkg/DEBIAN"
    printf 'Package: %s\nVersion: 1.0\nArchitecture: all\nMaintainer: Toskar <hello@yeix.io>\nDescription: apt repository test\n' "$package" > "$work/pkg/DEBIAN/control"
    dpkg-deb --build --root-owner-group "$work/pkg" "$work/debs/${package}_1.0_all.deb" >/dev/null
  fi
  # A key made for this test only.
  export GNUPGHOME="$work/gnupg"
  mkdir -m 700 "$GNUPGHOME"
  gpg --batch --quiet --pinentry-mode loopback --passphrase test --quick-generate-key "Toskar apt test <apt-test@toskar.invalid>" ed25519 sign 1d
  key="$(gpg --batch --pinentry-mode loopback --passphrase test --armor --export-secret-keys)"
  unset GNUPGHOME
  repo="$work/apt"
  APT_DEBS="$work/debs" APT_SIGNING_KEY="$key" APT_SIGNING_PASSPHRASE=test "$ROOT/scripts/build/apt-index.sh" "$repo"
fi

for f in dists/stable/InRelease dists/stable/Release.gpg toskar.asc; do
  if [ ! -s "$repo/$f" ]; then
    echo "The repository has no $f: it isn't signed." >&2
    exit 1
  fi
done

port="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')"
python3 -m http.server "$port" --bind 127.0.0.1 --directory "$repo" >/dev/null 2>&1 &
server=$!
sleep 1

# Only this repository, so the test doesn't depend on Ubuntu's mirrors.
docker run --rm --network host -v "$repo/toskar.asc:/toskar.asc:ro" "$IMAGE" bash -euo pipefail -c "
  rm -f /etc/apt/sources.list /etc/apt/sources.list.d/*
  mkdir -p /etc/apt/keyrings
  cp /toskar.asc /etc/apt/keyrings/toskar.asc
  echo 'deb [signed-by=/etc/apt/keyrings/toskar.asc] http://127.0.0.1:$port stable main' > /etc/apt/sources.list.d/toskar.list
  apt-get update 2>&1 | tee /tmp/update.log
  if grep -Eq '^(W|E|Err):' /tmp/update.log; then
    echo 'apt-get update warned or failed.' >&2
    exit 1
  fi
  DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends $package
  dpkg -s $package >/dev/null
"
echo "The apt repository is signed, and apt installs $package from it."
