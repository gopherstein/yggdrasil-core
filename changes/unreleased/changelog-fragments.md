### Changed

- Pull requests describe their changes in `changes/unreleased/` instead of editing `CHANGELOG.md`, so they no longer conflict over the changelog. `scripts/changelog.py` checks the fragments in CI, previews the next release, and writes them into `CHANGELOG.md` when a release is cut.
