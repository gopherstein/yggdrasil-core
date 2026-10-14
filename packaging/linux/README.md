# Linux packages

Release CI builds a `.deb` and `.rpm` for amd64 and arm64, and publishes a signed apt repository on the `apt` branch. Its key, `toskar-archive-keyring.asc` here, is also at `https://toskar.ai/apt/toskar.asc` and on the `apt` branch as `toskar.asc`: "Toskar apt repository <hello@yeix.io>", Ed25519, fingerprint `7124 28C8 E821 E64C D418 32F5 2D28 4623 90D0 C604`, until 2031-10-13.

```bash
sudo install -d -m 0755 /etc/apt/keyrings
curl -fsSL https://toskar.ai/apt/toskar.asc | sudo tee /etc/apt/keyrings/toskar.asc > /dev/null
echo "deb [signed-by=/etc/apt/keyrings/toskar.asc] https://raw.githubusercontent.com/yeixio/toskar-core/apt stable main" | sudo tee /etc/apt/sources.list.d/toskar.list
sudo apt-get update
sudo apt-get install toskar
```

Each package installs:

- `/usr/bin/toskar`
- `/usr/bin/toskarctl`
- bash, zsh, and fish completion for `toskarctl`
- `/usr/share/yggdrasil/web`
- a systemd service, `toskar.service`, also reachable as `yggdrasil.service`

The package was called `yggdrasil` before the rename. Each release also publishes a transitional `yggdrasil` deb that depends on `toskar`, so `apt-get upgrade` moves an existing install over; the rpm obsoletes `yggdrasil`, so `dnf upgrade` does the same. The system user `yggdrasil`, its home `/var/lib/yggdrasil`, and the data in it stay as they are.

Open `http://127.0.0.1:7331` after the service starts.

**Signing (#599).** A release signs the apt repository (`InRelease`, `Release.gpg`) when the `APT_SIGNING_KEY` and `APT_SIGNING_PASSPHRASE` secrets are set, and publishes the public key beside it as `toskar.asc`. A stable release fails without them, so the repository is never published unsigned by mistake. Once `packaging/linux/toskar-archive-keyring.asc` is in the repository, the deb also ships the key as `/usr/share/keyrings/toskar-archive-keyring.gpg`, and installing or upgrading it moves an apt source added with `[trusted=yes]` to the signed form (`/usr/lib/toskar/apt-signed-source.sh`). CI installs from a signed repository in a fresh Ubuntu container (`scripts/build/test-apt-repo.sh`), and the release runs the same test on what it's about to publish.

**When `apt-get update` fails.** If another repository on the computer is broken, such as a PPA with no build for a new Ubuntu release, `apt-get update` exits with an error and `&& apt-get install toskar` never runs. The error names that repository (`E: The repository '…' does not have a Release file`). Remove or fix its file in `/etc/apt/sources.list.d/`, then run the commands again.

Build the packages locally with `VERSION=1.1.0 ./scripts/build/package-core-release.sh`. That also writes the macOS archives used by the Homebrew formula.
