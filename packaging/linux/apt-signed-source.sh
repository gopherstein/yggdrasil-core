#!/bin/sh
# Moves an apt source for Toskar's repository that was added with
# [trusted=yes], which turns off apt's signature check, to the signed form
# (#599). The package runs it after installing; it changes nothing unless
# the package ships the repository's key, and leaves a source that's
# already signed alone. TOSKAR_APT_SOURCES and TOSKAR_APT_KEYRING point it
# elsewhere for tests.
set -e
sources="${TOSKAR_APT_SOURCES:-/etc/apt/sources.list.d}"
keyring="${TOSKAR_APT_KEYRING:-/usr/share/keyrings/toskar-archive-keyring.gpg}"
repo="https://raw.githubusercontent.com/yeixio/toskar-core/apt"

[ -f "$keyring" ] || exit 0
[ -d "$sources" ] || exit 0

moved=""
for file in "$sources/yggdrasil.list" "$sources/toskar.list"; do
  [ -f "$file" ] || continue
  # Only Toskar's repository, by its address before and after the rename,
  # and only when it skips the signature check.
  grep -Eq 'raw\.githubusercontent\.com/yeixio/(toskar|yggdrasil)-core/apt' "$file" || continue
  grep -q 'trusted=yes' "$file" || continue
  moved="$moved $file"
done
[ -n "$moved" ] || exit 0

tmp="$sources/.toskar.list.new"
printf '# Toskar (https://toskar.ai), checked against its signing key.\ndeb [signed-by=%s] %s stable main\n' "$keyring" "$repo" > "$tmp"
for file in $moved; do
  rm -f "$file"
done
mv "$tmp" "$sources/toskar.list"
echo "Toskar's apt source now checks the repository's signature: $sources/toskar.list" >&2
