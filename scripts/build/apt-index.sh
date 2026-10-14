#!/usr/bin/env bash
# Build an apt repository from the deb files in dist/. The result is a
# directory CI publishes as the apt branch.
#
# With APT_SIGNING_KEY (an armored secret key) and APT_SIGNING_PASSPHRASE,
# it signs the repository: InRelease and Release.gpg, which apt checks
# against the key's public half, published beside them as toskar.asc
# (#599). APT_REQUIRE_SIGNING=1 makes a missing key an error, as a stable
# release does, so an unsigned repository is never published by mistake.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

if ! command -v dpkg-scanpackages >/dev/null 2>&1; then
  echo "dpkg-scanpackages is required (apt/dpkg-dev)." >&2
  exit 1
fi
if ! command -v apt-ftparchive >/dev/null 2>&1; then
  echo "apt-ftparchive is required (apt-utils)." >&2
  exit 1
fi

OUT="${1:-dist/apt}"
# APT_DEBS is where the packages are; the repository test points it at its own.
DEBS="${APT_DEBS:-dist}"
rm -rf "$OUT"
mkdir -p "$OUT/pool/main"
cp "$DEBS"/*.deb "$OUT/pool/main/"

for arch in amd64 arm64; do
  dest="dists/stable/main/binary-${arch}"
  mkdir -p "$OUT/$dest"
  (
    cd "$OUT"
    dpkg-scanpackages -a "$arch" pool/main /dev/null > "$dest/Packages"
    gzip -9c "$dest/Packages" > "$dest/Packages.gz"
  )
done

# Origin and Label keep the names from before the rename (#237): apt refuses
# to update from a repository whose Origin or Label changed until each person
# accepts it with --allow-releaseinfo-change.
cat > "$OUT/dists/stable/Release" <<EOF
Origin: Yggdrasil
Label: Yggdrasil
Suite: stable
Codename: stable
Architectures: amd64 arm64
Components: main
Description: Toskar core packages
EOF
apt-ftparchive release "$OUT/dists/stable" >> "$OUT/dists/stable/Release"

if [ -n "${APT_SIGNING_KEY:-}" ]; then
  GNUPGHOME="$(mktemp -d)"
  export GNUPGHOME
  chmod 700 "$GNUPGHOME"
  trap 'rm -rf "$GNUPGHOME"' EXIT
  printf '%s\n' "$APT_SIGNING_KEY" | gpg --batch --quiet --import
  fpr="$(gpg --batch --with-colons --list-secret-keys | awk -F: '/^fpr:/ {print $10; exit}')"
  if [ -z "$fpr" ]; then
    echo "APT_SIGNING_KEY has no secret key in it." >&2
    exit 1
  fi
  sign() {
    gpg --batch --yes --pinentry-mode loopback --passphrase-fd 0 --local-user "$fpr" --digest-algo SHA512 "$@" <<<"${APT_SIGNING_PASSPHRASE:-}"
  }
  sign --clearsign -o "$OUT/dists/stable/InRelease" "$OUT/dists/stable/Release"
  sign --armor --detach-sign -o "$OUT/dists/stable/Release.gpg" "$OUT/dists/stable/Release"
  gpg --batch --armor --export "$fpr" > "$OUT/toskar.asc"
  # Check both before publishing, as apt will.
  gpg --batch --quiet --verify "$OUT/dists/stable/InRelease"
  gpg --batch --quiet --verify "$OUT/dists/stable/Release.gpg" "$OUT/dists/stable/Release"
  echo "Signed with $fpr"
elif [ "${APT_REQUIRE_SIGNING:-}" = "1" ]; then
  echo "APT_SIGNING_KEY isn't set, and this repository must be signed (#599)." >&2
  exit 1
else
  echo "Not signed: APT_SIGNING_KEY isn't set." >&2
fi
echo "Apt repository: $OUT"
