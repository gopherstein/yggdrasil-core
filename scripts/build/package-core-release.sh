#!/usr/bin/env bash
# Build the core release artifacts: Linux deb and rpm, macOS archives, and a Windows amd64 archive.
# Usage: VERSION=1.1.0-beta.3 ./scripts/build/package-core-release.sh
# Artifacts are not code-signed.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-0.1.0-dev}"
# Debian revision treats "-" as the package revision separator.
PKG_VERSION="${VERSION/-/~}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-X github.com/yeixio/yggdrasil-core/internal/version.Version=${VERSION} -X github.com/yeixio/yggdrasil-core/internal/version.Commit=${COMMIT} -X github.com/yeixio/yggdrasil-core/internal/version.BuildDate=${DATE}"

if ! command -v nfpm >/dev/null 2>&1; then
  echo "nfpm is required. Install it with: go install github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.41.3" >&2
  exit 1
fi

if [[ ! -f web/dist/index.html ]]; then
  (cd web && pnpm install --frozen-lockfile && pnpm build)
fi

chmod +x packaging/linux/postinstall.sh packaging/linux/preremove.sh
rm -rf dist
mkdir -p dist

build_binaries() {
  local goos="$1" goarch="$2" dest="$3"
  mkdir -p "$dest/web"
  echo "Building ${goos}/${goarch}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags "$LDFLAGS" \
    -o "$dest/yggdrasil-daemon" ./cmd/daemon
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags "$LDFLAGS" \
    -o "$dest/yggctl" ./cmd/devctl
  cp -R web/dist/. "$dest/web/"
  mkdir -p "$dest/completions"
  cp cmd/devctl/completions/* "$dest/completions/"
}

package_linux() {
  local goarch="$1" debarch="$2" rpmarch="$3"
  local stage="dist/stage-${debarch}"
  build_binaries linux "$goarch" "$stage"
  local cfg
  cfg="$(mktemp)"
  # Keep "~" out of the shell heredoc. On the release runner it expands to the home directory.
  {
    cat > "$cfg" <<EOF
name: yggdrasil
arch: ${debarch}
platform: linux
EOF
    printf 'version: %s\n' "$PKG_VERSION" >> "$cfg"
    cat >> "$cfg" <<EOF
maintainer: YEIXIO LLC <hello@yeix.io>
description: Local AI daemon and web UI
homepage: https://yggdrasil.yeix.io
license: AGPL-3.0-or-later
contents:
  - src: ${ROOT}/${stage}/yggdrasil-daemon
    dst: /usr/bin/yggdrasil-daemon
    file_info:
      mode: 0755
  - src: ${ROOT}/${stage}/yggctl
    dst: /usr/bin/yggctl
    file_info:
      mode: 0755
  - src: ${ROOT}/${stage}/web
    dst: /usr/share/yggdrasil/web
    type: tree
  - src: ${ROOT}/cmd/devctl/completions/yggctl.bash
    dst: /usr/share/bash-completion/completions/yggctl
  - src: ${ROOT}/cmd/devctl/completions/yggctl.fish
    dst: /usr/share/fish/vendor_completions.d/yggctl.fish
  # Debian and Fedora put packaged zsh completions in different fpath directories.
  - src: ${ROOT}/cmd/devctl/completions/_yggctl
    dst: /usr/share/zsh/vendor-completions/_yggctl
    packager: deb
  - src: ${ROOT}/cmd/devctl/completions/_yggctl
    dst: /usr/share/zsh/site-functions/_yggctl
    packager: rpm
  - src: ${ROOT}/packaging/linux/yggdrasil.service
    dst: /usr/lib/systemd/system/yggdrasil.service
scripts:
  postinstall: ${ROOT}/packaging/linux/postinstall.sh
  preremove: ${ROOT}/packaging/linux/preremove.sh
rpm:
  arch: ${rpmarch}
EOF
  }
  # File names keep the human version. "~" in a path expands to the runner home directory.
  nfpm package -p deb -f "$cfg" -t "dist/yggdrasil_${VERSION}_${debarch}.deb"
  nfpm package -p rpm -f "$cfg" -t "dist/yggdrasil-${VERSION}-1.${rpmarch}.rpm"
  rm -f "$cfg"
  rm -rf "$stage"
}

package_darwin() {
  local goarch="$1"
  local name="yggdrasil-${VERSION}-darwin-${goarch}-headless"
  local stage="dist/${name}"
  build_binaries darwin "$goarch" "$stage"
  tar -C dist -czf "dist/${name}.tar.gz" "$name"
  rm -rf "$stage"
}

package_windows() {
  local goarch="$1"
  local name="yggdrasil-${VERSION}-windows-${goarch}-headless"
  local stage="dist/${name}"
  mkdir -p "$stage/web"
  echo "Building windows/${goarch}"
  CGO_ENABLED=0 GOOS=windows GOARCH="$goarch" go build -trimpath -ldflags "$LDFLAGS" \
    -o "$stage/yggdrasil-daemon.exe" ./cmd/daemon
  CGO_ENABLED=0 GOOS=windows GOARCH="$goarch" go build -trimpath -ldflags "$LDFLAGS" \
    -o "$stage/yggctl.exe" ./cmd/devctl
  cp -R web/dist/. "$stage/web/"
  tar -C dist -czf "dist/${name}.tar.gz" "$name"
  rm -rf "$stage"
}

# ONLY builds one target, such as linux-amd64, for the installer test in CI.
want() { [[ -z "${ONLY:-}" || "${ONLY}" == "$1" ]]; }
if want linux-amd64; then package_linux amd64 amd64 x86_64; fi
if want linux-arm64; then package_linux arm64 arm64 aarch64; fi
if want darwin-arm64; then package_darwin arm64; fi
if want darwin-amd64; then package_darwin amd64; fi
if want windows-amd64; then package_windows amd64; fi

(
  cd dist
  : > SHA256SUMS.txt
  shopt -s nullglob
  for file in *.deb *.rpm *.tar.gz; do
    if command -v sha256sum >/dev/null 2>&1; then
      hash="$(sha256sum "$file" | awk '{print $1}')"
    else
      hash="$(shasum -a 256 "$file" | awk '{print $1}')"
    fi
    printf '%s  %s\n' "$hash" "$file" >> SHA256SUMS.txt
  done
)
echo "Core packages are in dist/"
ls -lh dist
