### Fixed

- Asking for "a 10-second video", "a 720p clip", or "a vertical video"
  makes it that long and that size, as far as the model allows. Before, a
  clip was always 2 seconds at the model's usual size.

### Added

- When a picture or clip has to be shorter or smaller than asked, a note
  under the answer says so, and why. For example: "Clips can be up to 5 s
  long for now, so this one is 5 s instead of the 10 s asked for."
