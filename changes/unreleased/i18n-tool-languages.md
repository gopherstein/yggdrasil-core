### Added

- Speech tools say which languages they work in (`languages`, and `auto_detect` for Whisper) in `GET /tools/providers` and in Diagnostics, and a speech call goes to a paired computer whose provider works in its language.

### Changed

- Read aloud speaks the text's language: a German answer is read by a German voice, not the English one. There are voices for 25 languages; text in a language without one, such as Japanese, says so instead of being read in the wrong voice.
