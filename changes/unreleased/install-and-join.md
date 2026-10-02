### Added

- One command installs Yggdrasil and joins a computer to your network. `yggctl join-token create` now also prints an install-and-join command: `install.sh` on Linux and macOS, and `install.ps1` on Windows, both attached to each release. They install the release for that computer, check it against the release's checksums, start Yggdrasil as a service (systemd, launchd, or a scheduled task), and join. A Yggdrasil that is already installed and running is left as it is.
