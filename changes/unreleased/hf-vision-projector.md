### Added

- Vision models installed from Hugging Face can see pictures. Browse all
  models marks a repository that ships an image projector as a vision
  model. Installing it downloads the projector with it, and the model's
  download is checked against the SHA-256 the repository lists. Reinstalling
  a vision model installed from Hugging Face before now adds its projector.

### Fixed

- Browse all models offers each repository's real model file instead of a
  guessed name. It skips speculative-decoding draft files, split parts,
  and extras in subfolders.
