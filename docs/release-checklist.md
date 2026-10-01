# Release checklist

Use this before tagging `v*`. The tag push runs [`.github/workflows/release.yml`](../.github/workflows/release.yml).

A tag with a pre-release suffix (`-alpha.N`, `-beta.N`, `-rc.N`) is published as a GitHub pre-release. It does not update the Homebrew formula or the apt repository, so their users stay on the latest stable release, and its release body says so. Its documentation snapshot and the desktop release still run.

After a successful release, the workflow sends a `core-release` event to `yeixio/yggdrasil-desktop`, which builds the desktop, Mac App Store, and iOS apps from that core release. Sending it needs the repository secret `YGGDRASIL_DESKTOP_TOKEN`: a fine-grained personal access token (or GitHub App token) for `yeixio/yggdrasil-desktop` only, with the **Contents: Read and write** permission. Without the secret, the run shows a "Desktop not notified" warning, and the desktop repository picks the release up in its daily check, or when its Release workflow is started by hand.

- [ ] Version passed into the release build matches the tag (`scripts/build/package-core-release.sh` strips a leading `v` in CI).
- [ ] [CHANGELOG.md](../CHANGELOG.md) has an entry for the release, moved out of Unreleased.
- [ ] CI is green on the commit being tagged, including `gofmt`, `go vet`, golangci-lint, `go test`, web lint, the web build, and the cross-compile job.
- [ ] `govulncheck` from the security workflow is green. `pnpm audit` is informational today because that step does not fail the job. Read its output.
- [ ] Linux amd64, Linux arm64, macOS amd64, macOS arm64, and Windows amd64 archives or packages built.
- [ ] Docs that describe user-facing behavior match the build. Guide snapshot process is in [user-guide/README.md](user-guide/README.md).
- [ ] API compatibility reviewed against [api.md](api.md) and [api/openapi.yaml](../api/openapi.yaml).
- [ ] GitHub Release contains the `.deb`, `.rpm`, and `.tar.gz` artifacts.
- [ ] `SHA256SUMS.txt` is attached and matches the artifacts.
- [ ] Release notes written. The workflow uses [packaging/release-install.md](../packaging/release-install.md) as the body.
- [ ] Install tested from a clean environment for each platform you claim in the notes.
- [ ] No secrets, data directories, or private keys in the artifacts or the notes.
- [ ] Known issues listed, including hardware combinations that are still untested.
- [ ] Signing status stated accurately. This workflow does not sign binaries. Do not describe artifacts as signed.

Code signing is planned. It is not implemented in this repository's release workflow.
