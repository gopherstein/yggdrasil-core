### Fixed

- `/v1/chat/completions` honors `temperature` and `max_tokens` (and
  `max_completion_tokens`) for the answer, on this computer, a paired one,
  or an external server. Temperature 0 gives the same most likely answer
  every time. Before, both were accepted and ignored.
