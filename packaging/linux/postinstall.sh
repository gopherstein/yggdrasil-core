#!/bin/sh
set -e
if ! getent passwd yggdrasil >/dev/null 2>&1; then
  useradd --system --home-dir /var/lib/yggdrasil --create-home --shell /usr/sbin/nologin yggdrasil
fi
# The service opens the GPU's render device (/dev/dri/renderD*), which the
# render group (video on older systems) may use (#317). Only groups this
# system has.
for group in render video; do
  if getent group "$group" >/dev/null 2>&1; then
    usermod -a -G "$group" yggdrasil || true
  fi
done
if command -v systemctl >/dev/null 2>&1; then
  systemctl daemon-reload || true
  systemctl enable toskar.service || true
  systemctl restart toskar.service || true
fi
# An apt source added with [trusted=yes] moves to the signed form, once the
# package ships the repository's key (#599).
if [ -x /usr/lib/toskar/apt-signed-source.sh ]; then
  /usr/lib/toskar/apt-signed-source.sh || true
fi
# Another Toskar running outside the service, such as a copy started by
# hand, keeps its own data and connects to the relay as a second computer,
# and can hold the port the service needs (#602). Say so; never stop it.
if command -v pgrep >/dev/null 2>&1; then
  main=0
  if command -v systemctl >/dev/null 2>&1; then
    main=$(systemctl show -p MainPID --value toskar.service 2>/dev/null || echo 0)
  fi
  for pid in $(pgrep -x toskar 2>/dev/null || true); do
    [ "$pid" = "$main" ] && continue
    owner=$(ps -o user= -p "$pid" 2>/dev/null | tr -d ' ')
    echo "Note: another Toskar is already running on this computer (process $pid, user ${owner:-unknown}), outside the toskar service." >&2
    echo "      Two copies keep separate chats and devices, and both connect to the relay. Stop the other one if you didn't mean to run both." >&2
  done
fi
