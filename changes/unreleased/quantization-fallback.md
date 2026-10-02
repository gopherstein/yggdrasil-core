### Changed

- When a model can't answer, Yggdrasil tries another installed quantization of the same model before switching to a different model: a smaller one when it ran out of memory. The answer's steps say which version answered.

### Fixed

- After a model ran out of memory, the fallback could pick a larger version of the same model; it no longer does.
