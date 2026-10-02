### Added

- `scripts/i18n.py status` counts only text that is probably untranslated: text meant to read the same as English, such as `PDF` or German `Name`, is listed in `i18n/same-as-english.json`, and CI says when a listed key is translated or gone.
