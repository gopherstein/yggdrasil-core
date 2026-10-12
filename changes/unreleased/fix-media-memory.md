### Fixed

- Making a picture or clip no longer fails when chat models are loaded:
  the models in its way are unloaded first, waiting for any answer being
  written, and a model starting meanwhile waits for the picture or clip
  to finish. One picture or clip is made at a time.
- When a picture or clip still fails, Toskar says why in plain words, such
  as the computer running out of memory or the graphics card stopping, and
  what to try, instead of the generator's raw error.
